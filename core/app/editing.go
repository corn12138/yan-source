package app

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/syntax/highlight"
	"github.com/corn12138/yan-source/core/text"
)

// 编辑命令的行为移植自 Mayoi（MIT，Copyright (c) 2026 chencheng，cmd/mayoi/editing.go、autoclose.go）：
// 回车沿用缩进、在左括号后多缩一级；Tab 与 Shift+Tab 缩进、反缩进；⌘/ 注释；⌥↑↓ 移动行；
// ⇧⌥↑↓ 复制行；⇧⌘K 删除行；⌘↵、⇧⌘↵ 在下面、上面插一行；⌘L 选中行；⇧⌘\ 跳到配对的括号；
// 括号和引号自动闭合。Mayoi 只有一个选区，这里每个命令都作用于所有光标。

// indentStyle 是文档的缩进方式：制表符，或 size 个空格。
type indentStyle struct {
	tabs bool
	size int
}

// unit 是一级缩进的文字。
func (s indentStyle) unit() string {
	if s.tabs {
		return "\t"
	}
	return strings.Repeat(" ", s.size)
}

func (s indentStyle) String() string {
	if s.tabs {
		return i18n.T("Tabs")
	}
	return i18n.F("Spaces: %s", string(rune('0'+s.size)))
}

// detectIndent 从文档开头的若干行看出它怎么缩进；看不出时按文件类型的习惯。
func detectIndent(s text.Snapshot, path string, tabWidth int) indentStyle {
	tabs, spaces := 0, 0
	var widths [9]int // 缩进每次加深 n 个空格出现了几次
	prev := 0
	for i := range min(s.LineCount(), 2000) {
		ls, le := s.LineRange(i)
		line := s.String(ls, min(le, ls+80))
		if strings.TrimSpace(line) == "" {
			continue
		}
		switch line[0] {
		case '\t':
			tabs++
			prev = 0
			continue
		case ' ':
			spaces++
		}
		n := len(line) - len(strings.TrimLeft(line, " "))
		if d := n - prev; d >= 2 && d <= 8 {
			widths[d]++
		}
		prev = n
	}
	if tabs > spaces && tabs > 0 {
		return indentStyle{tabs: true, size: tabWidth}
	}
	if spaces > 0 {
		best := 0
		for n := 2; n <= 8; n++ {
			if widths[n] > widths[best] {
				best = n
			}
		}
		if best != 0 {
			return indentStyle{size: best}
		}
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".mod":
		return indentStyle{tabs: true, size: tabWidth}
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".json", ".yaml", ".yml", ".html", ".css", ".scss", ".vue", ".svelte", ".rb", ".lua", ".dart", ".nix":
		return indentStyle{size: 2}
	}
	if strings.EqualFold(filepath.Base(path), "makefile") {
		return indentStyle{tabs: true, size: tabWidth}
	}
	return indentStyle{size: tabWidth}
}

// lineComments 是各语言的行注释，按 Chroma 的语言名。
var lineComments = map[string]string{
	"Go": "//", "C": "//", "C++": "//", "C#": "//", "Java": "//", "JavaScript": "//", "TypeScript": "//",
	"TSX": "//", "JSX": "//", "Rust": "//", "Swift": "//", "Kotlin": "//", "Scala": "//", "Dart": "//",
	"Zig": "//", "Objective-C": "//", "PHP": "//", "Groovy": "//", "Protocol Buffer": "//", "GLSL": "//",
	"JSON5": "//", "SCSS": "//", "Sass": "//", "Less": "//", "Odin": "//", "V": "//", "Gleam": "//",
	"Python": "#", "Python 2": "#", "Bash": "#", "Zsh": "#", "Fish": "#", "Ruby": "#", "Perl": "#",
	"YAML": "#", "TOML": "#", "Makefile": "#", "Docker": "#", "R": "#", "Elixir": "#", "Nix": "#",
	"PowerShell": "#", "CMake": "#", "Nim": "#", "Julia": "#", "Crystal": "#", "INI": ";", "Terraform": "#",
	"HCL": "#", "GDScript": "#", "Starlark": "#",
	"SQL": "--", "PostgreSQL SQL dialect": "--", "MySQL": "--", "Lua": "--", "Haskell": "--", "Elm": "--",
	"Common Lisp": ";", "Clojure": ";", "Scheme": ";", "Racket": ";", "Emacs Lisp": ";", "Fennel": ";",
	"VimL": `"`, "Erlang": "%", "TeX": "%", "MATLAB": "%", "Fortran": "!",
}

