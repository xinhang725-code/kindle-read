// SPDX-License-Identifier: AGPL-3.0-only
package changpei

import (
	"bytes"
	"context"
	"encoding/binary"
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

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(b []byte, h http.Header) *http.Response {
	return &http.Response{StatusCode: 200, Header: h, Body: io.NopCloser(bytes.NewReader(b))}
}
func TestLongPollSilenceIsWaitingAndCanBeCancelled(t *testing.T) {
	c, _ := client(t, func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "get_qrcode_token") {
			return response(frame(str(1, "pending")), nil), nil
		}
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	l, e := c.Start(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	start := time.Now()
	p, e := c.Poll(context.Background(), l.Generation)
	if e != nil || p.State != "waiting" {
		t.Fatal("a normal long-poll wait was reported as failure", p, e)
	}
	if time.Since(start) > 15*time.Second {
		t.Fatal("poll exceeds UI request deadline")
	}
	done := make(chan error, 1)
	go func() { _, e := c.Poll(context.Background(), l.Generation); done <- e }()
	time.Sleep(10 * time.Millisecond)
	c.Cancel()
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("cancelled poll accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not interrupt pending request")
	}
}
func TestRPCHTTPTrailerErrorIsNotSuccess(t *testing.T) {
	c, _ := client(t, func(*http.Request) (*http.Response, error) {
		r := response(frame(str(1, "must-not-accept")), nil)
		r.Trailer = http.Header{"Grpc-Status": {"16"}}
		return r, nil
	})
	if _, e := c.Start(context.Background()); e == nil || asError(e).Code != "AUTH_EXPIRED" {
		t.Fatal("ignored HTTP trailer", e)
	}
}
func TestRestoredExpiredSessionLocksAccountWithoutLeakingCachedIdentity(t *testing.T) {
	c, root := client(t, func(*http.Request) (*http.Response, error) {
		r := response(nil, nil)
		r.StatusCode = 401
		return r, nil
	})
	c.auth.Account = Account{ID: "123", Name: "fixture"}
	c.auth.Token = "expired"
	if e := c.save(c.auth); e != nil {
		t.Fatal(e)
	}
	if e := c.EnsureSession(context.Background()); e == nil || asError(e).Code != "AUTH_EXPIRED" {
		t.Fatal("HTTP expiry not recognized", e)
	}
	if c.Account().ID != "" {
		t.Fatal("expired account retained")
	}
	restored, e := Open(root, provider.NewNetwork([]string{"www.gongzicp.com"}))
	if e != nil || restored.Account().ID != "" {
		t.Fatal("expiry not persisted", e)
	}
}
func TestReadingTokenExpiryInvalidatesTheSession(t *testing.T) {
	c, _ := client(t, func(*http.Request) (*http.Response, error) {
		return response([]byte(`{"code":401,"data":{}}`), nil), nil
	})
	c.auth.Account = Account{ID: "123"}
	c.auth.Token = "expired"
	c.verified = time.Now()
	if _, e := c.Search(context.Background(), "测试", 1); e == nil || asError(e).Code != "AUTH_EXPIRED" {
		t.Fatal(e)
	}
	if c.Account().ID != "" {
		t.Fatal("expired session can still access cached chapters")
	}
}

// Test helpers encode independently of the production parser.
func str(n byte, s string) []byte {
	b := []byte{n*8 + 2}
	b = binary.AppendUvarint(b, uint64(len(s)))
	return append(b, []byte(s)...)
}
func num(n byte, v uint64) []byte { return binary.AppendUvarint([]byte{n * 8}, v) }
func join(bs ...[]byte) []byte    { return bytes.Join(bs, nil) }
func frame(b []byte) []byte {
	return append(binary.BigEndian.AppendUint32([]byte{0}, uint32(len(b))), b...)
}
func client(t *testing.T, f roundTrip) (*Client, string) {
	t.Helper()
	root := t.TempDir()
	n := provider.NewNetwork([]string{"www.gongzicp.com"})
	n.Client.Transport = f
	c, e := Open(root, n)
	if e != nil {
		t.Fatal(e)
	}
	return c, root
}
func TestQRLoginConfirmPersistAndLogout(t *testing.T) {
	status := uint64(1)
	c, root := client(t, func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("version") != "1" || r.Header.Get("imei") == "" {
			t.Fatal("missing official client metadata")
		}
		b, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/cp.UserExternal/get_qrcode_token":
			if !bytes.Equal(b, frame(str(1, "Login"))) {
				t.Fatalf("QR scene: %x", b)
			}
			return response(frame(str(1, "qr-fixture")), http.Header{"Content-Type": {"application/grpc"}}), nil
		case "/cp.UserExternal/qrcode_polling":
			payload := join(num(1, status), str(3, "qr-fixture"))
			if status == 4 {
				payload = append(payload, str(2, string(join(num(1, 1), str(4, string(join(str(1, "secret-session"), str(2, "secret-refresh")))))))...)
			}
			return response(frame(payload), nil), nil
		case "/cp.UserExternal/info":
			if r.Header.Get("token") != "secret-session" {
				t.Fatal("profile must verify candidate session")
			}
			return response(frame(join(num(1, 123), str(2, "读者"))), nil), nil
		default:
			t.Fatalf("unexpected request %s", r.URL)
			return nil, nil
		}
	})
	ctx := context.Background()
	l, e := c.Start(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(l.URL, "https://www.gongzicp.com/transfer/login/middleware?") || !strings.Contains(l.URL, "pageType=app_qr_code_login") {
		t.Fatal("wrong QR destination")
	}
	for _, v := range []struct {
		code  uint64
		state string
	}{{1, "waiting"}, {2, "scanned"}, {4, "ready"}} {
		status = v.code
		p, e := c.Poll(ctx, l.Generation)
		if e != nil || p.State != v.state {
			t.Fatalf("%d: %+v %v", status, p, e)
		}
		if status < 4 && c.Account().ID != "" {
			t.Fatal("unconfirmed login committed")
		}
	}
	if c.Account().ID != "123" {
		t.Fatal("account not committed")
	}
	restored, e := Open(root, provider.NewNetwork([]string{"www.gongzicp.com"}))
	if e != nil || restored.Account().Name != "读者" {
		t.Fatal("session not restored", e)
	}
	info, _ := os.Stat(filepath.Join(root, "changpei", "auth.json"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("session not private")
	}
	if e = c.Logout(); e != nil || c.Account().ID != "" {
		t.Fatal("logout", e)
	}
	b, _ := os.ReadFile(filepath.Join(root, "changpei", "auth.json"))
	if bytes.Contains(b, []byte("secret-")) {
		t.Fatal("logout retained credentials")
	}
}
func TestQRRejectsCancelledApprovalAndWrongToken(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "wrong token", true: "late cancelled approval"}[cancel], func(t *testing.T) {
			var c *Client
			c, _ = client(t, func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.URL.Path, "get_qrcode_token") {
					return response(frame(str(1, "pending")), nil), nil
				}
				if cancel {
					c.Cancel()
				}
				token := "other"
				if cancel {
					token = "pending"
				}
				return response(frame(join(num(1, 4), str(3, token), str(2, string(join(num(1, 1), str(4, string(str(1, "bad")))))))), nil), nil
			})
			l, e := c.Start(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			if _, e = c.Poll(context.Background(), l.Generation); e == nil {
				t.Fatal("invalid approval accepted")
			}
			if c.Account().ID != "" {
				t.Fatal("account replaced")
			}
		})
	}
}
func TestRPCRejectsErrorsAndMalformedFrames(t *testing.T) {
	for _, v := range []struct {
		name string
		b    []byte
		h    http.Header
	}{
		{"header error", frame(str(1, "must-not-accept")), http.Header{"Grpc-Status": {"10"}}},
		{"truncated", []byte{0, 0, 0, 0, 50, 10}, nil}, {"bad protobuf", frame([]byte{10, 99}), nil},
		{"compressed", []byte{1, 0, 0, 0, 0}, nil}, {"duplicate frame", join(frame(str(1, "a")), frame(str(1, "b"))), nil},
		{"trailer error", join(frame(str(1, "a")), append(binary.BigEndian.AppendUint32([]byte{128}, 15), []byte("grpc-status: 7\r\n")...)), nil},
	} {
		t.Run(v.name, func(t *testing.T) {
			c, _ := client(t, func(*http.Request) (*http.Response, error) { return response(v.b, v.h), nil })
			if _, e := c.Start(context.Background()); e == nil {
				t.Fatal("invalid RPC accepted")
			}
		})
	}
}
func TestSearchAndCatalogUseLiveAPIShape(t *testing.T) {
	c, _ := client(t, func(r *http.Request) (*http.Response, error) {
		var data any
		switch r.URL.Path {
		case "/webapi/search/novels":
			if r.URL.Query().Get("k") != "测试" || r.URL.Query().Get("page") != "2" {
				t.Fatal("wrong search query")
			}
			data = map[string]any{"count": 21, "list": []any{map[string]any{"novel_id": 42, "novel_name": "合成作品", "novel_author": "作者"}}}
		case "/webapi/novel/novelInfo":
			data = map[string]any{"novel_id": 42, "novel_name": "合成作品", "author_nickname": "作者", "novel_status": 1, "novel_lock": 0}
		case "/webapi/novel/chapterGetList":
			data = map[string]any{"list": []any{map[string]any{"type": "volume", "name": "正文"}, map[string]any{"type": "item", "id": 101, "name": "第一章", "chapter_status": 1, "chapter_clock": 0, "lock": false, "pay": false}, map[string]any{"type": "item", "id": 102, "name": "第二章", "chapter_status": 1, "chapter_clock": 0, "pay": true, "is_sub": false}}}
		default:
			t.Fatal(r.URL.Path)
		}
		b, _ := json.Marshal(map[string]any{"code": 200, "data": data})
		return response(b, nil), nil
	})
	result, e := c.Search(context.Background(), "测试", 2)
	if e != nil || result.Count != 21 || len(result.Books) != 1 || result.Books[0].ID != "42" {
		t.Fatal(result, e)
	}
	b, e := c.Catalog(context.Background(), "42")
	if e != nil || b.Author != "作者" || len(b.Chapters) != 2 || !b.Chapters[1].Locked {
		t.Fatal(b, e)
	}
}
func TestDiscoverKeepsOfficialRankingGroups(t *testing.T) {
	c, _ := client(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/webapi/home/initPcIndex" {
			t.Fatal(r.URL)
		}
		return response([]byte(`{"code":200,"data":{"rankingList":[{"name":"榜单甲","subList":[{"name":"昨日","list":[{"novel_id":42,"novel_name":"合成书","novel_author":"作者"}]},{"name":"本周","list":[]}]}]}}`), nil), nil
	})
	ranks, e := c.Discover(context.Background())
	if e != nil || len(ranks) != 1 || ranks[0].Title != "榜单甲 · 昨日" || ranks[0].Books[0].ID != "42" {
		t.Fatal(ranks, e)
	}
}
