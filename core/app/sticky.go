package app

import (
	"github.com/corn12138/yan-source/core/syntax/highlight"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/syntax/symbols"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
)

// 粘性滚动、面包屑里的块和转到符号，移植自 Mayoi（MIT，cmd/mayoi/sticky.go、symbols.go）：
// 块由缩进看出来，所以对几乎所有语言都管用；滚动时，开始所在块的那些行钉在窗格顶上，点一下就跳过去。

const (
	maxSticky    = 5
	maxScopeScan = 4 * maxLineScan
)

// isCode 报告文档的缩进是否表示块（Markdown、纯文本不是）。
func isCode(d *document) bool { return isCodeLang(d.lang()) }

// isCodePath 是文件 path 按名字看是不是代码（改动视图给 @@ 找作用域用）。
func isCodePath(path string) bool {
	lx := highlight.Lexer(path)
	return lx != nil && isCodeLang(highlight.Name(lx))
}

func isCodeLang(lang string) bool {
	switch lang {
	case "", "markdown", "Markdown", "reStructuredText", "TeX", "plaintext":
		return false
	}
	return true
}

// scopes 返回 line 所在的块的开头那些行，最外层在前：上面每个缩进更浅的最近一行，
// 就像函数、if 在它们的正文上面。
func (e *Editor) scopes(s text.Snapshot, line int, cache map[int]int) []int {
	n := s.LineCount()
	if line <= 0 || line >= n {
		return nil
	}
	indentOf := func(i int) (int, string) {
		if c, ok := cache[i]; ok && c < 0 {
			return -1, ""
		}
		ls, le := s.LineRange(i)
		t := s.String(ls, min(le, ls+400))
		c, ok := cache[i]
		if !ok {
			c = e.indentCols(s, i)
			cache[i] = c
		}
		return c, t
	}
	cur := -1 // 空行和它下面的行一样深
	for i := line; i < min(n, line+maxLineScan) && cur < 0; i++ {
		cur, _ = indentOf(i)
	}
	if cur <= 0 {
		return nil
	}
	var out []int
	for j := line - 1; j >= max(0, line-maxScopeScan) && cur > 0; j-- {
		ind, t := indentOf(j)
		if ind < 0 || ind >= cur {
			continue
		}
		cur = ind
		t = strings.TrimSpace(t)
		if strings.ContainsAny(t[:1], ")]}") {
			if !strings.ContainsAny(t[len(t)-1:], "{[(:") {
				continue // 块的结尾，比如函数参数的那一行
			}
			// 跨几行的参数列表之后开始的块：列表开始的那一行给它起名字。
			if !strings.HasPrefix(t, "}") {
				for k := j - 1; k >= max(0, j-maxLineScan); k-- {
					ki, kt := indentOf(k)
					if ki < 0 || ki > ind {
						continue
					}
					if ki == ind && !strings.ContainsAny(strings.TrimSpace(kt)[:1], ")]}") {
						j = k
					}
					break
				}
			}
		}
		out = append(out, j)
	}
	slices.Reverse(out)
	return out
}

// stickyLines 返回钉在窗格顶上的行（第一行是 top 的视图）：它所在的那些块的开头，每行都在它本来的位置上面。
func (e *Editor) stickyLines(s text.Snapshot, top, room int, cache map[int]int) []int {
	var lines []int
	// 钉住的行盖住了下面的行：由被盖住后露出来的第一行决定钉哪些，直到两者一致。
	for range 3 {
		next := e.scopes(s, top+len(lines), cache)
		if len(next) > room {
			next = next[len(next)-room:]
		}
		k := 0
		for _, j := range next {
			if j < top+k {
				next[k] = j
				k++
			}
		}
		next = next[:k]
		if len(next) == len(lines) {
			lines = next
			break
		}
		lines = next
	}
	return lines
}

// scopeLabel 是面包屑给块起的名字：开始它的那一行，去掉结尾的括号和冒号。
func scopeLabel(s text.Snapshot, line int) string {
	ls, le := s.LineRange(line)
	t := strings.TrimSpace(s.String(ls, min(le, ls+200)))
	t = strings.TrimSpace(strings.TrimRight(t, "{(:[ "))
	if utf8.RuneCountInString(t) > 48 {
		t = string([]rune(t)[:47]) + "…"
	}
	return t
}

