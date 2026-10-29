package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/i18n"
)

// languageFiles writes a user config.toml (none when user is "") and a project .yan.toml (none
// when project is ""), and returns the project folder.
func languageFiles(t *testing.T, user, project string) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if user != "" {
		os.MkdirAll(filepath.Dir(UserPath()), 0o755)
		if err := os.WriteFile(UserPath(), []byte(user), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	root := t.TempDir()
	if project != "" {
		if err := os.WriteFile(filepath.Join(root, ".yan.toml"), []byte(project), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func ptr(s string) *string { return &s }

func TestLanguagePrecedence(t *testing.T) {
	for _, c := range []struct {
		name, user string
		saved      *string
		want       i18n.Preference
		warn       string
	}{
		{"nothing", "", nil, i18n.System, ""},
		{"config", "[ui]\nlanguage = \"en\"\n", nil, "en", ""},
		{"saved wins", "[ui]\nlanguage = \"en\"\n", ptr("zh-Hans"), "zh-Hans", ""},
		{"saved system is a choice", "[ui]\nlanguage = \"en\"\n", ptr("system"), i18n.System, ""},
		{"invalid saved falls back", "[ui]\nlanguage = \"en\"\n", ptr("fr"), "en", "state.json"},
		{"invalid config ignored", "[ui]\nlanguage = \"zh-TW\"\n", nil, i18n.System, "ui.language"},
		{"invalid both", "[ui]\nlanguage = \"zh\"\n", ptr("zh-CN"), i18n.System, "state.json"},
	} {
		t.Run(c.name, func(t *testing.T) {
			languageFiles(t, c.user, "")
			cfg, warns := Load("")
			st := &State{Language: c.saved}
			p, stWarns := st.LanguagePreference(cfg)
			warns = append(warns, stWarns...)
			if p != c.want {
				t.Errorf("preference %q, want %q", p, c.want)
			}
			if got := strings.Join(warns, "\n"); c.warn == "" && got != "" || !strings.Contains(got, c.warn) {
				t.Errorf("warnings %q, want one about %q", got, c.warn)
			}
		})
	}
}

// A project's .yan.toml never sets the interface language, and declaring it costs none of the
// project's other settings.
func TestProjectCannotSetLanguage(t *testing.T) {
	root := languageFiles(t, "[ui]\nlanguage = \"zh-Hans\"\n", "[ui]\nlanguage = \"en\"\ntheme = \"matrix\"\n[editor]\ntab_width = 2\n")
	cfg, warns := Load(root)
	if cfg.Language != "zh-Hans" {
		t.Errorf("language %q: the project's entry applied", cfg.Language)
	}
	if cfg.TabWidth != 2 || cfg.Theme != "matrix" {
		t.Errorf("the project's other settings were dropped: tab width %d, theme %q", cfg.TabWidth, cfg.Theme)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], ".yan.toml") || !strings.Contains(warns[0], "ui.language") {
		t.Errorf("warnings %q, want one diagnostic about .yan.toml's ui.language", warns)
	}
	if p, _ := (&State{}).LanguagePreference(cfg); p != i18n.SimplifiedChinese.Preference() {
		t.Errorf("preference %q", p)
	}
}

func TestLanguageSavedForTheNextProcess(t *testing.T) {
	languageFiles(t, "[ui]\nlanguage = \"zh-Hans\"\n", "")
	path := filepath.Join(t.TempDir(), "state.json")
	st := LoadState(path)
	if err := st.SetLanguage(i18n.System); err != nil {
		t.Fatal(err)
	}
	cfg, _ := Load("")
	next := LoadState(path) // what the next launch reads
	if p, _ := next.LanguagePreference(cfg); p != i18n.System {
		t.Fatalf("next launch prefers %q, want an explicit system over config.toml's zh-Hans", p)
	}
	if err := next.SetLanguage("en"); err != nil {
		t.Fatal(err)
	}
	if p, _ := LoadState(path).LanguagePreference(cfg); p != "en" {
		t.Fatalf("next launch prefers %q", p)
	}
	// Other state survives a language change, and a damaged file reads as empty, not fatal.
	next.Theme = ptr("claude")
	next.Save()
	if s := LoadState(path); s.Theme == nil || *s.Theme != "claude" || s.Language == nil || *s.Language != "en" {
		t.Fatalf("state after save: %+v", s)
	}
	os.WriteFile(path, []byte("{\"language\": 42"), 0o644)
	if p, _ := LoadState(path).LanguagePreference(cfg); p != "zh-Hans" {
		t.Fatalf("damaged state prefers %q", p)
	}
}

func TestStartLocale(t *testing.T) {
	defer i18n.SetLocale(i18n.SimplifiedChinese)
	defer i18n.SetSystemLanguages(nil)
	languageFiles(t, "", "")
	for _, c := range []struct {
		saved  *string
		system []string
		want   i18n.Locale
	}{
		{nil, []string{"zh-Hans-CN", "en-US"}, i18n.SimplifiedChinese},
		{nil, []string{"en-US", "zh-Hans-CN"}, i18n.English},
		{ptr("en"), []string{"zh-Hans-CN"}, i18n.English},
		{ptr("zh-Hans"), []string{"en-US"}, i18n.SimplifiedChinese},
		{ptr("system"), []string{"zh-Hant-TW", "en-US"}, i18n.English},
		{ptr("bogus"), []string{"zh-CN"}, i18n.SimplifiedChinese},
	} {
		warns := StartLocale(&State{Language: c.saved}, c.system)
		if got := i18n.Current(); got != c.want {
			t.Errorf("saved %v, system %q: locale %q, want %q", c.saved, c.system, got, c.want)
		}
		if c.saved != nil && *c.saved == "bogus" && (len(warns) != 1 || !strings.Contains(warns[0], "state.json")) {
			t.Errorf("warnings %q", warns)
		}
	}
	// The warning is in the language chosen.
	warns := StartLocale(&State{Language: ptr("bogus")}, []string{"en"})
	if len(warns) != 1 || !strings.HasPrefix(warns[0], "state.json: unknown interface language") {
		t.Errorf("English warning %q", warns)
	}
}

func TestLanguageSaveFailureKeepsPreviousChoice(t *testing.T) {
	for _, previous := range []*string{nil, ptr("system"), ptr("zh-Hans")} {
		path := filepath.Join(t.TempDir(), "state.json")
		st := LoadState(path)
		st.Language = previous
		// A directory at the destination makes atomic replacement fail even as root.
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := st.SetLanguage("en"); err == nil {
			t.Fatal("expected a failed settings write")
		}
		if st.Language != previous {
			t.Fatal("failed save changed the choice shared by open windows")
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := st.SetLanguage("en"); err != nil {
			t.Fatal(err)
		}
		if got := LoadState(path).Language; got == nil || *got != "en" {
			t.Fatal("retry did not persist the selected language")
		}
	}
}
