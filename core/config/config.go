// Package config 加载用户设置：内置默认值 → ~/.config/yan/config.toml → 项目里的 .yan.toml，
// 后者覆盖前者。写错的键不会让编辑器起不来，只会在状态栏给出警告。
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"

	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/keymap"
)

// Config 是全部设置。
type Config struct {
	TabWidth     int
	LineNumbers  bool
	SidebarWidth int
	SidebarRight bool
	// 外观（[ui]）：主题 ""（Mayoi）、"matrix"、"claude"；外观 ""（跟随系统）、"light"、"dark"。
	// 终端里没有「系统外观」，空的外观按深色画。字体只对窗口版有效。
	Theme, Appearance string
	ThemeEffects      bool
	// PreviewMermaid 把 Markdown 预览里的 mermaid 代码块画成图（[ui] preview_mermaid，默认开）；
	// .mmd、.mermaid 文件的预览不看它，总是画图。
	PreviewMermaid bool
	PreviewMath    bool
	FontFamily     string
	FontSize       float64
	// Language is the interface language the user configuration names ([ui] language):
	// "system", "zh-Hans", "en", or "" when it names none. Only the user's file may set it: the
	// interface language is the application's, never a project's.
	Language   string
	LineHeight float64
	Blame      bool // 光标停下时在行尾显示这一行最后是谁改的（[git] blame）
	// DiffWhitespace is which whitespace the diffs leave out of the comparison ([diff] whitespace):
	// "" every character counts, "eol" line-end spaces and tabs do not, "all" no whitespace does,
	// "blank" also lines made only of whitespace. The text shown and staged stays the file's own.
	DiffWhitespace string
	// DiffCrossFiles is what a diff's change navigation does at the last (or first) change of a
	// file ([diff] cross_files): "" says so and crosses to the next file on one more press,
	// "always" crosses at once, "never" stays in the file.
	DiffCrossFiles string
	// DiffHighlight is how a changed line shows what changed within it ([diff] highlight): ""
	// by word, "char" by character, "line" not at all. DiffContext is how many unchanged lines
	// stay around each change ([diff] context, 0 to 10; the rest fold). DiffLayout is the diffs'
	// layout ([diff] layout): "" side by side when wide enough, "split" always, "inline" never.
	DiffHighlight string
	DiffContext   int
	DiffLayout    string
	// DiffSyncScroll keeps a side-by-side diff's two sides scrolling together ([diff]
	// sync_scroll); DiffAlignedRows lines their rows up, a line one side lacks holding a space on
	// the other ([diff] aligned_rows). Both apply to the window, whose split diff has two panes.
	DiffSyncScroll, DiffAlignedRows bool
	// AutoSave 是自动保存（[editor] auto_save）：停止编辑 1 秒、切换标签、窗口失去焦点时保存。
	// nil 表示没写，由前端决定（窗口版照着 Mayoi 默认打开，终端版默认关闭）。
	AutoSave *bool
	Vim      bool // 用 vim 的键编辑（[editor] vim）
	WordWrap bool // 长行折到窗格宽度（[editor] word_wrap）
	// TerminalCopyOnSelect 是窗口版的终端里选中就复制，同 Ghostty、iTerm2（[terminal] copy_on_select，默认开）。
	TerminalCopyOnSelect bool
	// PanelPosition is where the terminal panel sits: "" below the editors, "left" or "right" at
	// the window's edge ([terminal] position).
	PanelPosition string
	// TerminalLocation only routes new terminals; existing sessions keep their place.
	TerminalLocation string
	// Keys 是额外的按键绑定（上下文名 → 按键序列 → 命令 ID），覆盖内置 VS Code 预设；命令 ID 为 "" 表示解绑。
	Keys map[string]map[string]string
	// Keybindings 是 KeybindingsPath 里的文字（VS Code 的 keybindings.json 格式），在 Keys 之后生效；没有这个文件时为 nil。
	Keybindings []byte
}

// Default 返回内置默认配置：零配置即可用。
func Default() *Config {
	return &Config{ThemeEffects: true, PreviewMermaid: true, PreviewMath: true, TerminalCopyOnSelect: true, TabWidth: 4, DiffContext: 3, DiffSyncScroll: true, DiffAlignedRows: true, LineNumbers: true, SidebarWidth: 30, FontFamily: "monospace", FontSize: 14, LineHeight: 1.6, Blame: true,
		Keys: map[string]map[string]string{}}
}

