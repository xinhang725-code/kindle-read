// SPDX-License-Identifier: AGPL-3.0-only
package changpei

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/xinhang725-code/kindle-read/internal/library"
	"github.com/xinhang725-code/kindle-read/internal/provider"
)

type Account struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Login struct {
	URL        string `json:"url,omitempty"`
	Generation uint64 `json:"generation"`
	State      string `json:"state"`
}
type authFile struct {
	Version     int     `json:"version"`
	Fingerprint string  `json:"fingerprint"`
	Account     Account `json:"account"`
	Token       string  `json:"token,omitempty"`
	Refresh     string  `json:"refresh,omitempty"`
}
type Client struct {
	mu                sync.Mutex
	path, fingerprint string
	network           *provider.Network
	auth              authFile
	generation        uint64
	pending           string
	started, verified time.Time
	cancel            context.CancelFunc
}

func Open(root string, n *provider.Network) (*Client, error) {
	dir := filepath.Join(root, "changpei")
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	c := &Client{path: filepath.Join(dir, "auth.json"), network: n}
	b, e := os.ReadFile(c.path)
	if e == nil {
		if len(b) > 32<<10 || json.Unmarshal(b, &c.auth) != nil || c.auth.Version != 1 || len(c.auth.Fingerprint) != 32 || (c.auth.Account.ID != "" && (!validID(c.auth.Account.ID) || c.auth.Token == "")) {
			return nil, errors.New("长佩会话文件损坏，原文件已保留")
		}
	} else if errors.Is(e, os.ErrNotExist) {
		var entropy [16]byte
		if _, e = rand.Read(entropy[:]); e != nil {
			return nil, e
		}
		c.auth = authFile{Version: 1, Fingerprint: hex.EncodeToString(entropy[:])}
		if e = c.save(c.auth); e != nil {
			return nil, e
		}
	} else {
		return nil, e
	}
	c.fingerprint = c.auth.Fingerprint
	return c, nil
}
func (c *Client) save(a authFile) error {
	b, e := json.Marshal(a)
	if e != nil {
		return e
	}
	if library.AtomicWrite(c.path, b) != nil {
		return problem("STORAGE_ERROR", "登录状态未保存，请检查剩余空间")
	}
	return nil
}
func (c *Client) Account() Account { c.mu.Lock(); defer c.mu.Unlock(); return c.auth.Account }
func (c *Client) cancelLocked() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.generation++
	c.pending = ""
}
func (c *Client) Cancel() { c.mu.Lock(); defer c.mu.Unlock(); c.cancelLocked() }
func (c *Client) Start(ctx context.Context) (Login, error) {
	c.mu.Lock()
	c.cancelLocked()
	gen := c.generation
	requestCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.started = time.Now()
	c.mu.Unlock()
	defer cancel()
	m, e := c.rpc(requestCtx, "get_qrcode_token", stringField(1, "Login"), "")
	if e != nil {
		return Login{}, e
	}
	token := m.text(1)
	if token == "" || len(token) > 512 {
		return Login{}, malformed
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if gen != c.generation {
		return Login{}, problem("LOGIN_CANCELLED", "登录已取消")
	}
	c.pending = token
	q := url.Values{"token": {token}, "uid": {strconv.FormatUint(m.uint(2), 10)}, "pageType": {"app_qr_code_login"}}
	return Login{URL: base + "/transfer/login/middleware?" + q.Encode(), Generation: gen, State: "waiting"}, nil
}
func (c *Client) Poll(ctx context.Context, gen uint64) (Login, error) {
	c.mu.Lock()
	if gen != c.generation || c.pending == "" || time.Since(c.started) > 2*time.Minute {
		c.mu.Unlock()
		return Login{}, problem("QR_EXPIRED", "二维码已失效，请刷新")
	}
	pending := c.pending
	// The official RPC long-polls. Release the local HTTP request before its
	// 30-second UI deadline; silence is still waiting, never authentication.
	requestCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	c.cancel = cancel
	c.mu.Unlock()
	defer cancel()
	m, e := c.rpc(requestCtx, "qrcode_polling", stringField(1, pending), "")
	if e != nil {
		c.mu.Lock()
		active := gen == c.generation && c.pending == pending && time.Since(c.started) <= 2*time.Minute
		c.mu.Unlock()
		if active && ctx.Err() == nil && errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			return Login{Generation: gen, State: "waiting"}, nil
		}
		return Login{}, e
	}
	c.mu.Lock()
	active := gen == c.generation && c.pending == pending
	c.mu.Unlock()
	if !active {
		return Login{}, problem("LOGIN_CANCELLED", "登录已取消")
	}
	if m.text(3) != pending {
		return Login{}, malformed
	}
	out := Login{Generation: gen}
	switch m.uint(1) {
	case 1:
		out.State = "waiting"
		return out, nil
	case 2:
		out.State = "scanned"
		return out, nil
	case 3:
		return Login{}, problem("QR_REJECTED", "手机已取消授权，请刷新二维码")
	case 5:
		return Login{}, problem("QR_EXPIRED", "二维码已失效，请刷新")
	case 4:
	default:
		return Login{}, malformed
	}
	result, e := m.nested(2)
	if e != nil {
		return Login{}, e
	}
	if result.uint(1) != 1 {
		return Login{}, problem("VERIFICATION_REQUIRED", "请先在长佩官方 App 完成手机号或账号验证，再重新扫码")
	}
	web, e := result.nested(4)
	if e != nil {
		return Login{}, e
	}
	token, refresh := web.text(1), web.text(2)
	if token == "" || len(token) > 8192 || len(refresh) > 8192 {
		return Login{}, malformed
	}
	profile, e := c.rpc(requestCtx, "info", nil, token)
	if e != nil {
		return Login{}, e
	}
	uid := profile.uint(1)
	if uid == 0 || uid > 0xffffffff {
		return Login{}, malformed
	}
	next := authFile{Version: 1, Fingerprint: c.fingerprint, Account: Account{ID: strconv.FormatUint(uid, 10), Name: profile.text(2)}, Token: token, Refresh: refresh}
	c.mu.Lock()
	defer c.mu.Unlock()
	if gen != c.generation || pending != c.pending {
		return Login{}, problem("LOGIN_CANCELLED", "登录已取消，未保存新账号")
	}
	if e = c.save(next); e != nil {
		return Login{}, e
	}
	c.auth = next
	c.verified = time.Now()
	c.cancelLocked()
	out.State = "ready"
	return out, nil
}
func (c *Client) Logout() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	next := authFile{Version: 1, Fingerprint: c.fingerprint}
	if e := c.save(next); e != nil {
		return e
	}
	c.cancelLocked()
	c.auth = next
	c.verified = time.Time{}
	return nil
}

