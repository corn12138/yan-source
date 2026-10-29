package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
)

// macGUI 是 macOS 窗口版的键位环境。
var macGUI = keymap.Profile{Mods: []keymap.Mod{keymap.ModSuper}, GUI: true, OS: "darwin"}

func keybindingsEditor(t *testing.T, json string) (*Editor, []string) {
	t.Helper()
	cfg := config.Default()
	cfg.Keybindings = []byte(json)
	return New(cfg, fakeLayouter{}, &fakeHost{}, macGUI)
}

func resolve(t *testing.T, e *Editor, ctx keymap.Context, seq string) string {
	t.Helper()
	cs, err := keymap.Parse(seq, keymap.ModSuper)
	if err != nil {
		t.Fatal(err)
	}
	id, m := e.keys.Resolve(ctx, cs)
	if m != keymap.MatchExact {
		return ""
	}
	// A guard decides at the key press; here, as in ctx with nothing else known.
	for g, ok := e.guards[id]; ok; g, ok = e.guards[id] {
		if g.when.Eval(func(n string) bool { return slices.Contains(focusKeys[ctx], n) }) {
			id = g.cmd
		} else {
			id = g.fallback
		}
	}
	return id
}

// 从 VS Code 粘过来的 keybindings.json：连按、when、拿掉键、后写的赢、runCommands；砚没有的命令、
// 砚里不会成立的 when 跳过并说明。
func TestKeybindingsJSON(t *testing.T) {
	e, warns := keybindingsEditor(t, `[
	{ "key": "cmd+k cmd+s", "command": "workbench.action.files.saveAll" },
	{ "key": "ctrl+shift+alt+x", "command": "editor.action.deleteLines", "when": "editorTextFocus && !editorReadonly" },
	{ "key": "cmd+d", "command": "-editor.action.addSelectionToNextFindMatch" },
	{ "key": "cmd+l", "command": "workbench.action.quickOpen" },
	{ "key": "ctrl+k", "command": "workbench.action.showCommands" },
	{ "key": "cmd+j", "command": "runCommands", "args": { "commands": ["editor.action.selectAll", { "command": "editor.action.deleteLines" }] }, "when": "editorTextFocus" },
	{ "key": "cmd+u", "command": "cursor.generateGitCommitMessage" },
	{ "key": "cmd+i", "command": "workbench.action.terminal.focus", "when": "terminalFocus" },
	{ "key": "cmd+,", "command": "workbench.action.openSettings" },
]`)
	for _, tc := range []struct {
		ctx       keymap.Context
		seq, want string
	}{
		{keymap.CtxGlobal, "cmd+k cmd+s", "workbench.action.files.saveAll"},
		{keymap.CtxEditor, "ctrl+shift+alt+x", "editor.action.deleteLines"},
		{keymap.CtxTree, "ctrl+shift+alt+x", ""},                      // 只在编辑器里
		{keymap.CtxEditor, "cmd+d", ""},                               // 拿掉了
		{keymap.CtxEditor, "cmd+l", "workbench.action.quickOpen"},     // 全局的赢过编辑器里内置的「选中行」
		{keymap.CtxGlobal, "ctrl+k", "workbench.action.showCommands"}, // 内置的 ctrl+k 连按让开
		{keymap.CtxGlobal, "cmd+,", "yan.config.open"},                // VS Code 的叫法
	} {
		if got := resolve(t, e, tc.ctx, tc.seq); got != tc.want {
			t.Errorf("%s in %v = %q, want %q", tc.seq, tc.ctx, got, tc.want)
		}
	}
	if id := resolve(t, e, keymap.CtxEditor, "cmd+j"); !strings.HasPrefix(id, runCommandsID) {
		t.Errorf("runCommands = %q", id)
	}
	if got := e.KeyHint("workbench.action.files.saveAll"); got != "⌘K ⌘S" {
		t.Errorf("hint = %q", got)
	}
	joined := strings.Join(warns, "\n")
	if len(warns) != 1 || !strings.Contains(joined, "cursor.generateGitCommitMessage") || !strings.Contains(joined, "workbench.action.terminal.focus") {
		t.Fatalf("warnings %q", warns)
	}

	// runCommands 依次运行：全选，再删掉选中的行
	p := filepath.Join(t.TempDir(), "a.txt")
	os.WriteFile(p, []byte("one\ntwo\n"), 0o644)
	if err := e.OpenFile(p); err != nil {
		t.Fatal(err)
	}
	e.Handle(cmdKey('j', keymap.ModSuper))
	if s := e.cur().doc.buf.Snapshot(); s.String(0, s.Len()) != "" {
		t.Fatalf("after runCommands: %q", s.String(0, s.Len()))
	}
}

// 一条写错的绑定不让编辑器起不来；整个文件读不了时说一声，内置的键照常。
func TestKeybindingsJSONErrors(t *testing.T) {
	e, warns := keybindingsEditor(t, `[ { "key": "cmd+k", "command": `)
	if len(warns) != 1 || !strings.HasPrefix(warns[0], "keybindings.json：") {
		t.Fatalf("warnings %q", warns)
	}
	if got := resolve(t, e, keymap.CtxGlobal, "cmd+p"); got != "workbench.action.quickOpen" {
		t.Errorf("built-in cmd+p = %q", got)
	}
	_, warns = keybindingsEditor(t, `[{ "key": "cmd+9", "command": "editor.action.deleteLines", "args": { "x": 1 } }]`)
	if len(warns) != 1 || !strings.Contains(warns[0], "args") {
		t.Fatalf("args warnings %q", warns)
	}
}

// 从命令面板打开 keybindings.json：新文件先放好说明；在砚里保存后立即生效。
func TestKeybindingsJSONReloadsOnSave(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	e, warns := New(config.Default(), fakeLayouter{}, &fakeHost{}, macGUI)
	if len(warns) > 0 {
		t.Fatal(warns)
	}
	e.Run(cmdOpenKeybindings)
	b := e.cur().doc.buf
	if b.Path() != config.KeybindingsPath() || !strings.Contains(b.Snapshot().String(0, b.Snapshot().Len()), "VS Code") {
		t.Fatalf("opened %q", b.Path())
	}
	e.Run("editor.action.selectAll")
	typeText(e, `[{ "key": "cmd+k cmd+s", "command": "workbench.action.files.saveAll" }]`)
	e.Run("workbench.action.files.save")
	if got := resolve(t, e, keymap.CtxGlobal, "cmd+k cmd+s"); got != "workbench.action.files.saveAll" {
		t.Fatalf("after save: %q (status %q)", got, e.status)
	}
}
