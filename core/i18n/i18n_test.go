package i18n

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestMatch(t *testing.T) {
	for tag, want := range map[string]Locale{
		"en": English, "en-US": English, "en-GB": English, "en_AU.UTF-8": English, "EN-us": English,
		"zh-Hans": SimplifiedChinese, "zh-Hans-CN": SimplifiedChinese, "zh-Hans-US": SimplifiedChinese,
		"zh_CN.UTF-8": SimplifiedChinese, "zh-CN": SimplifiedChinese, "zh-SG": SimplifiedChinese,
		"zh": SimplifiedChinese, "zh_Hans_SG": SimplifiedChinese,
	} {
		if got, ok := Match(tag); !ok || got != want {
			t.Errorf("Match(%q) = %q, %v; want %q", tag, got, ok, want)
		}
	}
	// Traditional Chinese, explicit scripts winning over regions, other languages, garbage.
	for _, tag := range []string{"zh-Hant", "zh-TW", "zh-HK", "zh-MO", "zh-Hant-CN", "zh-Hant-TW", "zh_TW.Big5",
		"fr-FR", "ja", "yue-Hans-CN", "", "-", "_", "@", ".UTF-8", "C", "POSIX", "zh-Latn", "\x00", "en\x00"[:0]} {
		if got, ok := Match(tag); ok {
			t.Errorf("Match(%q) = %q, want no match", tag, got)
		}
	}
}

func TestResolve(t *testing.T) {
	for _, c := range []struct {
		pref   Preference
		system []string
		want   Locale
	}{
		{System, []string{"zh-Hans-CN", "en-US"}, SimplifiedChinese},
		{"", []string{"zh-Hans-CN", "en-US"}, SimplifiedChinese}, // absent behaves as System
		{System, []string{"en-GB", "zh-Hans"}, English},
		{System, []string{"fr-FR", "zh-CN", "en"}, SimplifiedChinese},
		{System, []string{"zh-Hant-TW", "en-US"}, English},
		{System, []string{"fr-FR"}, English},
		{System, nil, English},
		{System, []string{"", "garbage!!"}, English},
		{Preference(English), []string{"zh-Hans-CN"}, English},
		{Preference(SimplifiedChinese), []string{"en-US"}, SimplifiedChinese},
	} {
		if got := Resolve(c.pref, c.system); got != c.want {
			t.Errorf("Resolve(%q, %q) = %q, want %q", c.pref, c.system, got, c.want)
		}
	}
}

func TestParsePreference(t *testing.T) {
	for _, s := range []string{"system", "zh-Hans", "en"} {
		if p, ok := ParsePreference(s); !ok || string(p) != s {
			t.Errorf("ParsePreference(%q) = %q, %v", s, p, ok)
		}
	}
	// Only canonical values name a catalog: aliases and arbitrary identifiers do not.
	for _, s := range []string{"", "System", "zh", "zh-CN", "zh-Hant", "en-US", "fr", "../en", "zh-hans"} {
		if p, ok := ParsePreference(s); ok {
			t.Errorf("ParsePreference(%q) = %q, want rejected", s, p)
		}
	}
}

func TestEnvLanguages(t *testing.T) {
	env := func(kv ...string) func(string) string {
		m := map[string]string{}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return func(k string) string { return m[k] }
	}
	for _, c := range []struct {
		env  func(string) string
		want []string
	}{
		{env("LANG", "zh_CN.UTF-8"), []string{"zh_CN.UTF-8"}},
		{env("LANG", "zh_CN.UTF-8", "LC_ALL", "en_US.UTF-8"), []string{"en_US.UTF-8"}},
		{env("LANG", "en_US.UTF-8", "LC_MESSAGES", "zh_CN.UTF-8"), []string{"zh_CN.UTF-8"}},
		{env("LANG", "en_US.UTF-8", "LANGUAGE", "fr:zh_CN"), []string{"fr", "zh_CN", "en_US.UTF-8"}},
		{env("LANGUAGE", "zh_CN"), nil}, // gettext ignores LANGUAGE without a locale
		{env("LANG", "C", "LANGUAGE", "zh_CN"), nil},
		{env("LC_ALL", "C.UTF-8"), nil},
		{env(), nil},
	} {
		if got := EnvLanguages(c.env); !slices.Equal(got, c.want) {
			t.Errorf("EnvLanguages = %q, want %q", got, c.want)
		}
	}
	if got := Resolve(System, EnvLanguages(env("LANG", "en_US.UTF-8", "LANGUAGE", "fr:zh_CN"))); got != SimplifiedChinese {
		t.Errorf("LANGUAGE preference list resolved to %q", got)
	}
}

