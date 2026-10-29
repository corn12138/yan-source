package app

import (
	"context"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	sf "github.com/sahilm/fuzzy"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/search/fuzzy"
	"github.com/corn12138/yan-source/core/view"
)

// maxItems 是浮层一屏显示的候选数。
const maxItems = view.MaxOverlayItems

type ovItem struct {
	label, detail, hint string
	font                string // the window shows a sample in this font family (the font picker)
	theme               string
	labelHit, detailHit []int  // 命中字符的字节下标
	unreleased          bool   // a command the official release does not have yet
	group               string // Settings: the heading of the items from this one on
	checked             bool   // Settings: the option saved
	stay                bool   // choosing it leaves the overlay open (Settings)
	run                 func(e *Editor) error
}

type overlay struct {
	repository *repositoryContext

	kind   view.OverlayKind
	input  string
	items  []ovItem
	sel    int
	top    int
	footer string
	gen    uint64 // 模糊匹配的代号：过期的后台结果直接丢弃
	cancel context.CancelFunc
	// 问一个名字的浮层：标题、回车时做什么
	title  string
	accept func(e *Editor, input string) error
	hint   func(input string) string
	// 选一项的浮层：全部选项（输入的文字筛选它们）
	choices []ovItem
	preview *themePreview
}

// prompt 打开一个问名字的浮层：输入框里先填好 initial，回车时用输入的文字调用 accept。
func (e *Editor) prompt(title, initial string, hint func(string) string, accept func(*Editor, string) error) {
	e.closeOverlay()
	e.ov = &overlay{kind: view.OverlayPrompt, title: title, accept: accept, hint: hint}
	e.setOverlayInput(initial)
}

// choose 打开一个从几项里选一项的浮层：Mayoi 在这里弹确认框，砚用浮层，终端版也能选。
// Esc 就是取消。
func (e *Editor) choose(title string, items ...ovItem) {
	e.closeOverlay()
	e.ov = &overlay{kind: view.OverlayChoice, title: title, choices: items}
	e.setOverlayInput("")
}

func (e *Editor) closeOverlay() {
	if e.ThemePreviewing() {
		e.settingsGen++
	}
	if e.ov != nil && e.ov.cancel != nil {
		e.ov.cancel()
	}
	e.ov = nil
}

// openQuickOpen 打开快速打开；prefill 用于 Ctrl+G（预填 ":"）。
func (e *Editor) openQuickOpen(prefill string) error {
	e.closeOverlay()
	e.ov = &overlay{kind: view.OverlayFinder}
	e.setOverlayInput(prefill)
	return nil
}

func (e *Editor) openPalette() error {
	e.closeOverlay()
	e.ov = &overlay{kind: view.OverlayPalette}
	e.setOverlayInput("")
	return nil
}

// normalizePrefix 把中文标点前缀当成对应的英文前缀：中文输入法下 Shift+; 打出的是「：」。
func normalizePrefix(s string) string {
	for _, p := range [][2]string{{"：", ":"}, {"》", ">"}, {"＞", ">"}, {"＠", "@"}} {
		if strings.HasPrefix(s, p[0]) {
			return p[1] + s[len(p[0]):]
		}
	}
	return s
}

