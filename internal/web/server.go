// SPDX-License-Identifier: AGPL-3.0-only
package web

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	qrcode "github.com/skip2/go-qrcode"
	"github.com/xinhang725-code/kindle-read/internal/device"
	"github.com/xinhang725-code/kindle-read/internal/library"
	"github.com/xinhang725-code/kindle-read/internal/miuread"
	"github.com/xinhang725-code/kindle-read/internal/provider"
)

//go:embed assets/*
var assets embed.FS

type App struct {
	Root, Source, Prefix, Host, SourceArchive string
	LastSeen                                  atomic.Int64
	Exit                                      func()
	mu                                        sync.Mutex
	stores                                    map[string]*library.Store
	network                                   *provider.Network
	miu                                       *miuread.Client
	qr                                        string
}

func New(root, source string) (*App, error) {
	if !library.ValidSource(source) {
		return nil, errors.New("未知应用")
	}
	var secret [24]byte
	if _, e := rand.Read(secret[:]); e != nil {
		return nil, e
	}
	a := &App{Root: root, Source: source, Prefix: "/s/" + hex.EncodeToString(secret[:]) + "/", stores: map[string]*library.Store{}, network: provider.NewNetwork(provider.Hosts(source))}
	if source == "miuread" {
		m, e := miuread.Open(root, a.network)
		if e != nil {
			return nil, e
		}
		a.miu = m
	}
	if _, e := a.store(); e != nil {
		return nil, e
	}
	a.LastSeen.Store(time.Now().Unix())
	return a, nil
}
func (a *App) account() string {
	if a.miu != nil {
		if id := a.miu.Account().ID; id != "" {
			return id
		}
	}
	return "guest"
}
func (a *App) store() (*library.Store, error) {
	account := a.account()
	if s := a.stores[account]; s != nil {
		return s, nil
	}
	s, e := library.Open(a.Root, a.Source, account)
	if e == nil {
		a.stores[account] = s
	}
	return s, e
}
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func fail(w http.ResponseWriter, e error) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": e.Error()})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 3<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return errors.New("请求数据无效或超过大小限制")
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return errors.New("请求包含多余数据")
	}
	return nil
}
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self'; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	if a.Host == "" || r.Host != a.Host || !strings.HasPrefix(r.URL.Path, a.Prefix) || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+a.Host) || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "forbidden", 403)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, a.Prefix)
	a.LastSeen.Store(time.Now().Unix())
	if strings.HasPrefix(path, "api/") {
		if r.Header.Get("X-Kindle-Read") != "1" {
			http.Error(w, "forbidden", 403)
			return
		}
		method := "POST"
		if path == "api/state" || path == "api/chapter" || path == "api/health" {
			method = "GET"
		}
		if r.Method != method {
			w.Header().Set("Allow", method)
			http.Error(w, "method not allowed", 405)
			return
		}
		if path == "api/login/cancel" && a.miu != nil {
			a.miu.Cancel()
			writeJSON(w, 200, map[string]bool{"ok": true})
			return
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		a.api(w, r, path)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	if path == "source.tar.gz" {
		if a.SourceArchive == "" {
			http.Error(w, "Source is in the project checkout. Packaged builds include a source archive.", 404)
			return
		}
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", "attachment; filename=kindle-read-source.tar.gz")
		http.ServeFile(w, r, a.SourceArchive)
		return
	}
	if path == "qr.png" {
		a.mu.Lock()
		qr := a.qr
		a.mu.Unlock()
		if qr == "" {
			http.NotFound(w, r)
			return
		}
		png, e := qrcode.Encode(qr, qrcode.Medium, 420)
		if e != nil {
			http.Error(w, "QR unavailable", 500)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
		return
	}
	if path == "" {
		path = "index.html"
	}
	types := map[string]string{"index.html": "text/html; charset=utf-8", "app.js": "application/javascript", "reader.js": "application/javascript", "style.css": "text/css"}
	kind, ok := types[path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	data, e := assets.ReadFile("assets/" + path)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", kind)
	_, _ = w.Write(data)
}
func (a *App) api(w http.ResponseWriter, r *http.Request, path string) {
	if path == "api/health" {
		writeJSON(w, 200, map[string]string{"source": a.Source})
		return
	}
	s, e := a.store()
	if e != nil {
		fail(w, e)
		return
	}
	switch path {
	case "api/state":
		account := miuread.Account{}
		if a.miu != nil {
			account = a.miu.Account()
		}
		writeJSON(w, 200, map[string]any{"source": a.Source, "name": device.Name(a.Source), "library": s.Snapshot(), "account": account, "content_ready": a.Source != "miuread", "experimental": true})
	case "api/import":
		var v struct {
			Title string `json:"title"`
			Text  string `json:"text"`
		}
		if e = decode(w, r, &v); e == nil {
			var book library.Book
			book, e = s.Import(v.Title, v.Text)
			if e == nil {
				writeJSON(w, 200, book)
				return
			}
		}
		fail(w, e)
	case "api/book":
		if a.Source == "miuread" {
			fail(w, errors.New("MiuRead 请使用微信读书账号书架"))
			return
		}
		var v struct {
			URL string `json:"url"`
		}
		if e = decode(w, r, &v); e != nil {
			fail(w, e)
			return
		}
		book, paragraphs, e := provider.Fetch(r.Context(), a.network, a.Source, v.URL)
		if e != nil {
			fail(w, e)
			return
		}
		if e = s.PutBook(book); e == nil && len(paragraphs) > 0 {
			e = s.PutContent(book.ID, book.Chapters[0].ID, paragraphs)
		}
		if e != nil {
			fail(w, e)
			return
		}
		writeJSON(w, 200, book)
	case "api/chapter":
		bookID, chID := r.URL.Query().Get("book"), r.URL.Query().Get("chapter")
		book, e := s.Book(bookID)
		if e != nil {
			fail(w, e)
			return
		}
		var chapter *library.Chapter
		for _, ch := range book.Chapters {
			if ch.ID == chID {
				c := ch
				chapter = &c
				break
			}
		}
		if chapter == nil {
			fail(w, errors.New("章节不存在"))
			return
		}
		paragraphs, e := s.Content(bookID, chID)
		if e != nil {
			if chapter.Locked {
				fail(w, errors.New("此章需要授权或订阅，请在官方客户端处理"))
				return
			}
			if a.Source == "miuread" && !book.Local {
				fail(w, errors.New("微信读书正文解码尚未迁移，本版本不能阅读线上正文"))
				return
			}
			if book.Local {
				fail(w, e)
				return
			}
			_, paragraphs, e = provider.Fetch(r.Context(), a.network, a.Source, chapter.URL)
			if e != nil {
				fail(w, e)
				return
			}
			if e = s.PutContent(bookID, chID, paragraphs); e != nil {
				fail(w, e)
				return
			}
		}
		writeJSON(w, 200, map[string]any{"book": book, "chapter": chapter, "paragraphs": paragraphs})
	case "api/progress":
		var v library.Progress
		if e = decode(w, r, &v); e == nil {
			e = s.SaveProgress(v)
		}
		if e != nil {
			fail(w, e)
			return
		}
		writeJSON(w, 200, map[string]bool{"saved": true})
	case "api/login/start":
		if a.miu == nil {
			fail(w, errors.New("此平台账号登录尚未实现"))
			return
		}
		login, e := a.miu.Start(r.Context())
		if e != nil {
			fail(w, e)
			return
		}
		a.qr = login.URL
		login.URL = "qr.png"
		writeJSON(w, 200, login)
	case "api/login/poll":
		if a.miu == nil {
			fail(w, errors.New("此平台账号登录尚未实现"))
			return
		}
		var v struct {
			Generation uint64 `json:"generation"`
			OTP        string `json:"otp"`
		}
		if e = decode(w, r, &v); e != nil {
			fail(w, e)
			return
		}
		status, e := a.miu.Poll(r.Context(), v.Generation, v.OTP)
		if e != nil {
			fail(w, e)
			return
		}
		if status.State == "ready" {
			a.qr = ""
		}
		writeJSON(w, 200, status)
	case "api/shelf":
		if a.miu == nil {
			fail(w, errors.New("请使用添加链接或重新添加作品链接刷新目录"))
			return
		}
		books, e := a.miu.Shelf(r.Context())
		if e != nil {
			fail(w, e)
			return
		}
		for _, book := range books {
			if e = s.PutBook(book); e != nil {
				fail(w, e)
				return
			}
		}
		writeJSON(w, 200, map[string]int{"count": len(books)})
	case "api/logout":
		if a.miu == nil {
			fail(w, errors.New("没有已登录账号"))
			return
		}
		account := a.account()
		if e = a.miu.Logout(); e != nil {
			fail(w, e)
			return
		}
		a.qr = ""
		delete(a.stores, account)
		if account != "guest" {
			e = os.RemoveAll(filepath.Join(a.Root, a.Source, library.Key(account)))
			if e != nil {
				fail(w, errors.New("已退出登录，但账号缓存清理失败，请在设备文件中处理"))
				return
			}
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	case "api/exit":
		if a.miu != nil {
			a.miu.Cancel()
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
		if a.Exit != nil {
			go a.Exit()
		}
	default:
		http.NotFound(w, r)
	}
}
