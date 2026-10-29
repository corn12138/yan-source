package app

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
)

// 文件内查找与替换，移植自 Mayoi（MIT，cmd/mayoi/find.go）：区分大小写、全字匹配、正则（替换里 $1 引用分组）；
// 查找框在有焦点那一列的右上角，作用于那一列的文件；匹配在文件里都标出来，当前匹配更亮。

// maxMatches 是最多数几个匹配。
const maxMatches = 20000

// findField 是查找框里有键盘焦点的输入框。
type findField uint8

const (
	fieldNone findField = iota
	fieldQuery
	fieldReplace
)

type findState struct {
	open, replacing        bool
	selectGen              uint64 // Reinvoking Find must select the field even when focus is unchanged.
	query, with            string
	matchCase, word, regex bool

	matches [][2]int // 匹配的字节区间
	cur     int      // 当前匹配；-1 表示没有
	err     string

	key     string // 匹配是按哪个查询、哪个文档、哪个版本找的
	doc     *document
	version uint64
	re      *regexp.Regexp
}

func (f *findState) pattern() string {
	return fmt.Sprintf("%v|%v|%v|%s", f.matchCase, f.word, f.regex, f.query)
}

// compileQuery 把查询和选项编译成正则；查找和项目搜索共用。
func compileQuery(query string, matchCase, word, regex bool) (*regexp.Regexp, error) {
	expr := query
	if !regex {
		expr = regexp.QuoteMeta(expr)
	}
	if word {
		expr = `\b` + expr + `\b`
	}
	if !matchCase {
		expr = "(?i)" + expr
	}
	return regexp.Compile("(?m)" + expr)
}

// replaceTemplate 是替换用的模板：正则时 $1 引用分组，否则原样（$ 要转义）。
func replaceTemplate(with string, regex bool) string {
	if regex {
		return with
	}
	return strings.ReplaceAll(with, "$", "$$")
}

// openFind 打开查找框（replace 为真时连替换框一起），选区在一行之内时用它做查询。
func (e *Editor) openFind(replace bool) error {
	if t := e.cur(); t != nil && t.term != nil {
		return userError("请在文件标签中查找")
	}
	f := &e.find
	if t := e.cur(); t != nil {
		s := t.doc.buf.Snapshot()
		if r := t.doc.buf.Selections().Main(); !r.Empty() {
			if txt := s.String(r.From(), r.To()); !strings.Contains(txt, "\n") && len(txt) < 200 {
				f.query = txt
				f.key = "" // 重新找
			}
		}
	}
	f.replacing = replace || f.open && f.replacing
	f.open = true
	f.selectGen++
	e.findFocus = fieldQuery
	if replace && f.query != "" {
		e.findFocus = fieldReplace
	}
	e.closeOverlay()
	e.search(true)
	return nil
}

func (e *Editor) closeFind() {
	e.find.open = false
	e.findFocus = fieldNone
}

// search 找当前文档里的匹配（文字和查询都没变就不找）。fromCaret 为真时把光标后的第一个匹配作为当前匹配。
func (e *Editor) search(fromCaret bool) {
	f := &e.find
	t := e.cur()
	if t == nil {
		f.matches, f.cur = f.matches[:0], -1
		return
	}
	s := t.doc.buf.Snapshot()
	key := f.pattern()
	if f.key == key && f.doc == t.doc && f.version == s.Version() {
		return
	}
	changed := f.key != key || f.doc != t.doc
	f.key, f.doc, f.version = key, t.doc, s.Version()
	f.matches, f.err, f.re = f.matches[:0], "", nil
	if f.query == "" {
		f.cur = -1
		return
	}
	re, err := compileQuery(f.query, f.matchCase, f.word, f.regex)
	if err != nil {
		f.err, f.cur = "正则写错了", -1
		return
	}
	f.re = re
	f.matches = scanMatches(re, s, f.matches)
	if changed || fromCaret || f.cur >= len(f.matches) {
		caret := t.doc.buf.Selections().Main().From()
		f.cur = sort.Search(len(f.matches), func(i int) bool { return f.matches[i][1] > caret })
		if f.cur == len(f.matches) {
			f.cur = 0
		}
		if len(f.matches) == 0 {
			f.cur = -1
		}
	}
}

