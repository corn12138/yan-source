package app

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/syntax/highlight"
	"github.com/corn12138/yan-source/core/syntax/symbols"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/text/width"
	"github.com/corn12138/yan-source/core/view"
)

// 转到定义（VS Code 的 editor.action.revealDefinition：F12，或按住链接修饰键点击）。
// 砚没有语言服务器：定义按 symbols.Definitions 读行来认，名字一样就算，不看类型、导入和作用域。
// 先找当前文件（光标所在块里就近的局部变量、所在函数的参数，再是文件里的声明），
// 没有再在后台找打开的文件夹里同一种语言的文件。

// maxDefinitionFiles 是在文件夹里找定义时最多读的文件数，maxDefinitionHits 是最多列出的定义数。
const (
	maxDefinitionFiles = 20000
	maxDefinitionHits  = 50
)

// defTarget 是找到的一个定义：文件、行和名字在行里的字节位置。
type defTarget struct {
	path, rel string
	line, col int
	name      string
	kind      string
	detail    string
	depth     int
}

// member 报告定义是不是类型的成员（方法、字段，或缩进在块里的）：「x.名字」只可能是这些，
// 除非 x 是名字所在的包、模块（目录）。
func (d defTarget) member() bool {
	return d.kind == "method" || d.kind == symbols.KindField || d.depth > 0
}

// defState 是正在文件夹里找的定义：gen 换了、光标动了，晚到的结果就丢掉。
// 新的一次查找关掉 stop，旧的不再接着读文件夹：连按 F12 不会叠出好几遍整个文件夹的读取。
type defState struct {
	gen  int
	doc  *document
	ver  uint64
	head int
	stop chan struct{}
}

// LinkModifier 是点击转到定义、悬停时给标识符加下划线要按住的修饰键：keymap.Profile.Mods 里的那个，
// 和终端里点文件链接一样（macOS 窗口版是 Cmd，Windows、Linux 窗口版是 Ctrl，终端版是终端报得出的）。
func (e *Editor) LinkModifier() keymap.Mod {
	var m keymap.Mod
	for _, x := range e.profile.Mods {
		m |= x
	}
	return m
}

// linkClick 报告这次按下是不是按住链接修饰键的点击。
func (e *Editor) linkClick(in MouseInput) bool {
	m := e.LinkModifier()
	return m != 0 && in.Mods&m != 0
}

// definable 报告标签里的文字能不能转到定义：代码文件，不是改动视图、终端或预览。
func definable(t *tab) bool {
	return t != nil && t.term == nil && t.diff == nil && t.mergeSide == nil && !t.markdown && !t.image && !t.svg && isCode(t.doc)
}

// identifierAt 是 pos 处（或紧挨在它左边）的标识符；数字开头的、关键字、注释、字符串里的都不算。
func identifierAt(d *document, s text.Snapshot, pos int) (a, z int, ok bool) {
	line := s.LineAt(pos)
	ls, le := s.LineRange(line)
	txt := s.String(ls, le)
	i := min(max(pos-ls, 0), len(txt))
	if r, _ := utf8.DecodeRuneInString(txt[i:]); i == len(txt) || !nameRune(r) {
		r, n := utf8.DecodeLastRuneInString(txt[:i])
		if n == 0 || !nameRune(r) {
			return 0, 0, false
		}
		i -= n
	}
	a, z = i, i
	for a > 0 {
		r, n := utf8.DecodeLastRuneInString(txt[:a])
		if !nameRune(r) {
			break
		}
		a -= n
	}
	for z < len(txt) {
		r, n := utf8.DecodeRuneInString(txt[z:])
		if !nameRune(r) {
			break
		}
		z += n
	}
	if r, _ := utf8.DecodeRuneInString(txt[a:]); unicode.IsDigit(r) {
		return 0, 0, false
	}
	if d.hl != nil {
		at := utf8.RuneCountInString(txt[:a])
		for _, tk := range d.hl.Line(line, txt) {
			if tk.Start <= at && at < tk.End {
				switch tk.Kind {
				case highlight.Keyword, highlight.String, highlight.Number, highlight.Comment, highlight.Operator,
					highlight.Punctuation, highlight.Constant, highlight.Builtin:
					return 0, 0, false
				}
			}
		}
	}
	return ls + a, ls + z, true
}

