// SPDX-License-Identifier: AGPL-3.0-only
// Web QR/session/shelf protocol ported from MiuRead (miumiupy98-art),
// commit 581f9b660097aa42fe33170098346862cf0a9bbf. See THIRD_PARTY_NOTICES.md.
package miuread

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xinhang725-code/kindle-read/internal/library"
	"github.com/xinhang725-code/kindle-read/internal/provider"
)

const base = "https://weread.qq.com"

var baseURL, _ = url.Parse(base)

type Account struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type authFile struct {
	Version     int            `json:"version"`
	Fingerprint string         `json:"fingerprint"`
	Account     Account        `json:"account"`
	Cookies     []*http.Cookie `json:"cookies,omitempty"`
}
type Login struct {
	URL        string `json:"url"`
	Generation uint64 `json:"generation"`
	State      string `json:"state"`
}
type Client struct {
	mu               sync.Mutex
	path             string
	auth             authFile
	network, pending *provider.Network
	generation       uint64
	uid              string
	started          time.Time
	cancel           context.CancelFunc
}

func Open(root string, n *provider.Network) (*Client, error) {
	dir := filepath.Join(root, "miuread")
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	c := &Client{path: filepath.Join(dir, "auth.json"), network: n}
	b, e := os.ReadFile(c.path)
	if e == nil {
		if len(b) > 64<<10 || json.Unmarshal(b, &c.auth) != nil || c.auth.Version != 1 {
			return nil, errors.New("MiuRead 会话文件损坏，原文件已保留")
		}
	} else if errors.Is(e, os.ErrNotExist) {
		var entropy [4]byte
		if _, e = rand.Read(entropy[:]); e != nil {
			return nil, e
		}
		c.auth = authFile{Version: 1, Fingerprint: strconv.FormatUint(uint64(binary.BigEndian.Uint32(entropy[:])), 10)}
		if e = c.save(c.auth); e != nil {
			return nil, e
		}
	} else {
		return nil, e
	}
	if _, e := strconv.ParseUint(c.auth.Fingerprint, 10, 32); e != nil {
		return nil, errors.New("MiuRead 设备标识无效")
	}
	c.network.Client.Jar.SetCookies(baseURL, c.auth.Cookies)
	return c, nil
}
func (c *Client) save(a authFile) error {
	b, e := json.Marshal(a)
	if e != nil {
		return e
	}
	if library.AtomicWrite(c.path, b) != nil {
		return errors.New("登录状态未保存，请检查剩余空间后重试")
	}
	return nil
}
func (c *Client) Fingerprint() string { c.mu.Lock(); defer c.mu.Unlock(); return c.auth.Fingerprint }
func (c *Client) Account() Account    { c.mu.Lock(); defer c.mu.Unlock(); return c.auth.Account }
func (c *Client) newNetwork() *provider.Network {
	n := provider.NewNetwork(provider.Hosts("miuread"))
	n.Client.Transport = c.network.Client.Transport
	return n
}
func (c *Client) cancelLocked() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.generation++
	c.pending = nil
	c.uid = ""
}
func (c *Client) Cancel() { c.mu.Lock(); defer c.mu.Unlock(); c.cancelLocked() }
func headers() http.Header {
	return http.Header{"Referer": []string{base + "/r/weread-skills"}, "Origin": []string{base}, "Accept": []string{"application/json"}}
}
func request(ctx context.Context, n *provider.Network, method, path string, payload any) (map[string]json.RawMessage, error) {
	var b []byte
	if payload != nil {
		b, _ = json.Marshal(payload)
	}
	h := headers()
	if payload != nil {
		h.Set("Content-Type", "application/json")
	}
	raw, e := n.Do(ctx, method, base+path, b, h)
	if e != nil {
		return nil, e
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return nil, errors.New("平台未返回预期数据，可能需要在官方客户端完成验证")
	}
	for _, key := range []string{"errCode", "errcode"} {
		if v := scalar(m[key]); v != "" && v != "0" {
			return nil, fmt.Errorf("微信读书请求失败（代码 %s），请重新授权或稍后再试", safeCode(v))
		}
	}
	return m, nil
}
func safeCode(s string) string {
	if regexp.MustCompile(`^-?[0-9]{1,9}$`).MatchString(s) {
		return s
	}
	return "unknown"
}
func scalar(b json.RawMessage) string {
	var s string
	if json.Unmarshal(b, &s) == nil {
		return s
	}
	var n json.Number
	if json.Unmarshal(b, &n) == nil {
		return n.String()
	}
	return ""
}
func (c *Client) Start(ctx context.Context) (Login, error) {
	c.mu.Lock()
	c.cancelLocked()
	gen := c.generation
	n := c.newNetwork()
	c.pending = n
	c.started = time.Now()
	fp := c.auth.Fingerprint
	requestCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.mu.Unlock()
	n.Client.Jar.SetCookies(baseURL, []*http.Cookie{{Name: "wr_fp", Value: fp, Path: "/", Secure: true}})
	_, e := n.Do(requestCtx, "GET", base+"/r/weread-skills", nil, headers())
	if e != nil {
		return Login{}, e
	}
	m, e := request(requestCtx, n, "GET", "/api/auth/getLoginUid", nil)
	if e != nil {
		return Login{}, e
	}
	uid := scalar(m["uid"])
	if uid == "" || len(uid) > 512 {
		return Login{}, errors.New("平台没有返回登录二维码")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.generation != gen {
		return Login{}, errors.New("登录已取消")
	}
	c.uid = uid
	return Login{URL: base + "/web/confirm?uid=" + url.QueryEscape(uid), Generation: gen, State: "waiting"}, nil
}
func (c *Client) Poll(ctx context.Context, gen uint64, otp string) (Login, error) {
	c.mu.Lock()
	if gen != c.generation || c.pending == nil || c.uid == "" || time.Since(c.started) > 5*time.Minute {
		c.mu.Unlock()
		return Login{}, errors.New("登录已取消或二维码已过期")
	}
	n, uid, fp := c.pending, c.uid, c.auth.Fingerprint
	c.mu.Unlock()
	if otp != "" && !regexp.MustCompile(`^[0-9]{4}$`).MatchString(otp) {
		return Login{}, errors.New("请输入手机显示的四位验证码")
	}
	m, e := request(ctx, n, "GET", "/api/auth/getLoginInfo?uid="+url.QueryEscape(uid)+"&otp="+url.QueryEscape(otp), nil)
	if e != nil {
		return Login{}, e
	}
	result := Login{Generation: gen, State: "waiting"}
	var succeeded bool
	json.Unmarshal(m["succeed"], &succeeded)
	if !succeeded {
		switch scalar(m["logicCode"]) {
		case "NEED_OTP", "OTP_NOT_MATCH":
			result.State = "otp"
		case "LOGIN_DECLINED", "LOGIN_TIMEOUT", "OTP_EXPIRED":
			return Login{}, errors.New("授权未确认或已过期，请重新获取二维码")
		}
		return result, nil
	}
	vid, key := scalar(m["webLoginVid"]), scalar(m["accessToken"])
	if vid == "" || key == "" {
		return Login{}, errors.New("登录凭据不完整")
	}
	cookies := []*http.Cookie{{Name: "wr_fp", Value: fp, Path: "/", Secure: true}, {Name: "wr_vid", Value: vid, Path: "/", Secure: true}, {Name: "wr_skey", Value: key, Path: "/", Secure: true}, {Name: "wr_ql", Value: "0", Path: "/", Secure: true}}
	if rt := scalar(m["refreshToken"]); rt != "" {
		cookies = append(cookies, &http.Cookie{Name: "wr_rt", Value: url.QueryEscape(rt), Path: "/", Secure: true})
	}
	n.Client.Jar.SetCookies(baseURL, cookies)
	user, e := request(ctx, n, "GET", "/api/userInfo?userVid="+url.QueryEscape(vid), nil)
	if e != nil {
		return Login{}, e
	}
	name := scalar(user["name"])
	if name == "" {
		name = scalar(user["nickName"])
	}
	if name == "" {
		name = "微信读书账号"
	}
	next := authFile{Version: 1, Fingerprint: fp, Account: Account{ID: vid, Name: name}}
	for _, v := range n.Client.Jar.Cookies(baseURL) {
		if strings.HasPrefix(v.Name, "wr_") || v.Name == "ptcz" || v.Name == "RK" || v.Name == "pgv_pvid" {
			next.Cookies = append(next.Cookies, &http.Cookie{Name: v.Name, Value: v.Value, Path: "/", Secure: true, HttpOnly: true})
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.generation != gen || c.pending != n {
		return Login{}, errors.New("登录已取消，未保存新账号")
	}
	if e = c.save(next); e != nil {
		return Login{}, e
	}
	c.auth = next
	c.network = n
	c.cancelLocked()
	result.State = "ready"
	return result, nil
}
func (c *Client) Logout() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	next := authFile{Version: 1, Fingerprint: c.auth.Fingerprint}
	if e := c.save(next); e != nil {
		return e
	}
	c.cancelLocked()
	c.auth = next
	c.network = c.newNetwork()
	return nil
}
func (c *Client) Shelf(ctx context.Context) ([]library.Book, error) {
	c.mu.Lock()
	a, n, gen := c.auth.Account, c.network, c.generation
	c.mu.Unlock()
	if a.ID == "" {
		return nil, errors.New("请先扫码登录微信读书")
	}
	index, e := request(ctx, n, "GET", "/web/shelf/sync?onlyBookid=1&cbcount=1&userVid="+url.QueryEscape(a.ID), nil)
	if e != nil {
		return nil, e
	}
	var values []json.RawMessage
	if json.Unmarshal(index["bookIds"], &values) != nil {
		return nil, errors.New("微信读书书架索引结构待适配")
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, v := range values {
		id := scalar(v)
		if id != "" && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	books := []library.Book{}
	for start := 0; start < len(ids); start += 20 {
		end := start + 20
		if end > len(ids) {
			end = len(ids)
		}
		batch, e := request(ctx, n, "POST", "/web/shelf/syncBook", map[string]any{"bookIds": ids[start:end]})
		if e != nil {
			return nil, e
		}
		var rawBooks []json.RawMessage
		if json.Unmarshal(batch["books"], &rawBooks) != nil {
			return nil, errors.New("微信读书书架详情结构待适配")
		}
		for _, raw := range rawBooks {
			var item map[string]json.RawMessage
			if json.Unmarshal(raw, &item) != nil {
				continue
			}
			if nested := item["bookInfo"]; len(nested) > 0 {
				json.Unmarshal(nested, &item)
			}
			id, title := scalar(item["bookId"]), scalar(item["title"])
			if id == "" || title == "" {
				return nil, errors.New("微信读书书籍信息不完整")
			}
			books = append(books, library.Book{ID: id, Title: title, Author: scalar(item["author"]), Chapters: []library.Chapter{}})
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.auth.Account.ID != a.ID || c.generation != gen {
		return nil, errors.New("账号已切换，已丢弃旧书架响应")
	}
	return books, nil
}