// setOverlayInput 更新输入并重算候选。
func (e *Editor) setOverlayInput(in string) {
	if e.ov != nil && e.ov.repository != nil {
		defer e.bindRepository(e.ov.repository)()
	}
	ov := e.ov
	ov.input = in
	ov.sel, ov.top = 0, 0
	if ov.kind == view.OverlayTheme {
		e.themeItems()
		return
	}
	if ov.kind == view.OverlaySettings {
		e.settingsItems()
		return
	}
	q := normalizePrefix(in)
	if ov.kind == view.OverlayFinder && strings.HasPrefix(q, ">") {
		ov.kind = view.OverlayPalette // 与 VS Code 一样，">" 切到命令面板
	}
	if ov.kind == view.OverlayPalette {
		e.paletteItems(strings.TrimSpace(strings.TrimPrefix(q, ">")))
		return
	}
	if ov.kind == view.OverlayBranch {
		e.branchItems(in)
		return
	}
	if ov.kind == view.OverlayPrompt {
		ov.items, ov.footer = ov.items[:0], i18n.T("Enter to confirm · Esc to cancel")
		if strings.TrimSpace(in) != "" {
			accept := ov.accept
			ov.items = append(ov.items, ovItem{label: ov.hint(in), run: func(e *Editor) error { return accept(e, in) }})
		}
		return
	}
	if ov.kind == view.OverlayRecent {
		e.recentFolderItems(strings.TrimSpace(in))
		return
	}
	if ov.kind == view.OverlayChoice {
		ov.items, ov.footer = ov.items[:0], i18n.T("Enter to choose · Esc to cancel")
		q := strings.ToLower(strings.TrimSpace(in))
		for _, it := range ov.choices {
			if strings.Contains(strings.ToLower(it.label), q) {
				ov.items = append(ov.items, it)
			}
		}
		return
	}
	switch {
	case strings.HasPrefix(q, ":"):
		e.gotoItems(q[1:])
	case strings.HasPrefix(q, "@"):
		e.symbolItems(q[1:])
	case strings.TrimSpace(q) == "":
		e.recentItems()
	default:
		e.fuzzyItems(q)
	}
}

func (e *Editor) recentItems() {
	ov := e.ov
	ov.items = ov.items[:0]
	for _, p := range e.recent {
		name, dir := splitPath(e.rel(p))
		ov.items = append(ov.items, ovItem{label: name, detail: dir, run: openRun(p)})
	}
	ov.footer = i18n.T("Recently Opened")
	if len(ov.items) == 0 {
		ov.footer = i18n.T("Type a file name to search · : go to line · > commands")
	}
}

func (e *Editor) gotoItems(arg string) {
	ov := e.ov
	ov.items, ov.footer = ov.items[:0], ""
	t := e.cur()
	if t == nil {
		ov.footer = i18n.T("No open file")
		return
	}
	n := t.doc.buf.Snapshot().LineCount()
	line, col, ok := parseLineCol(arg)
	if !ok {
		ov.footer = i18n.F("Enter a line number from 1 to %d, optionally with :column", n)
		return
	}
	line = min(line, n)
	label := i18n.F("Go to line %d", line)
	if col > 0 {
		label += i18n.F(", column %d", col)
	}
	ov.items = append(ov.items, ovItem{label: label, run: func(e *Editor) error {
		e.gotoLineCol(line, col)
		return nil
	}})
}

func parseLineCol(s string) (int, int, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, "：", ":"))
	ls, cs, _ := strings.Cut(s, ":")
	line, err := strconv.Atoi(ls)
	if err != nil || line <= 0 {
		return 0, 0, false
	}
	col, _ := strconv.Atoi(cs)
	return line, max(col, 0), true
}

func (e *Editor) gotoLineCol(line, col int) {
	t := e.cur()
	if t == nil {
		return
	}
	b := t.doc.buf
	s := b.Snapshot()
	pos := s.LineStart(line - 1)
	for i := 1; i < col; i++ {
		pos = edit.NextBoundary(s, pos)
	}
	b.SetSelections(edit.Single(edit.Cursor(pos)))
	e.centerLine(t, line-1)
	e.area = areaEditor
	e.reveal()
}

// fuzzyItems 在后台 goroutine 里做模糊匹配，结果用 Post 送回；新的输入会取消旧的匹配。
func (e *Editor) fuzzyItems(q string) {
	ov := e.ov
	if e.ws == nil {
		ov.items, ov.footer = ov.items[:0], i18n.T("No folder open (yan folder)")
		return
	}
	if ov.cancel != nil {
		ov.cancel()
	}
	ov.gen++
	gen := ov.gen
	ctx, cancel := context.WithCancel(context.Background())
	ov.cancel = cancel
	paths := e.ws.index[:len(e.ws.index):len(e.ws.index)] // 三下标切片：后台只读这一段，所有者继续追加也不会碰到
	root := e.root
	go func() {
		res := fuzzy.Match(ctx, q, paths, 200)
		if ctx.Err() != nil {
			return
		}
		e.Post(func(e *Editor) {
			if e.ov != ov || ov.gen != gen {
				return // 用户已经继续打字或关掉了浮层
			}
			ov.items = ov.items[:0]
			for _, r := range res {
				name, dir := splitPath(r.Path)
				nh, dh := splitHits(r.Matched, len(dir))
				ov.items = append(ov.items, ovItem{label: name, detail: dir, labelHit: nh, detailHit: dh,
					run: openRun(filepath.Join(root, filepath.FromSlash(r.Path)))})
			}
			e.indexFooter(len(res))
		})
	}()
	e.indexFooter(-1)
}

