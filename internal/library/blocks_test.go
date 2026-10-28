package library

import "testing"

func TestStructuredBlocksRetainNotesAndLegacyAnchors(t *testing.T) {
	root := t.TempDir()
	s, _ := Open(root, "changpei", "123")
	book := Book{ID: "42", Title: "合成书", Chapters: []Chapter{{ID: "1", Title: "一"}}}
	if e := s.PutBook(book); e != nil {
		t.Fatal(e)
	}
	blocks := []Block{{Type: "p", Text: "正文😀"}, {Type: "note", Text: "作者的话"}}
	if e := s.PutBlocks("42", "1", blocks); e != nil {
		t.Fatal(e)
	}
	if e := s.SaveProgress(Progress{BookID: "42", ChapterID: "1", Paragraph: 1, Offset: 2, FontSize: 18}); e != nil {
		t.Fatal(e)
	}
	again, _ := Open(root, "changpei", "123")
	got, e := again.Blocks("42", "1")
	if e != nil || len(got) != 2 || got[1].Type != "note" {
		t.Fatal(got, e)
	}
	if again.Snapshot().Last.FontSize != 18 {
		t.Fatal("18px setting lost")
	}
	p, e := again.Content("42", "1")
	if e != nil || p[1] != "作者的话" {
		t.Fatal(p, e)
	}
	if e = s.PutBlocks("42", "1", []Block{{Type: "img", Text: "unimplemented"}}); e == nil {
		t.Fatal("unsupported block silently accepted")
	}
	got, _ = s.Blocks("42", "1")
	if got[1].Type != "note" {
		t.Fatal("failure destroyed existing chapter")
	}
	local, e := s.Import("本地", "第一段\n第二段")
	if e != nil {
		t.Fatal(e)
	}
	got, e = s.Blocks(local.ID, "text")
	if e != nil || got[0].Type != "p" {
		t.Fatal("legacy paragraphs unreadable", e)
	}
}
