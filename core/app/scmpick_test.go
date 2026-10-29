package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/view"
)

func threeChanged(t *testing.T) (*Editor, string) {
	t.Helper()
	root := gitRepo(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n", "c.txt": "c\n", "d.txt": "d\n"})
	for _, n := range []string{"a", "b", "c"} {
		os.WriteFile(filepath.Join(root, n+".txt"), []byte(strings.ToUpper(n)+"\n"), 0o644)
	}
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 3 })
	e.Run("workbench.view.scm")
	return e, root
}

// Shift chooses a run of files and the modifier one more or one less; space stages just those,
// and Select All takes every file.
func TestSCMMultiSelect(t *testing.T) {
	e, root := threeChanged(t)
	e.SCMClick(0, false)
	e.SCMPick(1, true, false)
	if got := len(e.scmPicked()); got != 2 {
		t.Fatalf("shift: %d picked", got)
	}
	e.SCMPick(1, false, true) // b out again
	e.SCMPick(2, false, true) // c in
	picked := e.scmPicked()
	if len(picked) != 2 || filepath.Base(picked[0].Path) != "a.txt" || filepath.Base(picked[1].Path) != "c.txt" {
		t.Fatalf("toggle: %+v", picked)
	}
	var f view.Frame
	e.Frame(&f)
	rows := scmFlat(f.Sidebar.SCM.Groups)
	if !rows[0].Picked || rows[1].Picked || !rows[2].Picked {
		t.Fatalf("frame picked: %+v", rows)
	}
	e.Run("list.toggleExpand") // space: stage the chosen ones
	pump(t, e, func() bool { return e.stagedCount() == 2 })
	if out := gitOut(t, root, "diff", "--cached", "--name-only"); out != "a.txt\nc.txt" {
		t.Fatalf("staged %q", out)
	}
	e.SCMClick(0, false) // a plain click chooses one again
	if e.scmPicked() != nil {
		t.Fatal("a click keeps the choice")
	}
	e.Run("list.selectAll")
	if got := len(e.scmPicked()); got != 3 {
		t.Fatalf("select all: %d", got)
	}
	e.Run("list.toggleExpand") // b staged, a and c unstaged
	pump(t, e, func() bool { return gitOut(t, root, "diff", "--cached", "--name-only") == "b.txt" })
}

// Delete on the chosen files asks once more, then discards only them.
func TestSCMDiscardPicked(t *testing.T) {
	e, root := threeChanged(t)
	e.SCMClick(0, false)
	e.SCMPick(1, true, false)
	e.Run("list.delete")
	if !strings.Contains(e.status, "放弃选中的 2 个文件") {
		t.Fatalf("first press: %q", e.status)
	}
	e.Run("list.delete")
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 1 })
	if b, _ := os.ReadFile(filepath.Join(root, "c.txt")); string(b) != "C\n" {
		t.Fatalf("c.txt %q", b)
	}
}

// With All Changes shown, a click in Source Control scrolls it to the file instead of opening
// another tab, and scrolling it selects the file at the top in Source Control.
func TestSCMRevealsInAllChanges(t *testing.T) {
	e, _ := threeChanged(t)
	e.SetPaneSize(0, view.Size{W: 100, H: 6})
	e.openAllChanges()
	pump(t, e, func() bool { return len(diffHeads(e.cur().diff)) == 3 })
	tb := e.cur()
	e.SCMClick(2, false) // c.txt
	if got := tabNames(e); len(got) != 1 {
		t.Fatalf("tabs %v", got)
	}
	if l := tb.diff.lines[tb.scrollY]; l.kind != diffFile || filepath.Base(tb.diff.secs[l.sec].path) != "c.txt" {
		t.Fatalf("top line %d", tb.scrollY)
	}
	e.area = areaEditor
	tb.scrollY = 1 // inside a.txt
	var f view.Frame
	e.Frame(&f)
	if e.scm.sel != 0 {
		t.Fatalf("selection followed to %d", e.scm.sel)
	}
}
