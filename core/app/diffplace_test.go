package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/view"
)

// Unfolding a gap above the view, or changing the layout, keeps the line at the top of the view.
func TestDiffKeepsItsPlace(t *testing.T) {
	var lines []string
	for i := 1; i <= 80; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	root := gitRepo(t, map[string]string{"a.txt": strings.Join(lines, "\n") + "\n"})
	lines[4], lines[60] = "five", "sixty-one"
	os.WriteFile(filepath.Join(root, "a.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 1 })
	e.cfg.DiffLayout = "inline"
	e.openChange(filepath.Join(root, "a.txt"), false)
	tb := e.cur()
	dt := tb.diff
	top := -1
	for i, l := range dt.lines {
		if l.kind == diffAdded && l.line == 60 {
			top = i
		}
	}
	tb.scrollY = top
	dt.expanded[1] = true // the gap between the changes, above the view
	e.refreshDiff(tb.doc, dt)
	if l := dt.lines[tb.scrollY]; l.kind != diffAdded || l.line != 60 {
		t.Fatalf("top is now %+v", l)
	}
	e.Run("toggle.diff.renderSideBySide") // inline → auto: still the same line on top
	if l := dt.lines[tb.scrollY]; l.line != 60 {
		t.Fatalf("after the layout changed, top is %+v", l)
	}
}

// All Changes offers restoring a deleted file, removing one Git does not track, and discarding
// a changed one, each after asking.
func TestDiffWholeFileActions(t *testing.T) {
	root := gitRepo(t, map[string]string{"gone.txt": "g\n", "kept.txt": "k\n"})
	os.Remove(filepath.Join(root, "gone.txt"))
	os.WriteFile(filepath.Join(root, "kept.txt"), []byte("K\n"), 0o644)
	os.WriteFile(filepath.Join(root, "new.txt"), []byte("n\n"), 0o644)
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 3 })
	e.openAllChanges()
	tb := e.cur()
	pump(t, e, func() bool { return !tb.diff.loading && len(tb.diff.secs) == 3 })
	var f view.Frame
	e.SetPaneSize(0, view.Size{W: 120, H: 40})
	e.Frame(&f)
	kinds := map[string]view.DiffBlock{}
	lineOf := map[string]int{}
	for _, r := range f.Columns[0].Pane.Rows {
		for _, n := range []string{"gone.txt", "kept.txt", "new.txt"} {
			if r.Block != view.DiffBlockNone && strings.Contains(string(r.Text), " "+n+"  ") {
				kinds[n], lineOf[n] = r.Block, int(r.Line)
			}
		}
	}
	if kinds["gone.txt"] != view.DiffBlockFileDeleted || kinds["new.txt"] != view.DiffBlockFileUntracked || kinds["kept.txt"] != view.DiffBlockFile {
		t.Fatalf("kinds %v", kinds)
	}
	e.DiffBlockAt(0, lineOf["gone.txt"], "discardFile")
	chooseItem(t, e, "恢复文件")
	pump(t, e, func() bool { _, err := os.Stat(filepath.Join(root, "gone.txt")); return err == nil })
}
