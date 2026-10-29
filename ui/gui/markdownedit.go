package gui

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

// Live editing in the Markdown preview: the document stays rendered, and a double-clicked block turns
// into a text box holding that block's Markdown in place, so the file stays plain Markdown and
// its undo, saving, Git and autosave are the document's own. Each change writes the block back
// through Editor.EditMarkdownLines, which refuses a write made from text that changed meanwhile.

// markdownEdit is the block of a preview being edited.
type markdownEdit struct {
	from    int            // the block's first source line
	n       int            // the source lines it spans in the document now
	text    string         // its Markdown, with "\n" line breaks
	version uint64         // the document's version after the last write
	spans   map[uint64]int // n at each version written, to hide the parsed blocks it covers
	focused bool           // the box has had the keyboard; losing it ends the edit
	in      ui.Handle
	sel     *[2]int // a selection a formatting key set, applied once the box holds its text
	// seen is the box's selection, in runes, and composing whether an input method was composing,
	// as the last frame built it: key handlers run between frames, where elements cannot be read.
	seen      [2]int
	composing bool
	// slash is the block menu a "/" at the start of a line opened, nil while none shows;
	// dismissed is the command Esc closed it on, so it stays closed until the command changes.
	slash     *mdSlashMenu
	dismissed string
}

// mdSlashMenu is the block menu of the edit box: the command's start, what it matches, the row chosen.
type mdSlashMenu struct {
	at, caret int
	items     []mdBlockKind
	sel       int
}

// lines returns the source lines of the document's text, without line breaks.
func markdownLines(src []byte) []string {
	return strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
}

// startMarkdownEdit turns block i into a text box. A preview parsed from older text waits for
// the next parse, so the block's lines are those of the text written back.
func (f *Frontend) startMarkdownEdit(m *markdownState, v *markdownViewport, pf *view.PaneFrame, i int) {
	if i < 0 || i >= len(m.blocks) || m.version != pf.Version {
		return
	}
	lines := markdownLines(pf.Markdown)
	from := m.blocks[i].line
	end := len(lines)
	for _, b := range m.blocks[i+1:] {
		if b.line > from {
			end = b.line
			break
		}
	}
	end = min(end, len(lines))
	for end > from+1 && strings.TrimSpace(lines[end-1]) == "" {
		end-- // the blank lines between blocks stay where they are
	}
	if from >= end {
		return
	}
	v.edit = &markdownEdit{from: from, n: end - from, text: strings.Join(lines[from:end], "\n"), version: pf.Version}
	v.edit.spans = map[uint64]int{pf.Version: v.edit.n}
}

// covers reports whether the edit shows in place of a block that starts on line, as the
// preview parsed at version laid it out.
func (e *markdownEdit) covers(line int, version uint64) bool {
	n, ok := e.spans[version]
	if !ok {
		n = e.n
	}
	return line >= e.from && line < e.from+n
}

// writeMarkdownEdit writes the box's text to the document, or ends the edit when it cannot.
func (f *Frontend) writeMarkdownEdit(v *markdownViewport, ci int, path string) {
	e := v.edit
	ver, err := f.ed.EditMarkdownLines(ci, path, e.version, e.from, e.from+e.n, e.text)
	if err != nil {
		f.ed.Notify(err.Error())
		v.edit = nil
		return
	}
	e.version, e.n = ver, strings.Count(e.text, "\n")+1
	e.spans[ver] = e.n
}

// markdownEditBox draws the text box of an edit, sized and set like the block it replaces.
func (f *Frontend) markdownEditBox(c *ui.Context, v *markdownViewport, ci int, path string, b *markdownBlock, width float32) {
	e := v.edit
	p := f.pal
	size := f.markdownSize(*b)
	_, lh := f.markdownBody() // the box takes the place of the prose it edits: the same lines
	font := c.Theme().Font
	if p.mono || b.kind == markdownCodeBlock || b.kind == markdownTable || b.kind == markdownTableHeader {
		font = f.opts.FontFamily
	}
	// As tall as its text wrapped at the box's width, so the blocks below stay where they read.
	shape := ui.Font{Family: font, Size: size}
	measured := e.text
	if measured == "" || strings.HasSuffix(measured, "\n") {
		measured += " "
	}
	_, h := ui.ShapeText(measured, shape, max(1, width-16)).Size()
	_, one := ui.ShapeText("0", shape, 0).Size()
	h = max(1, float32(math.Round(float64(h/max(one, 1)))))*size*lh + 8
	box := ui.TextAreaBase(c.Key("markdown-edit"), &e.text).Bind(&e.in).AutoFocus().WidthPercent(100).
		Height(min(h, size*lh*40)).
		Font(font).FontSize(size).LineHeight(lh).TextColor(p.text).Padding(4, 8).Radius(4).
		Background(p.chrome.Mix(p.editor, 0.6)).BorderWidth(0, 0, 0, 2).BorderColor(p.accent).
		Label("编辑 Markdown 块")
	box = box.HandleInput(f.markdownLineKeys(v, ci, path))
	if e.sel != nil {
		box.SetTextSelection(e.sel[0], e.sel[1])
		e.seen, e.sel = *e.sel, nil
	} else {
		e.seen[0], e.seen[1] = box.TextSelection()
	}
	e.composing = box.Composing()
	f.markdownSlashMenu(c, v, ci, path)
	if box.Changed() {
		f.writeMarkdownEdit(v, ci, path)
	}
	mod := ui.Ctrl
	if f.Profile().MacKeys() {
		mod = ui.Cmd
	}
	f.markdownFormatKeys(c, box, v, ci, path, mod)
	if v.edit != e {
		return // a write was refused and ended the edit
	}
	ui.Text(c, "Esc 或 "+f.modKeys(keymap.KeyEnter)+" 完成 · 修改即时写入文档，可在源码里撤销").
		FontSize(11).TextColor(p.muted).Margin(4, 0, 0, 0)
	done := box.Shortcut(mod, ui.KeyEnter)
	if e.slash == nil { // with the block menu open, Esc closes only the menu
		done = done || box.Shortcut(0, ui.KeyEscape)
	}
	switch focused := box.Focused(); {
	case focused:
		e.focused = true
	case e.focused:
		done = true // the keyboard went elsewhere: show the block rendered again
	}
	if done && v.edit == e {
		v.edit = nil
		f.ed.FocusColumn(ci)
	}
}