func (e *Editor) indexFooter(hits int) {
	ov, w := e.ov, e.ws
	var parts []string
	if hits >= 0 {
		parts = append(parts, i18n.F("%d results", hits))
	}
	if w.indexing {
		parts = append(parts, i18n.F("Indexing %d", len(w.index)))
	} else {
		parts = append(parts, i18n.F("%d files", len(w.index)))
	}
	ov.footer = strings.Join(parts, " · ")
}

// overlayIndexGrew 在索引新增一批后调用：快速打开里有查询时重新匹配，让结果随遍历变多。
func (e *Editor) overlayIndexGrew() {
	ov := e.ov
	if ov == nil || ov.kind != view.OverlayFinder {
		return
	}
	q := normalizePrefix(ov.input)
	if strings.TrimSpace(q) != "" && !strings.HasPrefix(q, ":") {
		sel := ov.sel
		e.fuzzyItems(q)
		ov.sel = sel
	}
}

func openRun(path string) func(e *Editor) error {
	return func(e *Editor) error {
		if err := e.openPath(path); err != nil {
			return err
		}
		e.area = areaEditor
		return nil
	}
}

func splitPath(rel string) (name, dir string) {
	i := strings.LastIndexByte(rel, '/')
	if i < 0 {
		return rel, ""
	}
	return rel[i+1:], rel[:i]
}

// splitHits 把整条路径上的命中下标拆成文件名部分和目录部分。
func splitHits(hits []int, dirLen int) (name, dir []int) {
	off := 0
	if dirLen > 0 {
		off = dirLen + 1
	}
	for _, h := range hits {
		if h >= off {
			name = append(name, h-off)
		} else if h < dirLen {
			dir = append(dir, h)
		}
	}
	return name, dir
}

// paletteItems 列出命令：中文标题和英文 ID 都能搜，最近用过的排在前面。
func (e *Editor) paletteItems(q string) {
	ov := e.ov
	ov.items, ov.footer = ov.items[:0], ""
	ids := make([]string, 0, len(e.commands))
	for id, c := range e.commands {
		if c.Title != "" && !c.Hidden {
			ids = append(ids, id)
		}
	}
	rank := func(id string) int {
		if i := slices.Index(e.recentCmds, id); i >= 0 {
			return i
		}
		return len(e.recentCmds)
	}
	slices.SortFunc(ids, func(a, b string) int {
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra - rb
		}
		return strings.Compare(e.commands[a].Title, e.commands[b].Title)
	})
	add := func(id string, th, ih []int) {
		c := e.commands[id]
		ov.items = append(ov.items, ovItem{label: c.Title, detail: id, hint: e.keyHint[id], labelHit: th, detailHit: ih, unreleased: CommandUnreleased(id),
			run: func(e *Editor) error {
				e.recentCmds = append([]string{id}, slices.DeleteFunc(e.recentCmds, func(s string) bool { return s == id })...)
				e.Run(id)
				return nil
			}})
	}
	if q == "" {
		for _, id := range ids {
			add(id, nil, nil)
		}
		return
	}
	type scored struct {
		id     string
		score  int
		th, ih []int
	}
	var hits []scored
	for _, id := range ids {
		best := scored{id: id, score: -1 << 30}
		if m := sf.Find(q, []string{e.commands[id].Title}); len(m) > 0 {
			best = scored{id, m[0].Score + 10, m[0].MatchedIndexes, nil} // 标题命中优先
		}
		if m := sf.Find(q, []string{id}); len(m) > 0 && m[0].Score > best.score {
			best = scored{id, m[0].Score, nil, m[0].MatchedIndexes}
		}
		if best.score > -1<<30 {
			hits = append(hits, best)
		}
	}
	slices.SortStableFunc(hits, func(a, b scored) int { return b.score - a.score })
	for _, h := range hits {
		add(h.id, h.th, h.ih)
	}
	if len(hits) == 0 {
		ov.footer = i18n.T("No matching commands")
	}
}

