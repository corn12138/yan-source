package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/i18n"
)

// Importing settings from VS Code, Cursor and the other editors that share its files. An import
// is applied, not only copied: config.toml gets the mapped keys (its comments and other keys
// kept), keybindings.json gets the imported bindings after Yan's own, and what the menus last
// saved in state.json for the same settings is cleared so it no longer overrides them. Then the
// configuration reloads in this window, and the file watchers carry it to the other windows.

// importVSCode lets the user choose among the editors found on this system.
func (e *Editor) importVSCode() error {
	srcs := config.VSCodeSources(e.profile.Platform())
	if len(srcs) == 0 {
		return errors.New(i18n.T("No settings of VS Code, Cursor or similar editors found: use \"Import Settings from File\" to choose a settings.json"))
	}
	items := make([]ovItem, 0, len(srcs))
	for _, s := range srcs {
		items = append(items, ovItem{label: s.Name, detail: shortHome(s.Dir), run: func(e *Editor) error {
			return e.importSettingsFrom(s.Name, s.Dir)
		}})
	}
	e.choose(i18n.T("Import settings and shortcuts from which editor (they take effect immediately)"), items...)
	return nil
}

// importSettingsFile asks for a settings.json, or the folder holding it and keybindings.json.
func (e *Editor) importSettingsFile() error {
	e.prompt(i18n.T("Import settings: path of settings.json or its folder (keybindings.json in the same folder is imported too)"), "", func(s string) string {
		return i18n.F("Import %s", strings.TrimSpace(s))
	}, func(e *Editor, s string) error {
		p := e.bindingPath(strings.TrimSpace(s))
		fi, err := os.Stat(p)
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			p = filepath.Dir(p)
		}
		return e.importSettingsFrom(filepath.Base(p), p)
	})
	return nil
}

// importSettingsFrom imports the settings.json and keybindings.json of folder dir.
func (e *Editor) importSettingsFrom(name, dir string) error {
	var sets []config.Setting
	var skipped []string
	found := false
	if data, err := os.ReadFile(filepath.Join(dir, "settings.json")); err == nil {
		found = true
		if sets, skipped, err = config.VSCodeSettings(data); err != nil {
			return err
		}
	}
	var keys []byte
	if data, err := os.ReadFile(filepath.Join(dir, "keybindings.json")); err == nil {
		found, keys = true, data
	}
	if !found {
		return fmt.Errorf(i18n.T("%s has no settings.json or keybindings.json"), shortHome(dir))
	}
	// Both files are checked before either is written, so a refusal changes nothing.
	user, kb := config.UserPath(), config.KeybindingsPath()
	for _, p := range []string{user, kb} {
		if d := e.docOf(p); d != nil && d.buf.Dirty() {
			return fmt.Errorf(i18n.T("%s has unsaved changes: save or close it before importing"), filepath.Base(p))
		}
	}
	var merged []byte
	added := 0
	if keys != nil {
		own, _ := os.ReadFile(kb)
		var err error
		if merged, added, err = config.MergeKeybindings(own, keys, name); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(user), 0o755); err != nil {
		return err
	}
	if len(sets) > 0 {
		src, _ := os.ReadFile(user)
		out := string(src)
		for _, s := range sets {
			out = config.SetTOML(out, s.Table, s.Key, s.Value)
		}
		if err := os.WriteFile(user, []byte(out), 0o644); err != nil {
			return err
		}
		e.clearStateFor(sets)
	}
	if added > 0 {
		if err := os.WriteFile(kb, merged, 0o644); err != nil {
			return err
		}
	}
	e.reloadDocs(user, kb)
	e.reloadConfig()
	msg := i18n.F("Imported %[2]d settings and %[3]d shortcuts from %[1]s; they are in effect", name, len(sets), added)
	if len(skipped) > 0 {
		shown := skipped[:min(len(skipped), 4)]
		msg += i18n.F("; %d items Yan has no equivalent for were not imported: %s", len(skipped), i18n.Join(shown))
		if len(skipped) > len(shown) {
			msg += "…"
		}
	}
	if e.configProblem == "" {
		e.status = msg
	}
	return nil
}

// clearStateFor drops what state.json holds for the imported settings: the menus save font size,
// appearance, autosave, word wrap, the sidebar's side and the terminal panel's there, and it overrides config.toml.
func (e *Editor) clearStateFor(sets []config.Setting) {
	if e.state == nil {
		return
	}
	for _, s := range sets {
		switch s.Table + "." + s.Key {
		case "ui.font_size":
			e.state.FontSize = nil
		case "ui.font_family":
			e.state.FontFamily = nil
		case "ui.line_height":
			e.state.LineHeight = nil
		case "ui.appearance":
			e.state.Appearance = nil
		case "editor.auto_save":
			e.state.AutoSave = nil
		case "editor.word_wrap":
			e.state.WordWrap = nil
		case "explorer.right":
			e.state.SidebarRight = nil
		case "terminal.position":
			e.state.PanelPosition = nil
		}
	}
	e.saveState()
}

// reloadDocs reloads open, clean documents of the files just written, so their tabs show them.
func (e *Editor) reloadDocs(paths ...string) {
	var docs []*document
	for _, p := range paths {
		if d := e.docOf(p); d != nil && !d.buf.Dirty() {
			docs = append(docs, d)
		}
	}
	e.checkDocs(docs)
}
