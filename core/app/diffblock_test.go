package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/text"
)

func TestReplaceLines(t *testing.T) {
	for _, c := range []struct {
		src   string
		at, n int
		lines []string
		want  string
	}{
		{"a\nb\nc\n", 1, 1, []string{"B"}, "a\nB\nc\n"},
		{"a\nb\nc\n", 1, 1, nil, "a\nc\n"},
		{"a\nb\nc", 2, 1, nil, "a\nb"}, // the last line, without a line break
		{"a\nb\nc\n", 1, 0, []string{"x", "y"}, "a\nx\ny\nb\nc\n"},
		{"a\nb\nc\n", 3, 0, []string{"z"}, "a\nb\nc\nz\n"}, // after the last line break
		{"a\nb", 2, 0, []string{"z"}, "a\nb\nz"},
		{"a\r\nb\r\n", 1, 1, []string{"B", "C"}, "a\r\nB\r\nC\r\n"},
	} {
		m := text.Meta{}
		if strings.Contains(c.src, "\r\n") {
			m.EOL = text.EOLCRLF
		}
		s := text.FromString(c.src, m)
		ch := replaceLines(s, c.at, c.n, c.lines)
		got := c.src[:ch.Start] + ch.Text + c.src[ch.End:]
		if got != c.want {
			t.Errorf("%q at %d n %d %q: %q, want %q", c.src, c.at, c.n, c.lines, got, c.want)
		}
	}
}

// r on a change of a one-file diff puts HEAD's lines back in the file's document, in a tab
// opened behind the diff, as one step that undo and redo in the diff take back and redo; the
// other change stays; a on a change appends HEAD's lines after it.
func TestDiffBlockRevertAndAppend(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\n"})
	p := filepath.Join(root, "a.txt")
	os.WriteFile(p, []byte("ONE\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nNINE\n"), 0o644)
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 1 })
	e.openChange(p, false)
	diff := e.cur()
	at := func(s string) {
		t.Helper()
		i := strings.Index(content(e), s)
		if i < 0 {
			t.Fatalf("no %q in\n%s", s, content(e))
		}
		diff.doc.buf.SetSelections(edit.Single(edit.Cursor(i)))
	}
	at("NINE")
	press(e, 'r')
	d := e.docOf(p)
	if d == nil || docText(d) != "ONE\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\n" || !d.buf.Dirty() {
		t.Fatalf("after revert: %v (status %q)", d, e.status)
	}
	if e.cur() != diff || len(d.tabs) != 1 {
		t.Fatalf("the diff should stay in front with the file's tab behind it: %v", tabNames(e))
	}
	if txt := content(e); strings.Contains(txt, "NINE") || !strings.Contains(txt, "ONE") {
		t.Fatalf("diff after revert:\n%s", txt)
	}
	e.Run("undo")
	if got := docText(d); !strings.HasSuffix(got, "NINE\n") || !strings.Contains(content(e), "NINE") {
		t.Fatalf("undo: %q", got)
	}
	e.Run("redo")
	if got := docText(d); !strings.HasSuffix(got, "nine\n") {
		t.Fatalf("redo: %q", got)
	}
	at("ONE")
	press(e, 'a')
	if got := docText(d); !strings.HasPrefix(got, "ONE\none\ntwo\n") {
		t.Fatalf("append: %q (status %q)", got, e.status)
	}
	e.Run("undo")
	if got := docText(d); !strings.HasPrefix(got, "ONE\ntwo\n") {
		t.Fatalf("undo append: %q", got)
	}
	if b, _ := os.ReadFile(p); !strings.HasPrefix(string(b), "ONE\n") || !strings.HasSuffix(string(b), "NINE\n") {
		t.Fatalf("the file on disk changed before saving: %q", b)
	}
}