// fileComments are the line comments of files Chroma has no language for, by name or extension.
var fileComments = map[string]string{
	".gitignore": "#", ".dockerignore": "#", ".npmignore": "#", ".prettierignore": "#", ".eslintignore": "#",
	".stylelintignore": "#", ".ignore": "#", ".gitattributes": "#", ".gitmodules": "#", ".npmrc": "#",
	".yarnrc": "#", ".editorconfig": "#", ".env": "#", "CODEOWNERS": "#", ".properties": "#", ".conf": "#",
	".gitkeep": "#", ".flake8": "#", ".pylintrc": "#",
}

// lineComment returns the line comment of a document: its language's, else that of its file name.
func (d *document) lineComment() string {
	if m := lineComments[d.lang()]; m != "" {
		return m
	}
	name := filepath.Base(d.buf.Path())
	if m := fileComments[name]; m != "" {
		return m
	}
	if strings.HasPrefix(name, ".env.") { // .env.local, .env.production
		return "#"
	}
	return fileComments[strings.ToLower(filepath.Ext(name))]
}

// lang 返回文档的语言名（Chroma 的叫法）；纯文本返回 ""。
func (d *document) lang() string {
	if d.hl == nil {
		return ""
	}
	return highlight.Name(d.hl.Lexer())
}

// eol 是文档的换行符。
func eolOf(s text.Snapshot) string {
	if s.Meta().EOL == text.EOLCRLF {
		return "\r\n"
	}
	return "\n"
}

// leading 返回 s 开头的空白。
func leading(s string) string { return s[:len(s)-len(strings.TrimLeft(s, " \t"))] }

// lineSpan 返回选区覆盖的行：跨几行的选区停在某行行首时不算那一行。
func lineSpan(s text.Snapshot, r edit.Range) (first, last int) {
	a, b := r.From(), r.To()
	first, last = s.LineAt(a), s.LineAt(b)
	if last > first && s.LineStart(last) == b {
		last--
	}
	return first, last
}

// blocks 把所有选区覆盖的行合并成不相交、不相邻的行块，按顺序排好。
func blocks(s text.Snapshot, sels edit.Selections) [][2]int {
	var out [][2]int
	for _, r := range sels.Ranges {
		f, l := lineSpan(s, r)
		if n := len(out); n > 0 && f <= out[n-1][1]+1 {
			out[n-1][1] = max(out[n-1][1], l)
			continue
		}
		out = append(out, [2]int{f, l})
	}
	return out
}

// lineEnd 返回第 line 行内容的结尾（不含换行符）。
func lineEnd(s text.Snapshot, line int) int { _, e := s.LineRange(line); return e }

// mapSels 把选区按修改集挪到新快照：非空选区的起点留在插入的文字前面，其余跟到后面——
// 整行选中后缩进，选区仍然从行首开始。
func mapSels(sels edit.Selections, cs text.ChangeSet) edit.Selections {
	out := edit.Selections{Ranges: make([]edit.Range, len(sels.Ranges)), Primary: sels.Primary}
	for i, r := range sels.Ranges {
		aAssoc, hAssoc := text.AssocAfter, text.AssocAfter
		if !r.Empty() {
			if r.Anchor < r.Head {
				aAssoc = text.AssocBefore
			} else {
				hAssoc = text.AssocBefore
			}
		}
		out.Ranges[i] = edit.Range{Anchor: cs.MapPos(r.Anchor, aAssoc), Head: cs.MapPos(r.Head, hAssoc), GoalX: -1}
	}
	return out
}

func (e *Editor) indentOf(t *tab) indentStyle {
	if t.doc.indent.size == 0 {
		t.doc.indent = detectIndent(t.doc.buf.Snapshot(), t.doc.buf.Path(), e.cfg.TabWidth)
	}
	return t.doc.indent
}

