package gui

import (
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

// findCountW 是查找框里「3 / 10」的宽度。
const findCountW = 74

// findView 是查找框：浮在有焦点那一列的右上角（样子照着 Mayoi）。输入框是原生的；
// 打的字、选项、上一个下一个、替换都交给内核。
func (f *Frontend) findView(c *ui.Context) {
	p := f.pal
	fd := &f.frame.Find
	if f.findQuery != fd.Query && !f.findEditing {
		f.findQuery = fd.Query // 内核改了查询（选区做查询）
	}
	if f.findWith != fd.With && !f.findEditing {
		f.findWith = fd.With
	}
	f.findEditing = false
	selectField := fd.SelectGen != f.findSelectGen
	focusRequested := fd.Focus != f.findFocus || selectField
	if focusRequested {
		f.findFocus = fd.Focus
		switch fd.Focus {
		case view.FindQuery:
			f.findQueryIn.Focus(c)
		case view.FindReplace:
			f.findWithIn.Focus(c)
		}
	}
	box := f.floating(c, ui.Column(c.Key("find")).Absolute().Top(4).Right(22).Width(430).Padding(5, 6).Gap(4).Radius(8)).
		Role(ui.RoleToolbar).Label(i18n.T("Find"))
	box.Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			arrow := iconChevronRight
			if fd.Replacing {
				arrow = iconChevronDown
			}
			if f.iconButton(c, arrow, i18n.T("Toggle Replace"), false).Size(18, 22).Clicked() {
				if fd.Replacing {
					f.ed.Run("actions.find")
				} else {
					f.ed.Run("editor.action.startFindReplaceAction")
				}
			}
			field := ui.Row(c).Grow(1).Height(26).Padding(0, 4, 0, 8).Gap(2).AlignItems(ui.Center).Radius(5).
				Background(p.editor).Border(1, p.border)
			field.Children(func() {
				in := ui.TextInputBase(c.Key("find-query"), &f.findQuery).Bind(&f.findQueryIn).Grow(1).FontSize(12.5).
					Placeholder(i18n.T("Find")).Label(i18n.T("Find"))
				if selectField && fd.Focus == view.FindQuery {
					in.Focus().SetTextSelection(0, utf8.RuneCountInString(f.findQuery))
					f.findSelectGen = fd.SelectGen
				}
				if in.Focused() {
					field.Border(1, p.accentDim)
					// Handle focus is applied after layout; the old field must not override the new request.
					if !focusRequested && fd.Focus != view.FindQuery {
						f.ed.FocusFind(int(view.FindQuery))
					}
				}
				if in.Composing() {
					// Native preedit may temporarily remove selected text; publish only the committed value.
					f.findEditing = true
				} else if in.Changed() {
					f.findEditing = true
					f.ed.SetFindQuery(f.findQuery)
				}
				if in.Shortcut(ui.Shift, ui.KeyEnter) {
					f.ed.Run("editor.action.previousMatchFindAction")
				} else if in.Submitted() {
					f.ed.Run("editor.action.nextMatchFindAction")
				}
				f.findToggle(c, iconCase, i18n.T("Match Case"), fd.MatchCase, "toggleFindCaseSensitive")
				f.findToggle(c, iconWord, i18n.T("Match Whole Word"), fd.Word, "toggleFindWholeWord")
				f.findToggle(c, iconRegex, i18n.T("Use Regular Expression"), fd.Regex, "toggleFindRegex")
			})
			col := p.muted
			if fd.Error {
				col = p.danger
			}
			ui.Text(c, fd.Count).FontSize(11.5).TextColor(col).Width(findCountW).SingleLine().TextAlign(ui.Center)
			if f.iconButton(c, iconArrowUp, i18n.F("Previous Match (%s)", f.chordHint(keymap.Chord{Mods: keymap.ModShift, Key: keymap.KeyEnter})), false).Clicked() {
				f.ed.Run("editor.action.previousMatchFindAction")
			}
			if f.iconButton(c, iconArrowDown, i18n.F("Next Match (%s)", f.chordHint(keymap.Chord{Key: keymap.KeyEnter})), false).Clicked() {
				f.ed.Run("editor.action.nextMatchFindAction")
			}
			if f.iconButton(c, iconX, i18n.T("Close (Esc)"), false).Clicked() {
				f.ed.Run("yan.find.close")
			}
		})
		if !fd.Replacing {
			return
		}
		ui.Row(c).Gap(4).AlignItems(ui.Center).Padding(0, 0, 0, 22).Children(func() {
			field := ui.Row(c).Grow(1).Height(26).Padding(0, 8).AlignItems(ui.Center).Radius(5).
				Background(p.editor).Border(1, p.border)
			field.Children(func() {
				in := ui.TextInputBase(c.Key("find-with"), &f.findWith).Bind(&f.findWithIn).Grow(1).FontSize(12.5).
					Placeholder(i18n.T("Replace with")).Label(i18n.T("Replace with"))
				if selectField && fd.Focus == view.FindReplace {
					in.Focus().SetTextSelection(0, utf8.RuneCountInString(f.findWith))
					f.findSelectGen = fd.SelectGen
				}
				if in.Focused() {
					field.Border(1, p.accentDim)
					if !focusRequested && fd.Focus != view.FindReplace {
						f.ed.FocusFind(int(view.FindReplace))
					}
				}
				if in.Composing() {
					// Native preedit may temporarily remove selected text; publish only the committed value.
					f.findEditing = true
				} else if in.Changed() {
					f.findEditing = true
					f.ed.SetFindReplace(f.findWith)
				}
				if in.Submitted() {
					f.ed.Run("editor.action.replaceOne")
				}
			})
			if f.iconButton(c, iconReplace, i18n.T("Replace (↩)"), false).Clicked() {
				f.ed.Run("editor.action.replaceOne")
			}
			if f.iconButton(c, iconReplaceAll, i18n.T("Replace All"), false).Clicked() {
				f.ed.Run("editor.action.replaceAll")
			}
			ui.Box(c).Width(findCountW + 3*26 + 4*4 - (2*26 + 3*4))
		})
	})
	if box.Shortcut(0, ui.KeyEscape) {
		f.ed.Run("yan.find.close")
		f.root.Focus(c)
	}
}

// findToggle 是查找选项的按钮（区分大小写等），开着时画成按下的样子。
func (f *Frontend) findToggle(c *ui.Context, icon *ui.SVG, label string, on bool, cmd string) {
	p := f.pal
	b := ui.Box(c.Key(label)).Size(22, 20).Radius(4).Justify(ui.Center).AlignItems(ui.Center).Label(label).Tooltip(label)
	col := p.muted
	if on {
		b.Background(p.active).Border(1, p.accentDim)
		col = p.text
	} else if b.Hovered() {
		b.Background(p.hover)
	}
	b.Children(func() { ui.Icon(c, icon).FontSize(13).TextColor(col) })
	if b.Clicked() {
		f.ed.Run(cmd)
	}
}
