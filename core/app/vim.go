package app

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/text"
)

// Vim 模式，移植自 Mayoi（MIT，Copyright (c) 2026 chencheng，cmd/mayoi/vim.go）。
// 每个标签有自己的模式，从普通模式开始，光标是块。
//   - 移动：h j k l、w b e ge、W B E gE、0 ^ $ g_、gg G、f F t T 和 ; ,、%、{ }、回车和 -；可以带次数。
//   - 操作符：d c y、> <、gc 注释、g~ gu gU 改大小写；接移动或文本对象（iw aw iW aW、引号、括号），
//     连按两次作用于整行（dd）。
//   - 命令：x X D C Y s S r J ~ p P o O，i a I A 进插入模式，u 和 ⌃R 撤销重做，. 重复上次修改，
//     ⌃D ⌃U ⌃F ⌃B 翻半页、一页，⌃E ⌃Y 滚一行。
//   - 可视模式：v 按字、V 按行；移动扩展选区，o 换端，操作符、r、J、~ u U、p 作用于选区。
// 插入模式里 Esc、⌃[、⌃C 回到普通模式；一次修改和随后打的字合成一步撤销。
// Mayoi 的偏移是 rune，yan 的是字节：这里在 rune 边界上走，行尾（不论 LF 还是 CRLF）当作一个 '\n'。

type vimMode uint8

const (
	vimNormal vimMode = iota
	vimInsert
	vimVisual
	vimVisualLine
)

func (m vimMode) String() string {
	switch m {
	case vimInsert:
		return "插入"
	case vimVisual:
		return "可视"
	case vimVisualLine:
		return "可视行"
	}
	return "普通"
}

func (m vimMode) visual() bool { return m == vimVisual || m == vimVisualLine }

type vimState struct {
	mode           vimMode
	keys           []rune
	anchor, cursor int // 可视模式下选区的两端（两端的字都选中）
	want           int // j k 保持的列
	hasWant        bool
	find           vimCmd // 上次的 f F t T
	last           vimCmd // 上次的修改，. 重复它
	hasLast        bool
	lastText       string // 上次修改之后打的字
	recording      bool
	insAt, insLen  int
	replay         bool
	group          bool // 插入模式正在合进上一次修改的撤销步
}

// label 是状态栏上显示的模式和打了一半的命令。
func (v *vimState) label() string {
	s := "-- " + v.mode.String() + " --"
	if len(v.keys) > 0 {
		var b strings.Builder
		for _, r := range v.keys {
			if r < ' ' {
				b.WriteString("^" + string(r+'@'))
			} else {
				b.WriteRune(r)
			}
		}
		s += "  " + b.String()
	}
	return s
}

type vimRegister struct {
	text     string
	linewise bool
}

type vimCmd struct {
	count int
	op    string // d c y > < gc g~ gu gU；"" 表示没有
	key   string // 移动、文本对象或命令；操作符连按两次是 "line"
	arg   rune   // f F t T r 的字
}

const (
	vimMore = iota
	vimDone
	vimBad
)

var vimOps = map[string]bool{"d": true, "c": true, "y": true, ">": true, "<": true, "gc": true, "g~": true, "gu": true, "gU": true}

const vimActions = "xXDCYpPoOJsS~u.iaIAvV\x04\x15\x06\x02\x05\x19\x12\x7f"

var vimVisualActions = map[string]bool{
	"d": true, "x": true, "X": true, "D": true, "y": true, "Y": true, "c": true, "s": true, "C": true, "S": true,
	"R": true, ">": true, "<": true, "~": true, "u": true, "U": true, "J": true, "p": true, "P": true,
	"gc": true, "g~": true, "gu": true, "gU": true, "o": true, "v": true, "V": true, "\x1b": true,
}

func parseVim(keys []rune, mode vimMode) (vimCmd, int) {
	var c vimCmd
	i := 0
	count := func() int {
		n := 0
		for i < len(keys) && (keys[i] >= '1' && keys[i] <= '9' || n > 0 && keys[i] == '0') {
			n = n*10 + int(keys[i]-'0')
			i++
		}
		return n
	}
	n1 := count()
	c.count = n1
	if i == len(keys) {
		return c, vimMore
	}
	k := keys[i]
	i++
	name := string(k)
	if k == 'g' {
		if i == len(keys) {
			return c, vimMore
		}
		name += string(keys[i])
		i++
	}
	if mode.visual() {
		switch {
		case vimVisualActions[name]:
			c.key = name
			return c, vimDone
		case name == "r":
			if i == len(keys) {
				return c, vimMore
			}
			if keys[i] < ' ' {
				return c, vimBad
			}
			c.key, c.arg = name, keys[i]
			return c, vimDone
		}
		return parseMotion(c, keys, i, k, name, true)
	}
	switch {
	case k == '\x1b':
		return c, vimBad
	case vimOps[name]:
		c.op = name
		n2 := count()
		if n1 > 0 || n2 > 0 {
			c.count = max(n1, 1) * max(n2, 1)
		}
		if i == len(keys) {
			return c, vimMore
		}
		k2 := keys[i]
		i++
		if last, _ := utf8.DecodeLastRuneInString(name); k2 == last {
			c.key = "line"
			return c, vimDone
		}
		return parseMotion(c, keys, i, k2, string(k2), true)
	case name == "r":
		if i == len(keys) {
			return c, vimMore
		}
		if keys[i] < ' ' {
			return c, vimBad
		}
		c.key, c.arg = name, keys[i]
		return c, vimDone
	case len(name) == 1 && strings.ContainsRune(vimActions, k):
		c.key = name
		return c, vimDone
	}
	return parseMotion(c, keys, i, k, name, false)
}

