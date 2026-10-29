package tui

import (
	"testing"

	"github.com/corn12138/yan-source/core/view"
)

// 合并提交 m 在第 0 道，两个父提交各占一道：点、直通的线和分出去的拐角。
func TestGraphCell(t *testing.T) {
	merge := &view.SCMCommit{Lane: 0, Merge: true, Above: []int{0}, Joins: []bool{true},
		Below: []int{0, 1}, From: []bool{true, true}, Through: []bool{false, false}}
	side := &view.SCMCommit{Lane: 1, Color: 1, Above: []int{0, 1}, Joins: []bool{false, true},
		Below: []int{0, 1}, From: []bool{false, true}, Through: []bool{true, false}}
	join := &view.SCMCommit{Lane: 0, Above: []int{0, 1}, Joins: []bool{true, true}, Below: []int{0}, From: []bool{true}, Through: []bool{false}}
	for _, tc := range []struct {
		c    *view.SCMCommit
		want string
	}{{merge, "○╮"}, {side, "│●"}, {join, "●╯"}} {
		var got []rune
		for l := range 2 {
			ch, _ := graphCell(tc.c, l)
			got = append(got, ch)
		}
		if string(got) != tc.want {
			t.Errorf("got %q, want %q", string(got), tc.want)
		}
	}
}
