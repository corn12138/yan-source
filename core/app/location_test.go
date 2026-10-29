package app

import (
	"testing"
	"unicode/utf8"
)

func TestGotoLocation(t *testing.T) {
	e, _, _ := newTestEditor(t, "first\r\n\t中😀x\r\n\r\nlast")
	defer e.Close()
	for _, tc := range []struct {
		line, column, want int
	}{
		{2, 1, 7}, {2, 2, 8}, {2, 3, 11}, {2, 4, 15},
		{2, 99, 16}, {3, 99, 18}, {4, 99, 24},
		{99, 3, 24}, {0, 0, 0}, {2, 0, 7},
	} {
		e.GotoLocation(tc.line, tc.column)
		buf := e.cur().doc.buf
		sel := buf.Selections().Main()
		if !sel.Empty() || sel.Head != tc.want {
			t.Errorf("location %d:%d: selection %+v, want %d", tc.line, tc.column, sel, tc.want)
		}
		s := buf.Snapshot()
		if !utf8.ValidString(s.String(0, sel.Head)) {
			t.Fatal("cursor split a UTF-8 character")
		}
	}
	e.GotoLine(2)
	if got := e.cur().doc.buf.Selections().Main().Head; got != 7 {
		t.Fatalf("GotoLine changed: %d", got)
	}
}
