package gui

import (
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
	east "github.com/yuin/goldmark/extension/ast"

	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/syntax/highlight"
	"github.com/corn12138/yan-source/core/view"
)

// Markdown rendering adapts Mayoi 46990a1 (MIT) to Yan's semantic Frame boundary.
// Parsing stays off the owner thread; every pane owns its selection and scroll state.

// markdownDelay 是打字停下多久才重新解析。
const markdownDelay = 150 * time.Millisecond

// markdownState 是一个文件的预览：解析好的块、缓存的图片和高亮，滚到了哪里。
type markdownState struct {
	blocks         []markdownBlock
	cache          markdownCache
	root           string
	version        uint64 // blocks 是按哪个版本解析的
	loaded         bool
	pending        uint64 // 正在等着解析的版本
	running        bool
	wait           *time.Timer
	gen            atomic.Uint64
	views          map[int]*markdownViewport
	slugs          map[string]int
	tables         map[int][]float32
	tableFont      ui.Font
	words, minutes int
	measure        float32
	measured       ui.Font
	// mermaid 是 blocks 里的 Mermaid 图按什么样式画的，nil 是显示源码；sources 是用户要看源码的图（按行）。
	mermaid *mermaidStyle
	sources map[int]bool
	math    bool
	places  map[*markdownInline]mathPlacement
}

func (f *Frontend) markdownOf(path string) *markdownState {
	if f.markdowns == nil {
		f.markdowns = map[string]*markdownState{}
	}
	m := f.markdowns[path]
	if m == nil {
		m = &markdownState{}
		f.markdowns[path] = m
	}
	return m
}

// followMarkdown 让预览跟上文档：第一次马上解析，之后停 150 毫秒；解析时文档又变了，就扔掉结果再解析一次。
func (f *Frontend) followMarkdown(pf *view.PaneFrame) *markdownState {
	m := f.markdownOf(pf.Path)
	var want *mermaidStyle
	if f.ed.PreviewMermaid() || isMermaidFile(pf.Path) {
		st := f.mermaidStyle()
		want = &st
	}
	// 换了主题（或关掉了 Mermaid）就按新的样式重画。
	same := want == nil && m.mermaid == nil || want != nil && m.mermaid != nil && *want == *m.mermaid
	mathEnabled := f.ed.PreviewMath()
	if !same || m.math != mathEnabled {
		m.loaded = false
	}
	if same && m.math == mathEnabled && (m.loaded && m.version == pf.Version && m.root == pf.Root || (m.running || m.wait != nil) && m.pending == pf.Version) {
		return m
	}
	m.mermaid, m.math = want, mathEnabled
	m.pending = pf.Version
	gen := m.gen.Add(1)
	if m.wait != nil {
		m.wait.Stop()
		m.wait = nil
	}
	if m.running {
		return m // 跑着的那次结果作废，它结束时再来
	}
	source, path, root, ver := slices.Clone(pf.Markdown), pf.Path, pf.Root, pf.Version
	start := func() {
		if m.gen.Load() != gen {
			return
		}
		m.wait, m.running = nil, true
		// 后台用缓存的一份拷贝，解析完在主线程上换回来。
		cache := &markdownCache{images: maps.Clone(m.cache.images), code: maps.Clone(m.cache.code), mermaid: want, math: mathEnabled, formulas: maps.Clone(m.cache.formulas)}
		if want != nil && m.cache.mermaid != nil && *want == *m.cache.mermaid {
			cache.diagrams = maps.Clone(m.cache.diagrams)
		}
		go func() {
			blocks := parseMarkdown(source, path, root, func() bool { return m.gen.Load() == gen }, cache)
			f.do(func() {
				m.running, m.cache = false, *cache
				if m.gen.Load() != gen {
					m.pending = 0 // 下一帧按新的版本再解析
					return
				}
				m.blocks, m.root, m.version, m.loaded = blocks, root, ver, true
				m.slugs, m.tables, m.places = map[string]int{}, nil, nil
				for i, b := range blocks {
					if b.slug != "" {
						m.slugs[b.slug] = i
					}
					for _, anchor := range b.anchors {
						m.slugs[anchor] = i
					}
				}
				m.words, m.minutes = markdownWords(blocks)
				for _, v := range m.views {
					v.resync = true
				}
			})
		}()
	}
	if !m.loaded {
		start()
	} else {
		m.wait = time.AfterFunc(markdownDelay, func() { f.do(start) })
	}
	return m
}