// file 是配置文件的结构。用指针区分「没写」和「写了零值」，这样才能逐层覆盖。
type file struct {
	Editor struct {
		TabWidth    *int  `toml:"tab_width"`
		LineNumbers *bool `toml:"line_numbers"`
		AutoSave    *bool `toml:"auto_save"`
		Vim         *bool `toml:"vim"`
		WordWrap    *bool `toml:"word_wrap"`
	} `toml:"editor"`
	Explorer struct {
		Width *int  `toml:"width"`
		Right *bool `toml:"right"`
	} `toml:"explorer"`
	Git struct {
		Blame *bool `toml:"blame"`
	} `toml:"git"`
	Diff struct {
		Whitespace *string `toml:"whitespace"`
		CrossFiles *string `toml:"cross_files"`
		Highlight  *string `toml:"highlight"`
		Context    *int    `toml:"context"`
		Layout     *string `toml:"layout"`
		SyncScroll *bool   `toml:"sync_scroll"`
		Aligned    *bool   `toml:"aligned_rows"`
	} `toml:"diff"`
	UI struct {
		Theme        *string  `toml:"theme"`
		Language     *string  `toml:"language"`
		ThemeEffects *bool    `toml:"theme_effects"`
		Mermaid      *bool    `toml:"preview_mermaid"`
		Math         *bool    `toml:"preview_math"`
		Appearance   *string  `toml:"appearance"`
		FontFamily   *string  `toml:"font_family"`
		FontSize     *float64 `toml:"font_size"`
		LineHeight   *float64 `toml:"line_height"`
	} `toml:"ui"`
	Terminal struct {
		CopyOnSelect *bool   `toml:"copy_on_select"`
		Position     *string `toml:"position"`
		Location     *string `toml:"location"`
	} `toml:"terminal"`
	Keys map[string]map[string]string `toml:"keys"`
}

func terminalLocation(v string) (string, bool) {
	switch v {
	case "", "panel":
		return "", true
	case "editor":
		return v, true
	}
	return "", false
}

// DiffWhitespaceMode reads a [diff] whitespace value: "exact" (or "") is every character.
func DiffWhitespaceMode(v string) (string, bool) {
	switch v {
	case "", "exact":
		return "", true
	case "eol", "all", "blank":
		return v, true
	}
	return "", false
}

// MaxDiffContext is the most unchanged lines [diff] context keeps around a change.
const MaxDiffContext = 10

// DiffHighlightMode reads a [diff] highlight value: "word" (or "") is by word.
func DiffHighlightMode(v string) (string, bool) {
	switch v {
	case "", "word":
		return "", true
	case "char", "line":
		return v, true
	}
	return "", false
}

// DiffLayoutMode reads a [diff] layout value: "auto" (or "") is side by side when wide enough.
func DiffLayoutMode(v string) (string, bool) {
	switch v {
	case "", "auto":
		return "", true
	case "split", "inline":
		return v, true
	}
	return "", false
}

// UserPath 返回用户配置文件路径（尊重 XDG_CONFIG_HOME）。
func UserPath() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "yan", "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "yan", "config.toml")
}

// KeybindingsPath 返回 VS Code 格式的按键文件，在用户配置文件旁边。
func KeybindingsPath() string { return filepath.Join(filepath.Dir(UserPath()), "keybindings.json") }

// Load 依次叠加用户配置和项目配置（projectRoot 为空时跳过项目配置），再读 keybindings.json。
func Load(projectRoot string) (*Config, []string) {
	c := Default()
	var warns []string
	paths := []string{UserPath()}
	if projectRoot != "" {
		paths = append(paths, filepath.Join(projectRoot, ".yan.toml"))
	}
	for _, p := range paths {
		warns = append(warns, c.merge(p, p != UserPath())...)
	}
	kp := KeybindingsPath()
	if b, err := os.ReadFile(kp); err == nil {
		c.Keybindings = b
		lastGood.Lock()
		if _, _, err := keymap.ParseVSCode(b); err == nil {
			lastGood.files[kp] = b
		} else if good, ok := lastGood.files[kp]; ok {
			// A malformed save keeps the keys every window was using, and says so.
			c.Keybindings = good
			warns = append(warns, i18n.F("keybindings.json: %v (keeping the last keys that could be read)", err))
		}
		lastGood.Unlock()
	} else if errors.Is(err, fs.ErrNotExist) {
		lastGood.Lock()
		delete(lastGood.files, kp)
		lastGood.Unlock()
	} else {
		warns = append(warns, i18n.F("%s: %v", "keybindings.json", err))
	}
	return c, warns
}

// lastGood keeps each configuration file's last content that parsed, process-wide, so that a
// malformed save in any window keeps the settings every window was using.
var lastGood = struct {
	sync.Mutex
	files map[string][]byte
}{files: map[string][]byte{}}

