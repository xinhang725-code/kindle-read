package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
func TestStrictOfficialURLs(t *testing.T) {
	for _, s := range []string{"http://book.qidian.com/info/1/", "https://book.qidian.com.evil.test/info/1/", "https://book.qidian.com:8443/info/1/", "https://u:p@book.qidian.com/info/1/", "https://www.gongzicp.com/novel-1.html", "https://127.0.0.1/"} {
		if _, e := ValidateURL("qidian", s); e == nil {
			t.Fatalf("accepted %s", s)
		}
	}
	for _, v := range [][2]string{{"qidian", "https://book.qidian.com/info/123/"}, {"qidian", "https://read.qidian.com/chapter/abc/def/"}, {"changpei", "https://www.gongzicp.com/novel-123.html"}} {
		if _, e := ValidateURL(v[0], v[1]); e != nil {
			t.Fatal(e)
		}
	}
}

// These pages are invented contract fixtures, NOT captures or live platform evidence.
func TestParsePublicChapterAndRejectChallenge(t *testing.T) {
	raw := `<h1>第一章</h1><div class="read-content"><p>第一段 &amp; 符号</p><p>第二段<br>换行</p><script>steal()</script></div>`
	b, p, e := ParsePage("qidian", "https://read.qidian.com/chapter/abc/def/", []byte(raw))
	if e != nil || b.Title != "第一章" || len(p) != 3 || p[0] != "第一段 & 符号" || p[2] != "换行" {
		t.Fatalf("%+v %v %v", b, p, e)
	}
	for _, raw := range []string{`<div id="captcha">请验证</div>`, `<div class="read-content"><p>试读</p><div class="vip-lock">购买</div></div>`, `<script>chapter='fake'</script>`} {
		if _, _, e := ParsePage("qidian", "https://read.qidian.com/chapter/a/b/", []byte(raw)); e == nil {
			t.Fatal("challenge/paywall/unknown accepted")
		}
	}
}
func TestCatalogFiltersOtherPlatformsAndDeduplicates(t *testing.T) {
	raw := `<meta property="og:novel:book_name" content="合成目录"><meta property="og:novel:author" content="作者"><div class="chapter-list"><a href="/read-12.html">一</a><a href="/read-12.html">重复</a><a href="https://evil.test/read-1.html">外链</a><a href="/read-13.html">二</a></div>`
	b, _, e := ParsePage("changpei", "https://www.gongzicp.com/novel-5.html", []byte(raw))
	if e != nil || len(b.Chapters) != 2 || b.Chapters[1].URL != "https://www.gongzicp.com/read-13.html" {
		t.Fatalf("%+v %v", b, e)
	}
}
func TestNetworkRejectsRedirectAndSanitizesFailures(t *testing.T) {
	n := NewNetwork([]string{"weread.qq.com"})
	calls := 0
	n.Client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		v := response(302, "")
		v.Header.Set("Location", "http://127.0.0.1/private")
		return v, nil
	})
	if _, e := n.Do(context.Background(), "GET", "https://weread.qq.com/api?token=secret", nil, nil); e == nil || strings.Contains(e.Error(), "secret") {
		t.Fatalf("unsafe %v", e)
	}
	if calls != 1 {
		t.Fatal("redirect followed")
	}
	n.Client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) { return response(429, `private token here`), nil })
	if _, e := n.Do(context.Background(), "GET", "https://weread.qq.com/api", nil, nil); e == nil || strings.Contains(e.Error(), "private") || !strings.Contains(e.Error(), "429") {
		t.Fatalf("%v", e)
	}
}

func TestCatalogIncludesAllVolumesAndRejectsAmbiguousBody(t *testing.T) {
	raw := `<h1>合成作品</h1><div class="chapter-list"><a href="/read-1.html">一</a></div><div class="chapter-list"><a href="/read-2.html">二</a></div>`
	b, _, e := ParsePage("changpei", "https://www.gongzicp.com/novel-1.html", []byte(raw))
	if e != nil || len(b.Chapters) != 2 {
		t.Fatalf("incomplete catalog: %+v %v", b, e)
	}
	wrapped := `<div id="j-catalogWrap">` + raw + `</div>`
	b, _, e = ParsePage("changpei", "https://www.gongzicp.com/novel-1.html", []byte(wrapped))
	if e != nil || len(b.Chapters) != 2 || b.Chapters[0].Title != "一" {
		t.Fatalf("nested catalog lost order or duplicated: %+v %v", b, e)
	}
	raw = `<h1>章</h1><div class="read-content"><p>正文</p></div><div class="read-content"><p>另一份内容</p></div>`
	if _, _, e = ParsePage("changpei", "https://www.gongzicp.com/read-1.html", []byte(raw)); e == nil {
		t.Fatal("ambiguous content silently accepted")
	}
}

func TestPublicRedirectCannotCacheAnotherChapterUnderRequestedID(t *testing.T) {
	n := NewNetwork(Hosts("changpei"))
	calls := 0
	n.Client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path == "/read-1.html" {
			res := response(302, "")
			res.Header.Set("Location", "https://www.gongzicp.com/read-2.html")
			return res, nil
		}
		return response(200, `<h1>第二章</h1><div class="read-content"><p>另一章正文</p></div>`), nil
	})
	if _, _, e := Fetch(context.Background(), n, "changpei", "https://www.gongzicp.com/read-1.html"); e == nil {
		t.Fatal("redirected chapter accepted with the original identity")
	}
	if calls != 1 {
		t.Fatal("public chapter redirect was followed")
	}
}
