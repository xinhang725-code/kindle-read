// SPDX-License-Identifier: AGPL-3.0-only
package changpei

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xinhang725-code/kindle-read/internal/library"
)

type SearchResult struct {
	Books []library.Book `json:"books"`
	Count int            `json:"count"`
	Page  int            `json:"page"`
}
type Ranking struct {
	Title string         `json:"title"`
	Books []library.Book `json:"books"`
}

func (c *Client) Discover(ctx context.Context) ([]Ranking, error) {
	data, e := c.get(ctx, "/home/initPcIndex", nil)
	if e != nil {
		return nil, e
	}
	ranks := []Ranking{}
	groups := data.list("rankingList")
	if groups == nil {
		return nil, malformed
	}
	for _, group := range groups {
		for _, sub := range group.list("subList") {
			rank := Ranking{Title: group.text("name") + " · " + sub.text("name"), Books: []library.Book{}}
			for _, v := range sub.list("list") {
				book := bookSummary(v)
				if !validID(book.ID) || book.Title == "" {
					return nil, malformed
				}
				rank.Books = append(rank.Books, book)
			}
			if len(rank.Books) > 0 {
				ranks = append(ranks, rank)
			}
		}
	}
	return ranks, nil
}

type Block = library.Block
type ChapterContent struct {
	ChapterID string  `json:"chapter_id"`
	Title     string  `json:"title"`
	PrevID    string  `json:"prev_id"`
	NextID    string  `json:"next_id"`
	Blocks    []Block `json:"blocks"`
}
type object map[string]json.RawMessage

