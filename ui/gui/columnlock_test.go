package gui

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// A middle click closes a background tab as it is let go of, without showing it first.
func TestMiddleClickClosesBackgroundTab(t *testing.T) {
	h := newHarness(t, sample, Options{})
	h.open("main.go")
	h.open("README.md")
	other := filepath.Join(h.root, "main.go")
	box := ui.Rect{W: 80, H: 30}
	if !h.f.tabMiddle(other, ui.InputEvent{Kind: ui.InputPointerDown, Button: middleButton, X: 10, Y: 10}, box) {
		t.Fatal("the press was not taken")
	}
	h.settle()
	col := h.f.frame.Columns[0]
	if len(col.Tabs) != 2 || col.Tabs[col.Active].Name != "README.md" {
		t.Fatalf("the press changed the tabs: %+v", col.Tabs)
	}
	h.f.tabMiddle(other, ui.InputEvent{Kind: ui.InputPointerUp, Button: middleButton, X: 10, Y: 10}, box)
	h.settle()
	col = h.f.frame.Columns[0]
	if len(col.Tabs) != 1 || col.Tabs[0].Name != "README.md" || h.f.middle != nil {
		t.Fatalf("after release: %+v", col.Tabs)
	}
	// Released off the tab, nothing closes.
	h.f.tabMiddle(filepath.Join(h.root, "README.md"), ui.InputEvent{Kind: ui.InputPointerDown, Button: middleButton}, box)
	h.f.tabMiddle(filepath.Join(h.root, "README.md"), ui.InputEvent{Kind: ui.InputPointerUp, Button: middleButton, X: 200, Y: 10}, box)
	h.settle()
	if len(h.f.frame.Columns[0].Tabs) != 1 {
		t.Fatal("a release off the tab closed it")
	}
}

// The column header's lock button locks the column; files then open in another column.
func TestLockButtonRoutesFiles(t *testing.T) {
	h := newHarness(t, sample, Options{})
	h.open("main.go")
	if err := h.tt.Click("锁定这一列：打开的文件和新终端去别的列"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if len(h.f.frame.Columns) != 2 || !h.f.frame.Columns[1].Locked || h.f.frame.Columns[0].Locked {
		t.Fatalf("columns after lock: %d", len(h.f.frame.Columns))
	}
	h.ed.FocusColumn(1)
	h.open("README.md")
	if c := h.f.frame.Columns; len(c[0].Tabs) != 1 || c[0].Tabs[0].Name != "README.md" || len(c[1].Tabs) != 1 {
		t.Fatalf("README went to the locked column")
	}
	h.shot("column-lock")
}

// Empty columns beside a file show a short note instead of another welcome page, and their header
// unlocks or closes them; with no file open, only one column shows the welcome page.
func TestEmptyColumnsCanBeClosed(t *testing.T) {
	h := newHarness(t, sample, Options{})
	h.open("main.go")
	// Lock an empty column on each side of the file, as in a left/right reference layout.
	h.ed.Run("workbench.action.splitEditor")
	h.ed.Run("workbench.action.splitEditor")
	for _, ci := range []int{2, 0} {
		h.ed.FocusColumn(ci)
		h.ed.Run("view.lockColumn")
		h.ed.Run("workbench.action.closeActiveEditor")
	}
	h.ed.FocusColumn(1)
	h.settle()
	cols := h.f.frame.Columns
	if len(cols) != 3 || !cols[0].Locked || !cols[2].Locked || len(cols[0].Tabs)+len(cols[2].Tabs) != 0 || len(cols[1].Tabs) != 1 {
		t.Fatalf("setup: %d columns", len(cols))
	}
	// Only one column carries the welcome page; locked empty columns can be closed.
	welcomes := 0
	for _, c := range cols {
		if len(c.Pane.Hints) > 0 {
			welcomes++
		}
	}
	if welcomes != 0 {
		t.Fatalf("%d welcome pages beside an open file", welcomes)
	}
	if !cols[0].Closable || !cols[2].Closable || cols[1].Closable {
		t.Fatal("only the empty columns may be closed")
	}
	h.shot("empty-columns")
	// Closing the file leaves one welcome page; the header buttons close the empty columns.
	h.ed.Run("workbench.action.closeActiveEditor")
	h.settle()
	for range 2 {
		if err := h.tt.Click("关闭这一列"); err != nil {
			t.Fatal(err)
		}
		h.settle()
	}
	if cols := h.f.frame.Columns; len(cols) != 1 || cols[0].Locked || len(cols[0].Pane.Hints) == 0 {
		t.Fatalf("after closing: %d columns", len(cols))
	}
	h.shot("empty-columns-closed")
}