func parseMotion(c vimCmd, keys []rune, i int, k rune, name string, objects bool) (vimCmd, int) {
	switch k {
	case 'h', 'j', 'k', 'l', 'w', 'W', 'b', 'B', 'e', 'E', '0', '^', '$', 'G', '%', '{', '}', ';', ',',
		'\r', '-', '+', '_', ' ', '\b':
		c.key = string(k)
		return c, vimDone
	case 'g':
		if len(name) < 2 {
			if i == len(keys) {
				return c, vimMore
			}
			name += string(keys[i])
		}
		switch name {
		case "gg", "ge", "gE", "g_":
			c.key = name
			return c, vimDone
		}
	case 'f', 'F', 't', 'T':
		if i == len(keys) {
			return c, vimMore
		}
		if keys[i] < ' ' {
			return c, vimBad
		}
		c.key, c.arg = string(k), keys[i]
		return c, vimDone
	case 'i', 'a':
		if !objects {
			break
		}
		if i == len(keys) {
			return c, vimMore
		}
		if strings.ContainsRune("wW\"'`()b[]{}B<>", keys[i]) {
			c.key = string(k) + string(keys[i])
			return c, vimDone
		}
	}
	return c, vimBad
}

func (c vimCmd) next(mode vimMode) vimMode {
	if mode.visual() {
		switch c.key {
		case "v":
			if mode == vimVisual {
				return vimNormal
			}
			return vimVisual
		case "V":
			if mode == vimVisualLine {
				return vimNormal
			}
			return vimVisualLine
		case "c", "s", "C", "S", "R":
			return vimInsert
		case "o":
			return mode
		}
		if vimVisualActions[c.key] || c.key == "r" {
			return vimNormal
		}
		return mode
	}
	switch {
	case c.op == "c":
		return vimInsert
	case c.op != "":
		return vimNormal
	}
	switch c.key {
	case "i", "a", "I", "A", "o", "O", "s", "S", "C":
		return vimInsert
	case "v":
		return vimVisual
	case "V":
		return vimVisualLine
	}
	return vimNormal
}

func (c vimCmd) changes() bool {
	if c.op != "" {
		return c.op != "y"
	}
	switch c.key {
	case "x", "\x7f", "X", "D", "C", "s", "S", "r", "J", "p", "P", "~", "o", "O", "i", "a", "I", "A":
		return true
	}
	return false
}

// ---- 按键 ----

// vimActive 报告当前标签是不是在用 vim 的键（不是插入模式）。
func (e *Editor) vimActive() (*tab, bool) {
	if !e.cfg.Vim || e.ov != nil || e.area != areaEditor || e.find.open && e.findFocus != fieldNone {
		return nil, false
	}
	t := e.cur()
	if t == nil || t.image || t.svg || t.markdown {
		return nil, false
	}
	return t, true
}

// vimChordRune 把按键换成 vim 的字：Ctrl 只认 vim 自己用的几个（其余仍是快捷键）；方向键是 hjkl。
func vimChordRune(c keymap.Chord) (rune, bool) {
	if c.Mods&(keymap.ModAlt|keymap.ModSuper) != 0 {
		return 0, false
	}
	if c.Mods&keymap.ModCtrl != 0 {
		if c.Key == keymap.KeyRune {
			switch c.Rune {
			case 'd', 'u', 'f', 'b', 'e', 'y', 'r':
				return c.Rune - 'a' + 1, true
			case '[':
				return '\x1b', true
			}
		}
		return 0, false
	}
	switch c.Key {
	case keymap.KeyRune:
		r := c.Rune
		if c.Mods&keymap.ModShift != 0 && r >= 'a' && r <= 'z' {
			r = unicode.ToUpper(r)
		}
		return r, true
	case keymap.KeyEnter:
		return '\r', true
	case keymap.KeyEsc:
		return '\x1b', true
	case keymap.KeyBackspace:
		return '\b', true
	case keymap.KeyDelete:
		return '\x7f', true
	case keymap.KeyLeft:
		return 'h', true
	case keymap.KeyRight:
		return 'l', true
	case keymap.KeyUp:
		return 'k', true
	case keymap.KeyDown:
		return 'j', true
	case keymap.KeyHome:
		return '0', true
	case keymap.KeyEnd:
		return '$', true
	case keymap.KeyPgDn:
		return '\x06', true
	case keymap.KeyPgUp:
		return '\x02', true
	}
	return 0, false
}

// vimKey 处理 vim 模式下的按键；返回 false 表示交给 keymap（Cmd 组合键照常是快捷键）。
func (e *Editor) vimKey(c keymap.Chord) bool {
	t, ok := e.vimActive()
	if !ok {
		return false
	}
	v := &t.vim
	if v.mode == vimInsert {
		esc := c.Mods == 0 && c.Key == keymap.KeyEsc ||
			c.Mods == keymap.ModCtrl && c.Key == keymap.KeyRune && (c.Rune == '[' || c.Rune == 'c')
		if !esc {
			return false
		}
		v.mode = vimNormal
		e.vimEndInsert(t)
		return true
	}
	r, ok := vimChordRune(c)
	if !ok {
		return false
	}
	e.vimRune(t, r)
	return true
}

// vimText 处理普通、可视模式下打的字：每个字都是命令的一个键。
func (e *Editor) vimText(s string) bool {
	t, ok := e.vimActive()
	if !ok || t.vim.mode == vimInsert {
		return false
	}
	for _, r := range s {
		if t.vim.mode == vimInsert { // 命令进了插入模式：剩下的字照常打
			e.handleText(TextInput{Text: s[strings.IndexRune(s, r):]})
			return true
		}
		e.vimRune(t, r)
	}
	return true
}

func (e *Editor) vimRune(t *tab, r rune) {
	v := &t.vim
	b := t.doc.buf
	if sels := b.Selections(); v.mode == vimNormal && !sels.Main().Empty() {
		// 鼠标拖出来的选区：当作可视模式的
		m := sels.Main()
		v.mode = vimVisual
		lo, hi := m.From(), m.To()
		nt := newVText(b.Snapshot())
		if m.Head < m.Anchor {
			v.anchor, v.cursor = nt.prev(hi), lo
		} else {
			v.anchor, v.cursor = lo, nt.prev(hi)
		}
	}
	v.keys = append(v.keys, r)
	cmd, st := parseVim(v.keys, v.mode)
	switch st {
	case vimMore:
		return
	case vimBad:
		v.keys = v.keys[:0]
		return
	}
	v.keys = v.keys[:0]
	mode := v.mode
	next := cmd.next(mode)
	v.mode = next
	if mode.visual() {
		e.vimVisualRun(t, cmd, mode, next)
	} else {
		e.vimNormalRun(t, cmd, next)
	}
	if v.mode == vimNormal {
		e.vimSettle(t)
	}
	e.reveal()
}

