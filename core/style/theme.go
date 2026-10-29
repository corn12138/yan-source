package style

import (
	"math"
	"strconv"
)

// 主题的配色移植自 Mayoi（MIT，Copyright (c) 2026 chencheng，cmd/mayoi/theme.go）：
// Mayoi 自己的浅色和深色、The Matrix、Claude 的浅色和深色。终端前端和 GUI 前端用同一套颜色。

// Color 是 0xRRGGBBAA。
type Color uint32

// Hex 解析 "#rrggbb"，不透明。
func Hex(s string) Color {
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil || len(s) != 7 {
		panic("style.Hex: " + s)
	}
	return Color(v<<8 | 0xff)
}

// RGBA 由 0–255 的分量和 0–1 的不透明度构成颜色。
func RGBA(r, g, b uint8, a float64) Color {
	return Color(uint32(r)<<24 | uint32(g)<<16 | uint32(b)<<8 | uint32(math.Round(a*255)))
}

// Parts 返回 0–255 的四个分量。
func (c Color) Parts() (r, g, b, a uint8) {
	return uint8(c >> 24), uint8(c >> 16), uint8(c >> 8), uint8(c)
}

// Over 返回 c 叠在不透明的 bg 上的颜色（终端画不出半透明）。
func (c Color) Over(bg Color) Color {
	r, g, b, a := c.Parts()
	br, bg2, bb, _ := bg.Parts()
	k := float64(a) / 255
	mix := func(x, y uint8) uint8 { return uint8(math.Round(float64(x)*k + float64(y)*(1-k))) }
	return RGBA(mix(r, br), mix(g, bg2), mix(b, bb), 1)
}

// Mix 返回 c 与 o 按 t（0–1）混合的颜色。
func (c Color) Mix(o Color, t float64) Color {
	r, g, b, a := c.Parts()
	or, og, ob, oa := o.Parts()
	mix := func(x, y uint8) uint8 { return uint8(math.Round(float64(x)*(1-t) + float64(y)*t)) }
	return Color(uint32(mix(r, or))<<24 | uint32(mix(g, og))<<16 | uint32(mix(b, ob))<<8 | uint32(mix(a, oa)))
}

// Alpha 返回不透明度乘以 k 的颜色。
func (c Color) Alpha(k float64) Color {
	r, g, b, a := c.Parts()
	return RGBA(r, g, b, float64(a)/255*k)
}

// Palette 是一个主题：外壳、代码和终端的颜色。
type Palette struct {
	Name string
	Dark bool
	// Mono 让整个窗口都用代码字体、文件图标用主题自己的颜色（The Matrix）。
	Mono bool

	Editor, Chrome, Panel, Border, Hover, Active Color
	Text, Muted, Faint, Accent, AccentDim        Color
	// Fill 是按钮底色（上面是 FillText）；FillHover、FillPressed 是悬停和按下。
	Fill, FillHover, FillPressed, FillText Color
	Surface, SurfaceHover, SurfacePressed  Color
	Scrollbar, Inverse, InverseText        Color
	Line, Selection                        Color // 光标所在行、选区
	Gutter, GutterNow                      Color // 行号、光标所在行的行号
	Status, StatusFg                       Color
	Modified, Danger                       Color
	Match, MatchNow, MatchRing             Color // 查找的匹配、当前匹配及其边框
	Guide, GuideNow, Bracket, Word         Color
	Shadow                                 Color
	Added, Changed, Removed                Color // git 改动标记

	Syntax [Inserted - Keyword + 1]Color // 按 Keyword…Inserted 编号
	ANSI   [16]Color
}

// Themes 是配置里 theme 可选的值。
var Themes = []string{"", "matrix", "claude"}

// ThemeFor 返回主题（""、"matrix"、"claude"）在浅色或深色外观下的配色。The Matrix 总是深色。
func ThemeFor(name string, dark bool) *Palette {
	switch {
	case name == "matrix":
		return matrix
	case name == "claude" && dark:
		return claudeDark
	case name == "claude":
		return claudeLight
	case dark:
		return mayoiDark
	}
	return mayoiLight
}

var (
	black, white = Hex("#000000"), Hex("#ffffff")

	mayoiDark   = darkPalette()
	mayoiLight  = lightPalette()
	matrix      = matrixPalette()
	claudeDark  = claudeDarkPalette()
	claudeLight = claudeLightPalette()
)

