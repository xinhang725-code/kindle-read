package web

import (
	"context"
	"encoding/json"
	"github.com/xinhang725-code/kindle-read/internal/library"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCachedChapterRespectsRefreshedRestriction(t *testing.T) {
	a, s := startTest(t, t.TempDir(), "changpei")
	authorizeCP(t, a)
	store, _ := a.store()
	b := library.Book{ID: "42", Title: "test", Chapters: []library.Chapter{{ID: "101", Title: "one"}}}
	if e := store.PutBook(b); e != nil {
		t.Fatal(e)
	}
	if e := store.PutBlocks("42", "101", []library.Block{{Type: "p", Text: "cached private body"}}); e != nil {
		t.Fatal(e)
	}
	b.Chapters[0].Locked = true
	if e := store.PutBook(b); e != nil {
		t.Fatal(e)
	}
	code, data := call(t, a, s, "GET", "api/chapter?book=42&chapter=101", "")
	t.Logf("locked cached request status=%d returnedBody=%v", code, strings.Contains(string(data), "cached private body"))
	if code == 200 {
		t.Error("locked chapter still served from cache")
	}
}
func TestRefreshInvalidatesInFlightQRBeforeItsApproval(t *testing.T) {
	a, s := startTest(t, t.TempDir(), "changpei")
	entered := make(chan struct{})
	cancelled := make(chan struct{})
	release := make(chan struct{})
	var starts int
	a.network.Client.Transport = cpTransport(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/cp.UserExternal/get_qrcode_token":
			starts++
			return cpResponse(cpFrame(cpString(1, "qr"))), nil
		case "/cp.UserExternal/qrcode_polling":
			close(entered)
			select {
			case <-r.Context().Done():
				close(cancelled)
			case <-release:
			}
			<-release
			b := []byte{8, 4}
			b = append(b, cpString(3, "qr")...)
			login := append([]byte{8, 1}, cpString(4, string(cpString(1, "session")))...)
			b = append(b, cpString(2, string(login))...)
			return cpResponse(cpFrame(b)), nil
		case "/cp.UserExternal/info":
			return cpResponse(cpFrame([]byte{8, 123})), nil
		}
		t.Fatal("unexpected request")
		return nil, nil
	})
	login, e := a.cp.Start(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	body, _ := json.Marshal(map[string]any{"generation": login.Generation})
	pollDone := make(chan struct{})
	go func() { call(t, a, s, "POST", "api/login/poll", string(body)); close(pollDone) }()
	<-entered
	refreshDone := make(chan struct{})
	go func() { call(t, a, s, "POST", "api/login/start", "{}"); close(refreshDone) }()
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Error("refresh did not cancel the pending poll")
	}
	close(release)
	<-pollDone
	<-refreshDone
	t.Logf("starts=%d account after refresh=%q", starts, a.cp.Account().ID)
	if a.cp.Account().ID != "" {
		t.Error("old QR approved and persisted while refresh waited on HTTP mutex")
	}
}