// EnsureSession validates a restored session before any protected screen/cache.
// Temporary network failure preserves credentials; it does not unlock the UI.
func (c *Client) EnsureSession(ctx context.Context) error {
	c.mu.Lock()
	a, verified, gen := c.auth, c.verified, c.generation
	c.mu.Unlock()
	if a.Account.ID == "" {
		return problem("AUTH_REQUIRED", "请先扫码登录长佩")
	}
	if time.Since(verified) < 5*time.Minute {
		return nil
	}
	profile, e := c.rpc(ctx, "info", nil, a.Token)
	if e != nil && asError(e).Code == "AUTH_EXPIRED" && a.Refresh != "" {
		var renewed message
		renewed, e = c.rpc(ctx, "get_token", stringField(5, a.Refresh), "")
		if e == nil {
			if strconv.FormatUint(renewed.uint(1), 10) != a.Account.ID || renewed.text(3) == "" {
				e = malformed
			} else {
				a.Token = renewed.text(3)
				profile, e = c.rpc(ctx, "info", nil, a.Token)
			}
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if gen != c.generation || a.Account.ID != c.auth.Account.ID {
		return problem("AUTH_EXPIRED", "登录状态已变化，请重新打开")
	}
	if e != nil {
		if asError(e).Code == "AUTH_EXPIRED" {
			next := authFile{Version: 1, Fingerprint: c.fingerprint}
			if err := c.save(next); err != nil {
				return err
			}
			c.auth = next
			c.cancelLocked()
		}
		return e
	}
	if strconv.FormatUint(profile.uint(1), 10) != a.Account.ID {
		return malformed
	}
	if a.Token != c.auth.Token {
		if e = c.save(a); e != nil {
			return e
		}
		c.auth = a
	}
	c.verified = time.Now()
	return nil
}
func (c *Client) token() string { c.mu.Lock(); defer c.mu.Unlock(); return c.auth.Token }

func (c *Client) invalidate(token string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if token == "" || c.auth.Token != token {
		return nil
	}
	next := authFile{Version: 1, Fingerprint: c.fingerprint}
	if e := c.save(next); e != nil {
		return e
	}
	c.auth = next
	c.verified = time.Time{}
	c.cancelLocked()
	return nil
}