// indentLines 把选区覆盖的行缩进一级，或反缩进一级。
func (e *Editor) indentLines(outdent bool) error {
	t := e.cur()
	if t == nil {
		return nil
	}
	style := e.indentOf(t)
	return e.editDoc(func(b *edit.Buffer) {
		s := b.Snapshot()
		sels := b.Selections()
		unit := style.unit()
		var cs text.ChangeSet
		for _, bl := range blocks(s, sels) {
			for i := bl[0]; i <= bl[1]; i++ {
				ls, le := s.LineRange(i)
				line := s.String(ls, le)
				if outdent {
					if n := outdentWidth(line, style, e.cfg.TabWidth); n > 0 {
						cs = append(cs, text.Change{Start: ls, End: ls + n})
					}
					continue
				}
				if strings.TrimSpace(line) == "" && bl[0] != bl[1] {
					continue // 空行保持空
				}
				cs = append(cs, text.Change{Start: ls, End: ls, Text: unit})
			}
		}
		if len(cs) > 0 {
			b.Apply(edit.Transaction{Changes: cs, After: mapSels(sels, cs), Origin: edit.OriginCommand})
			b.BreakGroup()
		}
	})
}

// outdentWidth 是一级反缩进要删的字节：一个制表符，或至多一级的空格。
func outdentWidth(line string, style indentStyle, tabWidth int) int {
	if strings.HasPrefix(line, "\t") {
		return 1
	}
	cols := style.size
	if style.tabs {
		cols = tabWidth
	}
	n := 0
	for n < len(line) && n < cols && line[n] == ' ' {
		n++
	}
	if n < len(line) && n < cols && line[n] == '\t' {
		n++
	}
	return n
}

// tabKey：选区跨行时缩进这些行；否则在光标处插入缩进，空格缩进时补到下一个缩进位。
func (e *Editor) tabKey() error {
	t := e.cur()
	if t == nil {
		return nil
	}
	b := t.doc.buf
	s := b.Snapshot()
	for _, r := range b.Selections().Ranges {
		if s.LineAt(r.From()) != s.LineAt(r.To()) {
			return e.indentLines(false)
		}
	}
	style := e.indentOf(t)
	return e.editDoc(func(b *edit.Buffer) {
		var cs text.ChangeSet
		after := edit.Selections{Primary: b.Selections().Primary}
		delta := 0
		for _, r := range b.Selections().Ranges {
			ins := "\t"
			if !style.tabs {
				ls := s.LineStart(s.LineAt(r.From()))
				col := e.columns(s.String(ls, r.From()))
				ins = strings.Repeat(" ", style.size-col%style.size)
			}
			cs = append(cs, text.Change{Start: r.From(), End: r.To(), Text: ins})
			at := r.From() + delta + len(ins)
			after.Ranges = append(after.Ranges, edit.Cursor(at))
			delta += len(ins) - (r.To() - r.From())
		}
		b.Apply(edit.Transaction{Changes: cs, After: after, Origin: edit.OriginTyping})
	})
}

// columns 返回一段行首文字占几列（制表符展开）。
func (e *Editor) columns(s string) int {
	tw := max(e.cfg.TabWidth, 1)
	col := 0
	for _, r := range s {
		if r == '\t' {
			col += tw - col%tw
		} else {
			col++
		}
	}
	return col
}

// pairs 是缩进块所在的括号。
var pairs = map[byte]byte{'{': '}', '[': ']', '(': ')'}

// newline 在光标处换行：新行沿用原行的缩进，在左括号（或 Python 这类语言的冒号）后多缩一级；
// 光标夹在一对括号中间时，右括号换到自己的一行。光标前行尾的空白一并删掉。
func (e *Editor) newline() error {
	t := e.cur()
	if t == nil {
		return nil
	}
	style := e.indentOf(t)
	colon := opensColonBlock(t.doc.lang())
	return e.editDoc(func(b *edit.Buffer) {
		s := b.Snapshot()
		nl := eolOf(s)
		var cs text.ChangeSet
		after := edit.Selections{Primary: b.Selections().Primary}
		delta := 0
		for _, r := range b.Selections().Ranges {
			a, z := r.From(), r.To()
			ls, _ := s.LineRange(s.LineAt(a))
			before := s.String(ls, a)
			after0 := s.String(z, lineEnd(s, s.LineAt(z)))
			indent := leading(before)
			if strings.TrimSpace(before) == "" {
				indent = before // 只有缩进的行：光标保留全部缩进
			}
			trimmed := strings.TrimRight(before, " \t")
			ins := nl + indent
			caret := len(ins)
			if trimmed != "" {
				last := trimmed[len(trimmed)-1]
				closer, opens := pairs[last]
				if opens || last == ':' && colon {
					ins += style.unit()
					caret = len(ins)
					if rest := strings.TrimLeft(after0, " \t"); opens && rest != "" && rest[0] == closer {
						ins += nl + indent
					}
				}
			}
			start := a
			if trimmed != "" {
				start = ls + len(trimmed) // 光标前行尾的空白不要了
			}
			cs = append(cs, text.Change{Start: start, End: z, Text: ins})
			after.Ranges = append(after.Ranges, edit.Cursor(start+delta+caret))
			delta += len(ins) - (z - start)
		}
		b.Apply(edit.Transaction{Changes: cs, After: after, Origin: edit.OriginTyping})
		b.BreakGroup()
	})
}