// trackClick notes a double click in the list, which the next frame turns into an edit of the
// block there. Only presses reach the list (the selectable text takes the release), and a
// single press stays free to place a selection or start a drag.
func (v *markdownViewport) trackClick(ev ui.InputEvent) {
	if ev.Button == 0 && ev.Kind == ui.InputPointerDown && ev.Clicks == 2 {
		v.click, v.clickAt = &[2]float32{ev.X, ev.Y}, time.Now()
	}
}

// clickMarkdown edits the block a double click landed on. A preview still parsing the text just
// written keeps the click for a moment, so a click right after typing lands where it was aimed.
func (f *Frontend) clickMarkdown(c *ui.Context, m *markdownState, v *markdownViewport, pf *view.PaneFrame, ci int) {
	if m.version != pf.Version {
		if time.Since(v.clickAt) > time.Second {
			v.click = nil
		}
		return
	}
	at := *v.click
	v.click = nil
	pane := f.cols[ci].pane.Bounds(c)
	x, y := pane.X+at[0], pane.Y+at[1]
	first, last := v.list.Visible()
	for i := max(0, first); i <= last && i < len(v.rows) && i < len(m.blocks); i++ {
		b := v.rows[i].Bounds(c)
		if b.H > 0 && y >= b.Y && y < b.Y+b.H && x >= b.X && x < b.X+b.W {
			if v.edit == nil || !v.edit.covers(m.blocks[i].line, m.version) {
				f.startMarkdownEdit(m, v, pf, i)
			}
			return
		}
	}
}

// markdownFormatKeys gives the edit box a document editor's keys: bold, italic, strikethrough,
// code and link around the selection, Enter continuing a list, task or quote, and Tab nesting
// list items. While an input method composes, its keys stay its own.
func (f *Frontend) markdownFormatKeys(c *ui.Context, box ui.Element, v *markdownViewport, ci int, path string, mod ui.Modifiers) {
	e := v.edit
	if box.Composing() {
		return
	}
	start, end := box.TextSelection()
	apply := func(r mdEdit) {
		e.text, e.sel = r.text, &[2]int{r.start, r.end}
		f.writeMarkdownEdit(v, ci, path)
		c.Invalidate()
	}
	switch {
	case box.Shortcut(mod, ui.KeyB):
		apply(mdToggleWrap(e.text, start, end, "**"))
	case box.Shortcut(mod, ui.KeyI):
		apply(mdToggleWrap(e.text, start, end, "*"))
	case box.Shortcut(mod|ui.Shift, ui.KeyX):
		apply(mdToggleWrap(e.text, start, end, "~~"))
	case box.Shortcut(mod, ui.KeyE):
		apply(mdToggleWrap(e.text, start, end, "`"))
	case box.Shortcut(mod, ui.KeyK):
		apply(mdLink(e.text, start, end))
	}
}

