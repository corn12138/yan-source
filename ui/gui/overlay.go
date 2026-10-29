package gui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/view"
)

// overlayView 是快速打开和命令面板：浮在标题栏下面的框，输入框是原生的（输入法、选区、剪贴板都有），
// 打的字交给内核去搜，方向键、回车、Esc 也交给内核。
func (f *Frontend) overlayView(c *ui.Context) {
	p := f.pal
	o := &f.frame.Overlay
	if f.ovInput != o.Input && !f.ovEditing {
		f.ovInput = o.Input // 内核预填（Ctrl+G 的 ":"、">" 切到命令面板）
	}
	f.ovEditing = false
	ui.Overlay(c, func() {
		back := ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).AlignItems(ui.Center).Modal()
		back.Children(func() {
			box := f.floating(c, ui.Column(c).Width(600).MaxWidthPercent(92).Margin(titleBarH+8, 0, 0, 0).Radius(10)).Clip().
				Role(ui.RoleDialog).Label(o.Title)
			box.Children(func() {
				ui.Row(c).Padding(10, 14).Gap(10).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(p.border).Children(func() {
					icon := iconSearch
					switch o.Kind {
					case view.OverlayPalette:
						icon = iconChevronRight
					case view.OverlaySettings:
						icon = iconSettings
					}
					ui.Icon(c, icon).FontSize(15).TextColor(p.muted)
					in := ui.TextInputBase(c.Key("overlay-input"), &f.ovInput).Grow(1).FontSize(14).AutoFocus().
						Placeholder(overlayHint(o)).Label(o.Title)
					for _, k := range []struct {
						ui ui.Key
						km keymap.Key
					}{{ui.KeyDown, keymap.KeyDown}, {ui.KeyUp, keymap.KeyUp}, {ui.KeyPageDown, keymap.KeyPgDn}, {ui.KeyPageUp, keymap.KeyPgUp}} {
						if in.Shortcut(0, k.ui) {
							f.ed.Handle(app.KeyInput{Chord: keymap.Chord{Key: k.km}})
						}
					}
					if f.ovInput == "" && in.Shortcut(0, ui.KeyBackspace) {
						f.ed.Handle(app.KeyInput{Chord: keymap.Chord{Key: keymap.KeyBackspace}}) // 空的「>」上退格回到快速打开
					}
					if o.Kind == view.OverlayTheme {
						// A nonempty query keeps arrows for caret movement and selection.
						// Native text inputs consume them before Shortcut sees them.
						composing := in.Composing()
						in = in.HandleInput(func(ev ui.InputEvent) bool {
							if composing || f.ovInput != "" || !f.ed.ThemePreviewing() || ev.Kind != ui.InputKeyDown || ev.Mods != 0 || (ev.Key != ui.KeyLeft && ev.Key != ui.KeyRight) {
								return false
							}
							key := keymap.KeyRight
							if ev.Key == ui.KeyLeft {
								key = keymap.KeyLeft
							}
							f.do(func() { f.ed.Handle(app.KeyInput{Chord: keymap.Chord{Key: key}}) })
							return true
						})
					}
					if in.Composing() {
						f.ovEditing = true
					}
					if in.Changed() {
						f.ovEditing = true
						if !in.Composing() {
							f.ed.SetOverlayInput(f.ovInput)
						}
					}
					if in.Submitted() {
						f.ed.Handle(app.KeyInput{Chord: keymap.Chord{Key: keymap.KeyEnter}})
					}
				})
				if o.Kind == view.OverlayTheme {
					ui.Row(c).Padding(6, 14).Gap(6).Children(func() {
						for _, ap := range []struct{ id, label string }{{"", i18n.T("Follow System")}, {"light", i18n.T("Light")}, {"dark", i18n.T("Dark")}} {
							b := ui.Button(c.Key(ap.id), ap.label).Label(i18n.F("Theme appearance: %s", ap.label)).Disabled(f.opts.Theme == "matrix")
							if ap.id == o.Appearance {
								b.Background(p.active)
							}
							if b.Clicked() {
								f.ed.PreviewThemeAppearance(ap.id)
							}
						}
					})
				}
				if o.Kind == view.OverlaySettings {
					f.settingsHeader(c, o)
				}
				f.overlayItems(c, o)
				if o.Footer != "" {
					ui.Row(c).Padding(6, 14).BorderWidth(1, 0, 0, 0).BorderColor(p.border).Justify(ui.End).Children(func() {
						ui.Text(c, o.Footer).FontSize(11.5).TextColor(p.faint).SingleLine()
					})
				}
			})
			if box.PressedOutside() || back.OverlayShortcut(0, ui.KeyEscape) {
				f.ed.CloseOverlay()
			}
		})
	})
}