// opensColonBlock 报告行尾的冒号在这种语言里是否开始一个块。
func opensColonBlock(lang string) bool {
	switch lang {
	case "Python", "Python 2", "YAML", "Nim", "GDScript", "Starlark", "CoffeeScript":
		return true
	}
	return false
}

// toggleComment 注释掉选区覆盖的行；它们都已是注释时取消注释。
func (e *Editor) toggleComment() error {
	t := e.cur()
	if t == nil {
		return nil
	}
	mark := t.doc.lineComment()
	if mark == "" {
		e.status = i18n.T("This kind of file has no line comments")
		return nil
	}
	return e.editDoc(func(b *edit.Buffer) {
		s := b.Snapshot()
		sels := b.Selections()
		var cs text.ChangeSet
		for _, bl := range blocks(s, sels) {
			type ln struct {
				start  int
				indent int
				text   string
			}
			var lines []ln
			all, minIndent := true, -1
			for i := bl[0]; i <= bl[1]; i++ {
				ls, le := s.LineRange(i)
				txt := s.String(ls, le)
				if strings.TrimSpace(txt) == "" {
					continue
				}
				ind := len(leading(txt))
				lines = append(lines, ln{ls, ind, txt})
				if !strings.HasPrefix(txt[ind:], mark) {
					all = false
				}
				if minIndent < 0 || ind < minIndent {
					minIndent = ind
				}
			}
			for _, l := range lines {
				if all {
					n := len(mark)
					if strings.HasPrefix(l.text[l.indent+n:], " ") {
						n++
					}
					cs = append(cs, text.Change{Start: l.start + l.indent, End: l.start + l.indent + n})
				} else {
					at := l.start + minIndent
					cs = append(cs, text.Change{Start: at, End: at, Text: mark + " "})
				}
			}
		}
		if len(cs) > 0 {
			b.Apply(edit.Transaction{Changes: cs, After: mapSels(sels, cs), Origin: edit.OriginCommand})
			b.BreakGroup()
		}
	})
}

// moveLines 把选区覆盖的行上移或下移一行，选区跟着走。
func (e *Editor) moveLines(down bool) error {
	return e.editDoc(func(b *edit.Buffer) {
		s := b.Snapshot()
		sels := b.Selections()
		nl := eolOf(s)
		bls := blocks(s, sels)
		n := s.LineCount()
		if !down && bls[0][0] == 0 || down && bls[len(bls)-1][1] == n-1 {
			return
		}
		var cs text.ChangeSet
		shift := make([]int, len(bls)) // 每个块里的位置挪多少
		for k, bl := range bls {
			bs, be := s.LineStart(bl[0]), lineEnd(s, bl[1])
			block := s.String(bs, be)
			if down {
				next := s.String(s.LineStart(bl[1]+1), lineEnd(s, bl[1]+1))
				cs = append(cs, text.Change{Start: bs, End: lineEnd(s, bl[1]+1), Text: next + nl + block})
				shift[k] = len(next) + len(nl)
			} else {
				ps := s.LineStart(bl[0] - 1)
				prev := s.String(ps, lineEnd(s, bl[0]-1))
				cs = append(cs, text.Change{Start: ps, End: be, Text: block + nl + prev})
				shift[k] = -(len(prev) + len(nl))
			}
		}
		after := edit.Selections{Primary: sels.Primary}
		for _, r := range sels.Ranges {
			f, _ := lineSpan(s, r)
			d := 0
			for k, bl := range bls {
				if f >= bl[0] && f <= bl[1] {
					d = shift[k]
				}
			}
			after.Ranges = append(after.Ranges, edit.Range{Anchor: r.Anchor + d, Head: r.Head + d, GoalX: -1})
		}
		b.Apply(edit.Transaction{Changes: cs, After: after, Origin: edit.OriginCommand})
		b.BreakGroup()
	})
}

