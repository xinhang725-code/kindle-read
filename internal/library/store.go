// SPDX-License-Identifier: AGPL-3.0-only
package library

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf16"
	"unicode/utf8"
)

type Chapter struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	URL    string `json:"url,omitempty"`
	Locked bool   `json:"locked,omitempty"`
}
type Book struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Author   string    `json:"author,omitempty"`
	URL      string    `json:"url,omitempty"`
	Chapters []Chapter `json:"chapters"`
	Local    bool      `json:"local,omitempty"`
}

// Offset is a UTF-16 offset, matching the browser's strings. Paragraphs are zero-based.
type Progress struct {
	BookID    string `json:"book_id"`
	ChapterID string `json:"chapter_id"`
	Paragraph int    `json:"paragraph"`
	Offset    int    `json:"offset"`
	FontSize  int    `json:"font_size"`
}
type State struct {
	Version  int                 `json:"version"`
	Books    []Book              `json:"books"`
	Last     *Progress           `json:"last,omitempty"`
	Progress map[string]Progress `json:"progress"`
	Detached map[string]Progress `json:"detached_progress,omitempty"`
}
type Store struct {
	mu        sync.Mutex
	root, dir string
	state     State
}

func ValidSource(source string) bool {
	return source == "qidian" || source == "changpei" || source == "miuread"
}
func Key(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func Open(root, source, account string) (*Store, error) {
	if !ValidSource(source) || account == "" {
		return nil, errors.New("无效来源或账号")
	}
	dir := filepath.Join(root, source, Key(account))
	if err := os.MkdirAll(filepath.Join(dir, "cache"), 0700); err != nil {
		return nil, err
	}
	s := &Store{root: root, dir: dir, state: State{Version: 1, Books: []Book{}, Progress: map[string]Progress{}}}
	b, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) > 8<<20 || json.Unmarshal(b, &s.state) != nil || s.state.Version != 1 || s.state.Books == nil || s.state.Progress == nil {
		return nil, errors.New("书架文件损坏或版本不兼容，原文件已保留")
	}
	return s, nil
}
func copyState(s State) State    { b, _ := json.Marshal(s); var c State; json.Unmarshal(b, &c); return c }
func (s *Store) Snapshot() State { s.mu.Lock(); defer s.mu.Unlock(); return copyState(s.state) }

// AtomicWrite never truncates the last committed file. A failed rename leaves it intact.
func AtomicWrite(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err = f.Chmod(0600); err != nil {
		return err
	}
	if _, err = f.Write(b); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	// Some FAT filesystems reject directory fsync. The rename has committed at this point.
	if d, e := os.Open(filepath.Dir(path)); e == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
func (s *Store) commit(next State) error {
	b, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if len(b) > 8<<20 {
		return errors.New("书架目录过大")
	}
	if err = AtomicWrite(filepath.Join(s.dir, "state.json"), b); err != nil {
		return errors.New("书架保存失败，请检查剩余空间")
	}
	s.state = next
	return nil
}
func (s *Store) PutBook(book Book) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if book.ID == "" || len(book.ID) > 2048 || strings.TrimSpace(book.Title) == "" || len(book.Chapters) > 20000 {
		return errors.New("书籍信息不完整")
	}
	seen := map[string]bool{}
	for _, ch := range book.Chapters {
		if ch.ID == "" || seen[ch.ID] {
			return errors.New("目录章节标识无效或重复")
		}
		seen[ch.ID] = true
	}
	if book.Chapters == nil {
		book.Chapters = []Chapter{}
	}
	book.Chapters = append([]Chapter{}, book.Chapters...)
	n := copyState(s.state)
	found := false
	for i, b := range n.Books {
		if b.ID == book.ID {
			n.Books[i] = book
			found = true
			break
		}
	}
	if !found {
		n.Books = append(n.Books, book)
	}
	if previous, ok := n.Progress[book.ID]; ok && !seen[previous.ChapterID] {
		if n.Detached == nil {
			n.Detached = map[string]Progress{}
		}
		n.Detached[book.ID] = previous
		delete(n.Progress, book.ID)
		if n.Last != nil && n.Last.BookID == book.ID {
			n.Last = nil
		}
	}
	if previous, ok := n.Detached[book.ID]; ok && seen[previous.ChapterID] {
		if _, active := n.Progress[book.ID]; !active {
			n.Progress[book.ID] = previous
		}
		delete(n.Detached, book.ID)
	}
	return s.commit(n)
}
func (s *Store) book(id string) (Book, error) {
	for _, b := range s.state.Books {
		if b.ID == id {
			return b, nil
		}
	}
	return Book{}, errors.New("书籍不存在")
}
func (s *Store) Book(id string) (Book, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, e := s.book(id)
	if e != nil {
		return b, e
	}
	b.Chapters = append([]Chapter{}, b.Chapters...)
	return b, nil
}
func (s *Store) chapter(book, id string) error {
	b, e := s.book(book)
	if e != nil {
		return e
	}
	for _, c := range b.Chapters {
		if c.ID == id {
			return nil
		}
	}
	return errors.New("章节不存在")
}
func (s *Store) cachePath(book, chapter string) string {
	return filepath.Join(s.dir, "cache", Key(book+"\x00"+chapter)+".json")
}

