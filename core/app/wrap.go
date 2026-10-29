package app

import (
	"slices"

	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
)

// 自动换行：长行在窗格宽度处折成几个视觉行；Alt+Z 只覆盖当前标签的设置。
// tab.scrollY 还是顶上的逻辑行，tab.scrollRow 记着它折出来的第几行在顶上；滚动、上下移动、点击都按视觉行算。
// 不折行时每个逻辑行只有一个视觉行，下面这些函数就和按行数算一样。

// vrow 是一个视觉行：第 line 个逻辑行折出来的第 sub 行。
type vrow struct{ line, sub int }

func (a vrow) before(b vrow) bool { return a.line < b.line || a.line == b.line && a.sub < b.sub }

// wrapWidth 返回列 c 里一个视觉行最多几格（少一格，行尾的光标才看得见）；0 表示不折行。
func (e *Editor) wrapWidth(c *column, t *tab, s text.Snapshot) view.Unit {
	if !e.wraps(t) {
		return 0
	}
	return max(c.size.W-e.gutter(s)-1, 1)
}

// layoutWrapped 把第 line 行按每行 w 格排进 e.scratch，返回该行内容的字节区间。
func (e *Editor) layoutWrapped(s text.Snapshot, line int, w view.Unit) (int, int) {
	ls, le := s.LineRange(line)
	e.lineBuf = s.AppendRange(e.lineBuf[:0], ls, le)
	e.lay.Layout(&e.scratch, e.lineBuf, view.LayoutStyle{TabWidth: e.cfg.TabWidth, Wrap: w})
	return ls, le
}

// rowCount 返回第 line 行折成几个视觉行。
func (e *Editor) rowCount(s text.Snapshot, line int, w view.Unit) int {
	if w <= 0 {
		return 1
	}
	if ls, le := s.LineRange(line); (le-ls)*max(e.cfg.TabWidth, 2) <= int(w) {
		return 1 // 每个字节都按最宽算也放得下
	}
	e.layoutWrapped(s, line, w)
	return max(len(e.scratch.Rows), 1)
}

// rowAt 返回 pos 所在的视觉行和它在这一行里的 x。
func (e *Editor) rowAt(s text.Snapshot, w view.Unit, pos int) (vrow, view.Unit) {
	line := s.LineAt(pos)
	ls, _ := e.layoutWrapped(s, line, w)
	return vrow{line, e.scratch.RowOf(pos - ls)}, e.scratch.XAt(pos - ls)
}

// stepRows 从视觉行 at 往下（n 为负时往上）走 n 行，走到文档头尾就停；返回走到的行和没走完的行数（带符号）。
func (e *Editor) stepRows(s text.Snapshot, w view.Unit, at vrow, n int) (vrow, int) {
	for n < 0 {
		if at.sub > 0 {
			k := min(at.sub, -n)
			at.sub -= k
			n += k
			continue
		}
		if at.line == 0 {
			break
		}
		at.line--
		at.sub = e.rowCount(s, at.line, w) - 1
		n++
	}
	for n > 0 {
		if rc := e.rowCount(s, at.line, w); at.sub+1 < rc {
			k := min(rc-1-at.sub, n)
			at.sub += k
			n -= k
			continue
		}
		if at.line+1 >= s.LineCount() {
			break
		}
		at = vrow{at.line + 1, 0}
		n--
	}
	return at, n
}

// rowsBetween 返回从视觉行 a 往下数到 b 隔了几行（b 不在 a 前面），数到 limit 就停。
func (e *Editor) rowsBetween(s text.Snapshot, w view.Unit, a, b vrow, limit int) int {
	n := -a.sub
	for l := a.line; l < b.line && n < limit; l++ {
		n += e.rowCount(s, l, w)
	}
	return min(n+b.sub, limit)
}

// topRow 返回标签顶上的视觉行。别处只改了 scrollY 时，scrollRow 作废，从那一行的第一个视觉行算。
func (e *Editor) topRow(t *tab, s text.Snapshot, w view.Unit) vrow {
	line := max(min(t.scrollY, s.LineCount()-1), 0)
	if t.scrollRow.line != t.scrollY || w <= 0 {
		return vrow{line, 0}
	}
	return vrow{line, min(t.scrollRow.sub, e.rowCount(s, line, w)-1)}
}

func (t *tab) setTop(v vrow) { t.scrollY, t.scrollRow = v.line, v }

// colOf 返回标签所在的列（找不到时是有焦点的列）。
func (e *Editor) colOf(t *tab) *column {
	for _, c := range e.cols {
		if slices.Contains(c.tabs, t) {
			return c
		}
	}
	return e.column()
}

// wraps 报告标签 t 折不折行：⌥Z 给这个标签单独定过就按它的，否则按设置（照着 Mayoi）。
func (e *Editor) wraps(t *tab) bool {
	if t != nil && t.wrap != 0 {
		return t.wrap > 0
	}
	return e.cfg.WordWrap
}

// toggleTabWrap 是 ⌥Z：只切换当前标签的自动换行，直到设置再变（照着 Mayoi）。
func (e *Editor) toggleTabWrap() error {
	t := e.cur()
	if t == nil {
		return nil
	}
	if e.wraps(t) {
		t.wrap = -1
	} else {
		t.wrap = 1
	}
	t.scrollX = 0
	e.reveal()
	if t.wrap > 0 {
		e.status = "这个标签自动换行：开"
	} else {
		e.status = "这个标签自动换行：关"
	}
	return nil
}

func (e *Editor) toggleWordWrap() error {
	e.cfg.WordWrap = !e.cfg.WordWrap
	if e.state != nil {
		v := e.cfg.WordWrap
		e.state.WordWrap = &v
		e.saveState()
	}
	e.settingsGen++
	for _, c := range e.cols {
		for _, t := range c.tabs {
			t.scrollX, t.wrap = 0, 0 // 设置变了：各标签单独定的不算了
		}
	}
	e.reveal()
	if e.cfg.WordWrap {
		e.status = "自动换行：开"
	} else {
		e.status = "自动换行：关"
	}
	return nil
}

// WordWrap 报告自动换行开着没有（菜单的勾）。
func (e *Editor) WordWrap() bool { return e.cfg.WordWrap }

// TerminalCopyOnSelect 报告窗口版的终端要不要选中就复制（[terminal] copy_on_select）。
func (e *Editor) TerminalCopyOnSelect() bool { return e.cfg.TerminalCopyOnSelect }

// wrapRow 把整个逻辑行的 row 切出字节区间 [a, z) 那个视觉行：样式区间裁到这一段、从段首算起；
// 行号、缩进参考线和「上面删掉了行」只在第一段，行尾的 blame、选中换行符多画的一格和「下面删掉了行」只在最后一段。
func wrapRow(row view.RowFrame, a, z int32, first, last bool) view.RowFrame {
	out := row
	out.Text = row.Text[a:z:z]
	out.Spans = nil
	n := int32(len(row.Text))
	for _, sp := range row.Spans {
		s0, s1 := max(sp.Start, a), min(sp.End, z)
		if last && sp.End > n {
			s1 = sp.End
		}
		if s1 > s0 {
			out.Spans = append(out.Spans, style.Span{Start: s0 - a, End: s1 - a, Style: sp.Style})
		}
	}
	if !first {
		out.Num, out.Guides = -1, nil
		if out.Mark == view.MarkDeleted {
			out.Mark = view.MarkNone
		}
	}
	if !last {
		out.Ghost = ""
		if out.Mark == view.MarkDeletedBelow {
			out.Mark = view.MarkNone
		}
	}
	return out
}
