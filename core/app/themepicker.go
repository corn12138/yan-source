package app

import (
	"slices"
	"strings"

	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/view"
	"github.com/sahilm/fuzzy"
)

// Theme selection follows Mayoi d81bfc4 (MIT); preview ownership remains in Yan's core.

// Preview is separate from config and persisted state: saving sessions or other
// settings while the picker is open must not commit an unconfirmed appearance.
type themePreview struct{ theme, appearance string }

var themeChoices = []struct{ id, name, detail string }{
	{"", "Mayoi", i18n.N("Clear light and dark interface")},
	{"matrix", "The Matrix", i18n.N("Green code on a dark background · always dark")},
	{"claude", "Claude", i18n.N("Warm interface with Clawd")},
}

var themeAppearances = []string{"", "light", "dark"}

func (e *Editor) openThemePicker() error {
	e.closeOverlay()
	e.ov = &overlay{kind: view.OverlayTheme, title: i18n.T("Color Theme"), preview: &themePreview{e.cfg.Theme, e.cfg.Appearance}}
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
		ov.items = append(ov.items, ovItem{label: choice.name, detail: i18n.T(choice.detail), theme: choice.id, labelHit: match.MatchedIndexes, run: func(e *Editor) error {
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
	name := map[string]string{"": i18n.T("Follow System"), "light": i18n.T("Light"), "dark": i18n.T("Dark")}[ov.preview.appearance]
	ov.footer = i18n.F("↑↓ preview themes · ←→ appearance: %s · Enter to save · Esc to cancel", name)
	if ov.input != "" {
		ov.footer = i18n.T("↑↓ preview themes · Enter to save · Esc to cancel")
	}
	if ov.preview.theme == "matrix" {
		ov.footer = i18n.T("The Matrix is always dark · Enter to save · Esc to cancel")
	}
	if len(ov.items) == 0 {
		ov.footer = i18n.T("No matching themes · Esc to cancel")
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