// markdownPane 是 Markdown 预览的窗格。
func (f *Frontend) markdownPane(c *ui.Context, pane ui.Element, ci int) {
	pf := &f.frame.Columns[ci].Pane
	m := f.followMarkdown(pf)
	v := m.viewport(ci)
	f.syncMarkdown(c, m, v, ci, pf)
	p := f.pal
	size, lh := f.markdownBody()
	// 正文最宽 80 个「0」，按正文实际用的字体量（The Matrix 的界面字体是等宽的）。
	font := ui.Font{Family: c.Theme().Font, Size: size}
	if p.mono {
		font.Family = f.opts.FontFamily
	}
	if m.measured != font || m.measure == 0 {
		m.measured = font
		m.measure, _ = ui.ShapeText(strings.Repeat("0", 80), font, 0).Size()
	}
	width := min(m.measure, max(1, pane.Bounds().W-2*mdPadX))
	path := pf.Path
	if v.click != nil && !f.mdClickTaken {
		f.clickMarkdown(c, m, v, pf, ci)
	} else {
		v.click = nil
	}
	defer func() { f.mdClickTaken = false }()
	pane.Children(func() {
		ui.List(c.Key("markdown:"+path), &v.list, len(m.blocks)).TrackScroll(&v.scroll).Fill().Background(p.editor).
			Label(i18n.F("Preview %s", path)).Padding(mdPadY, mdPadX).Selectable().HandleInput(func(ev ui.InputEvent) bool {
			if ev.Kind == ui.InputPointerDown {
				f.ed.FocusColumn(ci)
			}
			v.trackClick(ev)
			if ev.Kind == ui.InputScroll || ev.Kind == ui.InputPointerDown || ev.Kind == ui.InputKeyDown {
				v.touched = time.Now()
			}
			return false
		}).Rows(func(row ui.ListRow) {
			c := row.Context
			b := m.blocks[row.Index]
			top, bottom := f.markdownSpacing(b)
			if ed := v.edit; ed != nil && ed.covers(b.line, m.version) {
				if row.Index > 0 && ed.covers(m.blocks[row.Index-1].line, m.version) {
					ui.Box(c).Bind(&v.rows[row.Index]).Height(0) // shown in the text box above
					return
				}
				ui.Row(c).Bind(&v.rows[row.Index]).WidthPercent(100).Justify(ui.Center).Children(func() {
					ui.Column(c).WidthPercent(100).MaxWidth(width).Padding(top, 0, bottom, 0).Children(func() {
						f.markdownEditBox(c, v, ci, path, &b, width)
					})
				})
				return
			}
			next := func() *markdownBlock {
				if row.Index+1 < len(m.blocks) {
					return &m.blocks[row.Index+1]
				}
				return nil
			}()
			if b.tight && (next == nil || !next.tight) || b.kind == markdownTable && (next == nil || next.kind != markdownTable) {
				bottom = size * lh * 0.75
			}
			outside := float32(0)
			if b.quote > 0 && (next == nil || next.quote < b.quote) {
				outside, bottom = max(bottom, size*lh*0.75), 0
			}
			// 图比正文宽时可以用满窗格，免得把字缩到看不清。
			rowWidth := width
			if d := b.diagram; d != nil && d.err == "" && !m.sources[b.line] {
				rowWidth = max(width, min(pane.Bounds().W-2*mdPadX, d.w+2*size+2+(b.offset+b.markerWidth+float32(b.quote))*size))
			}
			ui.Row(c).Bind(&v.rows[row.Index]).WidthPercent(100).Justify(ui.Center).Margin(0, 0, outside, 0).Children(func() {
				ui.Row(c).WidthPercent(100).MaxWidth(rowWidth).AlignItems(ui.Start).Padding(0, 0, 0, b.offset*size).Gap(size * 0.5).Children(func() {
					for q := range b.quote {
						bar := p.guideNow
						if b.alert != alertNone && q == b.alertAt-1 {
							bar = f.markdownColor(b.alert)
						}
						ui.Box(c).Width(3).AlignSelf(ui.Stretch).Background(bar)
					}
					if b.marker != "" {
						if b.marker == "•" {
							ui.Box(c).Width(size * b.markerWidth).Height(size * lh).Children(func() {
								ui.Box(c).Width(size*0.35).Height(size*0.35).Radius(size*0.175).Background(p.text).Margin((size*lh-size*0.35)/2, 0)
							})
						} else {
							ms := size
							if b.footnote {
								ms = f.markdownSize(b) // a footnote's number is its smaller text's
							}
							ui.Text(c, b.marker).Width(size * b.markerWidth).TextColor(p.text).FontSize(ms).LineHeight(lh)
						}
					}
					ui.Column(c).Grow(1).MinWidth(0).Padding(top, 0, bottom, 0).Children(func() {
						f.markdownBlockView(c, m, &b, ci, path, rowWidth)
					})
				})
			})
		})
	})
}

