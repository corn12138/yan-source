package gui

import (
	"fmt"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/keymap"
)

// input 是工作台的输入：按键、打出的字、输入法组字、菜单的编辑命令。指针事件不拿，
// 让标签、文件树的行这些元素自己处理点击。
func (f *Frontend) input(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputKeyDown:
		return f.keyDown(ev)
	case ui.InputText:
		if f.swallowText {
			f.swallowText = false
			// 只丢掉这个键自己打出的那一个字；随后整段到来的文字（听写、粘贴）照收。
			if utf8.RuneCountInString(ev.Text) == 1 || ev.Text == "\r\n" {
				return true
			}
		}
		in := app.TextInput{Text: ev.Text}
		if f.preedit != "" { // 组完字提交：一步撤销，落进开始组字时的那个标签
			in.IME, in.Target = true, f.composeTarget
		}
		f.preedit = ""
		if ev.Text == "" {
			return true
		}
		f.ed.Handle(in)
		return true
	case ui.InputCompose:
		if f.preedit == "" && ev.Text != "" {
			f.composeTarget = f.ed.InputTarget()
		}
		f.preedit, f.preeditAt = ev.Text, ev.Caret
		return true
	case ui.InputCommand:
		id, ok := editCommands[ev.Text]
		if !ok {
			return false
		}
		f.ed.Run(id)
		return true
	}
	return false
}

// editCommands 把 Edit 菜单的系统命令对到内核命令。
var editCommands = map[string]string{
	"copy": "editor.action.clipboardCopyAction", "cut": "editor.action.clipboardCutAction",
	"paste": "editor.action.clipboardPasteAction", "selectAll": "editor.action.selectAll",
	"undo": "undo", "redo": "redo", "delete": "deleteRight",
}

// keyDown 把按键交给内核。打字键（没有 Cmd、Ctrl）照常打字，由随后的 InputText 送来文字；
// Option+字母先问内核有没有绑定，没有就让它打出特殊字符；连按中途（Ctrl+K 之后）的键都给内核。
func (f *Frontend) keyDown(ev ui.InputEvent) bool {
	f.swallowText = false
	if f.preedit != "" {
		return false // 正在组字：键归输入法
	}
	ch, typing, ok := chord(ev)
	if !ok {
		return false
	}
	switch {
	case f.ed.Pending():
		f.ed.Handle(app.KeyInput{Chord: ch})
	case typing && ch.Mods&^keymap.ModShift == 0:
		return false
	case typing && ch.Mods&(keymap.ModCtrl|keymap.ModSuper) == 0:
		if !f.ed.TryKey(ch) {
			return false
		}
	default:
		f.ed.Handle(app.KeyInput{Chord: ch})
	}
	// 这个键随后还会送来它打出的字（Cmd、Ctrl 组合不会）：那段字已经当快捷键用掉了，丢掉。
	f.swallowText = typing && ch.Mods&(keymap.ModCtrl|keymap.ModSuper) == 0 || ch.Key == keymap.KeyEnter || ch.Key == keymap.KeyTab
	return true
}

// chord 把 MyGo 的按键换成 keymap.Chord；typing 报告这个键本来会打出字。
func chord(ev ui.InputEvent) (keymap.Chord, bool, bool) {
	m := mods(ev.Mods)
	k := ev.Key
	if nk, ok := namedKeys[k]; ok {
		return keymap.Chord{Mods: m, Key: nk}, false, true
	}
	switch {
	case k == ui.KeySpace:
		return keymap.Chord{Mods: m, Key: keymap.KeyRune, Rune: ' '}, true, true
	case k >= ui.KeyA && k <= ui.KeyZ:
		return keymap.Chord{Mods: m, Key: keymap.KeyRune, Rune: 'a' + rune(k-ui.KeyA)}, true, true
	case k >= ui.Key0 && k <= ui.Key9:
		return keymap.Chord{Mods: m, Key: keymap.KeyRune, Rune: '0' + rune(k-ui.Key0)}, true, true
	}
	if r, ok := punct[k]; ok {
		return keymap.Chord{Mods: m, Key: keymap.KeyRune, Rune: r}, true, true
	}
	return keymap.Chord{}, false, false
}