// ---- 光标与文字 ----

func (e *Editor) vimPut(t *tab, c int) {
	t.doc.buf.SetSelections(edit.Single(edit.Cursor(c)))
	t.doc.buf.BreakGroup()
}

// vimLast 是 vim 数的最后一行：文件末尾换行之后的空行不算。
func vimLast(s text.Snapshot) int {
	last := s.LineCount() - 1
	if ls, le := s.LineRange(last); last > 0 && ls == le {
		last--
	}
	return last
}

// vimClamp 让普通模式的光标落在一个字上：不在行尾的换行上，也不在最后的空行上。
func vimClamp(s text.Snapshot, c int) int {
	c = min(max(c, 0), s.Len())
	l := min(s.LineAt(c), vimLast(s))
	ls, le := s.LineRange(l)
	if le == ls {
		return ls
	}
	last := newVText(s).prev(le) // 行里最后一个字
	return min(max(c, ls), last)
}

func (e *Editor) vimSettle(t *tab) {
	b := t.doc.buf
	s := b.Snapshot()
	c := vimClamp(s, b.Selections().Main().Head)
	if m := b.Selections().Main(); !m.Empty() || m.Head != c {
		e.vimPut(t, c)
	}
}

// vtext 按行读文字：偏移是字节，在 rune 边界上走；行尾当作一个 '\n'。
type vtext struct {
	s      text.Snapshot
	n      int
	line   int
	ls, le int
	str    string
}

func newVText(s text.Snapshot) *vtext { return &vtext{s: s, n: s.Len(), line: -1} }

func (t *vtext) load(i int) {
	if t.line < 0 || i < t.ls || i > t.le {
		t.line = t.s.LineAt(i)
		t.ls, t.le = t.s.LineRange(t.line)
		t.str = t.s.String(t.ls, t.le)
	}
}

// at 返回 i 处的字：行尾是 '\n'，文末之后是 0。
func (t *vtext) at(i int) rune {
	if i < 0 || i >= t.n {
		return 0
	}
	t.load(i)
	if i >= t.le {
		return '\n'
	}
	r, _ := utf8.DecodeRuneInString(t.str[i-t.ls:])
	return r
}

// next 返回 i 之后那个字的偏移（行尾跳到下一行行首）。
func (t *vtext) next(i int) int {
	if i >= t.n {
		return t.n
	}
	t.load(i)
	if i >= t.le {
		return min(t.s.LineStart(t.line+1), t.n)
	}
	_, size := utf8.DecodeRuneInString(t.str[i-t.ls:])
	return i + size
}

// prev 返回 i 之前那个字的偏移（行首跳到上一行的行尾）。
func (t *vtext) prev(i int) int {
	if i <= 0 {
		return 0
	}
	if i > t.n {
		i = t.n
	}
	t.load(i)
	if i == t.ls {
		_, le := t.s.LineRange(t.line - 1)
		return le
	}
	_, size := utf8.DecodeLastRuneInString(t.str[:i-t.ls])
	return i - size
}

// step 走 n 个字（负数往回），停在 [lo, hi] 里。
func (t *vtext) step(i, n, lo, hi int) int {
	for ; n > 0 && i < hi; n-- {
		i = t.next(i)
	}
	for ; n < 0 && i > lo; n++ {
		i = t.prev(i)
	}
	return min(max(i, lo), hi)
}

func vimSpace(r rune) bool { return r == 0 || unicode.IsSpace(r) }

func vimClass(r rune, big bool) int {
	switch {
	case vimSpace(r):
		return 0
	case big || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
		return 1
	}
	return 2
}

func wordFwd(t *vtext, i int, big bool) int {
	if i >= t.n {
		return t.n
	}
	if c := vimClass(t.at(i), big); c != 0 {
		for i < t.n && vimClass(t.at(i), big) == c {
			i = t.next(i)
		}
	}
	for i < t.n {
		r := t.at(i)
		if r == '\n' {
			i = t.next(i)
			if i < t.n && t.at(i) == '\n' {
				return i
			}
			continue
		}
		if !vimSpace(r) {
			break
		}
		i = t.next(i)
	}
	return i
}

func wordEnd(t *vtext, i int, big bool) int {
	if t.next(i) >= t.n {
		return i
	}
	i = t.next(i)
	for t.next(i) < t.n && vimSpace(t.at(i)) {
		i = t.next(i)
	}
	c := vimClass(t.at(i), big)
	for t.next(i) < t.n && c != 0 && vimClass(t.at(t.next(i)), big) == c {
		i = t.next(i)
	}
	return i
}

func wordBack(t *vtext, i int, big bool) int {
	if i <= 0 {
		return 0
	}
	i = t.prev(i)
	for i > 0 && vimSpace(t.at(i)) {
		if t.at(i) == '\n' && t.at(t.prev(i)) == '\n' {
			return i
		}
		i = t.prev(i)
	}
	c := vimClass(t.at(i), big)
	for i > 0 && c != 0 && vimClass(t.at(t.prev(i)), big) == c {
		i = t.prev(i)
	}
	return i
}

func wordEndBack(t *vtext, i int, big bool) int {
	if c := vimClass(t.at(i), big); c != 0 {
		for i > 0 && vimClass(t.at(i), big) == c {
			i = t.prev(i)
		}
	}
	for i > 0 && vimSpace(t.at(i)) {
		i = t.prev(i)
	}
	return i
}

func firstNonBlank(s text.Snapshot, l int) int {
	ls, le := s.LineRange(l)
	return ls + len(leading(s.String(ls, le)))
}

func (e *Editor) vimCol(s text.Snapshot, i int) int {
	return e.columns(s.String(s.LineStart(s.LineAt(i)), i))
}

