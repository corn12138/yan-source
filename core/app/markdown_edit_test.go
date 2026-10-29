package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func markdownPreviewEditor(t *testing.T, src string) (*Editor, string) {
	t.Helper()
	e, root := newFolderEditor(t, map[string]string{"doc.md": src})
	e.profile.GUI = true
	path := filepath.Join(root, "doc.md")
	if err := e.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	if err := e.previewToSide(); err != nil {
		t.Fatal(err)
	}
	return e, path
}

func TestEditMarkdownLinesInThePreview(t *testing.T) {
	e, path := markdownPreviewEditor(t, "# Title\n\nfirst para\n\n- a\n- b\n")
	doc := e.cols[1].tab().doc
	ver := doc.buf.Snapshot().Version()
	ver2, err := e.EditMarkdownLines(1, path, ver, 2, 3, "first paragraph,\nnow two lines")
	if err != nil {
		t.Fatal(err)
	}
	s := doc.buf.Snapshot()
	if got := s.String(0, s.Len()); got != "# Title\n\nfirst paragraph,\nnow two lines\n\n- a\n- b\n" {
		t.Fatalf("after edit %q", got)
	}
	if !doc.buf.Dirty() || ver2 == ver {
		t.Fatal("an edit in the preview must make the document dirty and move its version")
	}
	// A new block before line 4 (the list), and the last line edited without a final break.
	ver3, _ := e.EditMarkdownLines(1, path, ver2, 5, 5, "> quoted\n")
	ver4, _ := e.EditMarkdownLines(1, path, ver3, 8, 9, "- bee")
	s = doc.buf.Snapshot()
	if got := s.String(0, s.Len()); got != "# Title\n\nfirst paragraph,\nnow two lines\n\n> quoted\n\n- a\n- bee\n" {
		t.Fatalf("after insert %q", got)
	}
	// A preview built from older text cannot move lines that have since shifted.
	if _, err := e.EditMarkdownLines(1, path, ver2, 0, 1, "# Wrong"); err == nil {
		t.Fatal("a stale preview edit was applied")
	}
	if _, err := e.EditMarkdownLines(0, path, ver4, 0, 1, "# Source column"); err == nil {
		t.Fatal("an edit through a column that does not preview the file was applied")
	}
	// The source column shows the same text, and undo walks the preview's edits back.
	e.FocusColumn(0)
	for range 3 {
		e.Run("undo")
	}
	s = doc.buf.Snapshot()
	if got := s.String(0, s.Len()); got != "# Title\n\nfirst para\n\n- a\n- b\n" {
		t.Fatalf("after undo %q", got)
	}
}

func TestEditMarkdownLinesKeepsCRLF(t *testing.T) {
	e, path := markdownPreviewEditor(t, "a\r\n\r\nb\r\n")
	doc := e.cols[1].tab().doc
	if _, err := e.EditMarkdownLines(1, path, doc.buf.Snapshot().Version(), 2, 3, "b1\nb2"); err != nil {
		t.Fatal(err)
	}
	e.FocusColumn(0)
	e.Run("workbench.action.files.save")
	got, _ := os.ReadFile(path)
	if string(got) != "a\r\n\r\nb1\r\nb2\r\n" {
		t.Fatalf("saved %q", got)
	}
}

func TestSaveMarkdownImageNeverReplaces(t *testing.T) {
	e, path := markdownPreviewEditor(t, "# Doc\n")
	a, err := e.SaveMarkdownImage(path, []byte("png-1"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.SaveMarkdownImage(path, []byte("png-2"))
	if err != nil || a == b || !strings.HasPrefix(a, "assets/doc-") || !strings.HasSuffix(b, ".png") {
		t.Fatalf("links %q %q %v", a, b, err)
	}
	got, _ := os.ReadFile(filepath.Join(filepath.Dir(path), filepath.FromSlash(a)))
	if string(got) != "png-1" {
		t.Fatalf("first image %q", got)
	}
	if _, err := e.SaveMarkdownImage(path, nil); err == nil {
		t.Fatal("an empty clipboard must say so")
	}
}