type Block struct {
	Type string `json:"t"`
	Text string `json:"text"`
}

func (s *Store) PutContent(book, chapter string, paragraphs []string) error {
	blocks := make([]Block, len(paragraphs))
	for i, p := range paragraphs {
		blocks[i] = Block{Type: "p", Text: p}
	}
	return s.PutBlocks(book, chapter, blocks)
}
func (s *Store) PutBlocks(book, chapter string, blocks []Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.chapter(book, chapter); e != nil {
		return e
	}
	if len(blocks) == 0 {
		return errors.New("正文为空")
	}
	size := 0
	for _, block := range blocks {
		if block.Type != "p" && block.Type != "note" {
			return errors.New("不支持的正文块类型")
		}
		p := block.Text
		if !utf8.ValidString(p) {
			return errors.New("正文编码无效")
		}
		size += len(p)
	}
	if size == 0 || size > 2<<20 {
		return errors.New("章节大小超出限制")
	}
	b, err := json.Marshal(blocks)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(s.dir, "cache"))
	if err != nil {
		return err
	}
	total := int64(len(b))
	target := s.cachePath(book, chapter)
	for _, e := range entries {
		if e.Name() == filepath.Base(target) {
			continue
		}
		info, er := e.Info()
		if er != nil {
			return er
		}
		total += info.Size()
	}
	if total > 100<<20 {
		return errors.New("本账号缓存已满（100 MiB），未覆盖已有正文")
	}
	return AtomicWrite(target, b)
}
func (s *Store) blocks(book, chapter string) ([]Block, error) {
	if e := s.chapter(book, chapter); e != nil {
		return nil, e
	}
	b, err := os.ReadFile(s.cachePath(book, chapter))
	if err != nil {
		return nil, errors.New("章节尚未缓存")
	}
	if len(b) > 12<<20 {
		return nil, errors.New("缓存损坏，请重新下载")
	}
	var blocks []Block
	if json.Unmarshal(b, &blocks) != nil {
		var legacy []string
		if json.Unmarshal(b, &legacy) != nil {
			return nil, errors.New("缓存损坏，请重新下载")
		}
		for _, p := range legacy {
			blocks = append(blocks, Block{Type: "p", Text: p})
		}
	}
	if len(blocks) == 0 {
		return nil, errors.New("缓存正文为空")
	}
	for _, block := range blocks {
		if (block.Type != "p" && block.Type != "note") || !utf8.ValidString(block.Text) {
			return nil, errors.New("缓存正文格式不支持")
		}
	}
	return blocks, nil
}
func (s *Store) Blocks(book, chapter string) ([]Block, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.blocks(book, chapter)
}
func (s *Store) content(book, chapter string) ([]string, error) {
	blocks, e := s.blocks(book, chapter)
	if e != nil {
		return nil, e
	}
	p := make([]string, len(blocks))
	for i, b := range blocks {
		p[i] = b.Text
	}
	return p, nil
}
func (s *Store) Content(book, chapter string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.content(book, chapter)
}
func (s *Store) SaveProgress(p Progress) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	paragraphs, err := s.content(p.BookID, p.ChapterID)
	if err != nil {
		return err
	}
	if p.Paragraph < 0 || p.Paragraph >= len(paragraphs) || p.Offset < 0 || p.Offset > len(utf16.Encode([]rune(paragraphs[p.Paragraph]))) {
		return errors.New("阅读位置无效")
	}
	if p.FontSize < 18 || p.FontSize > 48 {
		p.FontSize = 28
	}
	n := copyState(s.state)
	n.Progress[p.BookID] = p
	n.Last = &p
	return s.commit(n)
}
func (s *Store) Import(title, text string) (Book, error) {
	if len(text) > 2<<20 || !utf8.ValidString(text) {
		return Book{}, errors.New("仅支持不超过 2 MiB 的 UTF-8 文本")
	}
	var paragraphs []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			paragraphs = append(paragraphs, line)
		}
	}
	if len(paragraphs) == 0 {
		return Book{}, errors.New("文本为空")
	}
	b := Book{ID: "local-" + Key(text), Title: title, Local: true, Chapters: []Chapter{{ID: "text", Title: title}}}
	if e := s.PutBook(b); e != nil {
		return Book{}, e
	}
	if e := s.PutContent(b.ID, "text", paragraphs); e != nil {
		return b, fmt.Errorf("文本未缓存：%w", e)
	}
	return b, nil
}
