// SPDX-License-Identifier: AGPL-3.0-only
package changpei

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/xinhang725-code/kindle-read/internal/library"
)

func encrypted(s string) string {
	b := []byte(s)
	padding := 16 - len(b)%16
	for i := 0; i < padding; i++ {
		b = append(b, byte(padding))
	}
	a, _ := aes.NewCipher([]byte("u0LRrbu$Enm84koA"))
	cipher.NewCBCEncrypter(a, []byte("$h$b3!iGzsYnnshj")).CryptBlocks(b, b)
	return base64.StdEncoding.EncodeToString(b)
}
func TestChapterBlocksAndPermissionGates(t *testing.T) {
	for _, test := range []struct {
		name, code string
		change     func(map[string]any, map[string]any, map[string]any)
	}{
		{"free", "", func(d, c, n map[string]any) {}},
		{"purchased", "", func(d, c, n map[string]any) { c["chapter_ispay"] = 1; c["chapterPrice"] = 10 }},
		{"verification", "VERIFICATION_REQUIRED", func(d, c, n map[string]any) { d["check"] = 2 }},
		{"missing gate", "UPSTREAM_ERROR", func(d, c, n map[string]any) { delete(d, "check") }},
		{"unpaid", "NOT_PURCHASED", func(d, c, n map[string]any) { c["chapter_ispay"] = 1; c["isSub"] = 0 }},
		{"locked", "CHAPTER_LOCKED", func(d, c, n map[string]any) { c["lock"] = 1 }},
		{"missing status", "UPSTREAM_ERROR", func(d, c, n map[string]any) { delete(c, "status") }},
		{"app only", "APP_REQUIRED", func(d, c, n map[string]any) { c["chapterPrice"] = 10; n["page_view_status"] = false }},
		{"welfare", "APP_REQUIRED", func(d, c, n map[string]any) { c["chapter_type"] = 1 }},
		{"wrong book", "UPSTREAM_ERROR", func(d, c, n map[string]any) { n["id"] = 999 }},
		{"unsupported rendering", "UNSUPPORTED_CONTENT", func(d, c, n map[string]any) { c["renderType"] = 2 }},
		{"bad ciphertext", "UPSTREAM_ERROR", func(d, c, n map[string]any) { c["content"] = "not base64" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			ch := map[string]any{"name": "合成章节", "status": 1, "lock": 0, "chapter_ispay": 0, "chapterPrice": 0, "isSub": 1, "chapter_type": 0, "renderType": 1, "content": encrypted("第一段。\n\n第二段😀。"), "postscript": "作者附言。"}
			novel := map[string]any{"id": 42, "page_view_status": true}
			data := map[string]any{"check": 0, "chapterInfo": ch, "novelInfo": novel}
			test.change(data, ch, novel)
			c, _ := client(t, func(r *http.Request) (*http.Response, error) {
				if r.URL.Query().Get("cid") != "101" {
					t.Fatal("wrong chapter")
				}
				b, _ := json.Marshal(map[string]any{"code": 200, "data": data})
				return response(b, nil), nil
			})
			got, e := c.Chapter(context.Background(), library.Book{ID: "42", Chapters: []library.Chapter{{ID: "101", Title: "合成章节"}}}, "101")
			if test.code != "" {
				if e == nil || asError(e).Code != test.code {
					t.Fatalf("want %s, got %+v %v", test.code, got, e)
				}
				return
			}
			if e != nil || len(got.Blocks) != 3 || got.Blocks[1].Text != "第二段😀。" || got.Blocks[2].Type != "note" {
				t.Fatalf("%+v %v", got, e)
			}
		})
	}
}
func TestInvalidBookLinkNeverReachesNetwork(t *testing.T) {
	for _, s := range []string{"https://evil.test/novel-42.html", "http://www.gongzicp.com/novel-42.html", "https://www.gongzicp.com@evil.test/novel-42.html", "https://www.gongzicp.com:444/novel-42.html", "https://www.gongzicp.com/read-1.html"} {
		if _, e := BookID(s); e == nil {
			t.Fatal("accepted", s)
		}
	}
	for _, s := range []string{"42", "https://www.gongzicp.com/novel-42.html?from=share", "https://m.gongzicp.com/novel-42.html"} {
		if id, e := BookID(s); e != nil || id != "42" {
			t.Fatal(id, e)
		}
	}
}
func TestDecryptRejectsBadPaddingAndOversize(t *testing.T) {
	for _, s := range []string{"", base64.StdEncoding.EncodeToString(make([]byte, 16)), strings.Repeat("A", 4<<20)} {
		if _, e := decodeContent(s); e == nil {
			t.Fatal("invalid ciphertext accepted")
		}
	}
}