// mdPadY、mdPadX 是预览上下、左右的留白。
const (
	mdPadY = 16
	mdPadX = 24
)

func (f *Frontend) markdownBlockView(c *ui.Context, m *markdownState, b *markdownBlock, ci int, path string, width float32) {
	p := f.pal
	size, lh := f.markdownBody()
	code := f.opts.FontSize // code blocks stay at the code font's size
	switch b.kind {
	case markdownAlertTitle:
		icon, title := markdownAlertHead(b.alert)
		color := f.markdownColor(b.alert)
		ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Text(c, icon+" "+title).FontSize(size).FontWeight(600).TextColor(color)
		})
	case markdownRule:
		ui.Box(c).Height(1).WidthPercent(100).Background(p.border).Margin(size*lh*0.375, 0)
	case markdownImageBlock:
		paneW := f.cols[ci].pane.Bounds(c).W - 2*mdPadX
		w := max(1, min(width, paneW)-(b.offset+b.markerWidth)*size-float32(b.quote)*size)
		f.markdownImage(c, ci, b.runs[0], w, m.root, path)
	case markdownCodeBlock, markdownHTML, markdownMathBlock:
		if fml := b.formula; fml != nil && fml.err == "" && !m.sources[b.line] {
			f.markdownFormula(c, m, b, size, max(1, width-(b.offset+b.markerWidth)*size-float32(b.quote)*size))
			return
		}
		if d := b.diagram; d != nil && d.err == "" && !m.sources[b.line] {
			f.markdownDiagram(c, m, b, size, max(1, width-(b.offset+b.markerWidth)*size-float32(b.quote)*size))
			return
		}
		ui.Column(c).WidthPercent(100).Padding(size*0.5, size*0.75).Background(p.chrome).Radius(6).Children(func() {
			if b.kind == markdownCodeBlock || b.kind == markdownMathBlock {
				ui.Row(c).WidthPercent(100).AlignItems(ui.Center).Children(func() {
					ui.Text(c, b.language).FontSize(code * 0.85).TextColor(p.muted)
					ui.Spacer(c)
					if fml := b.formula; fml != nil && fml.err == "" && f.iconButton(c, iconImage, i18n.T("Show Formula"), false).Clicked() {
						f.mdClickTaken = true
						delete(m.sources, b.line)
					}
					if d := b.diagram; d != nil && d.err == "" && f.iconButton(c, iconImage, i18n.T("Show Diagram"), false).Clicked() {
						f.mdClickTaken = true
						delete(m.sources, b.line)
					}
					if f.iconButton(c, iconCopy, i18n.T("Copy Code"), false).Clicked() {
						f.mdClickTaken = true
						if err := (host{f}).SetClipboard(b.codeText); err != nil {
							f.ed.Notify(err.Error())
						} else {
							f.ed.Notify(i18n.T("Code copied"))
						}
					}
				})
			}
			f.markdownText(c, ci, b.runs, code, b.kind == markdownHTML, m.root, path, true)
		})
		if d := b.diagram; d != nil && d.err != "" {
			ui.Text(c, i18n.F("Couldn't draw this Mermaid diagram: %s", d.err)).FontSize(size*0.85).TextColor(p.danger).Margin(size*0.4, 0, 0, 0)
		}
		if fml := b.formula; fml != nil && fml.err != "" {
			ui.Text(c, i18n.F("Couldn't draw this formula: %s", fml.err)).FontSize(size*0.85).TextColor(p.danger).Margin(size*0.4, 0, 0, 0)
		}
	case markdownTable, markdownTableHeader:
		head := b.kind == markdownTableHeader
		cols := f.markdownTableColumns(m, b.table, ui.Font{Family: c.Theme().Font, Size: size})
		total := float32(0)
		for _, w := range cols {
			total += w
		}
		ui.Row(c).WidthPercent(100).Children(func() {
			for i, cell := range b.cells {
				box := ui.Column(c).WidthPercent(100*cols[i]/total).Padding(size*0.35, size*0.5).Border(1, p.border).BorderWidth(0, 1, 1, 0)
				switch {
				case head && i == 0:
					box.BorderWidth(1)
				case head:
					box.BorderWidth(1, 1, 1, 0)
				case i == 0:
					box.BorderWidth(0, 1, 1, 1)
				}
				if head || b.row%2 == 0 {
					box.Background(p.chrome)
				}
				box.Children(func() {
					align := ui.Start
					if i < len(b.aligns) {
						switch b.aligns[i] {
						case east.AlignCenter:
							align = ui.Center
						case east.AlignRight:
							align = ui.End
						}
					}
					ui.Column(c).TextAlign(align).WidthPercent(100).Children(func() {
						for len(cell) > 0 {
							n := 0
							for n < len(cell) && cell[n].bitmap == nil {
								n++
							}
							if n > 0 {
								f.markdownText(c, ci, cell[:n], size, false, m.root, path, false, align)
							}
							if n < len(cell) {
								f.markdownImage(c, ci, cell[n], width/float32(len(b.cells))-size, m.root, path)
								n++
							}
							cell = cell[n:]
						}
					})
				})
			}
		})
	default:
		f.markdownText(c, ci, b.runs, f.markdownSize(*b), b.kind == markdownHeading && b.level == 6 || b.footnote, m.root, path, false)
		if b.kind == markdownHeading && b.level <= 2 {
			ui.Box(c).Height(1).WidthPercent(100).Background(p.border).Margin(size*0.4, 0, 0, 0)
		}
	}
}

