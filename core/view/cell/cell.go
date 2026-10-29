// Package cell 是按「格」排版的 LineLayouter：一个字素占它的显示宽度那么多格（中文 2 格），
// 制表符展开到下一个制表位。终端前端的格就是终端单元格；GUI 前端的格是代码字体里 0 的宽度，
// 这样两个前端的光标、列号和上下移动完全一致。宽度来自 core/text/width，与 tcell 同一套规则。
package cell

import (
	"github.com/corn12138/yan-source/core/text/width"
	"github.com/corn12138/yan-source/core/view"
)

// Layouter 是按格排版的 view.LineLayouter。
type Layouter struct{}

func (Layouter) Epoch() uint64 { return 0 }

func (Layouter) Layout(dst *view.LineLayout, line []byte, st view.LayoutStyle) {
	dst.Rows, dst.Stops = dst.Rows[:0], dst.Stops[:0]
	if st.Wrap <= 0 {
		x := 0
		ForEach(line, st.TabWidth, func(start, end, cx, w int) {
			dst.Stops = append(dst.Stops, view.Stop{Byte: int32(start), X: view.Unit(cx)})
			x = cx + w
		})
		dst.Stops = append(dst.Stops, view.Stop{Byte: int32(len(line)), X: view.Unit(x)})
		dst.Rows = append(dst.Rows, view.Row{Start: 0, End: int32(len(line)), Height: 1})
		return
	}
	for start := 0; ; {
		first := len(dst.Stops)
		end, x := wrapRow(dst, line, start, st.TabWidth, int(st.Wrap))
		dst.Stops = append(dst.Stops, view.Stop{Byte: int32(end), X: view.Unit(x)})
		dst.Rows = append(dst.Rows, view.Row{Start: int32(start), End: int32(end), Height: 1, Stop: int32(first)})
		if end >= len(line) {
			return
		}
		start = end
	}
}

// wrapRow 从 start 起排一个视觉行（x 从 0 算，制表符也从这一行的开头算制表位），把每个字素的 Stop 追加到 dst，
// 返回这一行在哪里结束、结束处的 x。放不下时在最后一个空白之后折，没有空白就在宽字符前后或放不下的字前面折；
// 一行至少放一个字。正好写满一行后的那个空格挂在行尾（内核给光标留了这一格），折行就在它后面。
func wrapRow(dst *view.LineLayout, line []byte, start, tabWidth, wrap int) (int, int) {
	if tabWidth <= 0 {
		tabWidth = 4
	}
	x := 0
	brk, brkAt := -1, 0 // 最后一个可以折的地方：字节、它在 dst.Stops 里的下标
	prevWide, prevSpace := false, false
	end, endX := len(line), -1
	width.Graphemes(line[start:], func(s, _, w int) bool {
		b := start + s
		c := line[b]
		switch {
		case c == '\t':
			w = tabWidth - x%tabWidth
		case w == 0:
			w = 1
		}
		wide := w >= 2 && c >= 0x80
		if x > 0 && (prevSpace || prevWide || wide) {
			brk, brkAt = b, len(dst.Stops)
		}
		if x > 0 && x+w > wrap && !(c == ' ' && x == wrap) { // 一个空格可以挂在行尾外面那一格
			end, endX = b, x
			return false
		}
		dst.Stops = append(dst.Stops, view.Stop{Byte: int32(b), X: view.Unit(x)})
		x += w
		prevWide, prevSpace = wide, c == ' ' || c == '\t'
		return true
	})
	if endX < 0 {
		return len(line), x
	}
	if brk > start && brk < end { // 折在单词中间：退回到最后一个空白（或宽字符）之后
		endX = int(dst.Stops[brkAt].X)
		dst.Stops = dst.Stops[:brkAt]
		return brk, endX
	}
	return end, endX
}

// ForEach 是布局器和绘制器共用的唯一遍历：给出每个字素的字节区间、起始列和占用列数。
// 制表符展开到下一个制表位；不可见的控制字符按 1 列画成替代符。
func ForEach(line []byte, tabWidth int, fn func(start, end, x, w int)) {
	if tabWidth <= 0 {
		tabWidth = 4
	}
	x := 0
	width.Graphemes(line, func(start, end, w int) bool {
		switch {
		case line[start] == '\t':
			w = tabWidth - x%tabWidth
		case w == 0:
			w = 1
		}
		fn(start, end, x, w)
		x += w
		return true
	})
}
