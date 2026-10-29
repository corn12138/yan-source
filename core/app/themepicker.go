package app

import (
	"slices"
	"strings"

	"github.com/corn12138/yan-source/core/view"
	"github.com/sahilm/fuzzy"
)

// Theme selection follows Mayoi d81bfc4 (MIT); preview ownership remains in Yan's core.

// Preview is separate from config and persisted state: saving sessions or other
// settings while the picker is open must not commit an unconfirmed appearance.
type themePreview struct{ theme, appearance string }

var themeChoices = []struct{ id, name, detail string }{
	{"", "Mayoi", "清晰的浅色与深色界面"},
	{"matrix", "The Matrix", "绿色代码与深色背景 · 始终使用深色"},
	{"claude", "Claude", "暖色界面与 Clawd"},
}

var themeAppearances = []string{"", "light", "dark"}

func (e *Editor) openThemePicker() error {
	e.closeOverlay()
	e.ov = &overlay{kind: view.OverlayTheme, title: "颜色主题", preview: &themePreview{e.cfg.Theme, e.cfg.Appearance}}
	e.settingsGen++
	e.setOverlayInput("")
	return nil
}

func (e *Editor) themeItems() {
	ov := e.ov
	p := ov.preview
	ov.items = nil
	names := make([]string, len(themeChoices))
	for i, choice := range themeChoices {
		names[i] = choice.name
	}
	query := strings.TrimSpace(ov.input)
	matches := fuzzy.Find(query, names)
	if query == "" {
		matches = nil
		for i, name := range names {
			matches = append(matches, fuzzy.Match{Index: i, Str: name})
		}
	}
	for _, match := range matches {
		choice := themeChoices[match.Index]
		ov.items = append(ov.items, ovItem{label: choice.name, detail: choice.detail, theme: choice.id, labelHit: match.MatchedIndexes, run: func(e *Editor) error {
			return e.keepTheme(choice.id, p.appearance)
		}})
		if choice.id == p.theme {
			ov.sel = len(ov.items) - 1
		}
	}
	e.previewThemeSelection()
	e.themeFooter()
}

func (e *Editor) themeFooter() {
	ov := e.ov
	name := map[string]string{"": "跟随系统", "light": "浅色", "dark": "深色"}[ov.preview.appearance]
	ov.footer = "↑↓ 预览主题 · ←→ 外观：" + name + " · 回车保存 · Esc 取消"
	if ov.input != "" {
		ov.footer = "↑↓ 预览主题 · 回车保存 · Esc 取消"
	}
	if ov.preview.theme == "matrix" {
		ov.footer = "The Matrix 始终使用深色 · 回车保存 · Esc 取消"
	}
	if len(ov.items) == 0 {
		ov.footer = "没有匹配的主题 · Esc 取消"
	}
}

func (e *Editor) previewThemeSelection() {
	ov := e.ov
	if ov == nil || ov.preview == nil || ov.sel < 0 || ov.sel >= len(ov.items) {
		return
	}
	if name := ov.items[ov.sel].theme; name != ov.preview.theme {
		ov.preview.theme = name
		e.settingsGen++
	}
	e.themeFooter()
}

// ThemePreviewing reports a window-local transaction; Frame and Settings expose
// its appearance without changing the configuration shared by other windows.
func (e *Editor) ThemePreviewing() bool { return e.ov != nil && e.ov.preview != nil }

func (e *Editor) PreviewThemeAppearance(appearance string) {
	if !e.ThemePreviewing() || e.ov.preview.theme == "matrix" || !slices.Contains(themeAppearances, appearance) {
		return
	}
	if e.ov.preview.appearance != appearance {
		e.ov.preview.appearance = appearance
		e.settingsGen++
		e.themeFooter()
	}
}

func (e *Editor) stepThemeAppearance(by int) {
	i := slices.Index(themeAppearances, e.ov.preview.appearance)
	e.PreviewThemeAppearance(themeAppearances[max(0, min(i+by, len(themeAppearances)-1))])
}

func (e *Editor) keepTheme(theme, appearance string) error {
	e.cfg.Theme, e.cfg.Appearance = theme, appearance
	if e.state != nil {
		e.state.Theme, e.state.Appearance = &theme, &appearance
		e.saveState()
	}
	e.settingsGen++
	return nil
}
