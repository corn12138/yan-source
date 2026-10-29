package keymap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// VSBinding 是 keybindings.json（VS Code 的格式）里的一条：按键序列，命令和它的参数，什么时候生效。
// 命令前面有 "-" 时是把这个键从那个命令上拿掉；Key 为空时拿掉那个命令所有的键。
type VSBinding struct {
	Key     string // 与 Parse 的写法相同，如 "cmd+k cmd+s"
	Command string
	Args    json.RawMessage
	When    *When // nil 表示总是生效
}

// Removal 报告这一条是不是拿掉键。
func (b VSBinding) Removal() bool { return strings.HasPrefix(b.Command, "-") }

type vsEntry struct {
	Key     string          `json:"key"`
	Command string          `json:"command"`
	Args    json.RawMessage `json:"args"`
	When    string          `json:"when"`
}

// ParseVSCode 读 keybindings.json：允许注释和最后一项后面的逗号，同 VS Code。
// 整个文件读不了时返回错误；写错的那几条跳过，在 warns 里说明。
func ParseVSCode(data []byte) (bs []VSBinding, warns []string, err error) {
	data = StripJSONC(data)
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil, nil
	}
	var es []vsEntry
	if err := json.Unmarshal(data, &es); err != nil {
		return nil, nil, err
	}
	for _, e := range es {
		if e.Command == "" || e.Command == "-" {
			continue
		}
		b := VSBinding{Key: strings.ToLower(strings.TrimSpace(e.Key)), Command: e.Command, Args: e.Args}
		if b.Key == "" && !b.Removal() {
			warns = append(warns, fmt.Sprintf("%s 没写 key", e.Command))
			continue
		}
		if b.Key != "" {
			if _, err := Parse(b.Key, ModCtrl); err != nil {
				warns = append(warns, fmt.Sprintf("%q：%v", e.Key, err))
				continue
			}
		}
		if strings.TrimSpace(e.When) != "" {
			w, err := ParseWhen(e.When)
			if err != nil {
				warns = append(warns, fmt.Sprintf("when %q：%v", e.When, err))
				continue
			}
			b.When = w
		}
		bs = append(bs, b)
	}
	return bs, warns, nil
}

// Names 返回条件里用到的上下文名，按出现的顺序，不重复。
func (w *When) Names() []string {
	var out []string
	var walk func(*When)
	walk = func(w *When) {
		if w == nil {
			return
		}
		if w.name != "" && !slices.Contains(out, w.name) {
			out = append(out, w.name)
		}
		for _, c := range w.children {
			walk(c)
		}
	}
	walk(w)
	return out
}
