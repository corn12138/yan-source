package tui

import "testing"

func TestRightSidebarGeometry(t *testing.T) {
	f := New(4)
	left := f.layout(100, 25, []bool{false, false}, true, false)
	right := f.layout(100, 25, []bool{false, false}, true, true)
	if right.side.X != 70 || right.side.W != 30 || right.cols[0].code.X != 0 {
		t.Fatalf("right layout %+v", right)
	}
	if left.cols[0].code.W != right.cols[0].code.W {
		t.Fatal("moving sidebar changed code width")
	}
	last := right.cols[len(right.cols)-1].code
	if last.X+last.W >= right.side.X {
		t.Fatal("sidebar overlaps editor")
	}
}

// Columns hidden by a maximized editor area take no room; the shown ones share it.
func TestLayoutSkipsHiddenColumns(t *testing.T) {
	f := &Frontend{SidebarWidth: 30}
	all := f.layout(100, 25, []bool{false, false, false}, false, false)
	l := f.layout(100, 25, []bool{false, true, false}, false, false)
	if l.cols[1].code.W != 0 || l.cols[1].tabs.W != 0 {
		t.Fatalf("hidden column has room: %+v", l.cols[1])
	}
	if l.cols[0].code.W <= all.cols[0].code.W || l.cols[2].code.X <= l.cols[0].code.X+l.cols[0].code.W {
		t.Fatalf("shown columns %+v %+v", l.cols[0], l.cols[2])
	}
}
