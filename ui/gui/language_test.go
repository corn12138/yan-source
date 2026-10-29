package gui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/view"
)

// The updater's window speaks the launch language, in a tag the plugin has a translation for,
// and carries Yan's own failure message in the same language.
func TestUpdaterFollowsInterfaceLanguage(t *testing.T) {
	for _, c := range []struct {
		l          i18n.Locale
		tag, check string
	}{
		{i18n.English, "en", "Checking for updates failed."},
		{i18n.SimplifiedChinese, "zh-Hans", "检查更新失败。"},
	} {
		o := updaterOptions(c.l)
		if o.Language != c.tag || !o.DisableAutomaticChecks {
			t.Errorf("%s: language %q, automatic checks off %v", c.l, o.Language, o.DisableAutomaticChecks)
		}
		if s, ok := o.Strings[c.tag]; !ok || !strings.HasPrefix(s.CheckError, c.check) || len(o.Strings) != 1 {
			t.Errorf("%s: strings %+v", c.l, o.Strings)
		}
	}
}

// menuItem finds the item labelled label at path (labels of submenus, then the item's).
func menuItem(items []*mygo.MenuItem, path ...string) *mygo.MenuItem {
	for _, it := range items {
		if it.Label == path[0] {
			if len(path) == 1 {
				return it
			}
			return menuItem(it.Submenu, path[1:]...)
		}
	}
	return nil
}

func TestMenusInInterfaceLanguage(t *testing.T) {
	defer i18n.SetLocale(i18n.SimplifiedChinese)
	old := nativeMenuBar
	defer func() { nativeMenuBar = old }()
	for _, c := range []struct {
		l                                   i18n.Locale
		file, settings, config, cut, window string
	}{
		{i18n.SimplifiedChinese, "文件", "设置…", "打开用户配置文件", "剪切", "窗口"},
		{i18n.English, "File", "Settings…", "Open User Configuration File", "Cut", "Window"},
	} {
		i18n.SetLocale(c.l)
		h := newHarness(t, sample, Options{})
		nativeMenuBar = func() bool { return true }
		items := h.f.menu().Items()
		app := items[0]
		if app.Role != mygo.RoleAppMenu {
			t.Fatalf("%s: first menu %+v", c.l, app)
		}
		s := menuItem(app.Submenu, c.settings)
		accelerator := "Ctrl+,"
		if h.f.Profile().MacKeys() {
			accelerator = "Cmd+,"
		}
		if s == nil || s.ID != "yan.settings.open" || s.Accelerator != accelerator {
			t.Errorf("%s: application menu Settings %+v", c.l, s)
		}
		for _, it := range app.Submenu {
			if it.Role == mygo.RoleQuit && !strings.HasSuffix(it.Label, mygo.App.Name()) {
				t.Errorf("%s: quit %q", c.l, it.Label)
			}
		}
		if it := menuItem(items, c.file, c.config); it == nil || it.ID != "yan.config.open" {
			t.Errorf("%s: %s › %s missing", c.l, c.file, c.config)
		}
		if it := menuItem(items, c.file, c.settings); it != nil {
			t.Errorf("%s: macOS shows Settings in the File menu too", c.l)
		}
		var cut *mygo.MenuItem
		for _, m := range items {
			for _, it := range m.Submenu {
				if it.Role == mygo.RoleCut {
					cut = it
				}
			}
		}
		if cut == nil || cut.Label != c.cut {
			t.Errorf("%s: cut %+v", c.l, cut)
		}
		if w := menuItem(items, c.window); w == nil || w.Role != mygo.RoleWindowMenu || len(w.Submenu) == 0 {
			t.Errorf("%s: window menu %+v", c.l, w)
		}
		nativeMenuBar = func() bool { return false }
		if it := menuItem(h.f.menu().Items(), c.file, c.settings); it == nil || it.ID != "yan.settings.open" || it.Accelerator != accelerator {
			t.Errorf("%s: in-window Settings missing or wrong shortcut: %+v", c.l, it)
		}
	}
}

