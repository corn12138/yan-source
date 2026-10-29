// Package keymap 负责组合键规范化与按上下文的序列 trie（支持 "ctrl+k v" 这样的连按）。
package keymap

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/corn12138/yan-source/core/i18n"
)

// Mod 是修饰键位掩码。
type Mod uint8

const (
	ModShift Mod = 1 << iota
	ModCtrl
	ModAlt
	ModSuper // Cmd
)

// Key 是按键种类。可打印字符统一用 KeyRune + Chord.Rune。
type Key uint16

const (
	KeyRune Key = iota
	KeyEnter
	KeyEsc
	KeyTab
	KeyBackspace
	KeyDelete
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyHome
	KeyEnd
	KeyPgUp
	KeyPgDn
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
)

var keyNames = map[string]Key{
	"enter": KeyEnter, "esc": KeyEsc, "escape": KeyEsc, "tab": KeyTab, "backspace": KeyBackspace,
	"delete": KeyDelete, "up": KeyUp, "down": KeyDown, "left": KeyLeft, "right": KeyRight,
	"home": KeyHome, "end": KeyEnd, "pageup": KeyPgUp, "pagedown": KeyPgDn,
	"f1": KeyF1, "f2": KeyF2, "f3": KeyF3, "f4": KeyF4, "f5": KeyF5, "f6": KeyF6,
	"f7": KeyF7, "f8": KeyF8, "f9": KeyF9, "f10": KeyF10, "f11": KeyF11, "f12": KeyF12,
	"space": KeyRune, // 特殊处理成 Rune ' '
}

// Chord 是一次按键（含修饰键）；可比较，可直接做 map 键。Rune 在 Key == KeyRune 时有效，统一小写。
type Chord struct {
	Mods Mod
	Key  Key
	Rune rune
}

// Context 是按键生效的焦点上下文。
type Context uint8

const (
	CtxGlobal Context = iota
	CtxEditor
	CtxTree    // 资源管理器
	CtxOverlay // 快速打开、命令面板
)

// ContextNames 是配置文件里 [keys.<名字>] 用的名字。
var ContextNames = map[string]Context{"global": CtxGlobal, "editor": CtxEditor, "explorer": CtxTree, "overlay": CtxOverlay}

// Match 是一次查找的结果。
type Match uint8

const (
	MatchNone   Match = iota
	MatchPrefix       // 是某个连按序列的前缀，等下一个键
	MatchExact
)

// Profile 描述当前终端：mod 解析成哪个修饰键。
type Profile struct {
	// Mods 是 "mod" 别名展开成的修饰键集合。Terminal.app 下只有 Ctrl；
	// 支持 kitty 键盘协议的终端里 Cmd 和 Ctrl 都能用；GUI 里只有 Cmd。
	Mods []Mod
	// GUI 为真时，快捷键提示用系统的写法（macOS 是 ⌘⇧P），也列出含 Cmd 的绑定。
	GUI bool
	// OS 是窗口版运行的系统，照 runtime.GOOS 的叫法；"" 当作 macOS（终端版不看它）。
	// 窗口版在 Windows、Linux 上用 VS Code 在那里的键位：mod 是 Ctrl，提示写成 Ctrl+Shift+P。
	OS string
}

// Platform is the system that when conditions such as isMac and isWindows name: OS for the
// window version, where "" means macOS, and the running system for the terminal version.
func (p Profile) Platform() string {
	switch {
	case p.OS != "":
		return p.OS
	case p.GUI:
		return "darwin"
	}
	return runtime.GOOS
}

// MacKeys 报告快捷键是不是 macOS 的（以 Cmd 为主），而不是 Windows、Linux 的（以 Ctrl 为主）。
func (p Profile) MacKeys() bool { return p.OS == "" || p.OS == "darwin" }