func nameRune(r rune) bool { return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// qualifierAt 是 a 处的名字前面的「x.」「x::」「x->」里的 x；没有时为空。
func qualifierAt(s text.Snapshot, a int) string {
	ls := s.LineStart(s.LineAt(a))
	before := s.String(ls, a)
	switch {
	case strings.HasSuffix(before, "?."):
		before = before[:len(before)-2]
	case strings.HasSuffix(before, "."):
		before = before[:len(before)-1]
	case strings.HasSuffix(before, "::"), strings.HasSuffix(before, "->"):
		before = before[:len(before)-2]
	default:
		return ""
	}
	k := len(before)
	for k > 0 {
		r, n := utf8.DecodeLastRuneInString(before[:k])
		if !nameRune(r) {
			break
		}
		k -= n
	}
	if q := before[k:]; q != "" {
		return q
	}
	return "." // a call's result, an index: something, but not a name
}

// selfQualifier 是指着自己的对象：后面的名字多半在同一个文件的同一个类型里。
func selfQualifier(q string) bool { return q == "this" || q == "self" || q == "Self" || q == "super" }

// indentOf 是一行开头的缩进有几格（tab 按 symbols 的算法算）。
func indentOf(s string) int {
	col := 0
	for _, r := range s {
		switch r {
		case '\t':
			col += 4 - col%4
		case ' ':
			col++
		default:
			return col
		}
	}
	return -1 // 空行
}

// scopeOpen 报告从 from 行之后到 to 行，所有非空行的缩进都至少 depth 格：from 那一行开始的块还没结束。
func scopeOpen(s text.Snapshot, from, to, depth int) bool {
	for l := from + 1; l < to; l++ {
		ls, le := s.LineRange(l)
		if in := indentOf(s.String(ls, le)); in >= 0 && in < depth {
			return false
		}
	}
	return true
}

// definitionsInFile 在文件 s 里找 pos 处的名字的定义。at 为真时 pos 就在一个定义上。
func definitionsInFile(s text.Snapshot, path string, name, qual string, pos int) (out []defTarget, at bool) {
	defs := symbols.Definitions(s)
	cur := s.LineAt(pos)
	cls, cle := s.LineRange(cur)
	col := pos - cls
	depth := max(indentOf(s.String(cls, cle)), 0)
	target := func(d symbols.Symbol) defTarget {
		return defTarget{path: path, line: d.Line, col: d.Col, name: name, kind: d.Kind, detail: d.Detail, depth: d.Depth}
	}
	for _, d := range defs {
		if d.Name == name && d.Line == cur && d.Col <= col && col <= d.Col+len(name) {
			return []defTarget{target(d)}, true
		}
	}
	if qual == "" {
		// 就近的局部变量：在光标前面，它所在的块还包着光标。
		for _, d := range slices.Backward(defs) {
			before := d.Line < cur || d.Line == cur && d.Col+len(name) < col
			if d.Name == name && d.Local() && before && d.Depth <= depth && scopeOpen(s, d.Line, cur, d.Depth) {
				return []defTarget{target(d)}, false
			}
		}
		// 包着光标的函数的参数（Go 的接收者也算），由里往外。
		for _, d := range slices.Backward(defs) {
			if d.Line >= cur || d.Depth >= depth || !scopeOpen(s, d.Line, cur, d.Depth+1) {
				continue
			}
			ls, le := s.LineRange(d.Line)
			line := s.String(ls, le)
			if k := symbols.Param(line, d, name); k >= 0 {
				return []defTarget{{path: path, line: d.Line, col: k, name: name, kind: "parameter"}}, false
			}
		}
	}
	for _, d := range defs {
		if t := target(d); d.Name == name && !d.Local() && (qual == "" || t.member()) {
			out = append(out, t)
		}
	}
	slices.SortStableFunc(out, func(a, b defTarget) int { return a.depth - b.depth })
	return out, false
}

// family 是在文件夹里找定义时算同一种语言的文件：JavaScript 和 TypeScript 一家，C、C++、Objective-C 一家，
// 别的按扩展名；没有扩展名的文件不找。
func family(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts", ".vue", ".svelte":
		return "js"
	case ".c", ".h", ".cc", ".cpp", ".cxx", ".hpp", ".hh", ".hxx", ".m", ".mm":
		return "c"
	}
	return ext
}

// revealDefinition 转到光标处的名字的定义（F12）。
func (e *Editor) revealDefinition() error {
	t := e.cur()
	if t == nil {
		return nil
	}
	return e.definitionAt(t, e.sels(t).Main().Head)
}