// markdownDiagram 画一张 Mermaid 图：原大或缩到栏宽，悬停时给「显示源码」「复制代码」。
func (f *Frontend) markdownDiagram(c *ui.Context, m *markdownState, b *markdownBlock, size, width float32) {
	p, d := f.pal, b.diagram
	pad := size
	w := min(d.w, max(1, width-2*pad-2))
	h := w * d.h / d.w
	card := ui.Column(c.Key("diagram")).WidthPercent(100).Padding(pad).Background(p.editor).Border(1, p.border).Radius(8).AlignItems(ui.Center)
	over := card.Hovered()
	card.Children(func() {
		ui.Box(c).Width(w).Height(h).Label(i18n.T("Mermaid diagram")).Draw(d.draw)
		if !over {
			return
		}
		ui.Row(c).Absolute().Top(size * 0.4).Right(size * 0.4).Gap(2).Background(p.chrome).Radius(4).Children(func() {
			if f.iconButton(c, iconFileCode, i18n.T("Show Source"), false).Clicked() {
				f.mdClickTaken = true
				if m.sources == nil {
					m.sources = map[int]bool{}
				}
				m.sources[b.line] = true
			}
			if f.iconButton(c, iconCopy, i18n.T("Copy Code"), false).Clicked() {
				f.mdClickTaken = true
				if err := (host{f}).SetClipboard(b.codeText); err != nil {
					f.ed.Notify(err.Error())
				} else {
					f.ed.Notify(i18n.T("Code copied"))
				}
			}
		})
	})
}

