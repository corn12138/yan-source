package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/view"
)

// i on a change opens the working tree's file beside the diff at the change's line; what is
// typed there shows in the diff, and undo is the file's.
func TestDiffEditBeside(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "one\ntwo\nthree\n"})
	p := filepath.Join(root, "a.txt")
	os.WriteFile(p, []byte("one\nTWO\nthree\n"), 0o644)
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 1 })
	e.openChange(p, false)
	diff := e.cur()
	diff.doc.buf.SetSelections(edit.Single(edit.Cursor(strings.Index(content(e), "\ntwo\n") + 1))) // the removed line
	press(e, 'i')
	if len(e.cols) != 2 || e.cur().doc.buf.Path() != p {
		t.Fatalf("columns %d, editing %q (%q)", len(e.cols), e.cur().doc.buf.Path(), e.status)
	}
	if s := e.cur().doc.buf.Snapshot(); s.LineAt(e.cur().doc.buf.Selections().Main().Head) != 1 {
		t.Fatal("not at the change's line")
	}
	e.Run("cursorEnd")
	typeText(e, "!")
	var f view.Frame
	e.Frame(&f)
	if !strings.Contains(tabText(diff), "TWO!") {
		t.Fatalf("diff did not follow:\n%s", tabText(diff))
	}
	e.Run("undo")
	e.Frame(&f)
	if strings.Contains(tabText(diff), "TWO!") {
		t.Fatalf("undo did not reach the diff:\n%s", tabText(diff))
	}
	// A commit's diff stays read only.
	e.FocusColumn(0)
	diff.diff.fixed = true
	press(e, 'i')
	if !strings.Contains(e.status, "只读") {
		t.Fatalf("status %q", e.status)
	}
}
