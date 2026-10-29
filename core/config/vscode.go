package config

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/corn12138/yan-source/core/keymap"
)

// Importing settings from VS Code and editors built on it. Their settings.json maps onto Yan's
// config.toml key by key; what Yan has no equivalent for is listed, never guessed. The user's
// own config.toml keeps its comments and other keys: only the imported keys are written.

// VSCodeSource is an installed editor whose user settings can be imported.
type VSCodeSource struct {
	Name string // as the editor calls itself, "Visual Studio Code"
	Dir  string // its User folder, holding settings.json and keybindings.json
}

// vscodeEditors are the editors whose settings share VS Code's format, by product folder.
var vscodeEditors = []struct{ folder, name string }{
	{"Code", "Visual Studio Code"}, {"Cursor", "Cursor"}, {"Code - Insiders", "VS Code Insiders"},
	{"VSCodium", "VSCodium"}, {"Windsurf", "Windsurf"}, {"Trae", "Trae"}, {"Kiro", "Kiro"},
}

// VSCodeSources lists the editors found for platform (runtime.GOOS names): those whose User
// folder holds a settings.json or keybindings.json.
func VSCodeSources(platform string) []VSCodeSource {
	home, _ := os.UserHomeDir()
	var base string
	switch platform {
	case "darwin":
		base = filepath.Join(home, "Library", "Application Support")
	case "windows":
		base = os.Getenv("APPDATA")
		if base == "" {
			base = filepath.Join(home, "AppData", "Roaming")
		}
	default:
		base = os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			base = filepath.Join(home, ".config")
		}
	}
	var out []VSCodeSource
	for _, ed := range vscodeEditors {
		dir := filepath.Join(base, ed.folder, "User")
		for _, f := range []string{"settings.json", "keybindings.json"} {
			if fi, err := os.Stat(filepath.Join(dir, f)); err == nil && fi.Mode().IsRegular() {
				out = append(out, VSCodeSource{Name: ed.name, Dir: dir})
				break
			}
		}
	}
	return out
}

// Setting is one key of config.toml: its table, its name and its value written as TOML.
type Setting struct {
	Table, Key, Value string
	From              string // the settings.json key it came from
}

// VSCodeSettings maps a settings.json onto config.toml. It returns the settings Yan applies
// and the keys it has no equivalent for (only those an editor user would expect to carry
// over: the editor, files, workbench and terminal ones).
func VSCodeSettings(data []byte) (sets []Setting, skipped []string, err error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(keymap.StripJSONC(data), &raw); err != nil {
		return nil, nil, fmt.Errorf("settings.json：%v", err)
	}
	str := func(k string) (string, bool) {
		var s string
		return s, raw[k] != nil && json.Unmarshal(raw[k], &s) == nil
	}
	num := func(k string) (float64, bool) {
		var f float64
		return f, raw[k] != nil && json.Unmarshal(raw[k], &f) == nil
	}
	boolean := func(k string) (bool, bool) {
		var b bool
		return b, raw[k] != nil && json.Unmarshal(raw[k], &b) == nil
	}
	add := func(table, key, value, from string) {
		sets = append(sets, Setting{Table: table, Key: key, Value: value, From: from})
	}
	if s, ok := str("editor.fontFamily"); ok && strings.TrimSpace(s) != "" {
		add("ui", "font_family", strconv.Quote(fontList(s)), "editor.fontFamily")
	}
	size, hasSize := num("editor.fontSize")
	if hasSize && size >= 9 && size <= 28 {
		add("ui", "font_size", formatNum(size), "editor.fontSize")
	} else if hasSize {
		skipped = append(skipped, "editor.fontSize（砚支持 9～28）")
	}
	if lh, ok := num("editor.lineHeight"); ok && lh > 0 {
		if lh >= 8 { // pixels, as VS Code reads values from 8 up
			base := size
			if !hasSize || base <= 0 {
				base = 13
			}
			lh /= base
		}
		if lh >= 1 && lh <= 3 {
			add("ui", "line_height", formatNum(math.Round(lh*100)/100), "editor.lineHeight")
		}
	}
	if n, ok := num("editor.tabSize"); ok && n >= 1 && n <= 16 {
		add("editor", "tab_width", formatNum(n), "editor.tabSize")
	}
	if s, ok := str("editor.lineNumbers"); ok {
		add("editor", "line_numbers", strconv.FormatBool(s != "off"), "editor.lineNumbers")
	}
	if s, ok := str("editor.wordWrap"); ok {
		add("editor", "word_wrap", strconv.FormatBool(s != "off"), "editor.wordWrap")
	}
	if s, ok := str("files.autoSave"); ok {
		add("editor", "auto_save", strconv.FormatBool(s != "off"), "files.autoSave")
	}
	if b, ok := boolean("window.autoDetectColorScheme"); ok && b {
		add("ui", "appearance", strconv.Quote("system"), "window.autoDetectColorScheme")
	} else if s, ok := str("workbench.colorTheme"); ok {
		appearance := "dark"
		if strings.Contains(strings.ToLower(s), "light") {
			appearance = "light"
		}
		add("ui", "appearance", strconv.Quote(appearance), "workbench.colorTheme")
	}
	if s, ok := str("workbench.sideBar.location"); ok {
		add("explorer", "right", strconv.FormatBool(s == "right"), "workbench.sideBar.location")
	}
	if s, ok := str("workbench.panel.defaultLocation"); ok {
		switch s {
		case "left", "right", "bottom":
			add("terminal", "position", strconv.Quote(s), "workbench.panel.defaultLocation")
		}
	}
	if b, ok := boolean("terminal.integrated.copyOnSelection"); ok {
		add("terminal", "copy_on_select", strconv.FormatBool(b), "terminal.integrated.copyOnSelection")
	}
	if b, ok := boolean("git.blame.editorDecoration.enabled"); ok {
		add("git", "blame", strconv.FormatBool(b), "git.blame.editorDecoration.enabled")
	}
	mapped := map[string]bool{}
	for _, s := range sets {
		mapped[s.From] = true
	}
	for k := range raw {
		if mapped[k] || k == "window.autoDetectColorScheme" || k == "editor.fontSize" {
			continue
		}
		for _, p := range []string{"editor.", "files.", "workbench.", "terminal.", "window."} {
			if strings.HasPrefix(k, p) {
				skipped = append(skipped, k)
				break
			}
		}
	}
	slices.Sort(skipped)
	return sets, skipped, nil
}

