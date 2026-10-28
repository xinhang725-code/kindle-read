package library

import (
	"os"
	"path/filepath"
	"testing"
)

func exampleBook() Book {
	return Book{ID: "same", Title: "测试书", Chapters: []Chapter{{ID: "one", Title: "第一章"}}}
}
func TestSourceAccountIsolationAndResume(t *testing.T) {
	root := t.TempDir()
	a, err := Open(root, "qidian", "guest")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.PutBook(exampleBook()); err != nil {
		t.Fatal(err)
	}
	if err = a.PutContent("same", "one", []string{"一段文字", "第二段"}); err != nil {
		t.Fatal(err)
	}
	if err = a.SaveProgress(Progress{BookID: "same", ChapterID: "one", Paragraph: 1, Offset: 1, FontSize: 28}); err != nil {
		t.Fatal(err)
	}
	again, err := Open(root, "qidian", "guest")
	if err != nil {
		t.Fatal(err)
	}
	if p := again.Snapshot().Last; p == nil || p.Paragraph != 1 || p.Offset != 1 {
		t.Fatalf("lost resume: %+v", p)
	}
	for _, v := range [][2]string{{"changpei", "guest"}, {"miuread", "guest"}, {"qidian", "other"}} {
		b, err := Open(root, v[0], v[1])
		if err != nil {
			t.Fatal(err)
		}
		if len(b.Snapshot().Books) != 0 {
			t.Fatal("account/source leak")
		}
		if _, err = b.Content("same", "one"); err == nil {
			t.Fatal("cache leaked")
		}
	}
}
func TestCorruptStateNeverResetsLibrary(t *testing.T) {
	s, _ := Open(t.TempDir(), "qidian", "guest")
	s.PutBook(exampleBook())
	if err := os.WriteFile(filepath.Join(s.dir, "state.json"), []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(s.root, "qidian", "guest"); err == nil {
		t.Fatal("corruption accepted as empty library")
	}
}
func TestFailedSaveDoesNotChangeInMemoryState(t *testing.T) {
	s, _ := Open(t.TempDir(), "qidian", "guest")
	os.Mkdir(filepath.Join(s.dir, "state.json"), 0700)
	if err := s.PutBook(exampleBook()); err == nil {
		t.Fatal("expected write failure")
	}
	if len(s.Snapshot().Books) != 0 {
		t.Fatal("failed book exposed")
	}
}
func TestRejectInvalidProgressAndKeepContent(t *testing.T) {
	s, _ := Open(t.TempDir(), "qidian", "../../guest")
	s.PutBook(exampleBook())
	s.PutContent("same", "one", []string{"你好😀"})
	for _, p := range []Progress{{BookID: "missing", ChapterID: "one"}, {BookID: "same", ChapterID: "one", Paragraph: 2}, {BookID: "same", ChapterID: "one", Offset: -1}, {BookID: "same", ChapterID: "one", Offset: 5}} {
		if err := s.SaveProgress(p); err == nil {
			t.Fatalf("accepted %+v", p)
		}
	}
	got, err := s.Content("same", "one")
	if err != nil || len(got) != 1 || got[0] != "你好😀" {
		t.Fatalf("%v %v", got, err)
	}
}
func TestCacheFailureAndUnknownVersion(t *testing.T) {
	s, _ := Open(t.TempDir(), "qidian", "guest")
	s.PutBook(exampleBook())
	if err := s.PutContent("same", "one", nil); err == nil {
		t.Fatal("accepted empty chapter")
	}
	if _, err := s.Content("same", "one"); err == nil {
		t.Fatal("empty marked cached")
	}
	os.WriteFile(filepath.Join(s.dir, "state.json"), []byte(`{"version":99,"books":[]}`), 0600)
	if _, err := Open(s.root, "qidian", "guest"); err == nil {
		t.Fatal("unknown schema silently accepted")
	}
}

func TestIncomingAndReturnedBooksCannotMutateCommittedState(t *testing.T) {
	s, _ := Open(t.TempDir(), "qidian", "guest")
	original := exampleBook()
	s.PutBook(original)
	original.Chapters[0].Title = "changed outside store"
	got, _ := s.Book("same")
	got.Chapters[0].Title = "changed returned book"
	if s.Snapshot().Books[0].Chapters[0].Title != "第一章" {
		t.Fatal("external slice mutated committed state")
	}
}

func TestCatalogRefreshKeepsExistingPositionAndInvalidatesRemovedChapter(t *testing.T) {
	s, _ := Open(t.TempDir(), "qidian", "guest")
	book := exampleBook()
	s.PutBook(book)
	s.PutContent("same", "one", []string{"段落"})
	s.SaveProgress(Progress{BookID: "same", ChapterID: "one", Offset: 1})
	book.Chapters = append([]Chapter{{ID: "zero", Title: "序"}}, book.Chapters...)
	if e := s.PutBook(book); e != nil {
		t.Fatal(e)
	}
	if s.Snapshot().Last == nil || s.Snapshot().Last.Offset != 1 {
		t.Fatal("catalog reorder lost a still-valid anchor")
	}
	book.Chapters = []Chapter{{ID: "zero", Title: "序"}}
	if e := s.PutBook(book); e != nil {
		t.Fatal(e)
	}
	if s.Snapshot().Last != nil {
		t.Fatal("resume still points at a removed chapter")
	}
	if _, ok := s.Snapshot().Progress["same"]; ok {
		t.Fatal("book still has an unusable resume link")
	}
	if s.Snapshot().Detached["same"].Offset != 1 {
		t.Fatal("temporary catalog change destroyed recoverable bookmark")
	}
	if e := s.PutBook(exampleBook()); e != nil {
		t.Fatal(e)
	}
	if s.Snapshot().Progress["same"].Offset != 1 {
		t.Fatal("returning chapter did not restore bookmark")
	}
}
