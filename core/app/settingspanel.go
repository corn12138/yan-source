package app

import (
	"strings"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/view"
)

// Settings is a panel of the choices that apply as they are chosen, starting with the General
// page's interface language; the configuration files stay a choice away for everything else.
// The interface language is the application's: it is saved in state.json, shared by every
// window, and applies from the next launch. Until then every window keeps the language the
// process started in, and the panel says what a restart would change.

const cmdOpenSettings = "yan.settings.open"

func (e *Editor) openSettings() error {
	e.closeOverlay()
	e.ov = &overlay{kind: view.OverlaySettings}
	e.setOverlayInput("")
	e.selectSavedLanguage()
	return nil
}

// selectSavedLanguage puts the selection on the saved interface language.
func (e *Editor) selectSavedLanguage() {
	for i, it := range e.ov.items {
		if it.checked {
			e.ov.sel = i
		}
	}
}

// LanguagePreference is the interface language saved for the next launch: chosen in Settings,
// else written in config.toml, else System.
func (e *Editor) LanguagePreference() i18n.Preference {
	if e.state == nil {
		if e.langPref != "" {
			return e.langPref
		}
		p, _ := (*config.State)(nil).LanguagePreference(e.cfg)
		return p
	}
	p, _ := e.state.LanguagePreference(e.cfg)
	return p
}

// LanguageRestartPending reports whether the next launch would show another interface language
// than this one.
func (e *Editor) LanguageRestartPending() bool {
	return i18n.Resolve(e.LanguagePreference(), i18n.SystemLanguages()) != i18n.Current()
}

// languageName is how settings name preference p: Follow System says what it resolves to.
func languageName(p i18n.Preference) string {
	if l, ok := p.Locale(); ok {
		return l.NativeName()
	}
	return i18n.F("Follow System — %s", i18n.Resolve(i18n.System, i18n.SystemLanguages()).NativeName())
}

func (e *Editor) setLanguagePreference(p i18n.Preference) error {
	if e.state == nil {
		e.langPref = p
	} else if e.state.Language == nil || *e.state.Language != string(p) {
		// An explicit Follow System is saved too: it wins over [ui] language in config.toml.
		if err := e.state.SetLanguage(p); err != nil {
			return userError(i18n.F("Couldn't save settings: %s", err.Error()))
		}
	}
	if h, ok := e.host.(interface{ SettingsChanged() }); ok {
		h.SettingsChanged() // other windows' settings show the new choice
	}
	e.settingsItems()
	return nil
}

func (e *Editor) settingsItems() {
	ov := e.ov
	sel := ov.sel
	ov.items = ov.items[:0]
	saved := e.LanguagePreference()
	langGroup := i18n.T("Interface Language / 界面语言")
	prefs := []i18n.Preference{i18n.System}
	for _, l := range i18n.Locales {
		prefs = append(prefs, i18n.Preference(l))
	}
	var all []ovItem
	for _, p := range prefs {
		all = append(all, ovItem{label: languageName(p), group: langGroup, checked: p == saved, stay: true,
			run: func(e *Editor) error { return e.setLanguagePreference(p) }})
	}
	files := i18n.T("Configuration Files")
	all = append(all,
		ovItem{label: i18n.T("Open User Configuration File"), detail: "config.toml", hint: e.KeyHint("yan.config.open"), group: files, run: (*Editor).openConfig},
		ovItem{label: i18n.T("Open Keyboard Shortcuts (keybindings.json)"), detail: "keybindings.json", hint: e.KeyHint(cmdOpenKeybindings), group: files, run: (*Editor).openKeybindings})
	q := strings.ToLower(strings.TrimSpace(ov.input))
	for _, it := range all {
		if q == "" || strings.Contains(strings.ToLower(it.label+" "+it.detail+" "+it.group), q) {
			ov.items = append(ov.items, it)
		}
	}
	ov.sel = max(0, min(sel, len(ov.items)-1))
	ov.footer = i18n.T("↑↓ choose · Enter to apply · Esc to close")
	if len(ov.items) == 0 {
		ov.footer = i18n.T("No matching settings · Esc to close")
	}
}

func (e *Editor) settingsFrame(dst *view.Overlay) {
	dst.Title, dst.Section = i18n.T("Settings"), i18n.T("General")
	e.settingsItems() // another window may have saved a choice even while this panel is filtered
	dst.Status = i18n.F("Interface language now: %s", i18n.Current().NativeName())
	dst.Notice = ""
	if e.LanguageRestartPending() {
		dst.Notice = i18n.F("Saved: %s. Restart Yan to apply the language change", languageName(e.LanguagePreference()))
	}
}