// stickyRows 算出钉在顶上的行，和面包屑里光标所在的（最里面两个）块。
func (e *Editor) stickyAndCrumbs(t *tab, s text.Snapshot, h int, head int, pane *view.PaneFrame, crumbs *[]view.Crumb) {
	pane.Sticky = pane.Sticky[:0]
	*crumbs = (*crumbs)[:0]
	if t.diff != nil {
		if line := diffStickyLine(t); line >= 0 && h > 6 {
			ls, le := s.LineRange(line)
			start := len(e.arena)
			e.arena = s.AppendRange(e.arena, ls, le)
			row := view.RowFrame{Line: int32(line), Text: e.arena[start:len(e.arena):len(e.arena)], Base: style.Text}
			t.diff.row(&row, line)
			pane.Sticky = append(pane.Sticky, row)
		}
		return
	}
	if !isCode(t.doc) {
		return
	}
	cache := map[int]int{}
	if t.scrollY > 0 && h > 6 {
		for _, line := range e.stickyLines(s, t.scrollY, min(maxSticky, h/3), cache) {
			ls, le := s.LineRange(line)
			start := len(e.arena)
			e.arena = s.AppendRange(e.arena, ls, le)
			row := view.RowFrame{Line: int32(line), Text: e.arena[start:len(e.arena):len(e.arena)], Base: style.Text}
			if hl := t.doc.hl; hl != nil {
				row.Spans = syntaxSpans(row.Spans, row.Text, hl.Line(line, string(row.Text)))
			}
			pane.Sticky = append(pane.Sticky, row)
		}
	}
	sc := e.scopes(s, s.LineAt(head), cache)
	for _, line := range sc[max(0, len(sc)-2):] {
		*crumbs = append(*crumbs, view.Crumb{Label: scopeLabel(s, line), Line: line})
	}
}

// jumpToScope 把光标放到开始块的那一行的文字开头，并把它滚到顶上（钉住的行下面）。
func (e *Editor) jumpToScope(line int) {
	t := e.cur()
	if t == nil {
		return
	}
	b := t.doc.buf
	s := b.Snapshot()
	line = max(min(line, s.LineCount()-1), 0)
	ls, le := s.LineRange(line)
	at := ls + len(leading(s.String(ls, le)))
	t.scrollY = max(line-len(e.scopes(s, line, map[int]int{})), 0)
	b.SetSelections(edit.Single(edit.Cursor(at)))
	b.BreakGroup()
	e.area = areaEditor
}

// JumpToLine 跳到开始块的第 line 行（面包屑、钉住的行被点了）。
func (e *Editor) JumpToLine(col, line int) {
	e.sync()
	defer e.sync()
	if col >= 0 && col < len(e.cols) {
		e.col = col
		e.sync()
	}
	e.jumpToScope(line)
}

// symbolItems 列出当前文件里符合输入的符号（快速打开里的「@」，⇧⌘O）。
func (e *Editor) symbolItems(q string) {
	ov := e.ov
	ov.items, ov.footer = ov.items[:0], ""
	t := e.cur()
	if t == nil {
		ov.footer = i18n.T("Open a file first")
		return
	}
	s := t.doc.buf.Snapshot()
	syms := symbols.Find(s, strings.EqualFold(t.doc.lang(), "markdown"))
	add := func(sym symbols.Symbol, hits []int) {
		ov.items = append(ov.items, ovItem{label: strings.Repeat("  ", min(sym.Depth/max(e.cfg.TabWidth, 1), 4)) + sym.Name,
			detail: sym.Detail, hint: symbolKind(sym.Kind), labelHit: shift(hits, 2*min(sym.Depth/max(e.cfg.TabWidth, 1), 4)),
			run: func(e *Editor) error {
				e.jumpToScope(sym.Line)
				if t := e.cur(); t != nil {
					ls := t.doc.buf.Snapshot().LineStart(sym.Line)
					t.doc.buf.SetSelections(edit.Single(edit.Range{Anchor: ls + sym.Col, Head: ls + sym.Col + len(sym.Name), GoalX: -1}))
				}
				return nil
			}})
	}
	q = strings.TrimSpace(q)
	if q == "" {
		for _, sym := range syms {
			add(sym, nil)
		}
	} else {
		names := make([]string, len(syms))
		for i, sym := range syms {
			names[i] = sym.Name
		}
		for _, m := range fuzzyFind(q, names) {
			add(syms[m.Index], m.MatchedIndexes)
		}
	}
	if len(ov.items) == 0 {
		ov.footer = i18n.T("No symbols")
	}
}

func shift(hits []int, by int) []int {
	out := make([]int, len(hits))
	for i, h := range hits {
		out[i] = h + by
	}
	return out
}

// symbolKind 是符号种类的中文名。
func symbolKind(k string) string {
	switch k {
	case "function":
		return i18n.T("Function")
	case "method":
		return i18n.T("Method")
	case "class":
		return i18n.T("Type")
	case "interface":
		return i18n.T("Interface")
	case "heading":
		return i18n.T("Heading")
	case "enum":
		return i18n.T("Enum")
	}
	return k
}