func (f *Frontend) overlayItems(c *ui.Context, o *view.Overlay) {
	p := f.pal
	if len(o.Items) == 0 {
		if o.Kind == view.OverlayTheme {
			ui.Text(c, i18n.T("No matching themes")).Padding(12, 16).TextColor(p.muted)
		}
		return
	}
	moved := o.Selected != f.ovSel
	f.ovSel = o.Selected
	ui.Scroll(c.Key("overlay-items")).MaxHeight(380).Padding(6).Children(func() {
		for i := range o.Items {
			it := &o.Items[i]
			if it.Group != "" && (i == 0 || o.Items[i-1].Group != it.Group) {
				ui.Text(c.Key(fmt.Sprint("group", i)), it.Group).FontSize(11).FontWeight(600).TextColor(p.muted).Padding(8, 10, 3, 10).Role(ui.RoleHeading)
			}
			row := ui.Row(c.Key(i)).Padding(5, 10).Gap(10).Radius(6).AlignItems(ui.Center)
			switch {
			case i == o.Selected:
				row.Background(p.active)
				if moved {
					row.ScrollIntoView()
				}
			case row.Hovered():
				row.Background(p.hover)
			}
			row.OnClick(func() { f.ed.ChooseOverlay(i) })
			row.Children(func() {
				if o.Kind == view.OverlayTheme {
					dark := f.dark
					if o.Appearance == "light" {
						dark = false
					}
					if o.Appearance == "dark" {
						dark = true
					}
					preview := style.ThemeFor(it.Theme, dark)
					ui.Row(c).Gap(2).Shrink(0).Children(func() {
						for _, color := range []style.Color{preview.Editor, preview.Text, preview.Accent} {
							ui.Box(c).Size(12, 18).Radius(2).Background(col(color)).Border(1, p.border)
						}
					})
				}
				if o.Kind == view.OverlayFinder {
					icon, tint := fileIcon(p, it.Label)
					ui.Icon(c, icon).FontSize(14).TextColor(tint)
				}
				if o.Kind == view.OverlaySettings {
					check := ui.Box(c).Size(14, 14).Shrink(0)
					if it.Checked {
						check.Children(func() { ui.Icon(c, iconCheck).FontSize(14).TextColor(p.accent) })
					}
				}
				f.hitText(c, it.Label, it.LabelHits, 13, p.text).Shrink(0)
				if it.Unreleased {
					f.unreleasedBadge(c)
				}
				if it.Detail != "" {
					f.hitText(c, it.Detail, it.DetailHits, 12, p.faint).Shrink(1)
				}
				ui.Spacer(c)
				if it.Font != "" { // the font picker: each family shown in itself
					ui.Text(c, fontSample).Font(it.Font).FontSize(f.opts.FontSize).TextColor(p.muted).SingleLine().Shrink(1)
				}
				if it.Hint != "" {
					ui.Text(c, it.Hint).FontSize(12).TextColor(p.muted).SingleLine().Shrink(0)
				}
			})
		}
	})
}

// settingsHeader names the Settings page and says what is in effect and what a saved choice
// still needs.
func (f *Frontend) settingsHeader(c *ui.Context, o *view.Overlay) {
	p := f.pal
	ui.Column(c).Padding(10, 16, 4, 16).Gap(3).Children(func() {
		ui.Text(c, o.Section).FontSize(14).FontWeight(600).TextColor(p.text).Role(ui.RoleHeading)
		ui.Text(c, o.Status).FontSize(12).TextColor(p.muted)
		if o.Notice != "" {
			ui.Text(c, o.Notice).FontSize(12).FontWeight(600).TextColor(p.accent).Label(o.Notice)
		}
	})
}

// fontSample is what the font picker writes in each family: Latin, look-alike characters and
// Chinese, so a font's width for Chinese shows.
const fontSample = "Aa 0O 1lI {} 中文注释"

// hitText 写一段文字，模糊匹配命中的字用强调色加粗。
func (f *Frontend) hitText(c *ui.Context, s string, hits []int, size float32, color ui.Color) ui.Element {
	if len(hits) == 0 {
		return ui.Text(c, s).FontSize(size).TextColor(color).SingleLine()
	}
	hit := make(map[int]bool, len(hits))
	for _, h := range hits {
		hit[h] = true
	}
	var spans []ui.Span
	start := 0
	for i := range s {
		if i > start && hit[i] != hit[start] {
			spans = append(spans, f.span(s[start:i], hit[start], size, color))
			start = i
		}
	}
	spans = append(spans, f.span(s[start:], hit[start], size, color))
	return ui.RichText(c, spans...).SingleLine()
}

func (f *Frontend) span(s string, hit bool, size float32, color ui.Color) ui.Span {
	if hit {
		return ui.Span{Text: s, Size: size, Color: f.pal.accent, Weight: 600}
	}
	return ui.Span{Text: s, Size: size, Color: color}
}

// overlayHint 是浮层输入框空着时的提示：问名字、选一项的浮层写着它问的是什么。
func overlayHint(o *view.Overlay) string {
	if (o.Kind == view.OverlayPrompt || o.Kind == view.OverlayChoice || o.Kind == view.OverlayTheme) && o.Title != "" {
		return o.Title
	}
	switch o.Kind {
	case view.OverlayPalette:
		return i18n.T("Type a command name")
	case view.OverlayBranch:
		return i18n.T("Find a branch, or type a new branch name")
	case view.OverlayRecent:
		return i18n.T("Find a recently opened folder by name")
	case view.OverlayPrompt:
		return i18n.T("Type a name")
	case view.OverlayChoice:
		return i18n.T("Choose one")
	case view.OverlaySettings:
		return i18n.T("Search settings")
	}
	return i18n.T("Find files by name, > run commands, : go to line, @ go to symbol")
}