// syn 依次填 Keyword、Type、Function、String、Number、Comment、Operator、Punctuation、
// Constant、Builtin、Tag、Attribute、Variable、Heading、Deleted、Inserted。
func (p *Palette) syn(hex ...string) {
	for i, h := range hex {
		p.Syntax[i] = Hex(h)
	}
}

func ansi(hex ...string) (out [16]Color) {
	for i, h := range hex {
		out[i] = Hex(h)
	}
	return out
}

// darkPalette 和 lightPalette 是默认的 Mayoi：安静编辑器设计的暖色纸面与陶土色强调（2026-10 定稿的
// 浅色、深色语义色值）；只有 Git 状态、增删和错误用有含义的颜色。
func darkPalette() *Palette {
	p := &Palette{
		Name: "", Dark: true,
		Editor: Hex("#1f1e1b"), Chrome: Hex("#262521"), Panel: Hex("#262521"), Border: Hex("#3b3933"),
		Hover: Hex("#302e29"), Active: Hex("#3a3731"),
		Text: Hex("#e7e3d9"), Muted: Hex("#a6a196"), Faint: Hex("#7b766c"),
		Accent: Hex("#d58e72"), AccentDim: RGBA(213, 142, 114, 0.55),
		Line: RGBA(231, 227, 217, 0.04), Selection: RGBA(213, 142, 114, 0.24),
		Gutter: Hex("#7f7a70"), GutterNow: Hex("#e7e3d9"),
		Status: Hex("#262521"), StatusFg: Hex("#a6a196"),
		Modified: Hex("#e0b060"), Danger: Hex("#f08a80"),
		Match: RGBA(224, 176, 96, 0.24), MatchNow: RGBA(224, 176, 96, 0.42), MatchRing: Hex("#e0b060"),
		Guide: RGBA(231, 227, 217, 0.07), GuideNow: RGBA(231, 227, 217, 0.22),
		Bracket: RGBA(231, 227, 217, 0.26), Word: RGBA(231, 227, 217, 0.09),
		Shadow: RGBA(0, 0, 0, 0.5),
		Added:  Hex("#7dbb8b"), Changed: Hex("#8fb8dd"), Removed: Hex("#e68a7e"),
		// 强调色在深色里偏亮：按钮上用深色字（对比度 6.5:1）。
		Fill: Hex("#d58e72"), FillText: Hex("#1f1e1b"),
		Surface: Hex("#1f1e1b"), SurfaceHover: Hex("#302e29"), SurfacePressed: Hex("#3a3731"),
		Scrollbar: RGBA(231, 227, 217, 0.18), Inverse: Hex("#e7e3d9"), InverseText: Hex("#1f1e1b"),
		ANSI: ansi("#3b3933", "#e68a7e", "#7dbb8b", "#e0b060", "#8fb8dd", "#d59bc0", "#8cc7c0", "#e7e3d9",
			"#706d65", "#f2a59a", "#9ccfa7", "#ecc684", "#abcbe8", "#e2b5d2", "#a8d8d1", "#f7f5f0"),
	}
	p.FillHover, p.FillPressed = p.Fill.Mix(white, 0.1), p.Fill.Mix(black, 0.15)
	p.syn("#d59bc0", "#8cc7c0", "#8fb8dd", "#93c4a6", "#e2a66f", "#8f8a7e", "#b9b4a8", "#a6a196",
		"#e2a66f", "#e0b060", "#e68a7e", "#e0b060", "#e7e3d9", "#d58e72", "#e68a7e", "#7dbb8b")
	return p
}