// vimAtCol 返回第 l 行第 col 列的偏移；行太短时是行里最后一个字。
func (e *Editor) vimAtCol(s text.Snapshot, l, col int) int {
	ls, le := s.LineRange(l)
	tw := max(e.cfg.TabWidth, 1)
	c, i := 0, ls
	for _, r := range s.String(ls, le) {
		w := 1
		if r == '\t' {
			w = tw - c%tw
		}
		if c+w > col {
			break
		}
		c += w
		i += utf8.RuneLen(r)
	}
	if i >= le && le > ls {
		i = newVText(s).prev(le)
	}
	return i
}

// ---- 移动 ----

type vimTarget struct {
	from, to  int
	linewise  bool
	inclusive bool
	object    bool
}

func (e *Editor) vimMotion(v *vimState, t *vtext, cur int, key string, arg rune, count int, op string) (vimTarget, bool) {
	s := t.s
	cnt := max(count, 1)
	line := s.LineAt(cur)
	ls, le := s.LineRange(line)
	last := vimLast(s)
	if key != "j" && key != "k" {
		v.hasWant = false
	}
	switch key {
	case "h", "\b":
		return vimTarget{to: t.step(cur, -cnt, ls, le)}, cur > ls
	case "l", " ":
		lim := le
		if op == "" && le > ls {
			lim = t.prev(le)
		}
		return vimTarget{to: t.step(cur, cnt, ls, lim)}, cur < lim
	case "j", "k", "+", "-", "\r", "_":
		by := cnt
		switch key {
		case "k", "-":
			by = -cnt
		case "_":
			by = cnt - 1
		}
		to := min(max(line+by, 0), last)
		if to == line && by != 0 {
			return vimTarget{}, false
		}
		if key == "j" || key == "k" {
			if !v.hasWant {
				v.want, v.hasWant = e.vimCol(s, cur), true
			}
			return vimTarget{to: e.vimAtCol(s, to, v.want), linewise: true}, true
		}
		return vimTarget{to: firstNonBlank(s, to), linewise: true}, true
	case "w", "W":
		big := key == "W"
		if op == "c" && !vimSpace(t.at(cur)) {
			to := cur
			for k := range cnt {
				if k > 0 || vimClass(t.at(t.next(to)), big) == vimClass(t.at(to), big) {
					to = wordEnd(t, to, big)
				}
			}
			return vimTarget{to: to, inclusive: true}, true
		}
		to := cur
		for range cnt {
			to = wordFwd(t, to, big)
		}
		if op != "" {
			if tl := s.LineAt(to); tl > line {
				if strings.TrimSpace(s.String(s.LineStart(tl), to)) == "" {
					_, pe := s.LineRange(tl - 1)
					to = max(pe, cur)
				}
			}
		}
		return vimTarget{to: to}, to != cur
	case "b", "B":
		to := cur
		for range cnt {
			to = wordBack(t, to, key == "B")
		}
		return vimTarget{to: to}, to != cur
	case "e", "E":
		to := cur
		for range cnt {
			to = wordEnd(t, to, key == "E")
		}
		return vimTarget{to: to, inclusive: true}, to != cur
	case "ge", "gE":
		to := cur
		for range cnt {
			to = wordEndBack(t, to, key == "gE")
		}
		return vimTarget{to: to, inclusive: true}, to != cur
	case "0":
		return vimTarget{to: ls}, true
	case "^":
		return vimTarget{to: firstNonBlank(s, line)}, true
	case "$":
		tl := min(line+cnt-1, last)
		tls, tle := s.LineRange(tl)
		to := tle
		if op == "" && tle > tls {
			to = t.prev(tle)
		}
		v.want, v.hasWant = 1<<30, true
		return vimTarget{to: to}, true
	case "g_":
		tl := min(line+cnt-1, last)
		tls, tle := s.LineRange(tl)
		trimmed := strings.TrimRight(s.String(tls, tle), " \t")
		to := tls + len(trimmed)
		if to > tls {
			to = t.prev(to)
		}
		return vimTarget{to: to, inclusive: true}, true
	case "G", "gg":
		tl := last
		if key == "gg" {
			tl = 0
		}
		if count > 0 {
			tl = min(count-1, last)
		}
		return vimTarget{to: firstNonBlank(s, tl), linewise: true}, true
	case "f", "F", "t", "T":
		v.find = vimCmd{key: key, arg: arg}
		return vimFindChar(t, cur, key, arg, cnt)
	case ";", ",":
		if v.find.key == "" {
			return vimTarget{}, false
		}
		k := v.find.key
		if key == "," {
			k = map[string]string{"f": "F", "F": "f", "t": "T", "T": "t"}[k]
		}
		return vimFindChar(t, cur, k, v.find.arg, cnt)
	case "%":
		for i := cur; i < le; i = t.next(i) {
			switch t.at(i) {
			case '(', ')', '[', ']', '{', '}':
				if _, pair, ok := matchBracket(s, t.next(i)); ok {
					return vimTarget{to: pair, inclusive: true}, true
				}
				return vimTarget{}, false
			}
		}
		return vimTarget{}, false
	case "{", "}":
		l := line
		blank := func(l int) bool { a, z := s.LineRange(l); return a == z }
		for range cnt {
			if key == "}" {
				for l < last && blank(l) {
					l++
				}
				for l < last && !blank(l) {
					l++
				}
			} else {
				for l > 0 && blank(l) {
					l--
				}
				for l > 0 && !blank(l) {
					l--
				}
			}
		}
		to := s.LineStart(l)
		if key == "}" && !blank(l) {
			_, to = s.LineRange(l)
		}
		return vimTarget{to: to}, to != cur
	}
	if len(key) == 2 && (key[0] == 'i' || key[0] == 'a') {
		from, to, ok := vimObject(t, cur, key, cnt)
		return vimTarget{from: from, to: to, object: true}, ok
	}
	return vimTarget{}, false
}

