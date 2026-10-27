// SPDX-License-Identifier: AGPL-3.0-only
package provider

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/xinhang725-code/kindle-read/internal/library"
	"golang.org/x/net/html"
)

var qBook = regexp.MustCompile(`^/(info|book)/[0-9]+/?$`)
var qChapter = regexp.MustCompile(`^/chapter/[A-Za-z0-9_-]+/[A-Za-z0-9_-]+/?$`)
var cpPage = regexp.MustCompile(`^/(novel|read)-[0-9]+\.html$`)

func Hosts(source string) []string {
	if source == "qidian" {
		return []string{"www.qidian.com", "book.qidian.com", "read.qidian.com"}
	}
	if source == "changpei" {
		return []string{"www.gongzicp.com"}
	}
	return []string{"weread.qq.com", "i.weread.qq.com"}
}
func ValidateURL(source, raw string) (*url.URL, error) {
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil {
		return nil, errors.New("链接无效")
	}
	if e = NewNetwork(Hosts(source)).validate(u); e != nil {
		return nil, e
	}
	valid := source == "qidian" && ((u.Hostname() == "read.qidian.com" && qChapter.MatchString(u.Path)) || ((u.Hostname() == "www.qidian.com" || u.Hostname() == "book.qidian.com") && qBook.MatchString(u.Path)))
	valid = valid || source == "changpei" && cpPage.MatchString(u.Path)
	if !valid {
		return nil, errors.New("请输入本平台官方作品或章节链接")
	}
	u.RawQuery = ""
	u.Fragment = ""
	u.RawPath = ""
	return u, nil
}
func isChapter(source string, u *url.URL) bool {
	return source == "qidian" && qChapter.MatchString(u.Path) || source == "changpei" && strings.HasPrefix(u.Path, "/read-")
}
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func class(n *html.Node, name string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == name {
			return true
		}
	}
	return false
}
func walk(n *html.Node, f func(*html.Node)) {
	f(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, f)
	}
}
func text(n *html.Node) string {
	if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "svg" || n.Data == "noscript") {
		return ""
	}
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(text(c))
	}
	return b.String()
}
func paragraphs(n *html.Node) []string {
	var b strings.Builder
	var scan func(*html.Node)
	scan = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "noscript" || n.Data == "svg") {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			return
		}
		block := n.Data == "p" || n.Data == "div" || n.Data == "br"
		if block {
			b.WriteByte('\n')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			scan(c)
		}
		if block {
			b.WriteByte('\n')
		}
	}
	scan(n)
	var out []string
	for _, s := range strings.Split(b.String(), "\n") {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// Selectors are an experimental HTML contract, not proof of current platform compatibility.
func ParsePage(source, raw string, data []byte) (library.Book, []string, error) {
	u, e := ValidateURL(source, raw)
	if e != nil {
		return library.Book{}, nil, e
	}
	root, e := html.Parse(bytes.NewReader(data))
	if e != nil {
		return library.Book{}, nil, errors.New("页面解析失败")
	}
	b := library.Book{ID: library.Key(u.String()), URL: u.String(), Chapters: []library.Chapter{}}
	var content, catalog *html.Node
	var heading string
	blocked := false
	walk(root, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		if n.Data == "meta" {
			switch attr(n, "property") {
			case "og:novel:book_name":
				b.Title = attr(n, "content")
			case "og:novel:author":
				b.Author = attr(n, "content")
			}
		}
		if n.Data == "h1" && heading == "" {
			heading = strings.TrimSpace(text(n))
		}
		if class(n, "read-content") {
			content = n
		}
		if attr(n, "id") == "j-catalogWrap" || class(n, "chapter-list") || class(n, "chapter_list") {
			catalog = n
		}
		for _, c := range []string{"vip-lock", "paywall", "captcha", "geetest_panel"} {
			if class(n, c) || attr(n, "id") == c {
				blocked = true
			}
		}
	})
	if blocked {
		return b, nil, errors.New("页面要求授权、订阅或验证，请在官方客户端处理")
	}
	if b.Title == "" {
		b.Title = heading
	}
	if b.Title == "" {
		return b, nil, errors.New("未识别到书名，平台页面结构待适配")
	}
	if isChapter(source, u) && content != nil {
		p := paragraphs(content)
		if len(p) == 0 {
			return b, nil, errors.New("正文为空")
		}
		b.Chapters = []library.Chapter{{ID: b.ID, Title: heading, URL: u.String()}}
		return b, p, nil
	}
	seen := map[string]bool{}
	if catalog != nil {
		walk(catalog, func(n *html.Node) {
			if n.Data != "a" {
				return
			}
			ref, e := url.Parse(attr(n, "href"))
			if e != nil {
				return
			}
			link, e := ValidateURL(source, u.ResolveReference(ref).String())
			if e != nil || !isChapter(source, link) || seen[link.String()] {
				return
			}
			title := strings.TrimSpace(text(n))
			if title == "" {
				return
			}
			seen[link.String()] = true
			b.Chapters = append(b.Chapters, library.Chapter{ID: library.Key(link.String()), Title: title, URL: link.String(), Locked: class(n, "vip") || class(n, "locked")})
		})
	}
	if len(b.Chapters) == 0 {
		return b, nil, errors.New("未识别目录或公开正文；动态页面尚未适配")
	}
	return b, nil, nil
}
func Fetch(ctx context.Context, network *Network, source, raw string) (library.Book, []string, error) {
	u, e := ValidateURL(source, raw)
	if e != nil {
		return library.Book{}, nil, e
	}
	data, e := network.Do(ctx, "GET", u.String(), nil, http.Header{"Accept": []string{"text/html"}})
	if e != nil {
		return library.Book{}, nil, e
	}
	return ParsePage(source, u.String(), data)
}