func lightPalette() *Palette {
	p := &Palette{
		Name:   "",
		Editor: Hex("#f7f5f0"), Chrome: Hex("#f0ede6"), Panel: Hex("#f0ede6"), Border: Hex("#ddd8ce"),
		Hover: Hex("#e8e4db"), Active: Hex("#e3ddd1"),
		Text: Hex("#292824"), Muted: Hex("#6a675f"), Faint: Hex("#938e83"),
		Accent: Hex("#a45f46"), AccentDim: RGBA(164, 95, 70, 0.5),
		Line: RGBA(41, 40, 36, 0.035), Selection: RGBA(164, 95, 70, 0.16),
		Gutter: Hex("#8c887f"), GutterNow: Hex("#292824"),
		Status: Hex("#f0ede6"), StatusFg: Hex("#6a675f"),
		Modified: Hex("#8f5e0e"), Danger: Hex("#b0302a"),
		Match: RGBA(222, 184, 60, 0.30), MatchNow: RGBA(222, 184, 60, 0.52), MatchRing: Hex("#8f5e0e"),
		Guide: RGBA(41, 40, 36, 0.08), GuideNow: RGBA(41, 40, 36, 0.24),
		Bracket: RGBA(41, 40, 36, 0.22), Word: RGBA(140, 115, 60, 0.12),
		Shadow: RGBA(41, 40, 36, 0.14),
		Added:  Hex("#3d7a4c"), Changed: Hex("#2f5f86"), Removed: Hex("#a8443a"),
		Fill: Hex("#a45f46"), FillText: Hex("#ffffff"),
		Surface: Hex("#f7f5f0"), SurfaceHover: Hex("#e8e4db"), SurfacePressed: Hex("#e3ddd1"),
		Scrollbar: RGBA(41, 40, 36, 0.20), Inverse: Hex("#292824"), InverseText: Hex("#f7f5f0"),
		ANSI: ansi("#292824", "#a8443a", "#3d7a4c", "#8f5e0e", "#2f5f86", "#8a4b6e", "#2e6b73", "#9c978c",
			"#706d65", "#b0302a", "#4a8a5a", "#a86e14", "#3b6e99", "#9c5a80", "#3a7e87", "#ddd8ce"),
	}
	p.FillHover, p.FillPressed = p.Fill.Mix(black, 0.1), p.Fill.Mix(black, 0.22)
	p.syn("#8a4b6e", "#2e6b73", "#2f5f86", "#3d6e58", "#9a5a1f", "#857f74", "#5f5b53", "#6e6a62",
		"#9a5a1f", "#8f5e0e", "#a8443a", "#8f5e0e", "#292824", "#a45f46", "#a8443a", "#3d7a4c")
	return p
}

// matrixPalette 是 The Matrix：黑底磷光绿，代码靠绿色的亮度区分；删除和出错仍是红色，
// 修改和查找命中是琥珀色，所以 diff 和错误照样看得清。
func matrixPalette() *Palette {
	p := &Palette{
		Name: "matrix", Dark: true, Mono: true,
		Editor: Hex("#050a06"), Chrome: Hex("#020503"), Panel: Hex("#040806"), Border: Hex("#0f2a16"),
		Hover: RGBA(0, 255, 65, 0.06), Active: RGBA(0, 255, 65, 0.14),
		Text: Hex("#4af07a"), Muted: Hex("#2a9d4c"), Faint: Hex("#1a5c2e"),
		Accent: Hex("#00ff41"), AccentDim: RGBA(0, 255, 65, 0.5),
		Line: RGBA(0, 255, 65, 0.04), Selection: RGBA(0, 255, 65, 0.22),
		Gutter: Hex("#1f6b36"), GutterNow: Hex("#6dff94"),
		Status: Hex("#020503"), StatusFg: Hex("#2a9d4c"),
		Modified: Hex("#d9b44a"), Danger: Hex("#ff4a5a"),
		Match: RGBA(217, 180, 74, 0.22), MatchNow: RGBA(217, 180, 74, 0.40), MatchRing: RGBA(217, 180, 74, 0.85),
		Guide: RGBA(0, 255, 65, 0.07), GuideNow: RGBA(0, 255, 65, 0.25),
		Bracket: RGBA(0, 255, 65, 0.35), Word: RGBA(0, 255, 65, 0.09),
		Shadow: RGBA(0, 0, 0, 0.6),
		Added:  Hex("#00d43a"), Changed: Hex("#3fd0b0"), Removed: Hex("#e0505e"),
		// 绿底黑字，像终端的反色。
		Fill: Hex("#00d43a"), FillText: Hex("#000000"),
		Surface: Hex("#08140b"), SurfaceHover: Hex("#0c1d10"), SurfacePressed: Hex("#102615"),
		Scrollbar: RGBA(0, 255, 65, 0.2), Inverse: Hex("#0f2a16"), InverseText: Hex("#8dffad"),
		ANSI: ansi("#0a1a0e", "#e0505e", "#00ff41", "#d9b44a", "#2fbf8f", "#8fd94a", "#00ff9c", "#4af07a",
			"#1f7a3a", "#ff6b78", "#6dff94", "#f0d070", "#5fe8c4", "#b0f070", "#7dffcf", "#c8ffd6"),
	}
	p.FillHover, p.FillPressed = p.Fill.Mix(white, 0.15), p.Fill.Mix(black, 0.15)
	p.syn("#b6ffc8", "#00ff9c", "#66ff8a", "#8fd94a", "#c8ff5a", "#1f7a3a", "#3fbf63", "#2e8f4a",
		"#c8ff5a", "#7dffcf", "#00ff9c", "#8fd94a", "#5af58a", "#b6ffc8", "#e0505e", "#00d43a")
	return p
}

