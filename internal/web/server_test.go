package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func startTest(t *testing.T, root, source string) (*App, *httptest.Server) {
	t.Helper()
	a, e := New(root, source)
	if e != nil {
		t.Fatal(e)
	}
	s := httptest.NewServer(a)
	u, _ := url.Parse(s.URL)
	a.Host = u.Host
	t.Cleanup(s.Close)
	return a, s
}
func call(t *testing.T, a *App, s *httptest.Server, method, path, body string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(method, s.URL+a.Prefix+path, strings.NewReader(body))
	req.Header.Set("X-Kindle-Read", "1")
	req.Header.Set("Content-Type", "application/json")
	r, e := s.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, b
}
func TestRealHTTPImportReadResumeAndSeparateApplications(t *testing.T) {
	root := t.TempDir()
	a, s := startTest(t, root, "qidian")
	code, b := call(t, a, s, "POST", "api/import", `{"title":"本地验证文本","text":"第一段\n第二段😀"}`)
	if code != 200 {
		t.Fatalf("%d %s", code, b)
	}
	var book struct {
		ID string `json:"id"`
	}
	json.Unmarshal(b, &book)
	code, b = call(t, a, s, "GET", "api/chapter?book="+book.ID+"&chapter=text", "")
	if code != 200 || !bytes.Contains(b, []byte("第二段😀")) {
		t.Fatalf("%d %s", code, b)
	}
	code, b = call(t, a, s, "POST", "api/progress", `{"book_id":"`+book.ID+`","chapter_id":"text","paragraph":1,"offset":2,"font_size":32}`)
	if code != 200 {
		t.Fatalf("%d %s", code, b)
	}
	again, server := startTest(t, root, "qidian")
	_, b = call(t, again, server, "GET", "api/state", "")
	if !bytes.Contains(b, []byte(`"paragraph":1`)) {
		t.Fatalf("%s", b)
	}
	other, ss := startTest(t, root, "changpei")
	_, b = call(t, other, ss, "GET", "api/state", "")
	if bytes.Contains(b, []byte(book.ID)) {
		t.Fatal("source data leaked")
	}
}
func TestRejectsWrongHostOriginMethodsAndUnknownSession(t *testing.T) {
	a, s := startTest(t, t.TempDir(), "qidian")
	for _, v := range []struct{ path, origin, host, method string }{{"/api/state", "", "", "GET"}, {a.Prefix + "api/import", "https://evil.test", "", "POST"}, {a.Prefix + "api/state", "", "evil.test", "GET"}, {a.Prefix + "api/import", "", "", "GET"}} {
		r, _ := http.NewRequest(v.method, s.URL+v.path, strings.NewReader(`{}`))
		r.Header.Set("X-Kindle-Read", "1")
		if v.origin != "" {
			r.Header.Set("Origin", v.origin)
		}
		if v.host != "" {
			r.Host = v.host
		}
		res, e := s.Client().Do(r)
		if e != nil {
			t.Fatal(e)
		}
		res.Body.Close()
		if res.StatusCode < 400 {
			t.Fatalf("accepted %+v", v)
		}
	}
	req, _ := http.NewRequest("POST", s.URL+a.Prefix+"api/import", strings.NewReader(`{"title":"x","text":"x"}`))
	r, _ := s.Client().Do(req)
	r.Body.Close()
	if r.StatusCode != 403 {
		t.Fatal("simple cross-site form request accepted")
	}
}
func TestUnsupportedMiuReadContentIsNotFakeReading(t *testing.T) {
	a, s := startTest(t, t.TempDir(), "miuread")
	code, b := call(t, a, s, "GET", "api/state", "")
	if code != 200 || !bytes.Contains(b, []byte(`"content_ready":false`)) {
		t.Fatalf("%d %s", code, b)
	}
	_, b = call(t, a, s, "POST", "api/book", `{"url":"https://weread.qq.com/web/reader/abc"}`)
	if !bytes.Contains(b, []byte("书架")) {
		t.Fatalf("%s", b)
	}
}