func vimFindChar(t *vtext, cur int, key string, ch rune, cnt int) (vimTarget, bool) {
	s := t.s
	ls, le := s.LineRange(s.LineAt(cur))
	switch key {
	case "f", "t":
		j := cur
		if key == "t" && t.next(j) < le && t.at(t.next(j)) == ch {
			j = t.next(j) // ; 重复 t 时越过它停下来的那个
		}
		for range cnt {
			j = t.next(j)
			for j < le && t.at(j) != ch {
				j = t.next(j)
			}
			if j >= le {
				return vimTarget{}, false
			}
		}
		if key == "t" {
			j = t.prev(j)
		}
		return vimTarget{to: j, inclusive: true}, true
	default:
		j := cur
		if key == "T" && j > ls && t.at(t.prev(j)) == ch {
			j = t.prev(j)
		}
		for range cnt {
			if j <= ls {
				return vimTarget{}, false
			}
			j = t.prev(j)
			for j > ls && t.at(j) != ch {
				j = t.prev(j)
			}
			if t.at(j) != ch {
				return vimTarget{}, false
			}
		}
		if key == "T" {
			j = t.next(j)
		}
		return vimTarget{to: j}, true
	}
}

var vimBrackets = map[byte][2]rune{
	'(': {'(', ')'}, ')': {'(', ')'}, 'b': {'(', ')'},
	'[': {'[', ']'}, ']': {'[', ']'},
	'{': {'{', '}'}, '}': {'{', '}'}, 'B': {'{', '}'},
	'<': {'<', '>'}, '>': {'<', '>'},
}

// vimObject 找光标处的文本对象：[from, to)。
func vimObject(t *vtext, cur int, key string, cnt int) (from, to int, ok bool) {
	s := t.s
	inner := key[0] == 'i'
	ls, le := s.LineRange(s.LineAt(cur))
	switch k := key[1]; k {
	case 'w', 'W':
		if ls == le {
			return 0, 0, false
		}
		big := k == 'W'
		pos := min(cur, t.prev(le))
		cls := vimClass(t.at(pos), big)
		a, z := pos, t.next(pos)
		for a > ls && vimClass(t.at(t.prev(a)), big) == cls {
			a = t.prev(a)
		}
		for z < le && vimClass(t.at(z), big) == cls {
			z = t.next(z)
		}
		if inner {
			return a, z, true
		}
		if cls == 0 {
			if z < le {
				c := vimClass(t.at(z), big)
				for z < le && vimClass(t.at(z), big) == c {
					z = t.next(z)
				}
			}
			return a, z, true
		}
		after := z
		for after < le && vimSpace(t.at(after)) {
			after = t.next(after)
		}
		if after > z {
			return a, after, true
		}
		for a > ls && vimSpace(t.at(t.prev(a))) {
			a = t.prev(a)
		}
		return a, z, true
	case '"', '\'', '`':
		q := rune(k)
		var quotes []int
		for i := ls; i < le; i = t.next(i) {
			if t.at(i) == q && (i == ls || t.at(t.prev(i)) != '\\') {
				quotes = append(quotes, i)
			}
		}
		for p := 0; p+1 < len(quotes); p += 2 {
			q1, q2 := quotes[p], quotes[p+1]
			if cur <= q2 {
				if inner {
					return t.next(q1), q2, true
				}
				z := t.next(q2)
				for z < le && vimSpace(t.at(z)) {
					z = t.next(z)
				}
				return q1, z, true
			}
		}
		return 0, 0, false
	}
	pair, isBracket := vimBrackets[key[1]]
	if !isBracket {
		return 0, 0, false
	}
	open, close := pair[0], pair[1]
	floor := max(0, cur-maxBracketScan)
	back := func(from int) int {
		depth := 0
		for j := from; j >= floor; j = t.prev(j) {
			switch t.at(j) {
			case close:
				depth++
			case open:
				if depth == 0 {
					return j
				}
				depth--
			}
			if j == 0 {
				break
			}
		}
		return -1
	}
	o := -1
	if t.at(cur) == open {
		o = cur
	} else if cur > 0 {
		o = back(t.prev(cur))
	}
	for k := 1; k < cnt && o > 0; k++ {
		o = back(t.prev(o))
	}
	if o < 0 {
		return 0, 0, false
	}
	c, depth := -1, 0
	for j := t.next(o); j < min(t.n, o+1+maxBracketScan); j = t.next(j) {
		r := t.at(j)
		if r == open {
			depth++
		} else if r == close {
			if depth == 0 {
				c = j
				break
			}
			depth--
		}
	}
	if c < 0 {
		return 0, 0, false
	}
	if !inner {
		return o, t.next(c), true
	}
	from, to = t.next(o), c
	if t.at(from) == '\n' {
		from = t.next(from)
	}
	if cl := s.LineAt(c); cl > s.LineAt(o) {
		if cls := s.LineStart(cl); strings.TrimSpace(s.String(cls, c)) == "" {
			to = max(cls, from)
		}
	}
	return from, max(to, from), true
}

// ---- 操作符 ----

type vimRange struct {
	start, end int
	linewise   bool
	l1, l2     int
}

func (e *Editor) vimYank(s text.Snapshot, r vimRange) {
	var txt string
	if r.linewise {
		_, le := s.LineRange(r.l2)
		txt = strings.ReplaceAll(s.String(s.LineStart(r.l1), le), "\r\n", "\n") + "\n"
	} else {
		txt = strings.ReplaceAll(s.String(r.start, r.end), "\r\n", "\n")
	}
	e.vimReg = vimRegister{txt, r.linewise}
	if e.host != nil {
		e.host.SetClipboard(txt)
	}
}

func (e *Editor) vimClip() (string, bool) {
	txt := ""
	if e.host != nil {
		txt, _ = e.host.Clipboard()
	}
	txt = strings.ReplaceAll(txt, "\r\n", "\n")
	if txt == e.vimReg.text {
		return txt, e.vimReg.linewise
	}
	return txt, strings.HasSuffix(txt, "\n")
}

// vimEdit 改文字并把光标放在 caret（新快照里的偏移）。
func (e *Editor) vimEdit(t *tab, caret int, cs ...text.Change) {
	b := t.doc.buf
	b.Apply(edit.Transaction{Changes: text.ChangeSet(cs).Normalize(), After: edit.Single(edit.Cursor(caret)), Origin: edit.OriginCommand})
}

