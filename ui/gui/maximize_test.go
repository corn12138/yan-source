package gui

import (
	"os"
	"testing"
)

// A merge beside a terminal maximizes from its own button: the terminal's column is hidden,
// its shell keeps running in the same session, and restoring brings the same layout back.
func TestMaximizeMergeBesideATerminal(t *testing.T) {
	h := newHarnessAt(t, mergeFixture(t), Options{})
	h.tt.SetSize(1100, 720)
	openMerge(t, h)
	h.ed.Run("workbench.action.splitEditor")
	h.settle()
	h.ed.Run("workbench.action.createTerminalEditor")
	h.settle()
	h.ed.FocusColumn(1)
	h.settle()
	n := len(h.f.frame.Columns)
	var id int
	for _, col := range h.f.frame.Columns {
		for _, tb := range col.Tabs {
			if tb.Terminal != 0 {
				id = tb.Terminal
			}
		}
	}
	s := h.f.tabTerms[id]
	if id == 0 || s == nil {
		t.Fatalf("no editor terminal: %d columns", n)
	}
	if err := h.tt.Click("最大化编辑区"); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		h.settle()
	}
	fr := &h.f.frame
	if !fr.Maximized || fr.Sidebar.Visible || len(fr.Columns) != n {
		t.Fatalf("maximized=%v sidebar=%v columns %d", fr.Maximized, fr.Sidebar.Visible, len(fr.Columns))
	}
	shown := 0
	for i, col := range fr.Columns {
		if !col.Hidden {
			shown++
			if w := col.Pane.Size.W; w < 38 { // about 300 pixels of code
				t.Errorf("merge column %d is %d characters wide", i, w)
			}
		}
	}
	if shown != 3 || h.f.tabTerms[id] != s || s.closed {
		t.Fatalf("%d columns shown; terminal session kept=%v closed=%v", shown, h.f.tabTerms[id] == s, s.closed)
	}
	h.shot("maximize-merge")
	if err := h.tt.Click("还原布局"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if fr.Maximized || !fr.Sidebar.Visible || h.f.tabTerms[id] != s || s.closed {
		t.Fatalf("restore: maximized=%v sidebar=%v session kept=%v", fr.Maximized, fr.Sidebar.Visible, h.f.tabTerms[id] == s)
	}
	if os.Getenv("SHOTS") != "" {
		h.shot("maximize-merge-restored")
	}
}