// claudeDarkPalette 是深色的 Claude：Anthropic 的暖黑与云白，强调色是 Claude 的橙。
func claudeDarkPalette() *Palette {
	p := &Palette{
		Name: "claude", Dark: true,
		Editor: Hex("#1f1e1d"), Chrome: Hex("#191918"), Panel: Hex("#1c1b1a"), Border: Hex("#2e2c2a"),
		Hover: RGBA(255, 245, 230, 0.045), Active: RGBA(217, 119, 87, 0.16),
		Text: Hex("#e8e6dc"), Muted: Hex("#a39e93"), Faint: Hex("#75716a"),
		Accent: Hex("#d97757"), AccentDim: RGBA(217, 119, 87, 0.55),
		Line: RGBA(255, 245, 230, 0.035), Selection: RGBA(217, 119, 87, 0.28),
		Gutter: Hex("#6b675f"), GutterNow: Hex("#c4bfb3"),
		Status: Hex("#191918"), StatusFg: Hex("#a39e93"),
		Modified: Hex("#e3b25f"), Danger: Hex("#e8705f"),
		Match: RGBA(227, 178, 95, 0.20), MatchNow: RGBA(227, 178, 95, 0.38), MatchRing: RGBA(227, 178, 95, 0.85),
		Guide: RGBA(255, 245, 230, 0.065), GuideNow: RGBA(255, 245, 230, 0.22),
		Bracket: RGBA(255, 245, 230, 0.30), Word: RGBA(255, 245, 230, 0.075),
		Shadow: RGBA(0, 0, 0, 0.35),
		Added:  Hex("#7ba05b"), Changed: Hex("#6a9bcc"), Removed: Hex("#d9634f"),
		// 白字压在 Claude 橙上对比度只有 3:1，按钮用更深的橙（4.6:1）。
		Fill: Hex("#b8573a"), FillText: Hex("#ffffff"),
		Surface: Hex("#262523"), SurfaceHover: Hex("#2d2b29"), SurfacePressed: Hex("#34322f"),
		Scrollbar: RGBA(255, 245, 230, 0.16), Inverse: Hex("#302e2b"), InverseText: Hex("#f0eee6"),
		ANSI: ansi("#2e2c2a", "#e8705f", "#9cbf7a", "#e3b25f", "#6a9bcc", "#c68fb5", "#7fbfb3", "#e8e6dc",
			"#6b675f", "#f08a78", "#b3d496", "#f0c87e", "#8fb6dd", "#d9a8cb", "#9fd6cb", "#faf9f5"),
	}
	p.FillHover, p.FillPressed = p.Fill.Mix(white, 0.1), p.Fill.Mix(black, 0.15)
	p.syn("#d97757", "#7fbfb3", "#8fb6dd", "#b3c98a", "#d4a27f", "#7d786f", "#b0aa9e", "#938d82",
		"#d4a27f", "#e3b25f", "#e8705f", "#e3b25f", "#ebe9e0", "#d97757", "#e8705f", "#9cbf7a")
	return p
}