// markdownLineKeys takes Enter and Tab before the text box does: Enter continues a list, task or
// quote, Tab and Shift+Tab nest list items. Elsewhere they do what the box would.
func (f *Frontend) markdownLineKeys(v *markdownViewport, ci int, path string) func(ui.InputEvent) bool {
	return func(ev ui.InputEvent) bool {
		e := v.edit
		if e == nil || ev.Kind != ui.InputKeyDown || e.composing {
			return false
		}
		if ev.Key == ui.KeyV && ev.Mods == f.uiMod() {
			return f.pasteMarkdownImage(v, ci, path)
		}
		if ev.Mods&^ui.Shift != 0 {
			return false
		}
		start, end := e.seen[0], e.seen[1]
		var r mdEdit
		ok := false
		if m := e.slash; m != nil && ev.Mods == 0 {
			switch ev.Key {
			case ui.KeyDown, ui.KeyUp:
				d := 1
				if ev.Key == ui.KeyUp {
					d = len(m.items) - 1
				}
				m.sel = (m.sel + d) % len(m.items)
				return true
			case ui.KeyEnter, ui.KeyTab:
				f.insertMarkdownBlock(v, ci, path, m.items[m.sel])
				return true
			case ui.KeyEscape:
				e.dismissed, e.slash = string([]rune(e.text)[m.at:m.caret]), nil
				return true
			}
		}
		switch {
		case ev.Key == ui.KeyEnter && ev.Mods == 0:
			r, ok = mdEnter(e.text, start, end)
		case ev.Key == ui.KeyTab:
			r, ok = mdIndent(e.text, start, end, ev.Mods == ui.Shift)
		}
		if !ok {
			return false
		}
		e.text, e.sel = r.text, &[2]int{r.start, r.end}
		f.writeMarkdownEdit(v, ci, path)
		return true
	}
}

// markdownSlashMenu shows the block menu while the caret ends a "/" command at a line's start.
func (f *Frontend) markdownSlashMenu(c *ui.Context, v *markdownViewport, ci int, path string) {
	e := v.edit
	q, at, ok := "", 0, false
	if e.seen[0] == e.seen[1] && !e.composing {
		q, at, ok = mdSlash(e.text, e.seen[1])
	}
	var items []mdBlockKind
	if ok {
		items = mdBlockMatches(q)
	}
	cmd := ""
	if ok {
		cmd = string([]rune(e.text)[at:e.seen[1]])
	}
	if len(items) == 0 || cmd == e.dismissed {
		e.slash = nil
		if !ok {
			e.dismissed = ""
		}
		return
	}
	sel := 0
	if e.slash != nil && e.slash.at == at {
		sel = min(e.slash.sel, len(items)-1)
	}
	e.slash = &mdSlashMenu{at: at, caret: e.seen[1], items: items, sel: sel}
	p := f.pal
	ui.Column(c.Key("markdown-slash")).Margin(4, 0, 0, 0).Padding(4).Radius(6).MaxWidth(260).
		Background(p.chrome).BorderWidth(1).BorderColor(p.border).Label("插入块").Children(func() {
		for i, k := range items {
			row := ui.Row(c.Key("slash:"+k.label)).Padding(3, 8).Radius(4).Gap(8).AlignItems(ui.Center).Label(k.label)
			if i == sel {
				row.Background(p.selected)
			}
			row.Children(func() {
				ui.Text(c, k.label).FontSize(12.5).TextColor(p.text)
				ui.Spacer(c)
				ui.Text(c, strings.SplitN(strings.SplitN(k.insert, "|", 2)[0], "\n", 2)[0]).Font(f.opts.FontFamily).FontSize(11).TextColor(p.muted)
			})
			if row.Clicked() {
				f.insertMarkdownBlock(v, ci, path, k)
				e.in.Focus(c)
			}
		}
	})
}

// insertMarkdownBlock replaces the slash command with a block and writes it through.
func (f *Frontend) insertMarkdownBlock(v *markdownViewport, ci int, path string, k mdBlockKind) {
	e := v.edit
	m := e.slash
	if m == nil {
		return
	}
	r := mdInsertBlock(e.text, m.at, m.caret, k)
	e.text, e.sel, e.slash = r.text, &[2]int{r.start, r.end}, nil
	f.writeMarkdownEdit(v, ci, path)
}

// uiMod is the window's command modifier: Cmd on macOS, Ctrl elsewhere.
func (f *Frontend) uiMod() ui.Modifiers {
	if f.Profile().MacKeys() {
		return ui.Cmd
	}
	return ui.Ctrl
}

// pasteMarkdownImage saves an image pasted into the edit box beside the document and links it
// where the selection was. Only an image without text counts: a copied text pastes as text.
func (f *Frontend) pasteMarkdownImage(v *markdownViewport, ci int, path string) bool {
	img := f.pastedImage()
	if img == nil {
		return false
	}
	e := v.edit
	link, err := f.ed.SaveMarkdownImage(path, img)
	if err != nil {
		f.ed.Notify(err.Error())
		return true
	}
	r := []rune(e.text)
	start, end := clampSel(len(r), e.seen[0], e.seen[1])
	ins := "![](" + link + ")"
	e.text = string(r[:start]) + ins + string(r[end:])
	at := start + len([]rune(ins))
	e.sel = &[2]int{at, at}
	f.writeMarkdownEdit(v, ci, path)
	return true
}

// pastedImage returns the clipboard's image as PNG when the clipboard holds no text.
func (f *Frontend) pastedImage() []byte {
	if f.clipImage != nil {
		return f.clipImage()
	}
	if f.winp.Load() == nil { // no window, as in headless tests: no system clipboard
		return nil
	}
	if mygo.Clipboard.ReadText() != "" {
		return nil
	}
	return mygo.Clipboard.ReadImage()
}
