package gui

import (
	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/style"
)

// palette 是 core/style 主题在 MyGo 里的颜色。外壳的配色照着 Mayoi 的 applyTheme。
type palette struct {
	src  *style.Palette
	dark bool
	mono bool

	editor, chrome, panel, border, hover, active ui.Color
	text, muted, faint, accent, accentDim        ui.Color
	fill, fillHover, fillPressed, fillText       ui.Color
	surface, surfaceHover, surfacePressed        ui.Color
	scrollbar, inverse, inverseText              ui.Color
	line, selection, selected, selectedDim       ui.Color
	gutter, gutterNow, status, statusFg          ui.Color
	modified, danger, match, matchNow, matchRing ui.Color
	guide, guideNow, bracket, word, shadow       ui.Color
	added, changed, removed, caret               ui.Color

	fg, bg [style.Count]ui.Color
	hasBg  [style.Count]bool
}

func col(c style.Color) ui.Color {
	r, g, b, a := c.Parts()
	return ui.RGBA(r, g, b, float32(a)/255)
}

func newPalette(p *style.Palette) *palette {
	t := &palette{
		src: p, dark: p.Dark, mono: p.Mono,
		editor: col(p.Editor), chrome: col(p.Chrome), panel: col(p.Panel), border: col(p.Border),
		hover: col(p.Hover), active: col(p.Active), text: col(p.Text), muted: col(p.Muted), faint: col(p.Faint),
		accent: col(p.Accent), accentDim: col(p.AccentDim),
		fill: col(p.Fill), fillHover: col(p.FillHover), fillPressed: col(p.FillPressed), fillText: col(p.FillText),
		surface: col(p.Surface), surfaceHover: col(p.SurfaceHover), surfacePressed: col(p.SurfacePressed),
		scrollbar: col(p.Scrollbar), inverse: col(p.Inverse), inverseText: col(p.InverseText),
		line: col(p.Line), selection: col(p.Selection),
		gutter: col(p.Gutter), gutterNow: col(p.GutterNow), status: col(p.Status), statusFg: col(p.StatusFg),
		modified: col(p.Modified), danger: col(p.Danger),
		match: col(p.Match), matchNow: col(p.MatchNow), matchRing: col(p.MatchRing),
		guide: col(p.Guide), guideNow: col(p.GuideNow), bracket: col(p.Bracket), word: col(p.Word), shadow: col(p.Shadow),
		added: col(p.Added), changed: col(p.Changed), removed: col(p.Removed),
	}
	// 选区画成不透明的，叠在编辑区底色上：有焦点时一种，没焦点时淡一半。
	t.selected = t.selection.Over(t.editor)
	t.selectedDim = t.selection.Alpha(0.5).Over(t.editor)
	t.caret = t.accent
	for id := style.ID(0); id < style.Count; id++ {
		t.fg[id] = col(p.Fg(id))
		if bg := p.Bg(id); bg != 0 {
			t.bg[id], t.hasBg[id] = col(bg), true
		}
	}
	t.bg[style.Selection] = t.selected
	return t
}

// applyTheme 用配色设置窗口的 ui.Theme，按钮、滚动条、提示框都跟着它。
func (f *Frontend) applyTheme(c *ui.Context) *palette {
	t := *c.Theme()
	dark := t.Dark
	f.dark = dark
	switch f.opts.Appearance { // 选了浅色或深色：不管桌面环境的配色（照着 Mayoi PR #57）
	case "light":
		dark = false
	case "dark":
		dark = true
	}
	p, moving := f.paletteFade.step(c.Now(), f.paletteFor(dark), c.Preferences().ReduceMotion || f.blurred)
	f.pal = p
	if moving {
		c.AnimationFrame()
	}
	t.Dark = p.dark
	t.Background = p.editor
	t.Accent, t.AccentText = p.fill, p.fillText
	t.AccentHover, t.AccentPressed = p.fillHover, p.fillPressed
	t.Danger, t.Warning, t.Success = p.danger, p.modified, p.added
	t.Selection = p.selection
	t.Border = p.border
	t.Text, t.TextMuted = p.text, p.muted
	t.Focus = p.accentDim
	t.Radius = 5
	t.Spacing = 3.5
	t.FontSize = 13
	t.ScrollbarWidth = 8
	t.Surface, t.SurfaceHover, t.SurfacePressed = p.surface, p.surfaceHover, p.surfacePressed
	t.Scrollbar = p.scrollbar
	t.Inverse, t.InverseText = p.inverse, p.inverseText
	if p.mono {
		t.Font = f.opts.FontFamily
	}
	c.SetTheme(&t)
	return p
}

// floating 给浮在窗口上的框（快速打开）加底色、边框和阴影；浅色窗口的阴影淡一些。
func (f *Frontend) floating(c *ui.Context, box ui.Element) ui.Element {
	p := f.pal
	box.Background(c.Theme().Surface).Border(1, p.border)
	if p.dark {
		box.Shadow(0, 10, 30, 0, ui.RGBA(0, 0, 0, 0.50))
	} else {
		box.Shadow(0, 8, 28, 0, ui.RGBA(41, 40, 36, 0.14))
	}
	return box
}

// lane 是提交图第 i 种颜色。
func (t *palette) lane(i int) ui.Color { return col(t.src.Lane(i)) }
