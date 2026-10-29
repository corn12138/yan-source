package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

// termHost is a window host with a terminal that records what bindings send to it.
type termHost struct {
	fakeHost
	open    bool
	focused bool
	sent    []string
}

func (h *termHost) SendTerminal(s string) bool {
	if h.open {
		h.sent = append(h.sent, s)
	}
	return h.open
}
func (h *termHost) TerminalFocused() bool { return h.focused }

func runtimeEditor(t *testing.T, p keymap.Profile, json string) (*Editor, *termHost) {
	t.Helper()
	cfg := config.Default()
	cfg.Keybindings = []byte(json)
	h := &termHost{}
	e, warns := New(cfg, fakeLayouter{}, h, p)
	if len(warns) > 0 {
		t.Fatalf("warnings: %v", warns)
	}
	t.Cleanup(e.Close)
	path := filepath.Join(t.TempDir(), "a.txt")
	os.WriteFile(path, []byte("one\ntwo\n"), 0o644)
	if err := e.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	return e, h
}

// A when that names the selection is told at the key press: without one the key does what it
// did before (cmd+l selects the line); with one it runs the binding.
func TestWhenSelectionIsEvaluatedAtKeyPress(t *testing.T) {
	e, _ := runtimeEditor(t, macGUI, `[
	{ "key": "cmd+l", "command": "editor.action.deleteLines", "when": "editorTextFocus && editorHasSelection" },
	{ "key": "alt+x", "command": "editor.action.selectAll", "when": "editorHasSelection" },
]`)
	e.Handle(cmdKey('l', keymap.ModSuper))
	if content(e) != "one\ntwo\n" || e.cur().doc.buf.Selections().Main().Empty() {
		t.Fatalf("without a selection cmd+l must select the line: %q", content(e))
	}
	e.Handle(cmdKey('l', keymap.ModSuper))
	if content(e) != "two\n" {
		t.Fatalf("with a selection cmd+l must delete it: %q", content(e))
	}
	// A typing key whose guard does not hold is left to type its character (Option, AltGr).
	altX := keymap.Chord{Mods: keymap.ModAlt, Key: keymap.KeyRune, Rune: 'x'}
	if e.TryKey(altX) {
		t.Fatal("alt+x without a selection must type")
	}
	e.Run("expandLineSelection")
	if !e.TryKey(altX) {
		t.Fatal("alt+x with a selection must run the binding")
	}
}

// isWindows and isMac follow the profile, not the system the binary was built for.
func TestWhenPlatformFollowsProfile(t *testing.T) {
	win := keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl}, GUI: true, OS: "windows"}
	bindings := `[
	{ "key": "ctrl+alt+u", "command": "editor.action.selectAll", "when": "isWindows" },
	{ "key": "ctrl+alt+i", "command": "editor.action.selectAll", "when": "isMac" },
]`
	e, _ := runtimeEditor(t, win, bindings)
	ctrlAlt := func(r rune) keymap.Chord {
		return keymap.Chord{Mods: keymap.ModCtrl | keymap.ModAlt, Key: keymap.KeyRune, Rune: r}
	}
	if !e.TryKey(ctrlAlt('u')) || e.TryKey(ctrlAlt('i')) {
		t.Fatal("Windows profile must hold isWindows and not isMac")
	}
	m, _ := runtimeEditor(t, macGUI, strings.ReplaceAll(bindings, "ctrl+alt", "cmd+alt"))
	cmdAlt := func(r rune) keymap.Chord {
		return keymap.Chord{Mods: keymap.ModSuper | keymap.ModAlt, Key: keymap.KeyRune, Rune: r}
	}
	if m.TryKey(cmdAlt('u')) || !m.TryKey(cmdAlt('i')) {
		t.Fatal("macOS profile must hold isMac and not isWindows")
	}
}