// scanMatches 返回 re 在快照里的匹配（字节区间），跳过空匹配。可以在任意 goroutine 上调用。
func scanMatches(re *regexp.Regexp, s text.Snapshot, dst [][2]int) [][2]int {
	txt := s.String(0, s.Len())
	for _, m := range re.FindAllStringIndex(txt, maxMatches) {
		if m[0] != m[1] {
			dst = append(dst, [2]int{m[0], m[1]})
		}
	}
	return dst
}

// step 走到下一个（或上一个）匹配并选中它，到头绕回。
func (e *Editor) step(by int) error {
	f := &e.find
	if !f.open {
		e.openFind(false)
	}
	e.search(false)
	if len(f.matches) == 0 {
		return nil
	}
	if f.cur < 0 {
		f.cur = 0
	} else {
		f.cur = (f.cur + by + len(f.matches)) % len(f.matches)
	}
	e.selectMatch()
	return nil
}

// selectMatch 选中当前匹配并滚到它。
func (e *Editor) selectMatch() {
	f := &e.find
	t := e.cur()
	if t == nil || f.cur < 0 || f.cur >= len(f.matches) {
		return
	}
	m := f.matches[f.cur]
	t.doc.buf.SetSelections(edit.Single(edit.Range{Anchor: m[0], Head: m[1], GoalX: -1}))
	t.doc.buf.BreakGroup()
	e.centerLine(t, t.doc.buf.Snapshot().LineAt(m[0]))
	e.reveal()
}

// replaceOne 替换当前匹配，走到下一个。
func (e *Editor) replaceOne() error {
	f := &e.find
	e.search(false)
	t := e.cur()
	if t == nil || f.cur < 0 || f.cur >= len(f.matches) || f.re == nil || !e.writable(t) {
		return nil
	}
	b := t.doc.buf
	m := f.matches[f.cur]
	with := f.re.ReplaceAllString(b.Snapshot().String(m[0], m[1]), replaceTemplate(f.with, f.regex))
	end := m[0] + len(with)
	b.Apply(edit.Transaction{Changes: text.ChangeSet{{Start: m[0], End: m[1], Text: with}},
		After: edit.Single(edit.Cursor(end)), Origin: edit.OriginCommand})
	b.BreakGroup()
	e.search(false)
	if len(f.matches) > 0 {
		f.cur = sort.Search(len(f.matches), func(i int) bool { return f.matches[i][0] >= end })
		if f.cur == len(f.matches) {
			f.cur = 0
		}
		e.selectMatch()
	}
	return nil
}

// replaceAll 替换所有匹配，作为一步撤销。
func (e *Editor) replaceAll() error {
	f := &e.find
	e.search(false)
	t := e.cur()
	if t == nil || len(f.matches) == 0 || f.re == nil || !e.writable(t) {
		return nil
	}
	b := t.doc.buf
	s := b.Snapshot()
	tmpl := replaceTemplate(f.with, f.regex)
	cs := make(text.ChangeSet, len(f.matches))
	for i, m := range f.matches {
		cs[i] = text.Change{Start: m[0], End: m[1], Text: f.re.ReplaceAllString(s.String(m[0], m[1]), tmpl)}
	}
	n := len(cs)
	b.Apply(edit.Transaction{Changes: cs, Origin: edit.OriginCommand})
	b.BreakGroup()
	e.search(false)
	e.status = fmt.Sprintf("替换了 %d 处", n)
	return nil
}

func (e *Editor) toggleFindOption(opt *bool) error {
	*opt = !*opt
	if !e.find.open {
		e.openFind(false)
	}
	e.search(true)
	return nil
}

