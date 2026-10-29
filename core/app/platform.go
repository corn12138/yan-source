package app

import (
	"strconv"
	"strings"

	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/keymap"
)

// 窗口版在 Windows、Linux 上的键位与叫法，照着 Mayoi v0.7.0（MIT，cmd/mayoi/platform.go）：
// 快捷键用 VS Code 在那里的（Ctrl 代替 Cmd，撞上或者成了 AltGr 的换成 VS Code 的键），
// 文件管理器和废纸篓用那个系统的叫法。现在只有 macOS 的窗口版，这些是给以后移植留的口子，
// 由 keymap.Profile.OS 决定；终端版不受影响。

// otherKeys 是内置键位里只适合 macOS 的绑定在 Windows、Linux 上换成的键；"" 表示去掉：
// 那里有别的键做同一件事，或者 Ctrl+Alt 在那里是 AltGr，要拿来打字。
// 没列在这里的 cmd+ 换成 ctrl+，和已有的绑定撞上时去掉。
var otherKeys = map[string]string{
	"cmd+q":       "", // ctrl+q
	"cmd+o":       "ctrl+k ctrl+o",
	"cmd+alt+o":   "ctrl+k shift+o",
	"cmd+shift+]": "", "cmd+shift+[": "", // ctrl+pagedown、ctrl+pageup
	"cmd+alt+right": "", "cmd+alt+left": "", // ctrl+k right、ctrl+k left
	// macOS 的行首行尾、文件头尾；Windows、Linux 上 Ctrl+←→ 是按词移动（见 otherExtra）。
	"cmd+left": "", "cmd+right": "", "shift+cmd+left": "", "shift+cmd+right": "",
	"cmd+up": "", "cmd+down": "", "shift+cmd+up": "", "shift+cmd+down": "",
	"cmd+backspace": "",
	"cmd+g":         "", "cmd+shift+g": "", // f3、shift+f3；ctrl+shift+g 是源代码管理
	"alt+mod+f": "ctrl+h",
	"alt+mod+c": "alt+c", "alt+mod+w": "alt+w", "alt+mod+r": "alt+r",
	// 后退、前进是 Alt+←→（otherExtra），Ctrl+- 留给缩小；按词移动是 Ctrl+←→。
	"ctrl+-": "", "ctrl+shift+-": "", "alt+left": "", "alt+right": "",
}

// otherExtra 是 Windows、Linux 上另加的绑定（VS Code 在那里的）。
var otherExtra = map[keymap.Context]map[string]string{
	keymap.CtxGlobal: {"alt+left": "workbench.action.navigateBack", "alt+right": "workbench.action.navigateForward"},
	keymap.CtxEditor: {
		"ctrl+left": "cursorWordLeft", "ctrl+right": "cursorWordEndRight",
		"ctrl+shift+left": "cursorWordLeftSelect", "ctrl+shift+right": "cursorWordEndRightSelect",
		"ctrl+backspace": "deleteWordLeft",
	},
}

func init() {
	// ⌘1…9 切到第几个标签；Windows、Linux 上是 Alt+1…9，同 VS Code。
	for i := 1; i <= 9; i++ {
		otherKeys["cmd+"+strconv.Itoa(i)] = "alt+" + strconv.Itoa(i)
		otherKeys["ctrl+cmd+"+strconv.Itoa(i)] = "" // 项目：Ctrl+Alt 是 AltGr；用菜单和项目栏
	}
	otherKeys["ctrl+cmd+]"], otherKeys["ctrl+cmd+["] = "", ""
}

// platformBindings 把内置键位换成 p 所在系统的：macOS 和终端版原样返回。
func platformBindings(b map[keymap.Context]map[string]string, p keymap.Profile) map[keymap.Context]map[string]string {
	if !p.GUI || p.MacKeys() {
		return b
	}
	// 撞没撞上按解析后的按键比：mod+s 和 ctrl+s 是同一个键。
	keys := func(m map[string]string) map[string]bool {
		set := map[string]bool{}
		for seq := range m {
			if _, replaced := otherKeys[seq]; replaced || strings.Contains(seq, "cmd+") {
				continue
			}
			if cs, err := keymap.Parse(seq, keymap.ModCtrl); err == nil {
				set[chordString(cs)] = true
			}
		}
		return set
	}
	global := keys(b[keymap.CtxGlobal])
	out := map[keymap.Context]map[string]string{}
	for ctx, m := range b {
		taken := keys(m)
		n := map[string]string{}
		for seq, id := range m {
			to, listed := otherKeys[seq]
			if !listed {
				if !strings.Contains(seq, "cmd+") {
					n[seq] = id
					continue
				}
				to = strings.ReplaceAll(seq, "cmd+", "ctrl+")
				if cs, err := keymap.Parse(to, keymap.ModCtrl); err != nil || taken[chordString(cs)] || ctx != keymap.CtxGlobal && global[chordString(cs)] {
					continue
				}
			}
			if to != "" {
				n[to] = id
			}
		}
		for seq, id := range otherExtra[ctx] {
			n[seq] = id
		}
		out[ctx] = n
	}
	return out
}

// FileManager 是系统里显示文件的程序：访达、文件资源管理器，或者文件管理器（照 runtime.GOOS 的叫法）。
func FileManager(goos string) string {
	switch goos {
	case "", "darwin":
		return i18n.T("Finder")
	case "windows":
		return i18n.T("File Explorer")
	}
	return i18n.T("File Manager")
}

// TrashName 是删掉的文件去的地方：macOS 上是废纸篓，别处是回收站。
func TrashName(goos string) string {
	if goos == "" || goos == "darwin" {
		return i18n.T("Trash")
	}
	return i18n.T("Recycle Bin")
}

func (e *Editor) trashName() string { return TrashName(e.profile.OS) }

// ShellQuote 给拖到终端上的文件路径加上 shell 的引号（照着 Mayoi 的 shellQuote）：不用加就原样。
// macOS、Linux 是 POSIX shell 的单引号；Windows 默认 PowerShell 用单引号并将内部单引号加倍；cmd.exe 需单独的 shell 适配。
func ShellQuote(goos, s string) string {
	safe := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_@%+=:,./-"
	if goos == "windows" {
		safe = strings.ReplaceAll(safe, "%", "") + `\`
	}
	if s != "" && strings.Trim(s, safe) == "" {
		return s
	}
	if goos == "windows" {
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// modName writes modifiers alone the way the profile writes keys: as macOS menus do in the
// macOS window (⌘), as Ctrl, Alt, Shift elsewhere (Windows and Linux call Option Alt).
func modName(m keymap.Mod, p keymap.Profile) string {
	mac := p.GUI && p.MacKeys()
	var parts []string
	for _, x := range []struct {
		m         keymap.Mod
		mac, name string
	}{{keymap.ModCtrl, "⌃", "Ctrl"}, {keymap.ModAlt, "⌥", "Alt"}, {keymap.ModShift, "⇧", "Shift"}, {keymap.ModSuper, "⌘", "Super"}} {
		switch {
		case m&x.m == 0:
		case mac:
			parts = append(parts, x.mac)
		default:
			parts = append(parts, x.name)
		}
	}
	if mac {
		return strings.Join(parts, "")
	}
	return strings.Join(parts, "+")
}