// syntaxColor 是代码里一类记号的颜色；普通的文字没有自己的颜色。
func (f *Frontend) syntaxColor(k highlight.Kind) (ui.Color, bool) {
	if k == highlight.Plain {
		return ui.Color{}, false
	}
	return f.pal.fg[style.Keyword+style.ID(k-highlight.Keyword)], true
}

// markdownSpan 把一段行内文字的样式（粗、斜、删除线、代码、链接）都算好。
func (f *Frontend) markdownSpan(r markdownInline, color ui.Color, size float32, blockCode bool) ui.Span {
	p := f.pal
	if r.math != nil && r.math.err == "" {
		return mathSpan(r.math, size)
	}
	span := ui.Span{Text: r.text, Color: color, Italic: r.italic, Strikethrough: r.strike}
	if p.mono || r.code || blockCode {
		span.Font = f.opts.FontFamily
	}
	if r.bold {
		span.Weight = 700
	}
	if r.raw || r.image != "" && r.bitmap == nil {
		span.Color = p.muted
	}
	if r.code {
		if blockCode {
			if col, ok := f.syntaxColor(r.kind); ok {
				span.Color = col
			}
		} else {
			span.Background = p.surfaceHover
			span.Text = " " + span.Text + " "
		}
	}
	if r.link != "" && r.image == "" {
		span.Color = p.accent
	}
	if r.strike {
		span.DecorationColor = span.Color
	}
	if r.math != nil {
		span.Color, span.Font = p.danger, f.opts.FontFamily
	}
	return span
}

// markdownBody is the preview's reading size and line height: larger and looser than the code font
// it follows (16/28 beside the default 14), as the Quiet UI design reads Chinese prose.
func (f *Frontend) markdownBody() (size, lineHeight float32) {
	return f.opts.FontSize * 8 / 7, 1.75
}

func (f *Frontend) markdownSize(b markdownBlock) float32 {
	size, _ := f.markdownBody()
	if b.footnote {
		size *= 0.875 // footnotes are a size smaller, as GitHub's
	}
	if b.kind == markdownHeading {
		size *= []float32{2, 1.5, 1.25, 1, 0.875, 0.85}[b.level-1]
	}
	return size
}

func (f *Frontend) markdownSpacing(b markdownBlock) (top, bottom float32) {
	size, lh := f.markdownBody()
	gap := size * lh * 0.75
	bottom = gap
	if b.tight || b.kind == markdownTable || b.kind == markdownTableHeader {
		bottom = 0
	}
	if b.kind == markdownHeading {
		top, bottom = gap, gap*0.5
	}
	return top, bottom
}

func (f *Frontend) markdownText(c *ui.Context, ci int, runs []markdownInline, size float32, muted bool, root, path string, blockCode bool, aligns ...ui.Align) {
	p := f.pal
	color := p.text
	if muted {
		color = p.muted
	}
	_, lh := f.markdownBody()
	if blockCode {
		lh = f.opts.LineHeight
	}
	align := ui.Start
	if len(aligns) > 0 {
		align = aligns[0]
	}
	t := ui.RichText(c).TextAlign(align).FontSize(size).LineHeight(lh).TextColor(color).DecorationColor(color).WidthPercent(100)
	if blockCode || p.mono {
		t.Font(f.opts.FontFamily)
	}
	font := ui.Font{Family: c.Theme().Font, Size: size}
	if blockCode || p.mono {
		font.Family = f.opts.FontFamily
	}
	if slices.ContainsFunc(runs, func(r markdownInline) bool { return r.math != nil && r.math.err == "" }) {
		f.markdownMath(f.markdowns[path], t, runs, font, color, blockCode, lh, align)
	}
	t.Children(func() {
		for _, r := range runs {
			span := fitMathSpan(f.markdownSpan(r, color, size, blockCode), t.Bounds().W)
			if r.link != "" {
				if e := f.markdownLink(c, ci, r.link, root, path, false); e.Valid() {
					e.Children(func() { ui.RichText(c, span) })
					continue
				}
			}
			ui.RichText(c, span)
		}
	})
}