// copyLines 在选区覆盖的行下面（或上面）复制一份，选中这份拷贝。
func (e *Editor) copyLines(down bool) error {
	return e.editDoc(func(b *edit.Buffer) {
		s := b.Snapshot()
		sels := b.Selections()
		nl := eolOf(s)
		bls := blocks(s, sels)
		var cs text.ChangeSet
		lens := make([]int, len(bls))
		for k, bl := range bls {
			bs, be := s.LineStart(bl[0]), lineEnd(s, bl[1])
			block := s.String(bs, be)
			lens[k] = len(block) + len(nl)
			if down {
				cs = append(cs, text.Change{Start: be, End: be, Text: nl + block})
			} else {
				cs = append(cs, text.Change{Start: bs, End: bs, Text: block + nl})
			}
		}
		after := edit.Selections{Primary: sels.Primary}
		for _, r := range sels.Ranges {
			f, _ := lineSpan(s, r)
			d := 0
			for k, bl := range bls {
				if bl[1] < f {
					d += lens[k] // 上面的块各多出一份
				}
				if down && f >= bl[0] && f <= bl[1] {
					d += lens[k] // 向下复制：选中下面那份
				}
			}
			after.Ranges = append(after.Ranges, edit.Range{Anchor: r.Anchor + d, Head: r.Head + d, GoalX: -1})
		}
		b.Apply(edit.Transaction{Changes: cs, After: after, Origin: edit.OriginCommand})
		b.BreakGroup()
	})
}

// deleteLines 删除选区覆盖的行；光标落到接替它们的那一行的同一列。
func (e *Editor) deleteLines() error {
	return e.editDoc(func(b *edit.Buffer) {
		s := b.Snapshot()
		sels := b.Selections()
		var cs text.ChangeSet
		for _, bl := range blocks(s, sels) {
			start, end := s.LineStart(bl[0]), lineEnd(s, bl[1])
			switch {
			case bl[1]+1 < s.LineCount():
				end = s.LineStart(bl[1] + 1) // 连同后面的换行符
			case bl[0] > 0:
				start = lineEnd(s, bl[0]-1) // 最后几行：删掉前面的换行符
			}
			cs = append(cs, text.Change{Start: start, End: end})
		}
		col := b.Selections().Main().Head - s.LineStart(s.LineAt(b.Selections().Main().Head))
		b.Apply(edit.Transaction{Changes: cs, Origin: edit.OriginCommand})
		ns := b.Snapshot()
		line := ns.LineAt(b.Selections().Main().Head)
		ls, le := ns.LineRange(line)
		b.SetSelections(edit.Single(edit.Cursor(ls + min(col, le-ls))))
		b.BreakGroup()
	})
}

// insertLine 在光标所在行的下面（或上面）开一个新行，带上缩进。
func (e *Editor) insertLine(above bool) error {
	t := e.cur()
	if t == nil {
		return nil
	}
	style := e.indentOf(t)
	return e.editDoc(func(b *edit.Buffer) {
		s := b.Snapshot()
		nl := eolOf(s)
		var cs text.ChangeSet
		after := edit.Selections{Primary: b.Selections().Primary}
		delta := 0
		seen := map[int]bool{}
		for _, r := range b.Selections().Ranges {
			line := s.LineAt(r.Head)
			if seen[line] {
				continue
			}
			seen[line] = true
			ls, le := s.LineRange(line)
			txt := s.String(ls, le)
			indent := leading(txt)
			if tr := strings.TrimRight(txt, " \t"); !above && tr != "" {
				if _, opens := pairs[tr[len(tr)-1]]; opens {
					indent += style.unit()
				}
			}
			if above {
				cs = append(cs, text.Change{Start: ls, End: ls, Text: indent + nl})
				after.Ranges = append(after.Ranges, edit.Cursor(ls+delta+len(indent)))
				delta += len(indent) + len(nl)
			} else {
				cs = append(cs, text.Change{Start: le, End: le, Text: nl + indent})
				after.Ranges = append(after.Ranges, edit.Cursor(le+delta+len(nl)+len(indent)))
				delta += len(nl) + len(indent)
			}
		}
		after.Primary = min(after.Primary, len(after.Ranges)-1)
		b.Apply(edit.Transaction{Changes: cs, After: after, Origin: edit.OriginCommand})
		b.BreakGroup()
	})
}