func TestLookupAndFallback(t *testing.T) {
	if got := In(SimplifiedChinese, "Settings"); got != "设置" {
		t.Errorf("Settings in Chinese = %q", got)
	}
	if got := In(English, "Settings"); got != "Settings" {
		t.Errorf("Settings in English = %q", got)
	}
	// A message missing from the catalog shows its English text in every locale.
	if got := In(SimplifiedChinese, "A message nobody translated"); got != "A message nobody translated" {
		t.Errorf("fallback = %q", got)
	}
	if got := In(SimplifiedChinese, "sidebar\x04Changes"); got != "改动" {
		t.Errorf("context message in Chinese = %q", got)
	}
	if got := In(English, "sidebar\x04Changes"); got != "Changes" {
		t.Errorf("context message in English = %q", got)
	}
	if got := In(English, "missing context\x04Fallback"); got != "Fallback" {
		t.Errorf("context fallback = %q", got)
	}
}

func TestFormattedMessages(t *testing.T) {
	path := "/Users/林/项目/说明 草稿.md"
	for _, c := range []struct {
		l      Locale
		format string
		args   []any
		want   string
	}{
		{English, "Saved %s", []any{"说明 草稿.md"}, "Saved 说明 草稿.md"},
		{SimplifiedChinese, "Saved %s", []any{"说明 草稿.md"}, "已保存 说明 草稿.md"},
		{English, "Saved %d files", []any{3}, "Saved 3 files"},
		{SimplifiedChinese, "Saved %d files", []any{3}, "已保存 3 个文件"},
		{English, "Couldn't open the folder: %s", []any{path}, "Couldn't open the folder: " + path},
		{SimplifiedChinese, "Couldn't open the folder: %s", []any{path}, "无法打开文件夹：" + path},
		// Indexed verbs let English reorder what Chinese says first.
		{English, "Replaced %[2]d occurrences in %[1]d files", []any{2, 7}, "Replaced 7 occurrences in 2 files"},
		{SimplifiedChinese, "Replaced %[2]d occurrences in %[1]d files", []any{2, 7}, "在 2 个文件里替换了 7 处"},
		{English, "Couldn't read %[2]s in %[1]s", []any{"HEAD", "a.go"}, "Couldn't read a.go in HEAD"},
		{SimplifiedChinese, "Couldn't read %[2]s in %[1]s", []any{"HEAD", "a.go"}, "无法读取 HEAD 中的 a.go"},
		{SimplifiedChinese, "Follow System — %s", []any{"English"}, "跟随系统 — English"},
	} {
		if got := fmt.Sprintf(In(c.l, c.format), c.args...); got != c.want {
			t.Errorf("%s %q = %q, want %q", c.l, c.format, got, c.want)
		}
	}
}

var verb = regexp.MustCompile(`%(\[\d+\])?[-+# 0]*\d*(\.\d+)?[a-zA-Z%]`)

// verbs are a format's verbs in argument order, %% left out.
func verbs(s string) []string {
	var out []string
	next := 1
	byArg := map[int]string{}
	for _, m := range verb.FindAllStringSubmatch(s, -1) {
		v := m[0]
		if v == "%%" {
			continue
		}
		if m[1] != "" {
			n := 0
			for _, r := range m[1][1 : len(m[1])-1] {
				n = n*10 + int(r-'0')
			}
			next = n
			v = strings.Replace(v, m[1], "", 1)
		}
		byArg[next] = v
		next++
	}
	for i := 1; i <= len(byArg); i++ {
		out = append(out, byArg[i])
	}
	return out
}

func TestCatalogFormatsMatch(t *testing.T) {
	for key, zh := range zhHans {
		if !slices.Equal(verbs(key), verbs(zh)) {
			t.Errorf("verbs differ: %q %v vs %q %v", key, verbs(key), zh, verbs(zh))
		}
		if strings.TrimSpace(zh) == "" {
			t.Errorf("empty translation of %q", key)
		}
	}
}

func TestLocales(t *testing.T) {
	for _, l := range Locales {
		if l.NativeName() == "" {
			t.Errorf("%q has no name", l)
		}
		if p, ok := ParsePreference(string(l)); !ok || Resolve(p, nil) != l {
			t.Errorf("%q is not a preference of its own", l)
		}
	}
	if SimplifiedChinese.NativeName() != "简体中文" || English.NativeName() != "English" {
		t.Error("native names changed")
	}
}
