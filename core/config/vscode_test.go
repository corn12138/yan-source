package config

import (
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/keymap"
)

func TestVSCodeSettingsMapping(t *testing.T) {
	sets, skipped, err := VSCodeSettings([]byte(`{
		// comments and trailing commas, as VS Code writes them
		"editor.fontFamily": "'JetBrains Mono', Menlo, monospace",
		"editor.fontSize": 15,
		"editor.lineHeight": 24,
		"editor.tabSize": 2,
		"editor.wordWrap": "on",
		"editor.lineNumbers": "off",
		"files.autoSave": "afterDelay",
		"workbench.colorTheme": "GitHub Light Default",
		"workbench.sideBar.location": "right",
		"editor.fontLigatures": true,
		"git.autofetch": true,
	}`))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, s := range sets {
		got[s.Table+"."+s.Key] = s.Value
	}
	want := map[string]string{
		"ui.font_family": `"JetBrains Mono, Menlo, monospace"`, "ui.font_size": "15", "ui.line_height": "1.6",
		"editor.tab_width": "2", "editor.word_wrap": "true", "editor.line_numbers": "false", "editor.auto_save": "true",
		"ui.appearance": `"light"`, "explorer.right": "true",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if strings.Join(skipped, ",") != "editor.fontLigatures" {
		t.Errorf("skipped %v (git.* is not an editor setting)", skipped)
	}
	if _, _, err := VSCodeSettings([]byte(`{"editor.fontSize": `)); err == nil {
		t.Error("a malformed settings.json must be reported")
	}
}

func TestSetTOMLKeepsTheRest(t *testing.T) {
	src := "# my settings\n[editor]\ntab_width = 8 # wide\nvim = true\n\n[ui]\ntheme = \"claude\"\n"
	out := SetTOML(src, "editor", "tab_width", "2")
	out = SetTOML(out, "ui", "font_size", "15")
	out = SetTOML(out, "terminal", "copy_on_select", "false")
	want := "# my settings\n[editor]\ntab_width = 2\nvim = true\n\n[ui]\ntheme = \"claude\"\nfont_size = 15\n\n[terminal]\ncopy_on_select = false\n"
	if out != want {
		t.Fatalf("got\n%s\nwant\n%s", out, want)
	}
	if got := SetTOML("", "ui", "font_size", "14"); got != "[ui]\nfont_size = 14\n" {
		t.Fatalf("empty file: %q", got)
	}
}

func TestMergeKeybindings(t *testing.T) {
	imported := []byte("// VS Code\n[\n\t{ \"key\": \"cmd+k cmd+s\", \"command\": \"workbench.action.files.saveAll\" },\n]\n")
	for name, own := range map[string]string{
		"missing":        "",
		"template":       "// 说明\n[\n\t// { \"key\": \"cmd+x\", \"command\": \"y\" },\n]\n",
		"no comma":       "[\n\t{ \"key\": \"cmd+1\", \"command\": \"a\" }\n]\n",
		"trailing comma": "[\n\t{ \"key\": \"cmd+1\", \"command\": \"a\" },\n]\n",
	} {
		out, n, err := MergeKeybindings([]byte(own), imported, "VS Code")
		if err != nil || n != 1 {
			t.Fatalf("%s: %d %v", name, n, err)
		}
		bs, _, err := keymap.ParseVSCode(out)
		if err != nil || bs[len(bs)-1].Command != "workbench.action.files.saveAll" {
			t.Fatalf("%s: merged file does not parse: %v\n%s", name, err, out)
		}
		if strings.Contains(own, "说明") && !strings.Contains(string(out), "说明") {
			t.Fatalf("%s: comments lost", name)
		}
		again, n, _ := MergeKeybindings(out, imported, "VS Code")
		if n != 0 || string(again) != string(out) {
			t.Fatalf("%s: a second import duplicated the bindings", name)
		}
	}
	if _, _, err := MergeKeybindings([]byte("[ {"), imported, "VS Code"); err == nil {
		t.Fatal("a broken own file must not be appended to")
	}
}