// The Settings panel by keyboard, in both languages, at a compact and a normal size, light and
// dark: the saved choice moves and the restart notice shows, the running interface stays.
func TestSettingsPanelWindow(t *testing.T) {
	defer i18n.SetLocale(i18n.SimplifiedChinese)
	defer i18n.SetSystemLanguages(nil)
	for _, c := range []struct {
		l                                  i18n.Locale
		system                             []string
		group, follow, saved, notice, menu string
	}{
		{i18n.SimplifiedChinese, []string{"zh-Hans-CN", "en-US"}, "界面语言 / Language", "跟随系统 — 简体中文", "English", "重启 Yan 后应用语言更改", "设置"},
		{i18n.English, []string{"en-GB", "zh-Hans-CN"}, "Interface Language / 界面语言", "Follow System — English", "简体中文", "Restart Yan to apply the language change", "Settings"},
	} {
		i18n.SetSystemLanguages(c.system)
		for _, size := range []struct{ w, h int }{{760, 520}, {1200, 780}} {
			for _, appearance := range []string{"light", "dark"} {
				i18n.SetLocale(c.l)
				t.Setenv("XDG_CONFIG_HOME", t.TempDir())
				h := newHarness(t, sample, Options{Appearance: appearance})
				h.ed.SetState(config.LoadState(filepath.Join(t.TempDir(), "state.json")))
				h.tt.SetSize(size.w, size.h)
				h.ed.Run("yan.settings.open")
				h.settle()
				for _, s := range []string{c.menu, c.group, c.follow, "简体中文", "English", "config.toml"} {
					if _, ok := h.tt.Find(s); !ok {
						t.Fatalf("%s %dx%d %s: %q not shown in %q", c.l, size.w, size.h, appearance, s, h.tt.Texts())
					}
				}
				if r, ok := h.tt.Find(c.group); !ok || r.X < 0 || r.X+r.W > float32(size.w) {
					t.Errorf("heading clipped: %+v", r)
				}
				name := fmt.Sprintf("settings-%s-%dx%d-%s", c.l, size.w, size.h, appearance)
				h.shot(name)
				if _, ok := h.tt.Find(c.notice); ok || containsText(h.tt.Texts(), c.notice) {
					t.Fatalf("%s: a restart notice before any change", c.l)
				}
				// Down to the language that isn't running, Enter: saved, notice, panel open.
				steps := 2
				if c.l == i18n.English {
					steps = 1
				}
				for range steps {
					h.tt.Key(0, ui.KeyDown)
					h.settle()
				}
				h.tt.Key(0, ui.KeyEnter)
				h.settle()
				if _, ok := h.tt.Find(c.notice); !ok && !containsText(h.tt.Texts(), c.notice) {
					t.Fatalf("%s: no restart notice in %q", c.l, h.tt.Texts())
				}
				if got := h.ed.LanguagePreference(); string(got) == "" || got == i18n.System {
					t.Fatalf("%s: saved %q", c.l, got)
				}
				if i18n.Current() != c.l {
					t.Fatal("the running interface changed language")
				}
				h.shot(name + "-pending")
				h.tt.Key(0, ui.KeyEscape)
				h.settle()
				if h.f.frame.Overlay.Visible {
					t.Fatal("Esc left Settings open")
				}
			}
		}
	}
}

func containsText(texts []string, s string) bool {
	for _, x := range texts {
		if strings.Contains(x, s) {
			return true
		}
	}
	return false
}

func TestSidebarViewsStayVisibleInBothLanguages(t *testing.T) {
	defer i18n.SetLocale(i18n.SimplifiedChinese)
	for _, locale := range i18n.Locales {
		for _, width := range []float32{170, 240} {
			t.Run(fmt.Sprintf("%s/%g", locale, width), func(t *testing.T) {
				i18n.SetLocale(locale)
				h := newHarness(t, sample, Options{SidebarWidth: width})
				h.f.frame.Sidebar.Changes = 123
				tt := ui.NewTester(func(c *ui.Context) {
					ui.Column(c).Width(width).FillHeight().Children(func() { h.f.sidebar(c) })
				}, 760, 520)
				tt.Frame()
				tt.Frame()
				for _, label := range []string{
					i18n.TC("sidebar", "Files"), i18n.T("Search"), i18n.TC("sidebar", "Changes"), i18n.T("Learn"), "99+",
				} {
					bounds, ok := tt.Find(label)
					if !ok || bounds.W <= 0 || bounds.X < 0 || bounds.X+bounds.W > width || bounds.Y+bounds.H > 520 {
						t.Fatalf("%q is outside the sidebar: %+v (found %v)", label, bounds, ok)
					}
				}
				if err := tt.Click(i18n.T("Learn: Feature Tour")); err != nil {
					t.Fatal(err)
				}
				h.settle()
				if h.f.frame.Sidebar.View != view.SideLearn {
					t.Fatal("the wrapped Learn view cannot be selected")
				}
			})
		}
	}
}