// markdownLink 是一个链接：网页用浏览器打开，本地文件在砚里打开；别的（mailto: 这类）不能点。
func (f *Frontend) markdownLink(c *ui.Context, ci int, dest, root, path string, image bool) ui.Element {
	u, err := url.Parse(dest)
	remote := err == nil && (u.Scheme == "http" || u.Scheme == "https")
	local := markdownPath(root, path, dest)
	heading := -1
	m := f.markdowns[path]
	if m != nil && err == nil && u.Scheme == "" && u.Host == "" && u.Path == "" && u.Fragment != "" {
		if at, ok := m.slugs[strings.ToLower(u.Fragment)]; ok {
			heading = at
		}
	}
	if !remote && local == "" && heading < 0 {
		return ui.Element{}
	}
	var e ui.Element
	if image {
		e = ui.Box(c).Cursor(ui.CursorPointer).Focusable()
	} else {
		e = ui.Link(c, "", "")
	}
	e = e.Label(i18n.F("Open link %s", dest))
	if remote {
		return e.OnClick(func() { f.mdClickTaken = true; c.OpenURL(dest) })
	}
	if heading >= 0 {
		return e.OnClick(func() {
			f.mdClickTaken = true
			v := m.viewport(ci)
			v.touched = time.Now()
			v.list.ScrollTo(heading, ui.Start)
		})
	}
	return e.OnClick(func() {
		f.mdClickTaken = true
		if err := f.ed.OpenFile(local); err != nil {
			f.ed.Notify(err.Error())
		}
	})
}

func (f *Frontend) markdownImage(c *ui.Context, ci int, r markdownInline, width float32, root, path string) {
	draw := func() {
		w := min(r.w, max(1, width))
		ui.Image(c, r.bitmap).Width(w).Height(w * r.h / r.w).Fit(ui.Contain)
	}
	if e := f.markdownLink(c, ci, r.link, root, path, true); e.Valid() {
		e.Children(draw)
	} else {
		draw()
	}
}

// Table widths are cached per parse and font, so scrolling does not repeatedly shape every cell.
func (f *Frontend) markdownTableColumns(m *markdownState, first int, font ui.Font) []float32 {
	if m.tables == nil || m.tableFont != font {
		m.tables = map[int][]float32{}
		m.tableFont = font
	}
	if cols, ok := m.tables[first]; ok {
		return cols
	}
	last := first
	for last+1 < len(m.blocks) && last-first < 200 && m.blocks[last+1].kind == markdownTable && m.blocks[last+1].table == first {
		last++
	}
	cols := markdownColumns(m.blocks[first:last+1], font.Size, func(r markdownInline, head bool) float32 {
		ft := font
		if r.bold || head {
			ft.Weight = 700
		}
		if r.code {
			ft.Family = f.opts.FontFamily
		}
		w, _ := ui.ShapeText(r.text, ft, 0).Size()
		return w
	})
	m.tables[first] = cols
	return cols
}

func (f *Frontend) markdownColor(k markdownAlert) ui.Color {
	if f.pal.mono {
		return f.pal.accent
	}
	colors := [...]string{"", "#4493f8", "#3fb950", "#ab7df8", "#d29922", "#f85149"}
	if !f.pal.dark {
		colors = [...]string{"", "#0969da", "#1a7f37", "#8250df", "#9a6700", "#d1242f"}
	}
	return ui.Hex(colors[k])
}
func markdownAlertHead(k markdownAlert) (string, string) {
	switch k {
	case alertTip:
		return "☀", i18n.T("Tip")
	case alertImportant:
		return "!", i18n.T("Important")
	case alertWarning:
		return "⚠", i18n.T("Warning")
	case alertCaution:
		return "!", i18n.T("Caution")
	default:
		return "ⓘ", i18n.T("Note")
	}
}