// selectLines 选中选区覆盖的整行（连同换行符）；已经选中整行时再多选下一行。
func (e *Editor) selectLines() error {
	return e.withDoc(func(b *edit.Buffer) {
		s := b.Snapshot()
		sels := b.Selections()
		out := edit.Selections{Primary: sels.Primary}
		for _, r := range sels.Ranges {
			f, l := lineSpan(s, r)
			start, end := s.LineStart(f), s.LineStart(l+1)
			if l+1 >= s.LineCount() {
				end = s.Len()
			}
			if r.From() == start && r.To() == end && l+2 <= s.LineCount() {
				end = s.LineStart(l + 2)
				if l+2 >= s.LineCount() {
					end = s.Len()
				}
			}
			out.Ranges = append(out.Ranges, edit.Range{Anchor: start, Head: end, GoalX: -1})
		}
		b.SetSelections(out)
	})
}

// jumpToBracket 把光标移到与旁边的括号配对的那个括号。
func (e *Editor) jumpToBracket() error {
	return e.withDoc(func(b *edit.Buffer) {
		s := b.Snapshot()
		head := b.Selections().Main().Head
		at, pair, ok := matchBracket(s, head)
		if !ok {
			return
		}
		to := pair
		if head > at {
			to = pair + 1 // 光标在括号哪一边，就停在配对括号的哪一边
		}
		b.SetSelections(edit.Single(edit.Cursor(to)))
	})
}

// maxBracketScan 是找配对括号时最多看多远（字节）。
const maxBracketScan = 40_000

var (
	bracketClosers = map[byte]byte{'(': ')', '[': ']', '{': '}'}
	bracketOpeners = map[byte]byte{')': '(', ']': '[', '}': '{'}
)

// matchBracket 找光标旁边的括号（先看前面）和与它配对的括号，返回两者的偏移。
func matchBracket(s text.Snapshot, caret int) (at, pair int, ok bool) {
	n := s.Len()
	for _, i := range []int{caret - 1, caret} {
		if i < 0 || i >= n {
			continue
		}
		c := s.ByteAt(i)
		if cl, open := bracketClosers[c]; open {
			end := min(n, i+1+maxBracketScan)
			txt := s.String(i+1, end)
			depth := 0
			for k := 0; k < len(txt); k++ {
				switch txt[k] {
				case c:
					depth++
				case cl:
					if depth == 0 {
						return i, i + 1 + k, true
					}
					depth--
				}
			}
			return 0, 0, false
		}
		if op, close := bracketOpeners[c]; close {
			start := max(0, i-maxBracketScan)
			txt := s.String(start, i)
			depth := 0
			for k := len(txt) - 1; k >= 0; k-- {
				switch txt[k] {
				case c:
					depth++
				case op:
					if depth == 0 {
						return i, start + k, true
					}
					depth--
				}
			}
			return 0, 0, false
		}
	}
	return 0, 0, false
}