// definitionAt 转到标签 t 里 pos 处的名字的定义：当前文件里有就跳过去，有几个就让人选，
// 没有就到文件夹里找。
func (e *Editor) definitionAt(t *tab, pos int) error {
	if !definable(t) {
		e.status = "这里不能转到定义"
		return nil
	}
	s := t.doc.buf.Snapshot()
	a, z, ok := identifierAt(t.doc, s, pos)
	if !ok {
		e.status = "光标处没有名字"
		return nil
	}
	name, qual := s.String(a, z), qualifierAt(s, a)
	if selfQualifier(qual) {
		qual = ""
	}
	path := t.doc.buf.Path()
	found, at := definitionsInFile(s, path, name, qual, pos)
	if at {
		e.status = name + " 就定义在这里"
		return nil
	}
	if len(found) > 0 {
		e.showDefinitions(name, found)
		return nil
	}
	if e.ws == nil || path == "" || family(path) == "" {
		e.status = "找不到 " + name + " 的定义"
		return nil
	}
	e.searchDefinition(t, name, qual, path)
	return nil
}

// showDefinitions 跳到唯一的定义，或者列出几个让人选。
func (e *Editor) showDefinitions(name string, found []defTarget) {
	if len(found) == 1 {
		e.jumpToDefinition(found[0])
		return
	}
	items := make([]ovItem, 0, len(found))
	for _, d := range found {
		where := fmt.Sprintf("%s:%d", d.rel, d.line+1)
		if d.rel == "" {
			where = fmt.Sprintf("第 %d 行", d.line+1)
		}
		items = append(items, ovItem{label: name, detail: strings.TrimSpace(where + "  " + d.detail), hint: symbolKind(d.kind),
			run: func(e *Editor) error { e.jumpToDefinition(d); return nil }})
	}
	e.choose(fmt.Sprintf("%s 的 %d 个定义", name, len(found)), items...)
}

// jumpToDefinition 记下现在的位置（后退回得来），打开定义所在的文件并选中名字。
func (e *Editor) jumpToDefinition(d defTarget) {
	e.pushNav()
	if t := e.cur(); t == nil || t.doc.buf.Path() != d.path {
		if err := e.openPath(d.path); err != nil {
			e.status = err.Error()
			return
		}
		e.sync()
	}
	t := e.cur()
	if t == nil {
		return
	}
	b := t.doc.buf
	s := b.Snapshot()
	line := min(d.line, s.LineCount()-1)
	ls, le := s.LineRange(line)
	from := min(ls+d.col, le)
	to := min(from+len(d.name), le)
	b.SetSelections(edit.Single(edit.Range{Anchor: from, Head: to, GoalX: -1}))
	b.BreakGroup()
	e.centerLine(t, line)
	e.area = areaEditor
	e.reveal()
	e.status = ""
}

// searchDefinition 在后台读文件夹里同一种语言的文件找 name 的定义；有未保存修改的按编辑器里的文字找。
func (e *Editor) searchDefinition(t *tab, name, qual, path string) {
	fam := family(path)
	var rels []string
	for _, rel := range e.ws.index {
		if family(rel) == fam {
			rels = append(rels, rel)
			if len(rels) == maxDefinitionFiles {
				break
			}
		}
	}
	texts := map[string]searchText{}
	for _, d := range e.docs {
		if d.buf.Dirty() {
			if rel := e.rel(d.buf.Path()); !filepath.IsAbs(rel) {
				snap := d.buf.Snapshot()
				texts[rel] = searchText{snap.String(0, snap.Len()), d, snap.Version()}
			}
		}
	}
	if e.def.stop != nil {
		close(e.def.stop)
	}
	e.def.gen++
	e.def.doc, e.def.ver, e.def.head = t.doc, t.doc.buf.Snapshot().Version(), e.sels(t).Main().Head
	e.def.stop = make(chan struct{})
	gen, root, here, stop := e.def.gen, e.root, e.rel(path), e.def.stop
	post := e.postHere()
	e.status = "正在找 " + name + " 的定义…"
	go func() {
		var found []defTarget
		for _, rel := range rels {
			select {
			case <-stop:
				return // 已有新的查找：这次的结果反正要丢掉
			default:
			}
			if rel == here {
				continue
			}
			txt, ok := texts[rel].text, false
			if _, dirty := texts[rel]; dirty {
				ok = true
			} else {
				txt, _, ok = readSearchText(filepath.Join(root, filepath.FromSlash(rel)))
			}
			if !ok || !strings.Contains(txt, name) {
				continue
			}
			for _, d := range symbols.Definitions(text.FromString(txt, text.Meta{})) {
				if d.Name == name && !d.Local() {
					found = append(found, defTarget{path: filepath.Join(root, filepath.FromSlash(rel)), rel: rel,
						line: d.Line, col: d.Col, name: name, kind: d.Kind, detail: d.Detail, depth: d.Depth})
				}
			}
			if len(found) >= 4*maxDefinitionHits {
				break
			}
		}
		post(func(e *Editor) { e.takeDefinitions(gen, name, qual, here, found) })
	}()
}