func (c *Config) merge(path string, project bool) []string {
	var f file
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		lastGood.Lock()
		delete(lastGood.files, path)
		lastGood.Unlock()
		return nil
	}
	if err != nil {
		return []string{i18n.F("%s: %v", filepath.Base(path), err)}
	}
	md, err := toml.Decode(string(data), &f)
	var warns []string
	lastGood.Lock()
	if err == nil {
		lastGood.files[path] = data
	} else if good, ok := lastGood.files[path]; ok {
		f = file{}
		md, _ = toml.Decode(string(good), &f)
		warns = append(warns, i18n.F("%s: %v (keeping the last content that could be read)", filepath.Base(path), err))
		err = nil
	}
	lastGood.Unlock()
	if err != nil {
		return []string{i18n.F("%s: %v", filepath.Base(path), err)}
	}
	for _, k := range md.Undecoded() {
		warns = append(warns, i18n.F("%s: unknown setting %s", filepath.Base(path), k))
	}
	if v := f.Editor.TabWidth; v != nil && *v > 0 && *v <= 16 {
		c.TabWidth = *v
	}
	if v := f.Editor.LineNumbers; v != nil {
		c.LineNumbers = *v
	}
	if v := f.Editor.AutoSave; v != nil {
		c.AutoSave = v
	}
	if v := f.Editor.Vim; v != nil {
		c.Vim = *v
	}
	if v := f.Terminal.CopyOnSelect; v != nil {
		c.TerminalCopyOnSelect = *v
	}
	if v := f.Terminal.Location; v != nil {
		if location, ok := terminalLocation(*v); ok {
			c.TerminalLocation = location
		} else {
			warns = append(warns, i18n.F("%s: terminal.location must be panel or editor", filepath.Base(path)))
		}
	}
	if v := f.Terminal.Position; v != nil {
		switch *v {
		case "", "bottom", "left", "right":
			c.PanelPosition = strings.TrimPrefix(*v, "bottom")
		default:
			warns = append(warns, i18n.F("%s: terminal.position must be bottom, left or right", filepath.Base(path)))
		}
	}
	if v := f.Editor.WordWrap; v != nil {
		c.WordWrap = *v
	}
	if v := f.Explorer.Width; v != nil && *v >= 12 {
		c.SidebarWidth = *v
	}
	if v := f.Explorer.Right; v != nil {
		c.SidebarRight = *v
	}
	if v := f.UI.Language; v != nil {
		switch _, ok := i18n.ParsePreference(*v); {
		case project:
			warns = append(warns, i18n.F("%s: ui.language is ignored here: the interface language is set in the user configuration or in Settings", filepath.Base(path)))
		case ok:
			c.Language = *v
		default:
			warns = append(warns, i18n.F("%s: ui.language must be system, zh-Hans or en", filepath.Base(path)))
		}
	}
	if v := f.UI.ThemeEffects; v != nil {
		c.ThemeEffects = *v
	}
	if v := f.UI.Math; v != nil {
		c.PreviewMath = *v
	}
	if v := f.UI.Mermaid; v != nil {
		c.PreviewMermaid = *v
	}
	if v := f.UI.Theme; v != nil {
		switch *v {
		case "", "mayoi", "matrix", "claude":
			c.Theme = strings.TrimPrefix(*v, "mayoi")
		default:
			warns = append(warns, i18n.F("%s: unknown theme %q (available: mayoi, matrix, claude)", filepath.Base(path), *v))
		}
	}
	if v := f.UI.Appearance; v != nil {
		switch *v {
		case "", "system", "light", "dark":
			c.Appearance = strings.TrimPrefix(*v, "system")
		default:
			warns = append(warns, i18n.F("%s: unknown appearance %q (available: system, light, dark)", filepath.Base(path), *v))
		}
	}
	if v := f.Git.Blame; v != nil {
		c.Blame = *v
	}
	if v := f.Diff.Whitespace; v != nil {
		if w, ok := DiffWhitespaceMode(*v); ok {
			c.DiffWhitespace = w
		} else {
			warns = append(warns, i18n.F("%s: diff.whitespace must be exact, eol, all or blank", filepath.Base(path)))
		}
	}
	if v := f.Diff.CrossFiles; v != nil {
		switch *v {
		case "", "ask", "always", "never":
			c.DiffCrossFiles = strings.TrimPrefix(*v, "ask")
		default:
			warns = append(warns, i18n.F("%s: diff.cross_files must be ask, always or never", filepath.Base(path)))
		}
	}
	if v := f.Diff.Highlight; v != nil {
		if h, ok := DiffHighlightMode(*v); ok {
			c.DiffHighlight = h
		} else {
			warns = append(warns, i18n.F("%s: diff.highlight must be word, char or line", filepath.Base(path)))
		}
	}
	if v := f.Diff.Context; v != nil {
		if *v >= 0 && *v <= MaxDiffContext {
			c.DiffContext = *v
		} else {
			warns = append(warns, i18n.F("%s: diff.context must be 0 to %d", filepath.Base(path), MaxDiffContext))
		}
	}
	if v := f.Diff.SyncScroll; v != nil {
		c.DiffSyncScroll = *v
	}
	if v := f.Diff.Aligned; v != nil {
		c.DiffAlignedRows = *v
	}
	if v := f.Diff.Layout; v != nil {
		if l, ok := DiffLayoutMode(*v); ok {
			c.DiffLayout = l
		} else {
			warns = append(warns, i18n.F("%s: diff.layout must be auto, split or inline", filepath.Base(path)))
		}
	}
	if v := f.UI.FontFamily; v != nil && *v != "" {
		c.FontFamily = *v
	}
	if v := f.UI.FontSize; v != nil && *v >= 9 && *v <= 28 {
		c.FontSize = *v
	}
	if v := f.UI.LineHeight; v != nil && *v >= 1 && *v <= 3 {
		c.LineHeight = *v
	}
	for ctx, m := range f.Keys {
		if c.Keys[ctx] == nil {
			c.Keys[ctx] = map[string]string{}
		}
		for seq, cmd := range m {
			c.Keys[ctx][seq] = cmd
		}
	}
	return warns
}