func (e *Editor) vimOperate(t *tab, nt *vtext, cur int, c vimCmd) {
	s := nt.s
	var r vimRange
	if c.key == "line" {
		l := s.LineAt(cur)
		r = vimRange{linewise: true, l1: l, l2: min(l+max(c.count, 1)-1, vimLast(s))}
	} else {
		tg, ok := e.vimMotion(&t.vim, nt, cur, c.key, c.arg, c.count, c.op)
		switch {
		case !ok:
			if c.op == "c" {
				t.vim.mode, t.vim.recording = vimNormal, false
			}
			return
		case tg.object:
			r = vimRange{start: tg.from, end: tg.to}
		case tg.linewise:
			l1, l2 := s.LineAt(cur), s.LineAt(tg.to)
			r = vimRange{linewise: true, l1: min(l1, l2), l2: max(l1, l2)}
		default:
			r = vimRange{start: min(cur, tg.to), end: max(cur, tg.to)}
			if tg.inclusive {
				r.end = min(nt.next(r.end), nt.n)
			}
		}
	}
	e.vimApply(t, c.op, r, cur)
}

func (e *Editor) vimApply(t *tab, op string, r vimRange, cur int) {
	b := t.doc.buf
	s := b.Snapshot()
	if r.linewise {
		r.start = s.LineStart(r.l1)
		_, r.end = s.LineRange(r.l2)
	} else {
		r.l1, r.l2 = s.LineAt(r.start), s.LineAt(max(r.end-1, r.start))
	}
	switch op {
	case "y":
		e.vimYank(s, r)
		caret := r.start
		if r.linewise {
			caret = cur
			if s.LineAt(cur) != r.l1 {
				caret = e.vimAtCol(s, r.l1, e.vimCol(s, cur))
			}
		}
		e.vimPut(t, caret)
	case "d":
		e.vimYank(s, r)
		if !r.linewise {
			if r.end > r.start {
				e.vimEdit(t, r.start, text.Change{Start: r.start, End: r.end})
			}
			return
		}
		start, end := r.start, r.end
		if r.l2+1 < s.LineCount() {
			end = s.LineStart(r.l2 + 1)
		} else if r.l1 > 0 {
			_, start = s.LineRange(r.l1 - 1)
		}
		e.vimEdit(t, start, text.Change{Start: start, End: end})
		after := b.Snapshot()
		e.vimPut(t, firstNonBlank(after, min(r.l1, vimLast(after))))
	case "c":
		e.vimYank(s, r)
		at := r.start
		if r.linewise {
			at = firstNonBlank(s, r.l1) // 整行改：保留第一行的缩进
		}
		edited := r.end > at
		if edited {
			e.vimEdit(t, at, text.Change{Start: at, End: r.end})
		}
		e.vimStartInsert(t, at, edited)
	case ">", "<", "gc":
		ls := s.LineStart(r.l1)
		_, le := s.LineRange(r.l2)
		if r.l1 == r.l2 {
			le = ls
		}
		b.SetSelections(edit.Single(edit.Range{Anchor: ls, Head: le, GoalX: -1}))
		if op == "gc" {
			e.toggleComment()
		} else {
			e.indentLines(op == "<")
		}
		e.vimPut(t, firstNonBlank(b.Snapshot(), r.l1))
	case "g~", "gu", "gU":
		if r.linewise {
			r.end = min(r.end, s.Len())
		}
		txt := s.String(r.start, r.end)
		var conv string
		switch op {
		case "gu":
			conv = strings.ToLower(txt)
		case "gU":
			conv = strings.ToUpper(txt)
		default:
			conv = toggleCase(txt)
		}
		if conv != txt {
			e.vimEdit(t, r.start, text.Change{Start: r.start, End: r.end, Text: conv})
		}
		e.vimPut(t, r.start)
	}
}

func toggleCase(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsUpper(r) {
			return unicode.ToLower(r)
		}
		return unicode.ToUpper(r)
	}, s)
}

// vimStartInsert 在 at 进插入模式；编辑过的话，随后打的字和那次修改合成一步撤销。
// . 重复时不进插入模式，而是把上次打的字再打一遍。
func (e *Editor) vimStartInsert(t *tab, at int, edited bool) {
	v := &t.vim
	b := t.doc.buf
	if v.replay {
		if v.lastText != "" {
			if edited {
				b.BeginGroup(true)
			}
			txt := v.lastText
			if eolOf(b.Snapshot()) == "\r\n" {
				txt = strings.ReplaceAll(txt, "\n", "\r\n")
			}
			e.vimEdit(t, at+len(txt), text.Change{Start: at, End: at, Text: txt})
			if edited {
				b.EndGroup()
			}
			at = newVText(b.Snapshot()).prev(at + len(txt))
		}
		e.vimPut(t, at)
		v.mode = vimNormal
		return
	}
	e.vimPut(t, at)
	v.insAt, v.insLen = at, b.Snapshot().Len()
	b.BeginGroup(edited)
	v.group = true
}

// vimEndInsert 结束插入模式：结束这一步撤销，光标退回最后打的那个字上。记下打的字给 . 用。
func (e *Editor) vimEndInsert(t *tab) {
	v := &t.vim
	b := t.doc.buf
	s := b.Snapshot()
	if v.recording {
		v.recording, v.lastText = false, ""
		grown := s.Len() - v.insLen
		if caret := b.Selections().Main().Head; grown > 0 && caret >= v.insAt && caret <= v.insAt+grown && v.insAt+grown <= s.Len() {
			v.lastText = strings.ReplaceAll(s.String(v.insAt, v.insAt+grown), "\r\n", "\n")
		}
	}
	if v.group {
		b.EndGroup()
		v.group = false
	}
	c := min(b.Selections().Main().From(), s.Len())
	if c > s.LineStart(s.LineAt(c)) {
		c = newVText(s).prev(c)
	}
	e.vimPut(t, vimClamp(s, c))
}

