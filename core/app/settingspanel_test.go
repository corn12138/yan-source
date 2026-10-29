package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

// settingsEditor is a window editor with a .go file open, sharing st with any other window.
func settingsEditor(t *testing.T, st *config.State) *Editor {
	t.Helper()
	root := t.TempDir()
	p := filepath.Join(root, "main.go")
	os.WriteFile(p, []byte("package main\n"), 0o644)
	e, _ := New(config.Default(), fakeLayouter{}, &fakeHost{}, keymap.Profile{Mods: []keymap.Mod{keymap.ModSuper}, GUI: true, OS: "darwin"})
	t.Cleanup(e.Close)
	e.SetPaneSize(0, view.Size{W: 80, H: 20})
	e.SetState(st)
	if err := e.OpenFile(p); err != nil {
		t.Fatal(err)
	}
	return e
}

// checkedLabel is the option Settings marks saved.
func checkedLabel(o view.Overlay) string {
	for _, it := range o.Items {
		if it.Checked {
			return it.Label
		}
	}
	return ""
}

func TestFilteredSettingsFollowAnotherWindow(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	st := config.LoadState(filepath.Join(t.TempDir(), "state.json"))
	first, second := settingsEditor(t, st), settingsEditor(t, st)
	first.Run(cmdOpenSettings)
	first.SetOverlayInput("English")
	second.Run(cmdOpenSettings)
	if err := second.setLanguagePreference("en"); err != nil {
		t.Fatal(err)
	}
	if got := checkedLabel(frameOf(first).Overlay); got != "English" {
		t.Fatalf("filtered settings still show the old choice: %q", got)
	}
	if err := second.setLanguagePreference("zh-Hans"); err != nil {
		t.Fatal(err)
	}
	if got := checkedLabel(frameOf(first).Overlay); got != "" {
		t.Fatalf("English stays checked after another window selected Chinese: %q", got)
	}
}

// chooseLabel picks the Settings item labelled label, by keyboard.
func chooseLabel(t *testing.T, e *Editor, label string) {
	t.Helper()
	o := frameOf(e).Overlay
	for i, it := range o.Items {
		if it.Label == label {
			for o.Selected < i {
				e.Handle(key(keymap.KeyDown, 0))
				o = frameOf(e).Overlay
			}
			for o.Selected > i {
				e.Handle(key(keymap.KeyUp, 0))
				o = frameOf(e).Overlay
			}
			e.Handle(key(keymap.KeyEnter, 0))
			return
		}
	}
	t.Fatalf("no %q in %+v", label, o.Items)
}

func TestSettingsLanguageChoice(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	i18n.SetSystemLanguages([]string{"zh-Hans-CN", "en-US"})
	defer i18n.SetSystemLanguages(nil)
	statePath := filepath.Join(t.TempDir(), "state.json")
	st := config.LoadState(statePath)
	e := settingsEditor(t, st)
	other := settingsEditor(t, st) // a second window of the same process

	e.Run(cmdOpenSettings)
	f := frameOf(e)
	o := f.Overlay
	if !o.Visible || o.Kind != view.OverlaySettings || o.Title != "设置" || o.Section != "通用" {
		t.Fatalf("settings overlay %+v", o)
	}
	if o.Items[0].Group != "界面语言 / Language" || o.Items[0].Label != "跟随系统 — 简体中文" || o.Items[1].Label != "简体中文" || o.Items[2].Label != "English" {
		t.Fatalf("language options %+v", o.Items)
	}
	if checkedLabel(o) != "跟随系统 — 简体中文" || o.Selected != 0 || o.Notice != "" || o.Status != "当前界面语言：简体中文" {
		t.Fatalf("default: checked %q, selected %d, notice %q, status %q", checkedLabel(o), o.Selected, o.Notice, o.Status)
	}
	if st.Language != nil {
		t.Fatal("opening Settings saved a language")
	}

	chooseLabel(t, e, "English")
	o = frameOf(e).Overlay
	if !o.Visible || checkedLabel(o) != "English" {
		t.Fatalf("after choosing English: visible %v, checked %q", o.Visible, checkedLabel(o))
	}
	if o.Notice != "已保存：English。重启 Yan 后应用语言更改" {
		t.Fatalf("notice %q", o.Notice)
	}
	if i18n.Current() != i18n.SimplifiedChinese || e.commands["workbench.action.files.save"].Title != "文件：保存" {
		t.Fatal("the running interface changed language")
	}
	if b, _ := os.ReadFile(statePath); !strings.Contains(string(b), `"language": "en"`) {
		t.Fatalf("state.json: %s", b)
	}
	// The other window agrees about the saved choice and keeps the launch language.
	other.Run(cmdOpenSettings)
	if o2 := frameOf(other).Overlay; checkedLabel(o2) != "English" || o2.Notice == "" || o2.Title != "设置" {
		t.Fatalf("other window: checked %q, notice %q, title %q", checkedLabel(o2), o2.Notice, o2.Title)
	}
	// A window opened after the change still starts in the launch language.
	if late := settingsEditor(t, st); late.commands["workbench.action.files.save"].Title != "文件：保存" {
		t.Fatal("a later window changed language")
	}

	// Choosing it again changes nothing; going back to what runs now clears the notice.
	chooseLabel(t, e, "English")
	if o = frameOf(e).Overlay; checkedLabel(o) != "English" || o.Notice == "" {
		t.Fatalf("idempotent choice: checked %q, notice %q", checkedLabel(o), o.Notice)
	}
	chooseLabel(t, e, "跟随系统 — 简体中文")
	if o = frameOf(e).Overlay; checkedLabel(o) != "跟随系统 — 简体中文" || o.Notice != "" {
		t.Fatalf("back to system: checked %q, notice %q", checkedLabel(o), o.Notice)
	}
	if st.Language == nil || *st.Language != "system" {
		t.Fatalf("explicit system not saved: %v", st.Language)
	}
	chooseLabel(t, e, "简体中文")
	if o = frameOf(e).Overlay; o.Notice != "" {
		t.Fatalf("an explicit choice resolving to the running language asks for a restart: %q", o.Notice)
	}
	if next := config.LoadState(statePath); next.Language == nil || *next.Language != "zh-Hans" {
		t.Fatal("the choice is not there for the next launch")
	}
}

