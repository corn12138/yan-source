package edit

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/corn12138/yan-source/core/text"
)

func TestEOLConversionHistoryAndSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	original := "中文\r\nsecond\nlast\r"
	os.WriteFile(path, append([]byte{239, 187, 191}, []byte(original)...), 0644)
	s, err := text.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	b := NewBuffer(s)
	b.SetSelections(Single(Cursor(s.LineStart(1) + 3)))
	b.SetEOL(text.EOLCRLF)
	want := "中文\r\nsecond\r\nlast\r"
	if got := b.Snapshot().String(0, b.Snapshot().Len()); got != want {
		t.Fatalf("converted %q", got)
	}
	if !b.Dirty() || b.Snapshot().Meta().MixedEOL {
		t.Fatal("not a normalized dirty revision")
	}
	if b.Selections().Main().Head != b.Snapshot().LineStart(1)+3 {
		t.Fatal("caret moved")
	}
	b.SetEOL(text.EOLCRLF)
	if _, ok := b.Undo(); !ok {
		t.Fatal("no undo")
	}
	if b.Dirty() || b.Snapshot().String(0, b.Snapshot().Len()) != original || !b.Snapshot().Meta().MixedEOL {
		t.Fatal("undo did not restore bytes and metadata")
	}
	b.Redo()
	if err := b.Save(); err != nil {
		t.Fatal(err)
	}
	bytes, _ := os.ReadFile(path)
	if string(bytes) != "\xef\xbb\xbf"+want {
		t.Fatalf("saved %q", bytes)
	}
	b.SetEOL(text.EOLLF)
	os.WriteFile(path, []byte("external content with a different length"), 0644)
	if err := b.Save(); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict: %v", err)
	}
	bytes, _ = os.ReadFile(path)
	if string(bytes) != "external content with a different length" {
		t.Fatal("external content overwritten")
	}
}

func TestEmptyAndReadOnlyEOL(t *testing.T) {
	b := NewBuffer(text.FromString("", text.Meta{}))
	b.SetEOL(text.EOLCRLF)
	if !b.Dirty() || b.Snapshot().Meta().EOL != text.EOLCRLF {
		t.Fatal("empty preference missing")
	}
	b.Undo()
	if b.Dirty() || b.Snapshot().Meta().EOL != text.EOLLF {
		t.Fatal("empty undo failed")
	}
	b = NewBuffer(text.FromString("x\n", text.Meta{ReadOnly: true}))
	b.SetEOL(text.EOLCRLF)
	if b.Dirty() {
		t.Fatal("changed read-only file")
	}
}

func TestReloadUpdatesEOL(t *testing.T) {
	b := NewBuffer(text.FromString("x\n", text.Meta{}))
	b.Reload(text.FromString("x\r\n", text.Meta{EOL: text.EOLCRLF}))
	if b.Snapshot().Meta().EOL != text.EOLCRLF || b.Dirty() {
		t.Fatal("reload metadata stale")
	}
	b.Undo()
	if b.Snapshot().Meta().EOL != text.EOLLF {
		t.Fatal("reload undo metadata stale")
	}
}