func (e *Editor) vimNormalRun(t *tab, c vimCmd, next vimMode) {
	v := &t.vim
	b := t.doc.buf
	s := b.Snapshot()
	nt := newVText(s)
	m := b.Selections().Main()
	cur := vimClamp(s, m.From())
	if m.Empty() {
		cur = vimClamp(s, m.Head)
	}
	cnt := max(c.count, 1)
	line := s.LineAt(cur)
	ls, le := s.LineRange(line)
	last := vimLast(s)
	if !v.replay && c.changes() {
		v.last, v.hasLast, v.lastText = c, true, ""
		v.recording = next == vimInsert
	}
	if c.op != "" {
		e.vimOperate(t, nt, cur, c)
		return
	}
	switch c.key {
	case "x", "\x7f":
		if le > ls {
			e.vimApply(t, "d", vimRange{start: cur, end: nt.step(cur, cnt, ls, le)}, cur)
		}
	case "X":
		if cur > ls {
			e.vimApply(t, "d", vimRange{start: nt.step(cur, -cnt, ls, le), end: cur}, cur)
		}
	case "D", "C":
		_, end := s.LineRange(min(line+cnt-1, last))
		op := map[string]string{"D": "d", "C": "c"}[c.key]
		e.vimApply(t, op, vimRange{start: cur, end: end}, cur)
	case "s":
		e.vimApply(t, "c", vimRange{start: cur, end: nt.step(cur, cnt, ls, le)}, cur)
	case "S":
		e.vimApply(t, "c", vimRange{linewise: true, l1: line, l2: min(line+cnt-1, last)}, cur)
	case "Y":
		e.vimApply(t, "y", vimRange{linewise: true, l1: line, l2: min(line+cnt-1, last)}, cur)
	case "p", "P":
		e.vimPaste(t, cur, c.key == "P", cnt)
	case "o", "O":
		e.vimPut(t, cur)
		e.insertLine(c.key == "O")
		e.vimStartInsert(t, b.Selections().Main().Head, true)
	case "J":
		e.vimJoin(t, line, max(cnt, 2))
	case "r":
		if end := nt.step(cur, cnt, ls, le); le > ls && end > cur {
			n := 0
			for i := cur; i < end; i = nt.next(i) {
				n++
			}
			if n == cnt {
				rep := strings.Repeat(string(c.arg), cnt)
				e.vimEdit(t, cur+len(rep)-utf8.RuneLen(c.arg), text.Change{Start: cur, End: end, Text: rep})
			}
		}
	case "~":
		if end := nt.step(cur, cnt, ls, le); end > cur {
			txt := s.String(cur, end)
			if conv := toggleCase(txt); conv != txt {
				e.vimEdit(t, cur, text.Change{Start: cur, End: end, Text: conv})
			}
			e.vimPut(t, min(end, nt.prev(le)))
		}
	case "u", "\x12":
		for range cnt {
			var ok bool
			if c.key == "u" {
				_, ok = b.Undo()
			} else {
				_, ok = b.Redo()
			}
			if !ok {
				break
			}
		}
		e.vimPut(t, b.Selections().Main().From())
	case ".":
		if !v.hasLast {
			return
		}
		rc := v.last
		if c.count > 0 {
			rc.count, v.last.count = c.count, c.count
		}
		v.replay = true
		e.vimNormalRun(t, rc, vimNormal)
		v.replay = false
		v.mode = vimNormal
	case "i":
		e.vimStartInsert(t, cur, false)
	case "a":
		e.vimStartInsert(t, nt.step(cur, 1, ls, le), false)
	case "I":
		e.vimStartInsert(t, firstNonBlank(s, line), false)
	case "A":
		e.vimStartInsert(t, le, false)
	case "v", "V":
		v.anchor, v.cursor = cur, cur
		e.vimShowVisual(t, next)
	case "\x04", "\x15", "\x06", "\x02", "\x05", "\x19":
		e.vimScroll(t, s, cur, c.key, c.count)
	default:
		if tg, ok := e.vimMotion(v, nt, cur, c.key, c.arg, c.count, ""); ok && !tg.object {
			e.vimPut(t, tg.to)
		}
	}
}

func (e *Editor) vimScroll(t *tab, s text.Snapshot, cur int, key string, count int) {
	v := &t.vim
	rows := max(e.paneHeight(), 1)
	var by int
	switch key {
	case "\x04":
		by = max(rows/2, 1)
	case "\x15":
		by = -max(rows/2, 1)
	case "\x06":
		by = max(rows-2, 1)
	case "\x02":
		by = -max(rows-2, 1)
	case "\x05":
		by = max(count, 1)
	case "\x19":
		by = -max(count, 1)
	}
	e.scrollTab(t, by)
	if key == "\x05" || key == "\x19" {
		return
	}
	to := min(max(s.LineAt(cur)+by, 0), vimLast(s))
	if !v.hasWant {
		v.want, v.hasWant = e.vimCol(s, cur), true
	}
	e.vimPut(t, e.vimAtCol(s, to, v.want))
}

func (e *Editor) vimPaste(t *tab, cur int, before bool, cnt int) {
	txt, linewise := e.vimClip()
	if txt == "" {
		return
	}
	b := t.doc.buf
	s := b.Snapshot()
	nl := eolOf(s)
	line := s.LineAt(cur)
	ls, le := s.LineRange(line)
	if linewise {
		body := strings.ReplaceAll(strings.TrimSuffix(strings.Repeat(txt, cnt), "\n"), "\n", nl)
		if before {
			e.vimEdit(t, ls, text.Change{Start: ls, End: ls, Text: body + nl})
			e.vimPut(t, firstNonBlank(b.Snapshot(), line))
		} else {
			e.vimEdit(t, le, text.Change{Start: le, End: le, Text: nl + body})
			e.vimPut(t, firstNonBlank(b.Snapshot(), line+1))
		}
		return
	}
	body := strings.ReplaceAll(strings.Repeat(txt, cnt), "\n", nl)
	at := cur
	if !before && le > ls {
		at = newVText(s).next(cur)
	}
	end := at + len(body)
	_, last := utf8.DecodeLastRuneInString(body)
	e.vimEdit(t, end-last, text.Change{Start: at, End: at, Text: body})
}

