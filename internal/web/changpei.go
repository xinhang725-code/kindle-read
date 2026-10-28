// SPDX-License-Identifier: AGPL-3.0-only
package web

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/xinhang725-code/kindle-read/internal/changpei"
	"github.com/xinhang725-code/kindle-read/internal/device"
	"github.com/xinhang725-code/kindle-read/internal/library"
)

func (a *App) changpeiAPI(w http.ResponseWriter, r *http.Request, path string) bool {
	switch path {
	case "api/login/start":
		a.qr = ""
		login, e := a.cp.Start(r.Context())
		if e != nil {
			fail(w, e)
			return true
		}
		a.qr = login.URL
		login.URL = "qr.png"
		writeJSON(w, 200, login)
		return true
	case "api/login/poll":
		var v struct {
			Generation uint64 `json:"generation"`
			OTP        string `json:"otp"`
		}
		if e := decode(w, r, &v); e != nil {
			fail(w, e)
			return true
		}
		login, e := a.cp.Poll(r.Context(), v.Generation)
		if e != nil {
			a.qr = ""
			fail(w, e)
			return true
		}
		if login.State == "ready" {
			a.qr = ""
		}
		writeJSON(w, 200, login)
		return true
	case "api/logout":
		account := a.account()
		if e := a.cp.Logout(); e != nil {
			fail(w, e)
			return true
		}
		a.qr = ""
		delete(a.stores, account)
		if account != "guest" {
			if e := os.RemoveAll(filepath.Join(a.Root, "changpei", library.Key(account))); e != nil {
				fail(w, errors.New("已退出登录，账号缓存清理失败，请在设备中手动处理"))
				return true
			}
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
		return true
	case "api/state":
		if a.cp.Account().ID != "" {
			if e := a.cp.EnsureSession(r.Context()); e != nil {
				var cp *changpei.Error
				if !errors.As(e, &cp) || cp.Code != "AUTH_EXPIRED" {
					fail(w, e)
					return true
				}
			}
		}
		locked := a.cp.Account().ID == ""
		snapshot := library.State{Version: 1, Books: []library.Book{}, Progress: map[string]library.Progress{}}
		if !locked {
			s, e := a.store()
			if e != nil {
				fail(w, e)
				return true
			}
			snapshot = s.Snapshot()
		}
		writeJSON(w, 200, map[string]any{"source": a.Source, "name": device.Name(a.Source), "library": snapshot, "account": a.cp.Account(), "login_required": locked, "content_ready": true, "experimental": true})
		return true
	case "api/exit":
		a.cp.Cancel()
		a.qr = ""
		writeJSON(w, 200, map[string]bool{"ok": true})
		if a.Exit != nil {
			go a.Exit()
		}
		return true
	}
	// This gate includes cached reads, search, imports and progress writes; the
	// hidden frontend is not an authorization boundary.
	if e := a.cp.EnsureSession(r.Context()); e != nil {
		fail(w, e)
		return true
	}
	s, e := a.store()
	if e != nil {
		fail(w, e)
		return true
	}
	switch path {
	case "api/discover":
		ranks, e := a.cp.Discover(r.Context())
		if e != nil {
			fail(w, e)
		} else {
			writeJSON(w, 200, map[string]any{"rankings": ranks})
		}
		return true
	case "api/search":
		var v struct {
			Query string `json:"query"`
			Page  int    `json:"page"`
		}
		if e = decode(w, r, &v); e != nil {
			fail(w, e)
			return true
		}
		out, e := a.cp.Search(r.Context(), v.Query, v.Page)
		if e != nil {
			fail(w, e)
		} else {
			writeJSON(w, 200, out)
		}
		return true
	case "api/book":
		var v struct {
			URL string `json:"url"`
		}
		if e = decode(w, r, &v); e != nil {
			fail(w, e)
			return true
		}
		id, e := changpei.BookID(v.URL)
		if e != nil {
			fail(w, e)
			return true
		}
		book, e := a.cp.Catalog(r.Context(), id)
		if e == nil {
			e = s.PutBook(book)
		}
		if e != nil {
			fail(w, e)
		} else {
			writeJSON(w, 200, book)
		}
		return true
	case "api/chapter":
		bookID, chID := r.URL.Query().Get("book"), r.URL.Query().Get("chapter")
		book, e := s.Book(bookID)
		if e != nil {
			fail(w, e)
			return true
		}
		var chapter *library.Chapter
		for _, v := range book.Chapters {
			if v.ID == chID {
				v := v
				chapter = &v
				break
			}
		}
		if chapter == nil {
			fail(w, &changpei.Error{Code: "NOT_FOUND", Message: "章节不存在"})
			return true
		}
		if chapter.Locked {
			fail(w, &changpei.Error{Code: "CHAPTER_LOCKED", Message: "目录显示此章不可阅读或需要授权，请在官方 App 处理后刷新目录"})
			return true
		}
		blocks, e := s.Blocks(bookID, chID)
		if e != nil {
			content, err := a.cp.Chapter(r.Context(), book, chID)
			if err != nil {
				fail(w, err)
				return true
			}
			blocks = content.Blocks
			if e = s.PutBlocks(bookID, chID, blocks); e != nil {
				fail(w, e)
				return true
			}
		}
		paragraphs := make([]string, len(blocks))
		for i, b := range blocks {
			paragraphs[i] = b.Text
		}
		writeJSON(w, 200, map[string]any{"book": book, "chapter": chapter, "paragraphs": paragraphs, "blocks": blocks})
		return true
	case "api/import":
		fail(w, &changpei.Error{Code: "UNSUPPORTED", Message: "长佩应用请从书城或官方作品链接添加作品"})
		return true
	case "api/shelf":
		// This is the local account shelf. Do not claim upstream shelf sync.
		writeJSON(w, 200, map[string]any{"count": len(s.Snapshot().Books), "scope": "device"})
		return true
	}
	return false
}