func (o object) number(k string) (int64, bool) {
	var n int64
	b, ok := o[k]
	if !ok {
		return 0, false
	}
	if json.Unmarshal(b, &n) == nil {
		return n, true
	}
	var s string
	if json.Unmarshal(b, &s) == nil {
		v, e := strconv.ParseInt(s, 10, 64)
		return v, e == nil
	}
	var flag bool
	if json.Unmarshal(b, &flag) == nil {
		if flag {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}
func (o object) is(k string, n int64) bool { v, ok := o.number(k); return ok && v == n }
func (o object) text(k string) string      { var s string; json.Unmarshal(o[k], &s); return s }
func (o object) id(k string) string {
	n, ok := o.number(k)
	if !ok || n <= 0 {
		return ""
	}
	return strconv.FormatInt(n, 10)
}
func (o object) child(k string) object  { var c object; json.Unmarshal(o[k], &c); return c }
func (o object) list(k string) []object { var c []object; json.Unmarshal(o[k], &c); return c }

func (c *Client) get(ctx context.Context, path string, params url.Values) (object, error) {
	h := http.Header{}
	h.Set("Accept", "application/json")
	h.Set("Client", "pc")
	h.Set("Referer", base+"/")
	token := c.token()
	if token != "" {
		h.Set("Token", token)
	}
	b, headers, e := c.network.DoResponse(ctx, "GET", base+"/webapi"+path+"?"+params.Encode(), nil, h)
	if e != nil {
		if headers.Get("Retry-After") != "" {
			return nil, problem("RATE_LIMITED", "长佩请求过于频繁，请 60 秒后重试")
		}
		err := asError(e)
		if err.Code == "AUTH_EXPIRED" {
			if failure := c.invalidate(token); failure != nil {
				return nil, failure
			}
		}
		return nil, err
	}
	var envelope struct {
		Code int    `json:"code"`
		Data object `json:"data"`
	}
	if json.Unmarshal(b, &envelope) != nil {
		return nil, malformed
	}
	switch envelope.Code {
	case 200:
		if envelope.Data == nil {
			return nil, malformed
		}
		return envelope.Data, nil
	case 401:
		if failure := c.invalidate(token); failure != nil {
			return nil, failure
		}
		return nil, problem("AUTH_EXPIRED", "登录已过期，请重新扫码")
	case 4204:
		return nil, problem("NOT_FOUND", "作品不存在或已下架")
	case 429:
		return nil, problem("RATE_LIMITED", "长佩请求过于频繁，请 60 秒后重试")
	default:
		return nil, problem("UPSTREAM_ERROR", "长佩暂未返回可用内容，请在官方 App 检查账号状态后重试")
	}
}

var novelPath = regexp.MustCompile(`^/novel-([1-9][0-9]{0,11})\.html$`)

func BookID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if validID(raw) {
		return raw, nil
	}
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") || (u.Hostname() != "www.gongzicp.com" && u.Hostname() != "m.gongzicp.com") {
		return "", problem("INVALID_REQUEST", "请输入长佩官方作品链接或作品编号")
	}
	match := novelPath.FindStringSubmatch(u.Path)
	if len(match) != 2 {
		return "", problem("INVALID_REQUEST", "请输入长佩作品链接，暂不支持单章分享链接")
	}
	return match[1], nil
}
func bookSummary(o object) library.Book {
	return library.Book{ID: o.id("novel_id"), Title: o.text("novel_name"), Author: o.text("novel_author"), URL: base + "/novel-" + o.id("novel_id") + ".html", Chapters: []library.Chapter{}}
}
func (c *Client) Search(ctx context.Context, query string, page int) (SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" || utf8.RuneCountInString(query) > 80 || page < 1 || page > 1000 {
		return SearchResult{}, problem("INVALID_REQUEST", "请输入 1–80 字的书名或作者")
	}
	data, e := c.get(ctx, "/search/novels", url.Values{"k": {query}, "page": {strconv.Itoa(page)}})
	if e != nil {
		return SearchResult{}, e
	}
	list := data.list("list")
	count, ok := data.number("count")
	if list == nil || !ok || count < 0 {
		return SearchResult{}, malformed
	}
	out := SearchResult{Books: []library.Book{}, Count: int(count), Page: page}
	for _, v := range list {
		b := bookSummary(v)
		if !validID(b.ID) || b.Title == "" {
			return SearchResult{}, malformed
		}
		out.Books = append(out.Books, b)
	}
	return out, nil
}
func (c *Client) Catalog(ctx context.Context, id string) (library.Book, error) {
	if !validID(id) {
		return library.Book{}, problem("INVALID_REQUEST", "作品编号无效")
	}
	info, e := c.get(ctx, "/novel/novelInfo", url.Values{"id": {id}})
	if e != nil {
		return library.Book{}, e
	}
	book := bookSummary(info)
	book.Author = info.text("author_nickname")
	if book.ID != id || book.Title == "" {
		return library.Book{}, malformed
	}
	if !info.is("novel_status", 1) || !info.is("novel_lock", 0) {
		return library.Book{}, problem("CHAPTER_LOCKED", "作品当前不可阅读，请在官方 App 查看")
	}
	data, e := c.get(ctx, "/novel/chapterGetList", url.Values{"nid": {id}})
	if e != nil {
		return library.Book{}, e
	}
	list := data.list("list")
	if list == nil {
		return library.Book{}, malformed
	}
	seen := map[string]bool{}
	for _, v := range list {
		if v.text("type") != "item" {
			continue
		}
		cid := v.id("id")
		if !validID(cid) || v.text("name") == "" || seen[cid] {
			return library.Book{}, malformed
		}
		seen[cid] = true
		price, _ := v.number("price")
		locked := !v.is("chapter_status", 1) || !v.is("chapter_clock", 0) || v.is("lock", 1) || (v.is("pay", 1) && !v.is("is_sub", 1) && !v.is("is_free_limit", 1)) || (v.is("is_whole_book", 1) && !v.is("whole_book_is_bought", 1) && price > 0) || v.is("chapter_type", 1)
		book.Chapters = append(book.Chapters, library.Chapter{ID: cid, Title: v.text("name"), URL: base + "/read-" + cid + ".html", Locked: locked})
	}
	return book, nil
}
func decodeContent(encoded string) (string, error) {
	if len(encoded) == 0 || len(encoded) > 3<<20 {
		return "", malformed
	}
	b, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil || len(b) == 0 || len(b)%aes.BlockSize != 0 {
		return "", malformed
	}
	// Parameters published by the official web reader; decoding follows explicit
	// entitlement and verification checks, never treats ciphertext as permission.
	keyPrefix, _ := base64.StdEncoding.DecodeString("dTBMUnJidSRFbg==")
	k, _ := strconv.ParseInt("4d5a6c8", 14, 64)
	v, _ := strconv.ParseInt("165455", 14, 64)
	block, e := aes.NewCipher([]byte(string(keyPrefix) + strconv.FormatInt(k, 36) + "A"))
	if e != nil {
		return "", malformed
	}
	cipher.NewCBCDecrypter(block, []byte("$h$b3!iGzsYn"+strconv.FormatInt(v, 32))).CryptBlocks(b, b)
	pad := int(b[len(b)-1])
	if pad < 1 || pad > 16 || pad > len(b) {
		return "", malformed
	}
	for _, v := range b[len(b)-pad:] {
		if int(v) != pad {
			return "", malformed
		}
	}
	b = b[:len(b)-pad]
	if len(b) == 0 || len(b) > 2<<20 || !utf8.Valid(b) {
		return "", malformed
	}
	return string(b), nil
}
func (c *Client) Chapter(ctx context.Context, book library.Book, id string) (ChapterContent, error) {
	out := ChapterContent{ChapterID: id, Blocks: []Block{}}
	if !validID(book.ID) || !validID(id) {
		return out, problem("INVALID_REQUEST", "作品或章节编号无效")
	}
	index := -1
	for i, ch := range book.Chapters {
		if ch.ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return out, problem("NOT_FOUND", "章节不在这本作品的目录中")
	}
	data, e := c.get(ctx, "/novel/chapterGetInfo", url.Values{"cid": {id}, "server": {"0"}})
	if e != nil {
		return out, e
	}
	check, ok := data.number("check")
	if !ok || check < 0 {
		return out, malformed
	}
	if check > 1 {
		return out, problem("VERIFICATION_REQUIRED", "长佩要求额外验证，当前无法读取此章；请在官方 App 查看，稍后重试")
	}
	ch, novel := data.child("chapterInfo"), data.child("novelInfo")
	if ch == nil || novel == nil || novel.id("id") != book.ID {
		return out, malformed
	}
	status, hasStatus := ch.number("status")
	lock, hasLock := ch.number("lock")
	if !hasStatus || !hasLock {
		return out, malformed
	}
	if status != 1 || lock != 0 {
		return out, problem("CHAPTER_LOCKED", "章节已锁定或不可阅读")
	}
	paid, hasPaid := ch.number("chapter_ispay")
	price, hasPrice := ch.number("chapterPrice")
	kind, hasKind := ch.number("chapter_type")
	if !hasPaid || !hasPrice || !hasKind || paid < 0 || paid > 1 || price < 0 {
		return out, malformed
	}
	if kind != 0 {
		return out, problem("APP_REQUIRED", "福利或特殊章节请在长佩官方 App 阅读")
	}
	if (paid == 1 || price > 0) && !novel.is("page_view_status", 1) {
		return out, problem("APP_REQUIRED", "此章仅支持长佩官方 App 阅读")
	}
	if book.Chapters[index].Locked || ((paid == 1 || price > 0) && !ch.is("isSub", 1) && !ch.is("is_free_limit", 1)) {
		return out, problem("NOT_PURCHASED", "该章需要购买或授权，请在长佩官方 App 处理后刷新目录")
	}
	if !ch.is("renderType", 1) {
		return out, problem("UNSUPPORTED_CONTENT", "此章使用尚未适配的图文格式，请在官方 App 阅读")
	}
	body, e := decodeContent(ch.text("content"))
	if e != nil {
		return out, e
	}
	out.Title = ch.text("name")
	if out.Title == "" {
		return out, malformed
	}
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out.Blocks = append(out.Blocks, Block{Type: "p", Text: line})
		}
	}
	if len(out.Blocks) == 0 {
		return out, malformed
	}
	for _, line := range strings.Split(strings.ReplaceAll(ch.text("postscript"), "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out.Blocks = append(out.Blocks, Block{Type: "note", Text: line})
		}
	}
	if index > 0 {
		out.PrevID = book.Chapters[index-1].ID
	}
	if index+1 < len(book.Chapters) {
		out.NextID = book.Chapters[index+1].ID
	}
	return out, nil
}