func (e *Editor) vimJoin(t *tab, line, count int) {
	b := t.doc.buf
	s := b.Snapshot()
	l2 := min(line+count-1, vimLast(s))
	if l2 <= line {
		return
	}
	ls, le := s.LineRange(line)
	txt := s.String(ls, le)
	at := 0
	for l := line + 1; l <= l2; l++ {
		a, z := s.LineRange(l)
		next := strings.TrimLeft(s.String(a, z), " \t")
		sep := " "
		if txt == "" || strings.HasSuffix(txt, " ") || strings.HasSuffix(txt, "\t") || next == "" || strings.HasPrefix(next, ")") {
			sep = ""
		}
		at = len(txt)
		txt += sep + next
	}
	_, end := s.LineRange(l2)
	e.vimEdit(t, ls+at, text.Change{Start: ls, End: end, Text: txt})
}

// vimShowVisual 选中可视模式的范围：从锚点到光标的字，或它们所在的行；光标在会动的那一端。
func (e *Editor) vimShowVisual(t *tab, mode vimMode) {
	v := &t.vim
	b := t.doc.buf
	s := b.Snapshot()
	nt := newVText(s)
	n := s.Len()
	v.anchor, v.cursor = min(max(v.anchor, 0), n), min(max(v.cursor, 0), n)
	lo, hi := min(v.anchor, v.cursor), max(v.anchor, v.cursor)
	var a, z int
	if mode == vimVisualLine {
		a = s.LineStart(s.LineAt(lo))
		_, z = s.LineRange(s.LineAt(hi))
		z = min(nt.next(z), n)
	} else {
		a, z = lo, min(nt.next(hi), n)
	}
	r := edit.Range{Anchor: a, Head: z, GoalX: -1}
	if v.cursor < v.anchor {
		r = edit.Range{Anchor: z, Head: a, GoalX: -1}
	}
	b.SetSelections(edit.Single(r))
}

func (e *Editor) vimVisualRun(t *tab, c vimCmd, mode, next vimMode) {
	v := &t.vim
	b := t.doc.buf
	s := b.Snapshot()
	nt := newVText(s)
	n := s.Len()
	lo, hi := min(v.anchor, v.cursor, n), min(max(v.anchor, v.cursor), n)
	r := vimRange{start: lo, end: min(nt.next(hi), n)}
	switch {
	case mode == vimVisualLine, c.key == "X", c.key == "D", c.key == "Y", c.key == "C", c.key == "S", c.key == "R":
		r = vimRange{linewise: true, l1: s.LineAt(lo), l2: s.LineAt(hi)}
	}
	switch c.key {
	case "\x1b", "v", "V":
		if next == vimNormal {
			e.vimPut(t, v.cursor)
		} else {
			e.vimShowVisual(t, next)
		}
	case "o":
		v.anchor, v.cursor = v.cursor, v.anchor
		e.vimShowVisual(t, next)
	case "d", "x", "X", "D":
		e.vimApply(t, "d", r, lo)
	case "y", "Y":
		e.vimApply(t, "y", r, lo)
	case "c", "s", "C", "S", "R":
		v.last, v.hasLast, v.recording = vimCmd{}, false, false
		e.vimApply(t, "c", r, lo)
	case ">", "<", "gc", "g~", "gu", "gU":
		e.vimApply(t, c.key, r, lo)
	case "~":
		e.vimApply(t, "g~", r, lo)
	case "u":
		e.vimApply(t, "gu", r, lo)
	case "U":
		e.vimApply(t, "gU", r, lo)
	case "J":
		l1, l2 := s.LineAt(lo), s.LineAt(hi)
		e.vimJoin(t, l1, max(l2-l1+1, 2))
	case "r":
		a, z := r.start, r.end
		if r.linewise {
			a = s.LineStart(r.l1)
			_, z = s.LineRange(r.l2)
		}
		rep := strings.Map(func(x rune) rune {
			if x == '\n' || x == '\r' {
				return x
			}
			return c.arg
		}, s.String(a, z))
		e.vimEdit(t, a, text.Change{Start: a, End: z, Text: rep})
	case "p", "P":
		txt, linewise := e.vimClip()
		a, z := r.start, r.end
		body := txt
		if r.linewise {
			a = s.LineStart(r.l1)
			_, z = s.LineRange(r.l2)
			if linewise {
				body = strings.TrimSuffix(txt, "\n")
			}
		} else if linewise {
			body = "\n" + strings.TrimSuffix(txt, "\n") + "\n"
		}
		e.vimEdit(t, a, text.Change{Start: a, End: z, Text: strings.ReplaceAll(body, "\n", eolOf(s))})
	default:
		if len(c.key) == 2 && (c.key[0] == 'i' || c.key[0] == 'a') {
			if from, to, ok := vimObject(nt, v.cursor, c.key, c.count); ok && to > from {
				v.anchor, v.cursor = from, nt.prev(to)
			}
		} else if tg, ok := e.vimMotion(v, nt, v.cursor, c.key, c.arg, c.count, ""); ok {
			v.cursor = tg.to
		}
		e.vimShowVisual(t, next)
	}
}

// toggleVim 打开或关掉 vim 模式（所有标签回到普通模式）。
func (e *Editor) toggleVim() error {
	e.cfg.Vim = !e.cfg.Vim
	for _, c := range e.cols {
		for _, t := range c.tabs {
			if t.vim.group {
				t.doc.buf.EndGroup()
			}
			t.vim = vimState{}
		}
	}
	if e.state != nil {
		v := e.cfg.Vim
		e.state.Vim = &v
		e.saveState()
	}
	e.settingsGen++
	if e.cfg.Vim {
		e.status = "Vim 模式：开"
	} else {
		e.status = "Vim 模式：关"
	}
	return nil
}

// VimMode 报告 vim 模式开着没有（菜单的勾）。
func (e *Editor) VimMode() bool { return e.cfg.Vim }
