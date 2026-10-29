package app

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
)

// 阅读辅助移植自 Mayoi（MIT，cmd/mayoi/decorations.go）：缩进参考线（光标所在的块画亮一些）、
// 与光标处配对的括号、光标处单词在可见行里的其他出现。只算可见的行。

// maxLineScan 是空行向上下找有字的行、找光标所在的块时最多看几行。
const maxLineScan = 1000

// decorations 是一帧里代码窗格的装饰。
type decorations struct {
	unit      int   // 一级缩进占几格
	levels    []int // 可见行的缩进级数
	active    int   // 光标所在块的级数（0 表示没有）
	from, to  int   // 光标所在块的行
	brackets  [2]int
	hasPair   bool
	word      string
	wholeWord bool
}

// indentCols 返回一行的缩进占几列；空行返回 -1。
func (e *Editor) indentCols(s text.Snapshot, line int) int {
	ls, le := s.LineRange(line)
	txt := s.String(ls, min(le, ls+400))
	if strings.TrimSpace(txt) == "" {
		return -1
	}
	return e.columns(leading(txt))
}

func (e *Editor) decorate(t *tab, s text.Snapshot, first, last int, sels edit.Selections) decorations {
	style := e.indentOf(t)
	unit := style.size
	if style.tabs {
		unit = e.cfg.TabWidth
	}
	unit = max(unit, 1)
	var dc decorations
	dc.unit = unit
	lines := s.LineCount()
	cache := map[int]int{}
	indentOf := func(i int) int {
		if c, ok := cache[i]; ok {
			return c
		}
		c := e.indentCols(s, i)
		cache[i] = c
		return c
	}
	// 空行取上下有字的行里更深的那个，参考线才能穿过它。
	level := func(i int) int {
		if c := indentOf(i); c >= 0 {
			return (c + unit - 1) / unit
		}
		above, below := 0, 0
		for j := i - 1; j >= max(0, i-maxLineScan); j-- {
			if c := indentOf(j); c >= 0 {
				above = (c + unit - 1) / unit
				break
			}
		}
		for j := i + 1; j < min(lines, i+maxLineScan); j++ {
			if c := indentOf(j); c >= 0 {
				below = (c + unit - 1) / unit
				break
			}
		}
		return max(above, below)
	}
	for i := first; i <= last; i++ {
		dc.levels = append(dc.levels, level(i))
	}
	main := sels.Main()
	// 光标所在的块：和它一样深的相邻行；光标在开始一个块的行上时，是下面那个块。
	cl := s.LineAt(main.Head)
	inView := cl >= first-1 && cl <= last+1
	if inView {
		dc.active = level(cl)
	}
	if inView && indentOf(cl) >= 0 {
		for j := cl + 1; j < min(lines, cl+maxLineScan); j++ {
			if c := indentOf(j); c >= 0 {
				dc.active = max(dc.active, (c+unit-1)/unit)
				break
			}
		}
		// 在结束一个块的行上：是它结束的那个块。
		if l := (indentOf(cl) + unit - 1) / unit; dc.active == l && cl > 0 && level(cl-1) > l {
			dc.active = l + 1
			cl--
		}
	}
	dc.from, dc.to = cl, cl
	if dc.active > 0 {
		if level(cl) < dc.active {
			cl++ // 开始块的那一行：块从它下面开始
			dc.from, dc.to = cl, cl
		}
		for dc.from > max(0, first) && level(dc.from-1) >= dc.active {
			dc.from--
		}
		for dc.to < min(lines-1, last) && level(dc.to+1) >= dc.active {
			dc.to++
		}
	}
	if main.Empty() {
		if at, pair, ok := matchBracket(s, main.Head); ok {
			dc.brackets, dc.hasPair = [2]int{at, pair}, true
		}
	}
	dc.word, dc.wholeWord = wordAtCaret(s, main)
	return dc
}

// wordAtCaret 返回光标处的单词（至少两个字、不以数字开头），或一行内选中的文字。
func wordAtCaret(s text.Snapshot, r edit.Range) (string, bool) {
	if !r.Empty() {
		if r.To()-r.From() > 200 || s.LineAt(r.From()) != s.LineAt(r.To()) {
			return "", false
		}
		w := s.String(r.From(), r.To())
		if strings.TrimSpace(w) == "" {
			return "", false
		}
		return w, false
	}
	ls, le := s.LineRange(s.LineAt(r.Head))
	if le-ls > 2000 {
		return "", false
	}
	line := s.String(ls, le)
	i, j := r.Head-ls, r.Head-ls
	for i > 0 {
		c, n := utf8.DecodeLastRuneInString(line[:i])
		if !isWordRune(c) {
			break
		}
		i -= n
	}
	for j < len(line) {
		c, n := utf8.DecodeRuneInString(line[j:])
		if !isWordRune(c) {
			break
		}
		j += n
	}
	w := line[i:j]
	if utf8.RuneCountInString(w) < 2 || len(w) > 240 {
		return "", false
	}
	if c, _ := utf8.DecodeRuneInString(w); unicode.IsDigit(c) {
		return "", false
	}
	return w, true
}

// row 给一行加上参考线、括号框和同词的底色。line 是行号，ls 是行首偏移；words 数着同词出现了几次。
func (dc *decorations) row(row *view.RowFrame, line, first, ls int, words *int) {
	for k := range dc.levels[line-first] {
		row.Guides = append(row.Guides, view.Guide{X: view.Unit(k * dc.unit), Active: k == dc.active-1 && line >= dc.from && line <= dc.to})
	}
	le := ls + len(row.Text)
	if dc.hasPair {
		for _, b := range dc.brackets {
			if b >= ls && b < le {
				row.Spans = append(row.Spans, style.Span{Start: int32(b - ls), End: int32(b - ls + 1), Style: style.Bracket})
			}
		}
	}
	if dc.word == "" || len(row.Text) > 2000 || *words > 400 {
		return
	}
	txt := string(row.Text)
	for at := 0; ; {
		k := strings.Index(txt[at:], dc.word)
		if k < 0 {
			break
		}
		k += at
		end := k + len(dc.word)
		at = end
		if dc.wholeWord {
			before, _ := utf8.DecodeLastRuneInString(txt[:k])
			after, _ := utf8.DecodeRuneInString(txt[end:])
			if k > 0 && isWordRune(before) || end < len(txt) && isWordRune(after) {
				continue
			}
		}
		row.Spans = append(row.Spans, style.Span{Start: int32(k), End: int32(end), Style: style.Word})
		*words++
	}
}