func (f *Frontend) markdownReadingTime() string {
	ci := f.frame.Active
	if ci < 0 || ci >= len(f.frame.Columns) {
		return ""
	}
	p := f.frame.Columns[ci].Pane
	if p.Kind != view.PaneMarkdown {
		return ""
	}
	if m := f.markdowns[p.Path]; m != nil && m.loaded && !isMermaidFile(p.Path) {
		return i18n.F("%d words · about %d min", m.words, m.minutes)
	}
	return ""
}

type mathPlacement struct {
	font       ui.Font
	width      float32
	spanWidth  float32
	lineHeight float32
	align      ui.Align
	places     []ui.Rect
}

func (f *Frontend) markdownMath(m *markdownState, text ui.Element, runs []markdownInline, font ui.Font, color ui.Color, blockCode bool, lineHeight float32, align ui.Align) {
	// Spans use the previous layout width; after resize, cache both that width and
	// the paint width so the next frame cannot reuse geometry from oversized spans.
	spanWidth := text.Bounds().W
	var spans []ui.Span
	var formulas []*mathFormula
	for _, r := range runs {
		spans = append(spans, fitMathSpan(f.markdownSpan(r, color, font.Size, blockCode), spanWidth))
		if r.math != nil && r.math.err == "" {
			formulas = append(formulas, r.math)
		}
	}
	lh := font.Size * lineHeight
	text.Draw(func(pt *ui.Painter, r ui.Rect) {
		var places []ui.Rect
		if m != nil {
			if pl, ok := m.places[&runs[0]]; ok && pl.font == font && pl.width == r.W && pl.spanWidth == spanWidth && pl.lineHeight == lh && pl.align == align {
				places = pl.places
			}
		}
		if places == nil {
			places = mathPlacesAligned(spans, font, r.W, lh, align)
			if m != nil {
				if m.places == nil {
					m.places = map[*markdownInline]mathPlacement{}
				}
				m.places[&runs[0]] = mathPlacement{font, r.W, spanWidth, lh, align, places}
			}
		}
		for i, at := range places {
			if i < len(formulas) {
				f := formulas[i]
				f.draw(pt, ui.Rect{X: r.X + at.X, Y: r.Y + at.Y, W: at.W, H: at.H + f.depth*at.W/f.w}, color)
			}
		}
	})
}

func (f *Frontend) markdownFormula(c *ui.Context, m *markdownState, b *markdownBlock, size, width float32) {
	p, formula := f.pal, b.formula
	size *= 1.1
	w := min(formula.w*size, max(1, width))
	h := w * (formula.ascent + formula.depth) / formula.w
	block := ui.Column(c.Key("math")).WidthPercent(100).AlignItems(ui.Center).Padding(size*0.25, 0)
	over := block.Hovered()
	block.Children(func() {
		ui.Box(c).Width(w).Height(h).Label(i18n.F("Formula: %s", b.codeText)).Draw(func(pt *ui.Painter, r ui.Rect) { formula.draw(pt, r, p.text) })
		if !over {
			return
		}
		ui.Row(c).Absolute().Top(0).Right(0).Background(p.editor).Radius(4).Children(func() {
			if f.iconButton(c, iconFileCode, i18n.T("Show Source"), false).Clicked() {
				f.mdClickTaken = true
				if m.sources == nil {
					m.sources = map[int]bool{}
				}
				m.sources[b.line] = true
			}
			if f.iconButton(c, iconCopy, i18n.T("Copy Code"), false).Clicked() {
				f.mdClickTaken = true
				if err := (host{f}).SetClipboard(b.codeText); err != nil {
					f.ed.Notify(err.Error())
				} else {
					f.ed.Notify(i18n.T("Code copied"))
				}
			}
		})
	})
}

func fitMathSpan(span ui.Span, width float32) ui.Span {
	if width > 0 && span.ObjectWidth > width {
		span.ObjectHeight *= width / span.ObjectWidth
		span.ObjectWidth = width
	}
	return span
}