// Parse 把 "mod+k v"、"ctrl+shift+z"、"f1" 解析成按键序列；mod 展开成给定修饰键。
func Parse(seq string, mod Mod) ([]Chord, error) {
	var out []Chord
	for _, part := range strings.Fields(strings.ToLower(seq)) {
		var c Chord
		toks := strings.Split(part, "+")
		for _, t := range toks[:len(toks)-1] {
			switch t {
			case "shift":
				c.Mods |= ModShift
			case "ctrl":
				c.Mods |= ModCtrl
			case "alt", "opt", "option":
				c.Mods |= ModAlt
			case "cmd", "super", "meta", "win": // meta、win 是 VS Code 在 Linux、Windows 上的叫法
				c.Mods |= ModSuper
			case "mod":
				c.Mods |= mod
			default:
				return nil, fmt.Errorf(i18n.T("Unknown modifier %q (in %q)"), t, seq)
			}
		}
		last := toks[len(toks)-1]
		if k, ok := keyNames[last]; ok {
			c.Key = k
			if last == "space" {
				c.Rune = ' '
			}
		} else if r := []rune(last); len(r) == 1 {
			c.Key, c.Rune = KeyRune, r[0]
		} else {
			return nil, fmt.Errorf(i18n.T("Unknown key %q (in %q)"), last, seq)
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, errors.New(i18n.T("Empty key sequence"))
	}
	return out, nil
}

// String 返回人类可读的形式，例如 "Ctrl+K"。状态栏提示连按时用。
func (c Chord) String() string {
	var b strings.Builder
	for _, m := range []struct {
		m Mod
		s string
	}{{ModCtrl, "Ctrl+"}, {ModAlt, "Opt+"}, {ModShift, "Shift+"}, {ModSuper, "Cmd+"}} {
		if c.Mods&m.m != 0 {
			b.WriteString(m.s)
		}
	}
	if c.Key == KeyRune {
		b.WriteString(strings.ToUpper(string(c.Rune)))
	} else {
		for name, k := range keyNames {
			if k == c.Key && name != "escape" && name != "space" {
				b.WriteString(strings.ToUpper(name[:1]) + name[1:])
				break
			}
		}
	}
	return b.String()
}

// Symbols 用 macOS 菜单的写法显示按键序列，例如 "⇧⌘P"、"⌃K ⌃P"。
func Symbols(seq []Chord) string {
	var b strings.Builder
	for i, c := range seq {
		if i > 0 {
			b.WriteByte(' ')
		}
		for _, m := range []struct {
			m Mod
			s string
		}{{ModCtrl, "⌃"}, {ModAlt, "⌥"}, {ModShift, "⇧"}, {ModSuper, "⌘"}} {
			if c.Mods&m.m != 0 {
				b.WriteString(m.s)
			}
		}
		b.WriteString(keySymbol(c))
	}
	return b.String()
}

var keySymbols = map[Key]string{
	KeyEnter: "↩", KeyEsc: "⎋", KeyTab: "⇥", KeyBackspace: "⌫", KeyDelete: "⌦",
	KeyUp: "↑", KeyDown: "↓", KeyLeft: "←", KeyRight: "→", KeyHome: "↖", KeyEnd: "↘", KeyPgUp: "⇞", KeyPgDn: "⇟",
}

func keySymbol(c Chord) string {
	if c.Key == KeyRune {
		if c.Rune == ' ' {
			return "Space"
		}
		return strings.ToUpper(string(c.Rune))
	}
	if s, ok := keySymbols[c.Key]; ok {
		return s
	}
	if c.Key >= KeyF1 && c.Key <= KeyF12 {
		return fmt.Sprintf("F%d", c.Key-KeyF1+1)
	}
	return c.String()
}

type node struct {
	next map[Chord]*node
	cmd  string
}

// Keymap 每个上下文一棵 trie。
type Keymap struct {
	roots  map[Context]*node
	single map[string][]Chord // 命令 → 单键绑定（GUI 菜单的快捷键用）
}

// New 返回空 keymap。
func New() *Keymap { return &Keymap{roots: map[Context]*node{}, single: map[string][]Chord{}} }

// First 返回命令最适合显示在菜单上的单键绑定：优先含 Cmd 的，其次修饰键少的。
func (k *Keymap) First(cmd string) (Chord, bool) {
	cs := k.single[cmd]
	if len(cs) == 0 {
		return Chord{}, false
	}
	score := func(c Chord) int {
		s := 0
		if c.Mods&ModSuper == 0 {
			s += 100
		}
		for m := c.Mods; m != 0; m &= m - 1 {
			s += 10
		}
		if c.Key >= KeyF1 && c.Key <= KeyF12 {
			s += 5
		}
		return s
	}
	best := cs[0]
	for _, c := range cs[1:] {
		if sc, sb := score(c), score(best); sc < sb || sc == sb && c.String() < best.String() {
			best = c
		}
	}
	return best, true
}

// Compile 按 Profile 编译一组绑定（上下文 → 按键序列 → 命令 ID）；返回告警而不是失败，
// 一条写错的绑定不应该让编辑器起不来。
func Compile(bindings map[Context]map[string]string, p Profile) (*Keymap, []string) {
	k := New()
	var warns []string
	for ctx, m := range bindings {
		for seq, cmd := range m {
			mods := p.Mods
			if !strings.Contains(seq, "mod+") {
				mods = mods[:min(1, len(mods))] // 不含 mod 的绑定只编译一次
			}
			for _, mod := range mods {
				if err := k.Bind(ctx, seq, cmd, mod); err != nil {
					warns = append(warns, err.Error())
				}
			}
		}
	}
	return k, warns
}

// Bind 绑定一条序列；拒绝「既是完整绑定又是别的绑定的前缀」这种二义情况。
func (k *Keymap) Bind(ctx Context, seq, cmd string, mod Mod) error {
	chords, err := Parse(seq, mod)
	if err != nil {
		return err
	}
	n := k.roots[ctx]
	if n == nil {
		n = &node{}
		k.roots[ctx] = n
	}
	for i, c := range chords {
		if n.cmd != "" {
			return fmt.Errorf(i18n.T("The prefix of %q is already bound to %s"), seq, n.cmd)
		}
		if n.next == nil {
			n.next = map[Chord]*node{}
		}
		child := n.next[c]
		if child == nil {
			child = &node{}
			n.next[c] = child
		}
		n = child
		if i == len(chords)-1 && len(n.next) > 0 {
			return fmt.Errorf(i18n.T("%q is a prefix of other chords and can't be bound on its own"), seq)
		}
	}
	n.cmd = cmd
	if len(chords) == 1 && cmd != "" {
		k.single[cmd] = append(k.single[cmd], chords[0])
	}
	return nil
}

// Resolve 先查 ctx，再查全局上下文。
func (k *Keymap) Resolve(ctx Context, seq []Chord) (string, Match) {
	if cmd, m := k.resolveIn(ctx, seq); m != MatchNone {
		return cmd, m
	}
	if ctx != CtxGlobal {
		return k.resolveIn(CtxGlobal, seq)
	}
	return "", MatchNone
}

func (k *Keymap) resolveIn(ctx Context, seq []Chord) (string, Match) {
	n := k.roots[ctx]
	for _, c := range seq {
		if n == nil {
			return "", MatchNone
		}
		n = n.next[c]
	}
	switch {
	case n == nil:
		return "", MatchNone
	case n.cmd != "":
		return n.cmd, MatchExact
	case len(n.next) > 0:
		return "", MatchPrefix
	}
	return "", MatchNone
}
