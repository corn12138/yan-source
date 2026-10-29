package cell

import (
	"testing"

	"github.com/corn12138/yan-source/core/view"
)

func rows(line string, wrap int) []string {
	var l view.LineLayout
	Layouter{}.Layout(&l, []byte(line), view.LayoutStyle{TabWidth: 4, Wrap: view.Unit(wrap)})
	var out []string
	for _, r := range l.Rows {
		out = append(out, line[r.Start:r.End])
	}
	return out
}

func TestWrapAtSpaces(t *testing.T) {
	got := rows("the quick brown fox", 9)
	want := []string{"the quick ", "brown fox"} // 「quick」正好写满，后面的空格挂在行尾
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWrapLongWordAndCJK(t *testing.T) {
	if got := rows("abcdefghij", 4); len(got) != 3 || got[0] != "abcd" || got[2] != "ij" {
		t.Fatalf("long word: %q", got)
	}
	// 中文两格一个字，任意两个字之间都能折
	if got := rows("中文中文中", 5); len(got) != 3 || got[0] != "中文" || got[2] != "中" {
		t.Fatalf("CJK: %q", got)
	}
	// 比一行还宽的字也要放下，不能死循环
	if got := rows("中中", 1); len(got) != 2 {
		t.Fatalf("too narrow: %q", got)
	}
	if got := rows("", 4); len(got) != 1 || got[0] != "" {
		t.Fatalf("empty: %q", got)
	}
}

func TestWrappedStops(t *testing.T) {
	var l view.LineLayout
	Layouter{}.Layout(&l, []byte("aaaa bbbb"), view.LayoutStyle{TabWidth: 4, Wrap: 6})
	// 第二行「bbbb」从 0 格算起；折行处（字节 5）算下一行的开头
	if r := l.RowOf(5); r != 1 {
		t.Fatalf("RowOf(5) = %d", r)
	}
	if x := l.XAt(7); x != 2 {
		t.Fatalf("XAt(7) = %d", x)
	}
	if b := l.RowByteAt(1, 2); b != 7 {
		t.Fatalf("RowByteAt(1, 2) = %d", b)
	}
	// 点在第一行末尾之外：停在这一行最后一个字前面，不跳到下一行
	if b := l.RowByteAt(0, 30); b != 4 {
		t.Fatalf("RowByteAt(0, 30) = %d", b)
	}
	if b := l.RowByteAt(1, 30); b != 9 {
		t.Fatalf("RowByteAt(1, 30) = %d", b)
	}
}