var namedKeys = map[ui.Key]keymap.Key{
	ui.KeyEnter: keymap.KeyEnter, ui.KeyEscape: keymap.KeyEsc, ui.KeyTab: keymap.KeyTab,
	ui.KeyBackspace: keymap.KeyBackspace, ui.KeyDelete: keymap.KeyDelete,
	ui.KeyUp: keymap.KeyUp, ui.KeyDown: keymap.KeyDown, ui.KeyLeft: keymap.KeyLeft, ui.KeyRight: keymap.KeyRight,
	ui.KeyHome: keymap.KeyHome, ui.KeyEnd: keymap.KeyEnd, ui.KeyPageUp: keymap.KeyPgUp, ui.KeyPageDown: keymap.KeyPgDn,
	ui.KeyF1: keymap.KeyF1, ui.KeyF2: keymap.KeyF2, ui.KeyF3: keymap.KeyF3, ui.KeyF4: keymap.KeyF4,
	ui.KeyF5: keymap.KeyF5, ui.KeyF6: keymap.KeyF6, ui.KeyF7: keymap.KeyF7, ui.KeyF8: keymap.KeyF8,
	ui.KeyF9: keymap.KeyF9, ui.KeyF10: keymap.KeyF10, ui.KeyF11: keymap.KeyF11, ui.KeyF12: keymap.KeyF12,
}

// punct 是标点键在美式键盘上（不按 Shift）打出的字：快捷键按位置认，与 MyGo 一致。
var punct = map[ui.Key]rune{
	ui.KeyMinus: '-', ui.KeyEqual: '=', ui.KeyComma: ',', ui.KeyPeriod: '.', ui.KeySlash: '/',
	ui.KeySemicolon: ';', ui.KeyQuote: '\'', ui.KeyBracketLeft: '[', ui.KeyBracketRight: ']',
	ui.KeyBackslash: '\\', ui.KeyBackquote: '`',
}

// uiKey 把 keymap.Chord 换回 MyGo 的修饰键和按键（菜单的快捷键用）。
func uiKey(c keymap.Chord) (string, bool) {
	var s string
	if c.Mods&keymap.ModCtrl != 0 {
		s += "Ctrl+"
	}
	if c.Mods&keymap.ModAlt != 0 {
		s += "Alt+"
	}
	if c.Mods&keymap.ModShift != 0 {
		s += "Shift+"
	}
	if c.Mods&keymap.ModSuper != 0 {
		s += "Cmd+"
	}
	switch c.Key {
	case keymap.KeyRune:
		if c.Rune == ' ' {
			return s + "Space", true
		}
		if !utf8.ValidRune(c.Rune) {
			return "", false
		}
		r := c.Rune
		if r >= 'a' && r <= 'z' {
			r -= 'a' - 'A'
		}
		return s + string(r), true
	case keymap.KeyEnter:
		return s + "Enter", true
	case keymap.KeyTab:
		return s + "Tab", true
	case keymap.KeyBackspace:
		return s + "Backspace", true
	case keymap.KeyDelete:
		return s + "Delete", true
	case keymap.KeyUp:
		return s + "Up", true
	case keymap.KeyDown:
		return s + "Down", true
	case keymap.KeyLeft:
		return s + "Left", true
	case keymap.KeyRight:
		return s + "Right", true
	case keymap.KeyEsc:
		return s + "Escape", true
	}
	if c.Key >= keymap.KeyF1 && c.Key <= keymap.KeyF12 {
		return fmt.Sprintf("%sF%d", s, c.Key-keymap.KeyF1+1), true
	}
	return "", false
}

// modKeys 是系统的主修饰键加上 k 的写法：macOS 上是 ⌘↩，Windows、Linux 上是 Ctrl+Enter。
func (f *Frontend) modKeys(k keymap.Key) string {
	return f.chordHint(keymap.Chord{Mods: f.Profile().Mods[0], Key: k})
}

// chordHint writes a chord as the platform does: ⇧↩ on macOS, Shift+Enter elsewhere.
func (f *Frontend) chordHint(c keymap.Chord) string {
	if f.Profile().MacKeys() {
		return keymap.Symbols([]keymap.Chord{c})
	}
	return c.String()
}
