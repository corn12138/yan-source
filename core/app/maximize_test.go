package app

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/view"
)

func frameOf(e *Editor) *view.Frame {
	var f view.Frame
	e.Frame(&f)
	return &f
}

// Maximizing draws one column alone and restores the exact layout: widths, locks, carets,
// scroll positions and the sidebar are never touched in between.
func TestMaximizeRestoresTheLayout(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": strings.Repeat("line\n", 200), "b.txt": "b\n"})
	if err := e.openPath(filepath.Join(root, "a.txt")); err != nil {
		t.Fatal(err)
	}
	if err := e.splitRight(); err != nil {
		t.Fatal(err)
	}
	if err := e.openPath(filepath.Join(root, "b.txt")); err != nil {
		t.Fatal(err)
	}
	e.toggleColumnLock()
	e.FocusColumn(0)
	e.cur().doc.buf.SetSelections(edit.Single(edit.Range{Anchor: 10, Head: 30, GoalX: -1}))
	e.cur().scrollY = 120
	e.ResizeColumns(1, 0.1, 0.05)
	weights := []float64{e.cols[0].weight, e.cols[1].weight}
	hiddenTabs := len(e.cols[1].tabs)

	e.Run(cmdMaximize)
	f := frameOf(e)
	if !f.Maximized || f.Columns[0].Hidden || !f.Columns[1].Hidden || f.Sidebar.Visible {
		t.Fatalf("maximized=%v hidden=%v,%v sidebar=%v", f.Maximized, f.Columns[0].Hidden, f.Columns[1].Hidden, f.Sidebar.Visible)
	}
	if len(f.Columns[1].Tabs) != hiddenTabs {
		t.Fatal("the hidden column lost its tabs")
	}
	e.Run(cmdMaximize)
	f = frameOf(e)
	if f.Maximized || f.Columns[1].Hidden || !f.Sidebar.Visible || !e.sidebar {
		t.Fatalf("not restored: maximized=%v sidebar=%v", f.Maximized, f.Sidebar.Visible)
	}
	if got := []float64{e.cols[0].weight, e.cols[1].weight}; !slices.Equal(got, weights) || !e.cols[1].locked {
		t.Fatalf("widths %v (were %v), locked %v", got, weights, e.cols[1].locked)
	}
	if m := e.sels(e.cur()).Main(); m.Anchor != 10 || m.Head != 30 || e.cur().scrollY != 120 {
		t.Fatalf("caret %+v scroll %d", m, e.cur().scrollY)
	}
}

// Going to a column that is not drawn, or to the sidebar, ends the maximize instead of working
// out of sight.
func TestMaximizeEndsWhenFocusLeaves(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a\n"})
	e.openPath(filepath.Join(root, "a.txt"))
	e.splitRight()
	e.FocusColumn(0)
	e.Run(cmdMaximize)
	e.FocusColumn(1)
	if f := frameOf(e); f.Maximized || !strings.Contains(e.status, "打开了别的列") {
		t.Fatalf("maximized=%v status %q", f.Maximized, e.status)
	}
	e.FocusColumn(0)
	e.Run(cmdMaximize)
	e.Run("workbench.view.explorer")
	if f := frameOf(e); f.Maximized || !f.Sidebar.Visible {
		t.Fatal("showing the Explorer kept the editor maximized")
	}
}

// A merge maximizes as its three columns; a terminal beside it is hidden, not closed, and the
// merge ending restores the layout.
func TestMaximizeMergeKeepsTheTerminal(t *testing.T) {
	e, root := mergeConflict(t, map[string][3]string{"shared.txt": {"top\na\nm\nz\n", "TOP\na\nours\nz\n", "top\na\nside\nz\n"}})
	if err := e.openMerge(filepath.Join(root, "shared.txt")); err != nil {
		t.Fatal(err)
	}
	term := newColumn()
	e.cols = append(e.cols, term)
	id := e.newTerminalTab(term, 0, root, "")
	e.FocusColumn(1)
	e.Run(cmdMaximize)
	f := frameOf(e)
	if !f.Maximized || f.Columns[0].Hidden || f.Columns[1].Hidden || f.Columns[2].Hidden || !f.Columns[3].Hidden {
		t.Fatalf("hidden %v %v %v %v", f.Columns[0].Hidden, f.Columns[1].Hidden, f.Columns[2].Hidden, f.Columns[3].Hidden)
	}
	if !f.Columns[0].Merge || !f.Columns[1].Merge || f.Columns[3].Merge {
		t.Fatal("merge columns not marked")
	}
	if tabs := f.Columns[3].Tabs; len(tabs) != 1 || tabs[0].Terminal != id {
		t.Fatalf("the terminal's tab left the frame: %+v", tabs)
	}
	if err := e.mergeClose(); err != nil {
		t.Fatal(err)
	}
	if f := frameOf(e); f.Maximized {
		t.Fatal("still maximized after the merge closed")
	}
	if !slices.ContainsFunc(e.cols, func(c *column) bool { return c == term }) || term.tabs[0].term == nil {
		t.Fatal("the terminal column did not come back")
	}
}

// Showing the Explorer switches the sidebar to it from another view (it used to stay on Source
// Control and only take the focus).
func TestShowExplorerFromAnotherView(t *testing.T) {
	e, _ := newFolderEditor(t, map[string]string{"a.txt": "a\n"})
	e.Run("workbench.view.scm")
	e.Run("workbench.view.explorer")
	if f := frameOf(e); f.Sidebar.View != view.SideExplorer || f.Focus != view.FocusSidebar {
		t.Fatalf("view %v focus %v", f.Sidebar.View, f.Focus)
	}
}