// fontList turns VS Code's CSS font list into Yan's comma-separated list, without quotes.
func fontList(s string) string {
	var out []string
	for _, f := range strings.Split(s, ",") {
		if f = strings.Trim(strings.TrimSpace(f), `"'`); f != "" {
			out = append(out, f)
		}
	}
	return strings.Join(out, ", ")
}

func formatNum(f float64) string {
	if f == math.Trunc(f) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

var tomlTable = regexp.MustCompile(`^\s*\[\s*([A-Za-z0-9_.-]+)\s*\]\s*(#.*)?$`)

// SetTOML writes key = value into table of a TOML document, replacing the key where the table
// has it, else adding it at the table's end, else adding the table. Comments, other keys and
// their order stay as they are. It handles the flat tables config.toml uses.
func SetTOML(src, table, key, value string) string {
	lines := strings.Split(src, "\n")
	keyLine := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(key) + `\s*=`)
	in, end := false, -1
	for i, l := range lines {
		if m := tomlTable.FindStringSubmatch(l); m != nil {
			if in {
				break
			}
			if m[1] == table {
				in, end = true, i+1
			}
			continue
		}
		if !in {
			continue
		}
		if keyLine.MatchString(l) {
			lines[i] = key + " = " + value
			return strings.Join(lines, "\n")
		}
		if strings.TrimSpace(l) != "" {
			end = i + 1
		}
	}
	if end >= 0 {
		lines = slices.Insert(lines, end, key+" = "+value)
		return strings.Join(lines, "\n")
	}
	out := strings.TrimRight(src, "\n")
	if out != "" {
		out += "\n\n"
	}
	return out + "[" + table + "]\n" + key + " = " + value + "\n"
}

// MergeKeybindings adds the bindings of an imported keybindings.json after those of Yan's own,
// as text, so both files keep their comments; later bindings win, so the imported ones do.
// It reports how many it added; an empty or unchanged import adds nothing.
func MergeKeybindings(own, imported []byte, from string) ([]byte, int, error) {
	bs, _, err := keymap.ParseVSCode(imported)
	if err != nil {
		return own, 0, fmt.Errorf("keybindings.json：%v", err)
	}
	if len(bs) == 0 {
		return own, 0, nil
	}
	start, end := strings.IndexByte(string(imported), '['), strings.LastIndexByte(string(imported), ']')
	if start < 0 || end <= start {
		return own, 0, nil
	}
	body := strings.TrimSpace(string(imported[start+1 : end]))
	if strings.Contains(string(own), body) {
		return own, 0, nil // imported before
	}
	if _, _, err := keymap.ParseVSCode(own); err != nil {
		return own, 0, fmt.Errorf("砚的 keybindings.json 读不了，先修好它：%v", err)
	}
	block := "\t// 从 " + from + " 导入\n\t" + body + "\n"
	at := strings.LastIndexByte(string(own), ']')
	if strings.TrimSpace(string(keymap.StripJSONC(own))) == "" || at < 0 {
		return []byte("[\n" + block + "]\n"), len(bs), nil
	}
	head := strings.TrimRight(string(own[:at]), " \t\n")
	// Without its closing bracket, the head keeps a trailing comma, which tells whether one is needed.
	sep := ""
	if last := strings.TrimSpace(string(keymap.StripJSONC([]byte(head)))); !strings.HasSuffix(last, "[") && !strings.HasSuffix(last, ",") {
		sep = ","
	}
	return []byte(head + sep + "\n" + block + string(own[at:])), len(bs), nil
}