// handleFindKey 处理查找框有焦点时的按键；返回 false 表示交给全局 keymap。
func (e *Editor) handleFindKey(c keymap.Chord) bool {
	f := &e.find
	if c.Mods&^keymap.ModShift != 0 {
		return false
	}
	field := &f.query
	if e.findFocus == fieldReplace {
		field = &f.with
	}
	switch c.Key {
	case keymap.KeyEsc:
		e.closeFind()
	case keymap.KeyEnter:
		switch {
		case e.findFocus == fieldReplace:
			e.replaceOne()
		case c.Mods == keymap.ModShift:
			e.step(-1)
		default:
			e.step(1)
		}
	case keymap.KeyTab:
		if f.replacing {
			e.findFocus = map[findField]findField{fieldQuery: fieldReplace, fieldReplace: fieldQuery}[e.findFocus]
		}
	case keymap.KeyBackspace:
		if *field != "" {
			_, n := utf8.DecodeLastRuneInString(*field)
			*field = (*field)[:len(*field)-n]
			e.search(true)
		}
	case keymap.KeyDown, keymap.KeyUp:
		e.findFocus = fieldNone // 方向键回到编辑器
		return false
	default:
		return c.Key == keymap.KeyRune && c.Mods&^keymap.ModShift == 0
	}
	return true
}

// findText 把打的字追加到查找框有焦点的输入框里。
func (e *Editor) findText(s string) {
	s = strings.NewReplacer("\n", "", "\r", "").Replace(s)
	if e.findFocus == fieldReplace {
		e.find.with += s
		return
	}
	e.find.query += s
	e.search(true)
	e.selectMatch()
}

// findSpans 给一行加上查找匹配的底色。
func (e *Editor) findSpans(row *view.RowFrame, d *document, ls int) {
	f := &e.find
	if !f.open || f.doc != d || len(f.matches) == 0 {
		return
	}
	le := ls + len(row.Text)
	i := sort.Search(len(f.matches), func(i int) bool { return f.matches[i][1] > ls })
	for ; i < len(f.matches) && f.matches[i][0] <= le; i++ {
		m := f.matches[i]
		st := style.SearchHit
		if i == f.cur {
			st = style.SearchCurrent
		}
		a, z := max(m[0], ls)-ls, min(m[1], le)-ls
		if m[1] > le {
			z++ // 匹配跨过行尾（正则）：多标一格表示换行符
		}
		if z > a {
			row.Spans = append(row.Spans, style.Span{Start: int32(a), End: int32(z), Style: st})
		}
	}
}

func (e *Editor) findFrame(dst *view.Find) {
	f := &e.find
	*dst = view.Find{Visible: f.open}
	if !f.open {
		return
	}
	e.search(false)
	dst.Replacing, dst.Query, dst.With = f.replacing, f.query, f.with
	dst.MatchCase, dst.Word, dst.Regex = f.matchCase, f.word, f.regex
	dst.Focus = view.FindField(e.findFocus)
	dst.SelectGen = f.selectGen
	switch {
	case f.err != "":
		dst.Count, dst.Error = f.err, true
	case f.query == "":
	case len(f.matches) == 0:
		dst.Count, dst.Error = "无结果", true
	case len(f.matches) >= maxMatches:
		dst.Count = fmt.Sprintf("%d / %d+", f.cur+1, maxMatches)
	default:
		dst.Count = fmt.Sprintf("%d / %d", f.cur+1, len(f.matches))
	}
}

// ---- 前端用的语义动作（GUI 的查找框是原生输入框） ----

// SetFindQuery 用前端输入框里的文字替换查询。
func (e *Editor) SetFindQuery(s string) {
	e.sync()
	defer e.sync()
	e.find.query = s
	e.search(true)
	e.selectMatch()
}

// SetFindReplace 用前端输入框里的文字替换「替换为」。
func (e *Editor) SetFindReplace(s string) { e.find.with = s }

// FocusFind 记下前端查找框里哪个输入框有焦点（0 没有、1 查询、2 替换）。
func (e *Editor) FocusFind(field int) { e.findFocus = findField(field) }
