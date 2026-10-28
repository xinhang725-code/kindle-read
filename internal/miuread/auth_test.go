package miuread

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xinhang725-code/kindle-read/internal/provider"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fixture(t *testing.T, root string) *Client {
	t.Helper()
	n := provider.NewNetwork(provider.Hosts("miuread"))
	n.Client.Transport = transport(func(r *http.Request) (*http.Response, error) {
		raw := "{}"
		switch r.URL.Path {
		case "/api/auth/getLoginUid":
			raw = `{"uid":"fixture-uid"}`
		case "/api/auth/getLoginInfo":
			raw = `{"succeed":true,"webLoginVid":"42","accessToken":"fixture-access","refreshToken":"fixture-refresh"}`
		case "/api/userInfo":
			raw = `{"name":"测试账号"}`
		case "/web/shelf/sync":
			raw = `{"bookIds":["123","456","123"]}`
		case "/web/shelf/syncBook":
			var p struct {
				BookIDs []string `json:"bookIds"`
			}
			json.NewDecoder(r.Body).Decode(&p)
			if len(p.BookIDs) != 2 {
				t.Errorf("unexpected IDs %+v", p)
			}
			raw = `{"books":[{"bookId":"123","title":"书一","author":"作者"},{"bookId":"456","title":"书二"}]}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Set-Cookie": []string{"session_only=do-not-persist; Path=/"}}, Body: io.NopCloser(strings.NewReader(raw))}, nil
	})
	c, e := Open(root, n)
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestLoginPersistsIdentityAndOnlyStableCookies(t *testing.T) {
	root := t.TempDir()
	c := fixture(t, root)
	id := c.Fingerprint()
	q, e := c.Start(context.Background())
	if e != nil || !strings.Contains(q.URL, "/web/confirm?uid=fixture-uid") {
		t.Fatalf("%+v %v", q, e)
	}
	status, e := c.Poll(context.Background(), q.Generation, "")
	if e != nil || status.State != "ready" {
		t.Fatalf("%+v %v", status, e)
	}
	again := fixture(t, root)
	if again.Fingerprint() != id || again.Account().ID != "42" {
		t.Fatal("session not restored")
	}
	b, _ := os.ReadFile(filepath.Join(root, "miuread", "auth.json"))
	if strings.Contains(string(b), "session_only") {
		t.Fatal("browser-only cookie persisted")
	}
	books, e := again.Shelf(context.Background())
	if e != nil || len(books) != 2 || books[0].Title != "书一" {
		t.Fatalf("%+v %v", books, e)
	}
}
func TestCancelledOrExpiredLoginCannotCommit(t *testing.T) {
	c := fixture(t, t.TempDir())
	q, _ := c.Start(context.Background())
	c.Cancel()
	if _, e := c.Poll(context.Background(), q.Generation, ""); e == nil {
		t.Fatal("cancelled login accepted")
	}
	q, _ = c.Start(context.Background())
	c.started = time.Now().Add(-6 * time.Minute)
	if _, e := c.Poll(context.Background(), q.Generation, ""); e == nil {
		t.Fatal("expired login accepted")
	}
	if c.Account().ID != "" {
		t.Fatal("expired auth committed")
	}
}
func TestFailedCredentialWriteIsNotSuccessfulLogin(t *testing.T) {
	root := t.TempDir()
	c := fixture(t, root)
	q, _ := c.Start(context.Background())
	path := filepath.Join(root, "miuread", "auth.json")
	os.Remove(path)
	os.Mkdir(path, 0700)
	if _, e := c.Poll(context.Background(), q.Generation, ""); e == nil {
		t.Fatal("failed save accepted")
	}
	if c.Account().ID != "" {
		t.Fatal("in-memory credentials falsely committed")
	}
}
func TestLogoutClearsSessionAndPendingLogin(t *testing.T) {
	root := t.TempDir()
	c := fixture(t, root)
	q, _ := c.Start(context.Background())
	c.Poll(context.Background(), q.Generation, "")
	if e := c.Logout(); e != nil {
		t.Fatal(e)
	}
	if fixture(t, root).Account().ID != "" {
		t.Fatal("logout not persisted")
	}
	if _, e := c.Shelf(context.Background()); e == nil {
		t.Fatal("unauthenticated shelf accepted")
	}
}

func TestQRKeepsInstallationFingerprintAndUsesUpstreamVerificationHeaders(t *testing.T) {
	c := fixture(t, t.TempDir())
	fp := c.Fingerprint()
	underlying := c.network.Client.Transport
	c.network.Client.Transport = transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/auth/getLoginUid" || r.URL.Path == "/api/auth/getLoginInfo" {
			cookie, e := r.Cookie("wr_fp")
			if e != nil || cookie.Value != fp {
				t.Error("server replaced installation fingerprint")
			}
		}
		if r.URL.Path == "/api/userInfo" && (r.Header.Get("X-Vid") != "42" || r.Header.Get("X-Skey") != "fixture-access") {
			t.Error("missing upstream user verification headers")
		}
		res, e := underlying.RoundTrip(r)
		if e == nil {
			res.Header.Add("Set-Cookie", "wr_fp=1; Path=/; Secure")
		}
		return res, e
	})
	q, e := c.Start(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Poll(context.Background(), q.Generation, ""); e != nil {
		t.Fatal(e)
	}
	for _, cookie := range c.auth.Cookies {
		if cookie.Name == "wr_fp" && cookie.Value != fp {
			t.Fatal("persisted a response fingerprint")
		}
	}
}

func TestOTPAndCancellationDuringAccountVerification(t *testing.T) {
	c := fixture(t, t.TempDir())
	underlying := c.network.Client.Transport
	entered, release := make(chan struct{}), make(chan struct{})
	c.network.Client.Transport = transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/auth/getLoginInfo" && r.URL.Query().Get("otp") == "" {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"succeed":false,"logicCode":"NEED_OTP"}`))}, nil
		}
		if r.URL.Path == "/api/userInfo" {
			close(entered)
			<-release
		}
		return underlying.RoundTrip(r)
	})
	q, e := c.Start(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	result, e := c.Poll(context.Background(), q.Generation, "")
	if e != nil || result.State != "otp" {
		t.Fatalf("%+v %v", result, e)
	}
	if _, e = c.Poll(context.Background(), q.Generation, "abcd"); e == nil {
		t.Fatal("invalid OTP accepted")
	}
	done := make(chan error, 1)
	go func() { _, e := c.Poll(context.Background(), q.Generation, "1234"); done <- e }()
	<-entered
	c.Cancel()
	close(release)
	if e = <-done; e == nil {
		t.Fatal("cancelled in-flight verification committed")
	}
	if c.Account().ID != "" {
		t.Fatal("cancelled credentials exposed")
	}
}