// In the tab of all the changes only the changes not staged revert; staged ones refuse.
func TestAllChangesBlockRevert(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "one\ntwo\n", "b.txt": "x\ny\n"})
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\nTWO\n"), 0o644)
	os.WriteFile(filepath.Join(root, "b.txt"), []byte("X\ny\n"), 0o644)
	gitOut(t, root, "add", "b.txt")
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 2 })
	e.openAllChanges()
	pump(t, e, func() bool { return len(diffHeads(e.cur().diff)) == 2 })
	diff := e.cur()
	put := func(s string) {
		i := strings.Index(content(e), s)
		diff.doc.buf.SetSelections(edit.Single(edit.Cursor(i)))
	}
	put("\nX\n")
	diff.doc.buf.SetSelections(edit.Single(edit.Cursor(strings.Index(content(e), "\nX\n") + 1)))
	press(e, 'r')
	if !strings.Contains(e.status, "已暂存的先取消暂存") {
		t.Fatalf("staged: %q", e.status)
	}
	put("TWO")
	press(e, 'r')
	if d := e.docOf(filepath.Join(root, "a.txt")); d == nil || docText(d) != "one\ntwo\n" {
		t.Fatalf("unstaged revert: %v (%q)", d, e.status)
	}
	if txt := content(e); strings.Contains(txt, "TWO") {
		t.Fatalf("diff not read again:\n%s", txt)
	}
}

// A comparison takes the left block into the right file with r, and the right into the left
// with R.
func TestCompareAcceptLeftAndRight(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"left.txt": "one\ntwo\nthree\n", "right.txt": "one\nchanged\nthree\n"})
	left, right := filepath.Join(root, "left.txt"), filepath.Join(root, "right.txt")
	if err := e.CompareFiles(left, right); err != nil {
		t.Fatal(err)
	}
	diff := e.cur()
	i := strings.Index(content(e), "changed")
	diff.doc.buf.SetSelections(edit.Single(edit.Cursor(i)))
	press(e, 'r')
	if d := e.docOf(right); d == nil || docText(d) != "one\ntwo\nthree\n" {
		t.Fatalf("accept left: %v (%q)", d, e.status)
	}
	if txt := content(e); !strings.HasPrefix(txt, "（两个文件的文字一样）") {
		t.Fatalf("diff:\n%s", txt)
	}
	e.Run("undo")
	diff.doc.buf.SetSelections(edit.Single(edit.Cursor(strings.Index(content(e), "changed"))))
	press(e, 'R')
	if d := e.docOf(left); d == nil || docText(d) != "one\nchanged\nthree\n" {
		t.Fatalf("accept right: %v (%q)", d, e.status)
	}
}

// A commit's changes, and lines outside a change, take no block action.
func TestDiffBlockRefusals(t *testing.T) {
	e, _ := twoChangedFiles(t)
	e.openAllChanges()
	pump(t, e, func() bool { return len(diffHeads(e.cur().diff)) == 3 })
	e.cur().doc.buf.SetSelections(edit.Single(edit.Cursor(0)))
	press(e, 'r')
	if e.status != "光标放在一处改动里" {
		t.Fatalf("outside: %q", e.status)
	}
	press(e, 'R')
	if !strings.Contains(e.status, "只有两个文件的比较") && e.status != "光标放在一处改动里" {
		t.Fatalf("accept right: %q", e.status)
	}
}

func docText(d *document) string { s := d.buf.Snapshot(); return s.String(0, s.Len()) }

// Undo in a diff with nothing done there leaves the diff as it is (it once undid the diff's own
// reading and emptied it).
func TestUndoInADiffKeepsItsText(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "one\ntwo\n"})
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\nTWO\n"), 0o644)
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 1 })
	e.openChange(filepath.Join(root, "a.txt"), false)
	before := content(e)
	e.Run("undo")
	if content(e) != before || e.status != "改动视图里没有可撤销的操作" {
		t.Fatalf("undo: %q, status %q", content(e), e.status)
	}
}
