package gui

import (
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/view"
)

// A search result dragged onto a column's edge opens in a new column there with its match
// selected.
func TestDragSearchResultToColumnEdge(t *testing.T) {
	h := newHarness(t, map[string]string{"a.txt": "one\nfind me here\n", "b.txt": "nothing\n"}, Options{})
	h.open("b.txt")
	h.ed.Run("workbench.action.findInFiles")
	h.ed.SetSearchField(0, "find me")
	for i := 0; i < 20 && (h.f.frame.Sidebar.Search.Running || len(h.f.frame.Sidebar.Search.Rows) < 2); i++ {
		h.settle()
	}
	from, ok := h.tt.Find("find me here")
	if !ok {
		t.Fatalf("no result row: %+v", h.f.frame.Sidebar.Search.Rows)
	}
	pane := h.f.cols[0].size
	x, y := from.X+from.W/2, from.Y+from.H/2
	tx, ty := pane.X+pane.W-12, pane.Y+pane.H/2
	h.tt.Press(x, y)
	for k := 1; k <= 10; k++ {
		h.tt.Move(x+(tx-x)*float32(k)/10, y+(ty-y)*float32(k)/10)
		h.settle()
	}
	h.settle()
	h.settle()
	h.tt.Release(tx, ty)
	h.settle()
	cols := h.f.frame.Columns
	if len(cols) != 2 || cols[1].Tabs[0].Name != "a.txt" || cols[0].Tabs[0].Name != "b.txt" {
		t.Fatalf("columns after the drop: %d", len(cols))
	}
	if h.f.frame.Active != 1 || h.f.frame.Focus != view.FocusEditor {
		t.Fatal("the dropped result must have the keyboard")
	}
	if s := h.f.frame.Columns[1].Pane; !strings.Contains(string(s.Rows[1].Text), "find me") || s.Current != 1 {
		t.Fatalf("match not shown: current line %d", s.Current)
	}
}