// handleOverlayKey 处理浮层里的按键；返回 false 表示交给全局 keymap。
func (e *Editor) handleOverlayKey(c keymap.Chord) bool {
	ov := e.ov
	if c.Mods&^keymap.ModShift != 0 && !(c.Mods == keymap.ModCtrl && (c.Rune == 'n' || c.Rune == 'j')) {
		return false
	}
	switch c.Key {
	case keymap.KeyLeft, keymap.KeyRight:
		if ov.preview == nil || ov.input != "" || c.Mods != 0 {
			return false
		}
		by := 1
		if c.Key == keymap.KeyLeft {
			by = -1
		}
		e.stepThemeAppearance(by)
	case keymap.KeyEsc:
		e.closeOverlay()
	case keymap.KeyEnter:
		if ov.sel >= 0 && ov.sel < len(ov.items) {
			it := ov.items[ov.sel]
			if it.stay {
				if err := it.run(e); err != nil {
					e.status = err.Error()
				}
				return true
			}
			e.closeOverlay()
			if err := e.runRepositoryItem(ov.repository, it.run); err != nil {
				e.status = err.Error()
			}
			e.reveal()
		}
	case keymap.KeyUp:
		e.overlayMove(-1)
	case keymap.KeyDown:
		e.overlayMove(1)
	case keymap.KeyPgUp:
		e.overlayMove(-maxItems)
	case keymap.KeyPgDn:
		e.overlayMove(maxItems)
	case keymap.KeyBackspace:
		if in := ov.input; in != "" {
			_, n := utf8.DecodeLastRuneInString(in)
			e.setOverlayInput(in[:len(in)-n])
		} else if ov.kind == view.OverlayPalette {
			e.openQuickOpen("") // 与 VS Code 一样：在空的「>」上退格回到快速打开
		}
	case keymap.KeyRune:
		if c.Mods == keymap.ModCtrl { // Ctrl+N / Ctrl+J：下一项（Ctrl+P 被快速打开占用）
			e.overlayMove(1)
			return true
		}
		return false
	default:
		return c.Key == keymap.KeyTab
	}
	return true
}

func (e *Editor) overlayMove(n int) {
	ov := e.ov
	if len(ov.items) == 0 {
		return
	}
	ov.sel = max(min(ov.sel+n, len(ov.items)-1), 0)
	e.previewThemeSelection()
	if ov.sel < ov.top {
		ov.top = ov.sel
	} else if ov.sel >= ov.top+maxItems {
		ov.top = ov.sel - maxItems + 1
	}
}

// ---- 前端用的语义动作 ----

// SetOverlayInput 用前端输入框里的文字替换浮层的输入（GUI 的输入框自己处理编辑和输入法）。
func (e *Editor) SetOverlayInput(s string) {
	if e.ov == nil {
		return
	}
	e.sync()
	defer e.sync()
	e.setOverlayInput(s)
}

// ChooseOverlay 选中并执行浮层的第 i 项（鼠标点击）。
func (e *Editor) ChooseOverlay(i int) {
	if e.ov == nil || i < 0 || i >= len(e.ov.items) {
		return
	}
	e.sync()
	defer e.sync()
	e.ov.sel = i
	e.handleOverlayKey(enterChord)
}

// SelectOverlay 只移动浮层的选中项（鼠标悬停）。
func (e *Editor) SelectOverlay(i int) {
	if e.ov != nil && i >= 0 && i < len(e.ov.items) {
		e.ov.sel = i
		e.previewThemeSelection()
	}
}

// CloseOverlay 关闭浮层（点在浮层外面）。
func (e *Editor) CloseOverlay() { e.closeOverlay() }

// overlayKey 报告浮层会不会自己处理这个键（方向键、回车、Esc、退格……）。
func overlayKey(c keymap.Chord) bool {
	if c.Mods&^keymap.ModShift != 0 {
		return c.Mods == keymap.ModCtrl && (c.Rune == 'n' || c.Rune == 'j')
	}
	return c.Key != keymap.KeyRune
}

// fuzzyFind 在 names 里模糊找 q，按分数排好。
func fuzzyFind(q string, names []string) sf.Matches { return sf.Find(q, names) }
