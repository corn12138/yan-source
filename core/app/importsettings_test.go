package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
)

// An import is applied, not only copied: a font size the menus once saved in state.json does
// not keep overriding the imported one, and imported keys work at once.
func TestImportVSCodeSettingsTakeEffect(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	os.MkdirAll(filepath.Dir(config.UserPath()), 0o755)
	writeFile(t, config.UserPath(), "# mine\n[editor]\nvim = false\n")
	vs := t.TempDir()
	writeFile(t, filepath.Join(vs, "settings.json"), `{"editor.fontSize": 16, "editor.tabSize": 2, "editor.fontFamily": "Fira Code, monospace", "editor.fontLigatures": true}`)
	writeFile(t, filepath.Join(vs, "keybindings.json"), `[{ "key": "cmd+k cmd+s", "command": "workbench.action.files.saveAll" }]`)

	e, _ := New(config.Default(), fakeLayouter{}, &fakeHost{}, macGUI)
	t.Cleanup(e.Close)
	st := config.LoadState(filepath.Join(t.TempDir(), "state.json"))
	e.SetState(st)
	e.zoom(4) // the menu's zoom saves the default + 4 in state.json
	e.reloadConfig()
	if e.cfg.FontSize != config.Default().FontSize+4 {
		t.Fatalf("zoom = %v", e.cfg.FontSize)
	}
	if err := e.importSettingsFrom("VS Code", vs); err != nil {
		t.Fatal(err)
	}
	if e.cfg.FontSize != 16 || e.cfg.TabWidth != 2 || e.cfg.FontFamily != "Fira Code, monospace" {
		t.Fatalf("not applied: size %v, tab %d, family %q", e.cfg.FontSize, e.cfg.TabWidth, e.cfg.FontFamily)
	}
	if family, lh := e.Font(); family != "Fira Code, monospace" || lh <= 0 {
		t.Fatalf("Font() = %q %v", family, lh)
	}
	if got := resolve(t, e, keymap.CtxGlobal, "cmd+k cmd+s"); got != "workbench.action.files.saveAll" {
		t.Fatalf("imported key = %q", got)
	}
	if !strings.Contains(e.status, "已生效") || !strings.Contains(e.status, "editor.fontLigatures") {
		t.Fatalf("status %q", e.status)
	}
	toml, _ := os.ReadFile(config.UserPath())
	if !strings.Contains(string(toml), "# mine") || !strings.Contains(string(toml), "vim = false") {
		t.Fatalf("config.toml lost the user's own lines:\n%s", toml)
	}
	// A config file with unsaved edits is not written behind its tab.
	e.OpenFile(config.UserPath())
	typeText(e, "# draft\n")
	if err := e.importSettingsFrom("VS Code", vs); err == nil || !strings.Contains(err.Error(), "未保存") {
		t.Fatalf("import over a dirty config.toml: %v", err)
	}
}

func TestImportSettingsFileAcceptsAFileOrFolder(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	vs := t.TempDir()
	writeFile(t, filepath.Join(vs, "settings.json"), `{"editor.tabSize": 3}`)
	e, _ := New(config.Default(), fakeLayouter{}, &fakeHost{}, macGUI)
	t.Cleanup(e.Close)
	e.importSettingsFile()
	if err := e.ov.accept(e, filepath.Join(vs, "settings.json")); err != nil {
		t.Fatal(err)
	}
	if e.cfg.TabWidth != 3 {
		t.Fatalf("tab width %d", e.cfg.TabWidth)
	}
	if err := e.importSettingsFrom("empty", t.TempDir()); err == nil {
		t.Fatal("a folder without settings must say so")
	}
}

// The terminal panel's place: from config.toml, from the menus (kept in state.json), and from
// VS Code's panel location on import.
func TestPanelPositionSettings(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	os.MkdirAll(filepath.Dir(config.UserPath()), 0o755)
	writeFile(t, config.UserPath(), "[terminal]\nposition = \"left\"\n")
	cfg, warns := config.Load("")
	e, _ := New(cfg, fakeLayouter{}, &fakeHost{}, macGUI)
	t.Cleanup(e.Close)
	e.SetState(config.LoadState(filepath.Join(t.TempDir(), "state.json")))
	if len(warns) > 0 || e.PanelPosition() != "left" {
		t.Fatalf("from config: %q %v", e.PanelPosition(), warns)
	}
	gen := e.settingsGen
	e.Run("workbench.action.positionPanelRight")
	if e.PanelPosition() != "right" || e.settingsGen == gen || *e.state.PanelPosition != "right" {
		t.Fatal("menu choice not applied and kept")
	}
	e.reloadConfig()
	if e.PanelPosition() != "right" {
		t.Fatal("the menu's choice must survive a reload")
	}
	vs := t.TempDir()
	writeFile(t, filepath.Join(vs, "settings.json"), `{"workbench.panel.defaultLocation": "bottom"}`)
	if err := e.importSettingsFrom("VS Code", vs); err != nil || e.PanelPosition() != "" {
		t.Fatalf("import: %q %v", e.PanelPosition(), err)
	}
	writeFile(t, config.UserPath(), "[terminal]\nposition = \"top\"\n")
	if _, warns := config.Load(""); len(warns) != 1 {
		t.Fatalf("a wrong position must warn: %v", warns)
	}
}

// A project's mark and colour, chosen in the palette, are kept per folder; an empty mark with
// the automatic colour forgets the choice.
func TestCustomizeProjectLook(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a\n"})
	e.SetState(config.LoadState(filepath.Join(t.TempDir(), "state.json")))
	e.Run("yan.project.customize")
	if err := e.ov.accept(e, "🚀"); err != nil {
		t.Fatal(err)
	}
	if err := e.ov.choices[3].run(e); err != nil { // 橙
		t.Fatal(err)
	}
	if mark, color := e.ProjectLook(root); mark != "🚀" || color != 3 {
		t.Fatalf("look %q %d", mark, color)
	}
	e.Run("yan.project.customize")
	if err := e.ov.accept(e, "太长了"); err == nil {
		t.Fatal("three characters must be refused")
	}
	e.ov.accept(e, "")
	e.ov.choices[0].run(e)
	if mark, color := e.ProjectLook(root); mark != "" || color != 0 || len(e.state.ProjectLooks) != 0 {
		t.Fatal("reset must forget the look")
	}
}

// A chosen picture is copied beside the configuration, so the square keeps it when the original
// goes; the same picture is kept once, and a copy no project shows is deleted.
func TestProjectIconCopies(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a\n"})
	e.SetState(config.LoadState(filepath.Join(t.TempDir(), "state.json")))
	src := filepath.Join(t.TempDir(), "logo.png")
	writeFile(t, src, "png bytes")
	if err := e.SetProjectIcon(root, src); err != nil {
		t.Fatal(err)
	}
	icon := e.ProjectIcon(root)
	os.Remove(src)
	if data, err := os.ReadFile(icon); err != nil || string(data) != "png bytes" {
		t.Fatalf("copy %q: %q %v", icon, data, err)
	}
	if err := e.SetProjectIcon(root, filepath.Join(root, "a.txt")); err == nil {
		t.Fatal("a text file must be refused")
	}
	e.ClearProjectIcon(root)
	if _, err := os.Stat(icon); !os.IsNotExist(err) || e.ProjectIcon(root) != "" {
		t.Fatal("an icon no project shows must be deleted")
	}
}
