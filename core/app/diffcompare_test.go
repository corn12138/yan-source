package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/keymap"
)

func TestWhitespaceHunks(t *testing.T) {
	base := []string{"a", "b c", "d", ""}
	cur := []string{"a  ", "bc", "", "  ", "d", ""}
	count := func(mode string) int { return len(whitespaceHunks(base, cur, mode)) }
	if hs := whitespaceHunks(base, cur, ""); len(hs) != 1 || hs[0].A != 0 {
		t.Fatalf("exact: %+v", hs)
	}
	if hs := whitespaceHunks(base, cur, "eol"); len(hs) != 1 || hs[0].A != 1 || hs[0].ALen != 1 {
		t.Fatalf("eol keeps the inner space change: %+v", hs)
	}
	if hs := whitespaceHunks(base, cur, "all"); len(hs) != 1 || hs[0].ALen != 0 || hs[0].BLen != 2 {
		t.Fatalf("all leaves the blank lines: %+v", hs)
	}
	if n := count("blank"); n != 0 {
		t.Fatalf("blank: %d hunks", n)
	}
}

// The whitespace filter changes what a diff shows, counts and stages, never the text: the line
// only spaces were added to is shown as unchanged, the blank line staged is the one shown.
func TestDiffWhitespaceFilter(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "one\ntwo\nthree\n"})
	p := filepath.Join(root, "a.txt")
	os.WriteFile(p, []byte("one  \ntwo\n\nthree\n"), 0o644)
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 1 })
	if err := e.openChange(p, false); err != nil {
		t.Fatal(err)
	}
	if txt := content(e); !strings.Contains(txt, "@@ -1,1 +1,1 @@") || !strings.Contains(txt, "@@ -3,0 +3,1 @@") {
		t.Fatalf("exact diff:\n%s", txt)
	}
	e.Run("yan.diff.whitespace")
	if e.ov == nil || len(e.ov.items) != 4 || e.ov.items[0].hint != "当前" {
		t.Fatalf("menu = %+v", e.ov)
	}
	e.Handle(key(keymap.KeyDown, 0))
	e.Handle(key(keymap.KeyEnter, 0)) // 忽略行尾空白
	if e.cfg.DiffWhitespace != "eol" {
		t.Fatalf("mode = %q (status %q)", e.cfg.DiffWhitespace, e.status)
	}
	txt := content(e)
	if strings.Contains(txt, "@@ -1,1") || !strings.Contains(txt, "@@ -3,0 +3,1 @@") || !strings.Contains(txt, "one  ") {
		t.Fatalf("eol diff:\n%s", txt)
	}
	// s on the hunk shown stages the blank line, not the line-end spaces it hides.
	at := strings.Index(txt, "@@ -3,0")
	e.cur().doc.buf.SetSelections(edit.Single(edit.Cursor(at)))
	if err := e.diffStage(e.cur(), e.cur().doc.buf.Snapshot().LineAt(at)); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return strings.Contains(gitOut(t, root, "diff", "--cached"), "@@") })
	if cached := gitOut(t, root, "diff", "--cached"); strings.Contains(cached, "+one  ") {
		t.Fatalf("staged the hidden change:\n%s", cached)
	}
	if err := e.setDiffWhitespace("blank"); err != nil {
		t.Fatal(err)
	}
	if txt := content(e); !strings.Contains(txt, "（和 HEAD 一样，没有改动；忽略空白和空行）") {
		t.Fatalf("blank diff:\n%s", txt)
	}
	if b, _ := os.ReadFile(p); string(b) != "one  \ntwo\n\nthree\n" {
		t.Fatalf("file changed: %q", b)
	}
}

// Any two files compare, in a repository or not, through the command's prompts: open
// documents count with their unsaved text, the removed lines open the left file, and nothing
// is staged.
func TestCompareTwoFiles(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"left.txt": "one\ntwo\nthree\n"})
	other := filepath.Join(t.TempDir(), "right.txt")
	os.WriteFile(other, []byte("one\nchanged\nthree\n"), 0o644)
	if err := e.OpenFile(filepath.Join(root, "left.txt")); err != nil {
		t.Fatal(err)
	}
	e.Run("yan.diff.compareFiles")
	if e.ov == nil || e.ov.title != "比较：左边（旧）的文件" {
		t.Fatalf("overlay = %+v", e.ov)
	}
	e.Handle(key(keymap.KeyEnter, 0)) // the file being edited is filled in
	if e.ov == nil || e.ov.title != "比较：右边（新）的文件" {
		t.Fatalf("second prompt = %+v (status %q)", e.ov, e.status)
	}
	typeText(e, other)
	e.Handle(key(keymap.KeyEnter, 0))
	if got := tabNames(e); len(got) != 2 || got[1] != "left.txt ↔ right.txt*" {
		t.Fatalf("tabs = %v (status %q)", got, e.status)
	}
	txt := content(e)
	if !strings.Contains(txt, "@@ -2,1 +2,1 @@\ntwo\nchanged") {
		t.Fatalf("compare:\n%s", txt)
	}
	diff := e.cur()
	if err := e.diffStage(diff, 1); err == nil || !strings.Contains(err.Error(), "两个文件的比较") {
		t.Fatalf("stage = %v", err)
	}
	// An unsaved edit of the left file shows at once.
	left := e.docOf(filepath.Join(root, "left.txt"))
	left.buf.SetSelections(edit.Single(edit.Cursor(0)))
	left.buf.Insert("zero\n", edit.OriginCommand)
	e.refreshDiffs()
	if txt := content(e); !strings.Contains(txt, "@@ -1,1 +1,0 @@\nzero") {
		t.Fatalf("after editing left:\n%s", txt)
	}
	// Enter on a removed line opens the left file there.
	at := strings.Index(content(e), "\ntwo\n") + 1
	diff.doc.buf.SetSelections(edit.Single(edit.Cursor(at)))
	e.Handle(key(keymap.KeyEnter, 0))
	if p := e.cur().doc.buf.Path(); p != filepath.Join(root, "left.txt") {
		t.Fatalf("opened %q", p)
	}
	if s := e.cur().doc.buf.Snapshot(); s.LineAt(e.cur().doc.buf.Selections().Main().Head) != 2 {
		t.Fatal("should land on the removed line")
	}
	if err := e.CompareFiles(other, other); err == nil {
		t.Fatal("a file compared with itself")
	}
	if err := e.CompareFiles(root, other); err == nil {
		t.Fatal("a folder compared")
	}
}

// The explorer chooses a file for comparison, then compares another with it.
func TestExplorerCompareWithSelected(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	e.Run("workbench.view.explorer")
	selectRel := func(rel string) {
		t.Helper()
		e.refreshVisible()
		for i, n := range e.ws.visible {
			if n.Rel == rel {
				e.SelectTree(i)
				return
			}
		}
		t.Fatalf("no %s in the tree", rel)
	}
	if e.CanCompareWithSelected() {
		t.Fatal("nothing chosen yet")
	}
	selectRel("a.txt")
	e.Run("selectForCompare")
	selectRel("b.txt")
	e.Run("compareFiles")
	if got := tabNames(e); len(got) != 1 || got[0] != "a.txt ↔ b.txt*" {
		t.Fatalf("tabs = %v (status %q)", got, e.status)
	}
	if dt := e.cur().diff; dt.peer != filepath.Join(root, "a.txt") || dt.path != filepath.Join(root, "b.txt") {
		t.Fatalf("sides = %q, %q", dt.peer, dt.path)
	}
}