// type and sendSequence go where the keyboard is: the editor, or the terminal.
func TestBindingArgsTypeSendSequenceAndOpen(t *testing.T) {
	e, h := runtimeEditor(t, macGUI, `[
	{ "key": "cmd+alt+t", "command": "type", "args": { "text": "hi " } },
	{ "key": "cmd+alt+s", "command": "workbench.action.terminal.sendSequence", "args": { "text": "\u0003ls\r" } },
	{ "key": "cmd+alt+o", "command": "vscode.open", "args": "b.txt" },
	{ "key": "cmd+alt+r", "command": "type", "args": { "text": "run" }, "when": "terminalFocus" },
]`)
	key := func(r rune) KeyInput {
		return KeyInput{keymap.Chord{Mods: keymap.ModSuper | keymap.ModAlt, Key: keymap.KeyRune, Rune: r}}
	}
	e.Handle(key('t'))
	if content(e) != "hi one\ntwo\n" {
		t.Fatalf("type = %q", content(e))
	}
	e.Handle(key('s'))
	if !strings.Contains(e.status, "终端") {
		t.Fatalf("sendSequence without a terminal must say so: %q", e.status)
	}
	h.open = true
	e.Handle(key('s'))
	e.Handle(key('r')) // terminalFocus does not hold: nothing typed, nothing sent
	h.focused = true
	e.Handle(key('r'))
	if strings.Join(h.sent, "|") != "\x03ls\r|run" || content(e) != "hi one\ntwo\n" {
		t.Fatalf("sent %q, text %q", h.sent, content(e))
	}
	h.focused = false

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bee\n"), 0o644)
	e.root = dir
	e.Handle(key('o'))
	if p := e.cur().doc.buf.Path(); p != filepath.Join(dir, "b.txt") {
		t.Fatalf("vscode.open opened %q (status %q)", p, e.status)
	}
	if got := e.bindingPath("file://" + filepath.ToSlash(filepath.Join(dir, "b.txt"))); got != filepath.Join(dir, "b.txt") {
		t.Fatalf("file URI = %q", got)
	}
}

func TestBindingArgsAreCheckedOnLoad(t *testing.T) {
	cfg := config.Default()
	cfg.Keybindings = []byte(`[
	{ "key": "cmd+1", "command": "type" },
	{ "key": "cmd+2", "command": "vscode.open", "args": { "x": 1 } },
	{ "key": "cmd+3", "command": "runCommands", "args": { "commands": [{ "command": "type" }] } },
]`)
	_, warns := New(cfg, fakeLayouter{}, &fakeHost{}, macGUI)
	if len(warns) != 3 {
		t.Fatalf("warnings %q", warns)
	}
}

// runCommands waits for background Git commands and stops at the first failure, also one
// that comes back later.
func TestRunCommandsWaitsAndStopsOnFailure(t *testing.T) {
	e, _ := runtimeEditor(t, macGUI, `[]`)
	bindings := `[
	{ "key": "cmd+alt+1", "command": "runCommands", "args": { "commands": ["test.async", { "command": "type", "args": { "text": "after " } }] } },
	{ "key": "cmd+alt+2", "command": "runCommands", "args": { "commands": ["test.fail", { "command": "type", "args": { "text": "never " } }] } },
	{ "key": "cmd+alt+3", "command": "runCommands", "args": { "commands": ["test.asyncFail", { "command": "type", "args": { "text": "never " } }] } },
]`
	var done func(err error)
	async := func(e *Editor) error {
		e.gitPending++
		done = func(err error) {
			e.Post(func(e *Editor) {
				if err != nil {
					e.fails++
				}
				e.gitPending--
				e.pumpChain()
			})
		}
		return nil
	}
	e.Register(Command{ID: "test.async", Run: async})
	e.Register(Command{ID: "test.asyncFail", Run: async})
	e.Register(Command{ID: "test.fail", Run: func(*Editor) error { return errors.New("failed") }})
	e.cfg.Keybindings = []byte(bindings)
	if w := e.compileKeys(e.cfg); len(w) > 0 {
		t.Fatal(w)
	}
	key := func(r rune) KeyInput {
		return KeyInput{keymap.Chord{Mods: keymap.ModSuper | keymap.ModAlt, Key: keymap.KeyRune, Rune: r}}
	}

	e.Handle(key('1'))
	if content(e) != "one\ntwo\n" {
		t.Fatalf("ran before the background command finished: %q", content(e))
	}
	done(nil)
	pump(t, e, func() bool { return content(e) == "after one\ntwo\n" })

	e.Handle(key('2'))
	if content(e) != "after one\ntwo\n" || e.status != "failed" {
		t.Fatalf("a failure must stop the rest: %q, %q", content(e), e.status)
	}
	e.Handle(key('3'))
	done(errors.New("push rejected"))
	settle(t, e, 100e6)
	if content(e) != "after one\ntwo\n" || len(e.chain) != 0 {
		t.Fatalf("a background failure must stop the rest: %q", content(e))
	}
}