// claudeLightPalette 是浅色的 Claude：象牙白与近黑，强调色用更深的橙，压在浅底上看得清。
func claudeLightPalette() *Palette {
	p := &Palette{
		Name:   "claude",
		Editor: Hex("#faf9f5"), Chrome: Hex("#f0eee6"), Panel: Hex("#f5f4ee"), Border: Hex("#e3dfd3"),
		Hover: RGBA(20, 20, 19, 0.045), Active: RGBA(201, 100, 66, 0.13),
		Text: Hex("#141413"), Muted: Hex("#6b675f"), Faint: Hex("#938d82"),
		Accent: Hex("#c96442"), AccentDim: RGBA(201, 100, 66, 0.5),
		Line: RGBA(20, 20, 19, 0.03), Selection: RGBA(217, 119, 87, 0.22),
		Gutter: Hex("#a39e93"), GutterNow: Hex("#3d3a35"),
		Status: Hex("#f0eee6"), StatusFg: Hex("#6b675f"),
		Modified: Hex("#b5791f"), Danger: Hex("#bf4d3a"),
		Match: RGBA(212, 162, 64, 0.24), MatchNow: RGBA(212, 162, 64, 0.44), MatchRing: RGBA(181, 121, 31, 0.9),
		Guide: RGBA(20, 20, 19, 0.07), GuideNow: RGBA(20, 20, 19, 0.24),
		Bracket: RGBA(20, 20, 19, 0.28), Word: RGBA(20, 20, 19, 0.065),
		Shadow: RGBA(40, 30, 20, 0.12),
		Added:  Hex("#5e8a3a"), Changed: Hex("#4a7fb5"), Removed: Hex("#c4513c"),
		Fill: Hex("#b8573a"), FillText: Hex("#ffffff"),
		Surface: Hex("#ffffff"), SurfaceHover: Hex("#f5f3ec"), SurfacePressed: Hex("#ece9df"),
		Scrollbar: RGBA(20, 20, 19, 0.18), Inverse: Hex("#141413"), InverseText: Hex("#faf9f5"),
		ANSI: ansi("#141413", "#bf4d3a", "#5e8a3a", "#a3680f", "#3b6ea8", "#9a4f86", "#2f8378", "#b0aa9e",
			"#6b675f", "#d9634f", "#76a34f", "#c4862a", "#5a8cc4", "#b56aa1", "#45a093", "#e3dfd3"),
	}
	p.FillHover, p.FillPressed = p.Fill.Mix(black, 0.1), p.Fill.Mix(black, 0.22)
	p.syn("#b8573a", "#2f8378", "#3b6ea8", "#5e8a3a", "#a85a1f", "#938d82", "#5c5850", "#75716a",
		"#a85a1f", "#a3680f", "#bf4d3a", "#a3680f", "#141413", "#b8573a", "#bf4d3a", "#5e8a3a")
	return p
}

// Fg 返回样式的前景色（可能半透明）。
func (p *Palette) Fg(id ID) Color {
	switch {
	case id >= Keyword && id <= Inserted:
		return p.Syntax[id-Keyword]
	}
	switch id {
	case Gutter:
		return p.Gutter
	case GutterCurrent:
		return p.GutterNow
	case Tab, Dim, Border, DiffHeader:
		return p.Muted
	case Status:
		return p.StatusFg
	case StatusAccent:
		return p.Text
	case Match:
		return p.Accent
	case Composing:
		return p.Text
	}
	return p.Text
}

// Bg 返回样式的背景色（可能半透明；零值表示透明，沿用下面的底色）。
func (p *Palette) Bg(id ID) Color {
	switch id {
	case Text, Gutter, GutterCurrent:
		return p.Editor
	case CurrentLine:
		return p.Line
	case Selection:
		return p.Selection
	case Tab:
		return p.Chrome
	case TabActive:
		return p.Editor
	case Status, StatusAccent:
		return p.Status
	case Overlay:
		return p.Surface
	case OverlaySelected, TreeFocused:
		return p.Active
	case TreeCursor:
		return p.Hover
	case SearchHit:
		return p.Match
	case SearchCurrent:
		return p.MatchNow
	case Bracket:
		return p.Bracket
	case Word:
		return p.Word
	case DiffAdded:
		return p.Added.Alpha(0.14)
	case DiffRemoved:
		return p.Removed.Alpha(0.14)
	case DiffAddedWord:
		return p.Added.Alpha(0.30)
	case DiffRemovedWord:
		return p.Removed.Alpha(0.30)
	case DiffHeader:
		return p.Chrome
	case MergeConflict:
		return p.ANSI[3].Alpha(0.18)
	}
	return 0
}

// Lane 是提交图第 i 种颜色：强调色，然后是终端的黄、绿、红、品红、青（照着 Mayoi 的 graphColor）。
func (p *Palette) Lane(i int) Color {
	cols := [...]Color{p.Accent, p.ANSI[3], p.ANSI[2], p.ANSI[1], p.ANSI[5], p.ANSI[6]}
	return cols[max(i, 0)%len(cols)]
}