// takeDefinitions 收下文件夹里找到的定义：限定名所在的目录、同一个目录里的优先，只留最好的那一档。
func (e *Editor) takeDefinitions(gen int, name, qual, here string, found []defTarget) {
	if gen != e.def.gen {
		return
	}
	if e.status == "正在找 "+name+" 的定义…" {
		e.status = ""
	}
	t := e.cur()
	if t == nil || t.doc != e.def.doc || t.doc.buf.Snapshot().Version() != e.def.ver || e.sels(t).Main().Head != e.def.head {
		return // 光标已经走了：不再跳
	}
	dir := filepath.ToSlash(filepath.Dir(here))
	rank := func(d defTarget) int {
		in := filepath.ToSlash(filepath.Dir(d.rel))
		switch {
		case qual != "" && qual != "." && filepath.Base(in) == qual:
			return 0
		case in == dir:
			return 1
		}
		return 2
	}
	best := 3
	for _, d := range found {
		best = min(best, rank(d))
	}
	found = slices.DeleteFunc(found, func(d defTarget) bool {
		return rank(d) != best || qual != "" && best != 0 && !d.member()
	})
	slices.SortStableFunc(found, func(a, b defTarget) int {
		if a.rel != b.rel {
			return strings.Compare(a.rel, b.rel)
		}
		return a.line - b.line
	})
	if len(found) == 0 {
		e.status = "找不到 " + name + " 的定义"
		return
	}
	e.showDefinitions(name, found[:min(len(found), maxDefinitionHits)])
}

// LinkAt 是列 col 的窗格里格子 (x, y) 处、按住链接修饰键时要加下划线的名字：在第 y 行，
// 从格子 from 到 to（同 Caret.X，含行号栏）。不是能转到定义的名字时 ok 为假。
func (e *Editor) LinkAt(col int, x, y view.Unit) (from, to view.Unit, ok bool) {
	if col < 0 || col >= len(e.cols) {
		return 0, 0, false
	}
	c := e.cols[col]
	t := c.tab()
	if !definable(t) {
		return 0, 0, false
	}
	s := t.doc.buf.Snapshot()
	gutter := e.gutter(s)
	if x < gutter {
		return 0, 0, false
	}
	if h := int(c.size.H); t.scrollY > 0 && h > 6 && int(y) < len(e.stickyLines(s, t.scrollY, min(maxSticky, h/3), map[int]int{})) {
		return 0, 0, false // 钉在顶上的行：点它是跳到那个块
	}
	w := e.wrapWidth(c, t, s)
	row, left := e.stepRows(s, w, e.topRow(t, s, w), int(y))
	if left > 0 {
		return 0, 0, false
	}
	a, z, ok := identifierAt(t.doc, s, e.posAt(c, t, x, y))
	if !ok {
		return 0, 0, false
	}
	scroll := t.scrollX
	if w > 0 {
		scroll = 0
	}
	at, ax := e.rowAt(s, w, a)
	end, zx := e.rowAt(s, w, z)
	if at != row {
		return 0, 0, false
	}
	if end != row { // 名字写到了这一行的末尾，或者被折行折断了
		r, n := utf8.DecodeLastRuneInString(s.String(a, z))
		lr, lx := e.rowAt(s, w, z-n)
		if lr != row {
			return 0, 0, false
		}
		zx = lx + view.Unit(max(width.String(string(r)), 1))
	}
	from, to = gutter+ax-scroll, gutter+zx-scroll
	if x < from || x > to || to <= from {
		return 0, 0, false
	}
	return max(from, gutter), to, true
}