func isWordRune(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// ---- 括号、引号自动闭合 ----

var autoPairs = map[rune]rune{'(': ')', '[': ']', '{': '}', '"': '"', '\'': '\'', '`': '`'}

// runeAt 返回偏移 i 处的字；越界返回 0。
func runeAt(s text.Snapshot, i int) rune {
	if i < 0 || i >= s.Len() {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(s.String(i, min(i+4, s.Len())))
	return r
}

// runeBefore 返回偏移 i 之前的字。
func runeBefore(s text.Snapshot, i int) rune {
	if i <= 0 {
		return 0
	}
	r, _ := utf8.DecodeLastRuneInString(s.String(max(i-4, 0), i))
	return r
}

// typeRune 打一个字：选区上打左括号或引号就把选区包起来；打的正是编辑器补上的右括号就跨过去；
// 打左括号或引号且后面是行尾、空白或右括号时补上配对的那个。返回 false 表示照常插入。
// 只在有语法高亮（代码）的文件里生效。
func (e *Editor) typeRune(t *tab, r rune) bool {
	d := t.doc
	lang := d.lang()
	if lang == "" {
		return false
	}
	b := d.buf
	s := b.Snapshot()
	sels := b.Selections()
	closeWith, isOpener := autoPairs[r]
	quote := isOpener && closeWith == r
	var cs text.ChangeSet
	after := edit.Selections{Primary: sels.Primary}
	delta := 0
	handled := false
	var newClosers []int
	for _, rg := range sels.Ranges {
		a, z := rg.From(), rg.To()
		switch {
		case a < z && isOpener && (s.LineAt(a) == s.LineAt(z) || !quote):
			// 包住选区
			inner := s.String(a, z)
			cs = append(cs, text.Change{Start: a, End: z, Text: string(r) + inner + string(closeWith)})
			after.Ranges = append(after.Ranges, edit.Range{Anchor: a + delta + 1, Head: a + delta + 1 + len(inner), GoalX: -1})
			delta += 2
			handled = true
		case a == z && runeAt(s, a) == r && slices.Contains(d.closers, a):
			// 跨过编辑器补上的右括号
			after.Ranges = append(after.Ranges, edit.Cursor(a+delta+utf8.RuneLen(r)))
			d.closers = slices.DeleteFunc(d.closers, func(c int) bool { return c == a })
			handled = true
		case a == z && isOpener && e.closes(s, a, r, lang):
			cs = append(cs, text.Change{Start: a, End: a, Text: string(r) + string(closeWith)})
			at := a + delta + utf8.RuneLen(r)
			after.Ranges = append(after.Ranges, edit.Cursor(at))
			newClosers = append(newClosers, at)
			delta += utf8.RuneLen(r) + utf8.RuneLen(closeWith)
			handled = true
		default:
			ins := string(r)
			cs = append(cs, text.Change{Start: a, End: z, Text: ins})
			after.Ranges = append(after.Ranges, edit.Cursor(a+delta+len(ins)))
			delta += len(ins) - (z - a)
		}
	}
	if !handled {
		return false
	}
	if len(cs) > 0 {
		b.Apply(edit.Transaction{Changes: cs, After: after, Origin: edit.OriginTyping})
	} else {
		b.SetSelections(after)
	}
	d.closers = append(d.closers, newClosers...) // OnEdit 已经把旧的挪过了；新的偏移是新快照里的
	return true
}

// closes 报告在 at 处打 r 时要不要补上配对的那个。
func (e *Editor) closes(s text.Snapshot, at int, r rune, lang string) bool {
	next := runeAt(s, at)
	if next != 0 && next != '\n' && next != '\r' && !strings.ContainsRune(" \t)]};,:", next) {
		return false // 在词前面打括号不补
	}
	if autoPairs[r] != r {
		return true
	}
	// 引号：不在词后面（那是撇号），不在同一个引号后面，也看语言用不用它。
	if prev := runeBefore(s, at); isWordRune(prev) || prev == r {
		return false
	}
	switch lang {
	case "markdown", "Markdown", "reStructuredText", "plaintext", "TeX":
		return false
	}
	if r == '\'' && strings.Contains("Go Rust OCaml Haskell", lang) {
		return false
	}
	if r == '`' && !strings.Contains("Go JavaScript TypeScript TSX JSX Markdown Bash Zsh Kotlin Scala", lang) {
		return false
	}
	return true
}

// deletePair 在一对自动补上的括号中间按退格：两个一起删。返回 false 表示照常删除。
func (e *Editor) deletePair(t *tab) bool {
	d := t.doc
	b := d.buf
	s := b.Snapshot()
	var cs text.ChangeSet
	for _, rg := range b.Selections().Ranges {
		a := rg.Head
		if !rg.Empty() || !slices.Contains(d.closers, a) {
			return false
		}
		open := runeBefore(s, a)
		if c, ok := autoPairs[open]; !ok || runeAt(s, a) != c {
			return false
		}
		cs = append(cs, text.Change{Start: a - utf8.RuneLen(open), End: a + utf8.RuneLen(autoPairs[open])})
	}
	if len(cs) == 0 {
		return false
	}
	b.Apply(edit.Transaction{Changes: cs, Origin: edit.OriginTyping})
	return true
}

// trackClosers 让自动补上的右括号随编辑挪动；被删掉或改掉的就忘了它。
func (d *document) trackClosers(ed edit.Edit) {
	if len(d.closers) == 0 {
		return
	}
	out := d.closers[:0]
	for _, c := range d.closers {
		n := ed.Changes.MapPos(c, text.AssocAfter)
		if n < ed.After.Len() && strings.ContainsRune(")]}\"'`", rune(ed.After.ByteAt(n))) {
			out = append(out, n)
		}
	}
	d.closers = out
}

// ---- 排序行、驼峰、缩进方式 ----
// 照着 Mayoi v0.7.0（MIT，cmd/mayoi/keymap.go 的 sortLines、camelCase，commands.go 的缩进方式），
// 偏移改成字节，作用于所有光标。

// sortLines 按字节序排好选区覆盖的行（各个选区各排各的）；只有一个光标、没有选区时排整个文档，
// 末尾的空行留在末尾。行的总长不变，选区原样留着。
func (e *Editor) sortLines(descending bool) error {
	return e.editDoc(func(b *edit.Buffer) {
		s := b.Snapshot()
		sels := b.Selections()
		bls := blocks(s, sels)
		if len(sels.Ranges) == 1 && sels.Main().Empty() {
			last := s.LineCount() - 1
			if last > 0 && s.LineStart(last) == lineEnd(s, last) {
				last-- // 文件末尾的换行符后面那个空行
			}
			bls = [][2]int{{0, last}}
		}
		var cs text.ChangeSet
		for _, bl := range bls {
			if bl[1] <= bl[0] {
				continue
			}
			lines := make([]string, 0, bl[1]-bl[0]+1)
			for i := bl[0]; i <= bl[1]; i++ {
				ls, le := s.LineRange(i)
				lines = append(lines, s.String(ls, le))
			}
			sorted := slices.Clone(lines)
			slices.Sort(sorted)
			if descending {
				slices.Reverse(sorted)
			}
			if slices.Equal(sorted, lines) {
				continue
			}
			start, end := s.LineStart(bl[0]), lineEnd(s, bl[1])
			cs = append(cs, text.Change{Start: start, End: end, Text: strings.Join(sorted, eolOf(s))})
		}
		if len(cs) > 0 {
			b.Apply(edit.Transaction{Changes: cs, After: sels, Origin: edit.OriginCommand})
			b.BreakGroup()
		}
	})
}

// toCamelCase 把用空格、下划线、连字符隔开的词连成驼峰：第一个词原样，同 VS Code。
func toCamelCase(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' || unicode.IsSpace(r) })
	if len(words) == 0 {
		return s
	}
	var b strings.Builder
	b.WriteString(words[0])
	for _, w := range words[1:] {
		r, n := utf8.DecodeRuneInString(w)
		b.WriteRune(unicode.ToUpper(r))
		b.WriteString(w[n:])
	}
	return b.String()
}

// camelCase 把每个选区写成驼峰；没有选区的光标改它所在的词（连同 _ 和 -），改完选中改好的文字。
func (e *Editor) camelCase() error {
	return e.editDoc(func(b *edit.Buffer) {
		s := b.Snapshot()
		sels := b.Selections()
		word := func(r rune) bool { return r == '-' || isWordRune(r) }
		var cs text.ChangeSet
		after := edit.Selections{Primary: sels.Primary}
		delta, prev := 0, -1
		for _, r := range sels.Ranges {
			lo, hi := r.From(), r.To()
			if lo == hi {
				ls, le := s.LineRange(s.LineAt(lo))
				for lo > ls && word(runeBefore(s, lo)) {
					_, n := utf8.DecodeLastRuneInString(s.String(ls, lo))
					lo -= n
				}
				for hi < le && word(runeAt(s, hi)) {
					_, n := utf8.DecodeRuneInString(s.String(hi, min(hi+4, le)))
					hi += n
				}
			}
			if lo < prev { // 两个光标在同一个词里：只改一次
				continue
			}
			prev = hi
			old := s.String(lo, hi)
			out := toCamelCase(old)
			if out != old {
				cs = append(cs, text.Change{Start: lo, End: hi, Text: out})
			}
			after.Ranges = append(after.Ranges, edit.Range{Anchor: lo + delta, Head: lo + delta + len(out), GoalX: -1})
			delta += len(out) - len(old)
		}
		after.Primary = min(after.Primary, len(after.Ranges)-1)
		if len(cs) > 0 {
			b.Apply(edit.Transaction{Changes: cs, After: after, Origin: edit.OriginCommand})
			b.BreakGroup()
		}
	})
}

// setIndent 改当前文档的缩进方式（Tab 键、缩进行、回车后的缩进都按它来），不改已有的文字。
func (e *Editor) setIndent(style indentStyle) error {
	t := e.cur()
	if t == nil {
		return errors.New(i18n.T("No open file"))
	}
	t.doc.indent = style
	e.status = i18n.F("Indentation: %s", style.String())
	return nil
}
