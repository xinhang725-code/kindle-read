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
