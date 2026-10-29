package tui

import "github.com/corn12138/yan-source/core/view/cell"

// forCells 是布局器和绘制器共用的唯一遍历（见 core/view/cell）。
func forCells(line []byte, tabWidth int, fn func(start, end, x, w int)) {
	cell.ForEach(line, tabWidth, fn)
}
