package web

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/xinhang725-code/kindle-read/internal/changpei"
	"github.com/xinhang725-code/kindle-read/internal/library"
)

type cpTransport func(*http.Request) (*http.Response, error)

func (f cpTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func cpFrame(b []byte) []byte {
	return append(binary.BigEndian.AppendUint32([]byte{0}, uint32(len(b))), b...)
}
func cpString(field byte, s string) []byte {
	return append(binary.AppendUvarint([]byte{field*8 + 2}, uint64(len(s))), []byte(s)...)
}
func cpResponse(b []byte) *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(b))}
}
func authorizeCP(t *testing.T, a *App) {
	t.Helper()
	a.network.Client.Transport = cpTransport(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/cp.UserExternal/get_qrcode_token":
			return cpResponse(cpFrame(cpString(1, "qr"))), nil
		case "/cp.UserExternal/qrcode_polling":
			b := []byte{8, 4}
			b = append(b, cpString(3, "qr")...)
			login := []byte{8, 1}
			login = append(login, cpString(4, string(cpString(1, "session")))...)
			b = append(b, cpString(2, string(login))...)
			return cpResponse(cpFrame(b)), nil
		case "/cp.UserExternal/info":
			return cpResponse(cpFrame(append([]byte{8, 123}, cpString(2, "读者")...))), nil
		default:
			t.Fatal("unexpected auth path", r.URL.Path)
			return nil, nil
		}
	})
	l, e := a.cp.Start(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.cp.Poll(context.Background(), l.Generation); e != nil {
		t.Fatal(e)
	}
}

func TestChangpeiRequiresLoginBeforeEveryReadingEndpoint(t *testing.T) {
	a, s := startTest(t, t.TempDir(), "changpei")
	store, _ := a.store()
	b, _ := store.Import("guest private", "private body")
	for _, v := range []struct{ method, path, body string }{
		{"POST", "api/search", `{"query":"test","page":1}`}, {"POST", "api/book", `{"url":"42"}`},
		{"GET", "api/chapter?book=" + b.ID + "&chapter=text", ""}, {"POST", "api/import", `{"title":"a","text":"a"}`},
		{"POST", "api/progress", `{}`}, {"POST", "api/shelf", `{}`},
	} {
		code, b := call(t, a, s, v.method, v.path, v.body)
		if code != 401 || !bytes.Contains(b, []byte(`"code":"AUTH_REQUIRED"`)) {
			t.Fatalf("%s: %d %s", v.path, code, b)
		}
	}
	code, body := call(t, a, s, "GET", "api/state", "")
	if code != 200 || !bytes.Contains(body, []byte(`"login_required":true`)) || bytes.Contains(body, []byte("guest private")) {
		t.Fatalf("state leaked guest library: %s", body)
	}
}
func TestChangpeiQRNeverExposesCredentialsAndLogoutClosesGate(t *testing.T) {
	a, s := startTest(t, t.TempDir(), "changpei")
	authorizeCP(t, a)
	code, b := call(t, a, s, "GET", "api/state", "")
	if code != 200 || bytes.Contains(b, []byte("session")) || !bytes.Contains(b, []byte(`"id":"123"`)) {
		t.Fatalf("state: %s", b)
	}
	code, b = call(t, a, s, "POST", "api/logout", `{}`)
	if code != 200 {
		t.Fatal(code, string(b))
	}
	code, b = call(t, a, s, "POST", "api/search", `{"query":"test","page":1}`)
	if code != 401 {
		t.Fatal(code, string(b))
	}
	code, b = call(t, a, s, "POST", "api/login/start", `{}`)
	if code != 200 {
		t.Fatal(code, string(b))
	}
	var login changpei.Login
	json.Unmarshal(b, &login)
	if login.URL != "qr.png" || strings.Contains(string(b), "qr-fixture") {
		t.Fatal("raw QR content exposed", string(b))
	}
	call(t, a, s, "POST", "api/login/cancel", `{}`)
	req, _ := http.NewRequest("GET", s.URL+a.Prefix+"qr.png", nil)
	res, e := s.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatal("cancelled QR still served")
	}
}
func TestChangpeiSearchUsesAccountAndReturnsSafeError(t *testing.T) {
	a, s := startTest(t, t.TempDir(), "changpei")
	authorizeCP(t, a)
	a.network.Client.Transport = cpTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Token") != "session" {
			t.Fatal("account missing")
		}
		return cpResponse([]byte(`{"code":200,"data":{"count":1,"list":[{"novel_id":42,"novel_name":"合成书","novel_author":"作者"}]}}`)), nil
	})
	code, b := call(t, a, s, "POST", "api/search", `{"query":"合成","page":1}`)
	if code != 200 || !bytes.Contains(b, []byte("合成书")) {
		t.Fatal(code, string(b))
	}
	// Account-bound caches must not be reachable through the guest store after logout.
	store, _ := a.store()
	book, _ := store.Import("account book", "private text")
	call(t, a, s, "POST", "api/logout", `{}`)
	guest, _ := library.Open(a.Root, "changpei", "guest")
	if _, e := guest.Book(book.ID); e == nil {
		t.Fatal("account cache leaked")
	}
}