// A malformed reload keeps the last keys and settings that read, and keeps saying so.
func TestMalformedReloadKeepsLastValidConfiguration(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	os.MkdirAll(filepath.Dir(config.UserPath()), 0o755)
	writeFile(t, config.UserPath(), "[editor]\ntab_width = 8\n")
	writeFile(t, config.KeybindingsPath(), `[{ "key": "cmd+k cmd+s", "command": "workbench.action.files.saveAll" }]`)
	cfg, warns := config.Load("")
	e, w2 := New(cfg, fakeLayouter{}, &fakeHost{}, macGUI)
	if len(warns)+len(w2) > 0 {
		t.Fatal(warns, w2)
	}
	t.Cleanup(e.Close)

	writeFile(t, config.KeybindingsPath(), `[{ "key": "cmd+k cmd+s", "command": `)
	writeFile(t, config.UserPath(), "[editor]\ntab_width = \n")
	e.reloadConfig()
	if got := resolve(t, e, keymap.CtxGlobal, "cmd+k cmd+s"); got != "workbench.action.files.saveAll" {
		t.Fatalf("malformed keybindings.json dropped the keys: %q", got)
	}
	if e.cfg.TabWidth != 8 {
		t.Fatalf("malformed config.toml dropped the settings: tab width %d", e.cfg.TabWidth)
	}
	e.status = "" // a key press clears the status; the problem stays visible
	var f view.Frame
	e.Frame(&f)
	if !strings.Contains(f.Status.Message, "沿用上次") {
		t.Fatalf("status = %q", f.Status.Message)
	}
	writeFile(t, config.KeybindingsPath(), `[]`)
	writeFile(t, config.UserPath(), "")
	e.reloadConfig()
	e.status = ""
	e.Frame(&f)
	if f.Status.Message != "" || resolve(t, e, keymap.CtxGlobal, "cmd+k cmd+s") == "workbench.action.files.saveAll" {
		t.Fatalf("a clean reload must apply and clear the problem: %q", f.Status.Message)
	}
}

// Saved elsewhere — another window, another program — the user's files apply in every window,
// and a project's own settings still win.
func TestExternalConfigChangesApplyToEveryWindow(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	os.MkdirAll(filepath.Dir(config.UserPath()), 0o755)
	a, root := newFolderEditor(t, map[string]string{".yan.toml": "[editor]\ntab_width = 2\n"})
	b, _ := New(config.Default(), fakeLayouter{}, &fakeHost{}, macGUI)
	t.Cleanup(b.Close)
	if a.cfgWatcher == nil || b.cfgWatcher == nil {
		t.Skip("no file events on this platform")
	}
	a.reloadConfig()
	writeFile(t, config.UserPath(), "[editor]\ntab_width = 6\n")
	writeFile(t, config.KeybindingsPath(), `[{ "key": "cmd+k cmd+s", "command": "workbench.action.files.saveAll" }]`)
	pump(t, b, func() bool { return b.cfg.TabWidth == 6 && resolve(t, b, keymap.CtxGlobal, "cmd+k cmd+s") != "" })
	pump(t, a, func() bool { return resolve(t, a, keymap.CtxGlobal, "cmd+k cmd+s") != "" })
	if a.cfg.TabWidth != 2 {
		t.Fatalf("project override lost: %d", a.cfg.TabWidth)
	}
	writeFile(t, filepath.Join(root, ".yan.toml"), "[editor]\ntab_width = 3\n")
	pump(t, a, func() bool { return a.cfg.TabWidth == 3 })
}