// The document's language in the status bar is the file's, whatever the interface language.
func TestInterfaceLanguageLeavesDocumentLanguage(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	e := settingsEditor(t, config.LoadState(filepath.Join(t.TempDir(), "state.json")))
	doc := func() string { r := frameOf(e).Status.Right; return r[len(r)-1].Text }
	before := doc()
	e.Run(cmdOpenSettings)
	chooseLabel(t, e, "English")
	e.CloseOverlay()
	if after := doc(); after != before || after != "Go" {
		t.Fatalf("document language %q → %q", before, after)
	}
}

// Settings keeps the configuration files a choice away, and the commands that name a file
// still open it.
func TestSettingsCommandsAndFiles(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	e := settingsEditor(t, config.LoadState(filepath.Join(t.TempDir(), "state.json")))
	if id, ok := e.Binding(cmdOpenSettings); !ok || id.Rune != ',' || id.Mods != keymap.ModSuper {
		t.Fatalf("Settings key %+v %v", id, ok)
	}
	if h := e.KeyHint(cmdOpenSettings); h != "⌘," {
		t.Fatalf("hint %q", h)
	}
	// workbench.action.openSettings and openSettingsJson reach these through keybindings.json:
	// TestKeybindingsJSON.
	e.Run("yan.config.open")
	if got := e.cur().doc.buf.Path(); got != config.UserPath() {
		t.Fatalf("yan.config.open opened %q", got)
	}
	e.Run("workbench.action.closeActiveEditor")
	e.Run(cmdOpenSettings)
	chooseLabel(t, e, "打开用户配置文件")
	if got := e.cur().doc.buf.Path(); got != config.UserPath() || frameOf(e).Overlay.Visible {
		t.Fatalf("the configuration item opened %q, Settings visible %v", got, frameOf(e).Overlay.Visible)
	}
	e.Run("yan.settings.language")
	chooseLabel(t, e, "打开键盘快捷方式（keybindings.json）")
	if got := e.cur().doc.buf.Path(); got != config.KeybindingsPath() {
		t.Fatalf("keybindings item opened %q", got)
	}
	e.Run(cmdOpenSettings)
	e.SetOverlayInput("config")
	if o := frameOf(e).Overlay; len(o.Items) != 1 || o.Items[0].Detail != "config.toml" {
		t.Fatalf("filtered settings %+v", o.Items)
	}
}

// The terminal edition shows the same panel and catalogs.
func TestSettingsInTerminal(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	e, _, _ := newTestEditor(t, "x")
	e.SetState(config.LoadState(filepath.Join(t.TempDir(), "state.json")))
	e.Run("workbench.action.showCommands")
	e.SetOverlayInput("界面语言")
	o := frameOf(e).Overlay
	if len(o.Items) == 0 || !strings.Contains(o.Items[0].Label, "界面语言") {
		t.Fatalf("palette %+v", o.Items)
	}
	e.Handle(key(keymap.KeyEnter, 0))
	if o = frameOf(e).Overlay; o.Kind != view.OverlaySettings {
		t.Fatalf("overlay %+v", o)
	}
}

// With the English locale, Yan's own text is English: commands, Settings, templates, errors.
func TestEnglishInterface(t *testing.T) {
	i18n.SetLocale(i18n.English)
	defer i18n.SetLocale(i18n.SimplifiedChinese)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	e := settingsEditor(t, config.LoadState(filepath.Join(t.TempDir(), "state.json")))
	if got := e.commands["workbench.action.files.save"].Title; got != "File: Save" {
		t.Fatalf("command title %q", got)
	}
	e.Run(cmdOpenSettings)
	o := frameOf(e).Overlay
	if o.Title != "Settings" || o.Section != "General" || o.Items[0].Group != "Interface Language / 界面语言" || o.Items[1].Label != "简体中文" {
		t.Fatalf("English settings %+v", o)
	}
	chooseLabel(t, e, "简体中文")
	if o = frameOf(e).Overlay; o.Notice != "Saved: 简体中文. Restart Yan to apply the language change" {
		t.Fatalf("notice %q", o.Notice)
	}
	e.CloseOverlay()
	e.Run("yan.config.open")
	if s := content(e); !strings.HasPrefix(s, "# Yan's user configuration.") || !strings.Contains(s, `# language = "system"`) {
		t.Fatalf("config template %q", s)
	}
	e.Run("no.such.command")
	if f := frameOf(e); f.Status.Message != "Unknown command: no.such.command" {
		t.Fatalf("message %q", f.Status.Message)
	}
	if got := errUntitled.Error(); got != "Files never saved need \"Save As\" first" {
		t.Fatalf("lazy error %q", got)
	}
	var texts []string
	for _, it := range frameOf(e).Status.Right {
		texts = append(texts, it.Text)
	}
	if !strings.HasPrefix(texts[0], "Ln ") || !strings.Contains(texts[0], ", Col ") || texts[1] != "Spaces: 4" {
		t.Fatalf("status %q", texts)
	}
}
