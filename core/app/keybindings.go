package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/keymap"
)

// keybindings.json：照着 Mayoi v0.7.0（MIT，internal/keymap、cmd/mayoi/keymap.go），按 VS Code 的格式写的快捷键，
// 可以直接粘贴 VS Code、Cursor 里的。砚的命令 ID 本来就是 VS Code 的名字，所以大多数一条对一条。
// 它在内置键位和配置文件的 [keys] 之后生效：后写的赢，"-命令" 拿掉那个命令的键。只说焦点的 when 条件按砚的
// 四个上下文（全局、编辑器、侧边栏、浮层）各求一次值，在哪个上下文里成立就绑在哪个上下文里；说到选区、终端、
// 查找框这类按键那一刻才知道的名字时，绑成一个守卫命令（whenGuard），按下时求值，不成立就交给原来的键。

// runCommandsID 是 keybindings.json 里第 n 条带参数的绑定注册成的命令；whenID 是守卫命令。
const (
	runCommandsID = "keybindings.runCommands."
	whenID        = "keybindings.when."
)

// runtimeWhen are the context names only the moment of the key press can tell.
var runtimeWhen = []string{
	"editorHasSelection", "editorHasMultipleSelections", "editorReadonly", "editorLangId",
	"terminalFocus", "terminalIsOpen", "isInDiffEditor", "inDiffEditor", "imagePreviewFocus",
	"findWidgetVisible", "findInputFocussed", "searchViewletFocus", "searchViewletVisible",
	"inputFocus", "textInputFocus", "editorTextFocus", "editorFocus", "sideBarFocus", "listFocus",
	"explorerViewletFocus", "filesExplorerFocus", "inQuickOpen", "scmFocus", "commitMessageFocus",
}

// focusKeys 是砚的每个上下文里成立的 VS Code 上下文名；when 条件里别的名字都不成立（同 VS Code：没定义的就是假）。
var focusKeys = map[keymap.Context][]string{
	keymap.CtxEditor:  {"editorFocus", "editorTextFocus", "textInputFocus", "inputFocus", "editorIsOpen", "activeEditor"},
	keymap.CtxTree:    {"sideBarFocus", "sideBarVisible", "listFocus", "explorerViewletFocus", "explorerViewletVisible", "filesExplorerFocus", "editorIsOpen"},
	keymap.CtxOverlay: {"inQuickOpen", "textInputFocus", "inputFocus", "editorIsOpen"},
}

// systemKey 报告 isMac、isLinux、isWindows 这类说系统的上下文名在这里成立没有；不是这类名字时 known 为 false。
// 系统取自键位环境（keymap.Profile），不是编译时的系统：Windows、Linux 的窗口版按它们自己的求值。
func systemKey(p keymap.Profile, name string) (holds, known bool) {
	switch name {
	case "isMac":
		return p.Platform() == "darwin", true
	case "isLinux":
		return p.Platform() == "linux", true
	case "isWindows":
		return p.Platform() == "windows", true
	case "isWeb":
		return false, true
	}
	return false, false
}

// knownWhen 报告 name 是不是砚认得的上下文名（在某个上下文里会成立的，或者说系统的）。
func knownWhen(name string) bool {
	if _, known := systemKey(keymap.Profile{}, name); known || slices.Contains(runtimeWhen, name) {
		return true
	}
	for _, names := range focusKeys {
		if slices.Contains(names, name) {
			return true
		}
	}
	return false
}

// whenContexts 返回条件成立的上下文：没有条件，或者在三个上下文里都成立，就是全局。
func whenContexts(p keymap.Profile, w *keymap.When) []keymap.Context {
	if w == nil {
		return []keymap.Context{keymap.CtxGlobal}
	}
	var in []keymap.Context
	for _, c := range []keymap.Context{keymap.CtxEditor, keymap.CtxTree, keymap.CtxOverlay} {
		holds := w.Eval(func(n string) bool {
			if h, known := systemKey(p, n); known {
				return h
			}
			return slices.Contains(focusKeys[c], n)
		})
		if holds {
			in = append(in, c)
		}
	}
	if len(in) == 3 {
		return []keymap.Context{keymap.CtxGlobal}
	}
	return in
}

// vscodeAliases 是 VS Code 里叫法和砚不同的命令。
var vscodeAliases = map[string]string{
	"workbench.action.openSettings":          cmdOpenSettings,
	"workbench.action.openSettingsJson":      "yan.config.open",
	"workbench.action.openGlobalKeybindings": cmdOpenKeybindings,
	"workbench.action.files.openFileFolder":  "workbench.action.files.openFolder",
	"workbench.action.togglePanel":           "workbench.action.terminal.toggleTerminal",
	"markdown.showPreview":                   "yan.svg.togglePreview",
}

const cmdOpenKeybindings = "workbench.action.openGlobalKeybindingsFile"

// sequences 是 seq 在当前键位环境下展开成的按键序列：mod 有几种就有几种。
func (e *Editor) sequences(seq string) [][]keymap.Chord {
	mods := e.profile.Mods
	if len(mods) == 0 {
		mods = []keymap.Mod{keymap.ModCtrl}
	}
	var out [][]keymap.Chord
	for _, m := range mods {
		if cs, err := keymap.Parse(seq, m); err == nil {
			out = append(out, cs)
		}
	}
	return out
}

// overlaps 报告两组按键序列里有没有一样的（same 为真时要一模一样，否则一个是另一个的前缀也算）。
func overlaps(a, b [][]keymap.Chord, same bool) bool {
	for _, x := range a {
		for _, y := range b {
			n := min(len(x), len(y))
			if slices.Equal(x[:n], y[:n]) && (!same || len(x) == len(y)) {
				return true
			}
		}
	}
	return false
}

// applyKeybindings 把 keybindings.json 的内容加到 bindings 上，返回跳过了什么。
func (e *Editor) applyKeybindings(bindings map[keymap.Context]map[string]string, data []byte) []string {
	for id := range e.commands {
		if strings.HasPrefix(id, runCommandsID) || strings.HasPrefix(id, whenID) {
			delete(e.commands, id) // 上次读的
		}
	}
	e.guards = map[string]whenGuard{}
	if data == nil {
		return nil
	}
	bs, warns, err := keymap.ParseVSCode(data)
	if err != nil {
		return []string{i18n.F("%s: %s", "keybindings.json", err.Error())}
	}
	for i := range warns {
		warns[i] = i18n.F("%s: %s", "keybindings.json", warns[i])
	}
	var unknown, unknownWhen []string
	ctxs := []keymap.Context{keymap.CtxGlobal, keymap.CtxEditor, keymap.CtxTree, keymap.CtxOverlay}
	for n, b := range bs {
		where := whenContexts(e.profile, b.When)
		guarded := runtimeNames(b.When)
		if guarded {
			where = []keymap.Context{keymap.CtxEditor, keymap.CtxTree, keymap.CtxOverlay}
		}
		if len(where) == 0 {
			for _, name := range b.When.Names() {
				if !knownWhen(name) && !slices.Contains(unknownWhen, name) {
					unknownWhen = append(unknownWhen, name)
				}
			}
			continue
		}
		if b.Removal() {
			id := e.commandOf(b.Command[1:])
			var keys [][]keymap.Chord
			if b.Key != "" {
				keys = e.sequences(b.Key)
			}
			if b.When == nil {
				where = ctxs
			}
			for _, c := range where {
				for seq, cmd := range bindings[c] {
					if (cmd == id || e.guards[cmd].cmd == id) && (keys == nil || overlaps(keys, e.sequences(seq), true)) {
						if g, ok := e.guards[cmd]; ok && cmd != id && g.fallback != "" {
							bindings[c][seq] = g.fallback
							continue
						}
						delete(bindings[c], seq)
					}
				}
			}
			continue
		}
		id, err := e.boundCommand(b, n)
		if err != nil {
			if errors.Is(err, errUnknownCommand) {
				if !slices.Contains(unknown, b.Command) {
					unknown = append(unknown, b.Command)
				}
			} else {
				warns = append(warns, i18n.F("%s: %s: %v", "keybindings.json", b.Command, err))
			}
			continue
		}
		keys := e.sequences(b.Key)
		if guarded {
			for _, c := range where {
				// The key a guard replaces runs when its condition does not hold.
				fallback := ""
				for seq, cmd := range bindings[c] {
					if overlaps(keys, e.sequences(seq), true) {
						fallback = cmd
						delete(bindings[c], seq)
					} else if overlaps(keys, e.sequences(seq), false) {
						delete(bindings[c], seq)
					}
				}
				if fallback == "" {
					for seq, cmd := range bindings[keymap.CtxGlobal] {
						if overlaps(keys, e.sequences(seq), true) {
							fallback = cmd
						}
					}
				}
				gid := fmt.Sprintf("%s%d.%d", whenID, n, c)
				e.guards[gid] = whenGuard{when: b.When, cmd: id, fallback: fallback}
				e.Register(Command{ID: gid, Hidden: true, Run: func(e *Editor) error {
					if id := e.resolveGuard(gid); id != "" {
						if err := e.commands[id].Run(e); err != nil {
							return err
						}
						e.guideSaw(id)
					}
					return nil
				}})
				if bindings[c] == nil {
					bindings[c] = map[string]string{}
				}
				bindings[c][b.Key] = gid
			}
			continue
		}
		for _, c := range where {
			// 后写的赢：同一个上下文里一样的、互为前缀的键让开；绑在全局时，别的上下文里一样的键也让开，
			// 不然编辑器里内置的键会先被查到。
			for _, other := range ctxs {
				if other != c && c != keymap.CtxGlobal {
					continue
				}
				for seq := range bindings[other] {
					if overlaps(keys, e.sequences(seq), other != c) {
						delete(bindings[other], seq)
					}
				}
			}
			if bindings[c] == nil {
				bindings[c] = map[string]string{}
			}
			bindings[c][b.Key] = id
		}
	}
	if len(unknown) > 0 {
		warns = append(warns, i18n.F("keybindings.json: Yan doesn't have these %d commands, skipped: %s", len(unknown), i18n.Join(unknown)))
	}
	if len(unknownWhen) > 0 {
		warns = append(warns, i18n.F("keybindings.json: when conditions never hold in Yan (%s); skipped these keys", i18n.Join(unknownWhen)))
	}
	return warns
}

var errUnknownCommand = i18n.Error("No such command")

// commandOf 返回 VS Code 命令名在砚里的命令 ID。
func (e *Editor) commandOf(name string) string {
	if _, ok := e.commands[name]; !ok {
		if alias, ok := vscodeAliases[name]; ok {
			return alias
		}
	}
	return name
}

// boundCommand 返回第 n 条绑定要运行的命令：带参数的（runCommands、type、终端的 sendSequence、vscode.open）
// 注册成一个新命令，参数在读配置时就检查好。
func (e *Editor) boundCommand(b keymap.VSBinding, n int) (string, error) {
	args := strings.TrimSpace(string(b.Args))
	if argCommands[b.Command] || args != "" && args != "null" {
		if err := e.checkBinding(b.Command, b.Args); err != nil {
			return "", err
		}
		id := runCommandsID + strconv.Itoa(n)
		name, raw := b.Command, b.Args
		e.Register(Command{ID: id, Hidden: true, Run: func(e *Editor) error { return e.runBinding(name, raw) }})
		return id, nil
	}
	id := e.commandOf(b.Command)
	if _, ok := e.commands[id]; !ok {
		return "", errUnknownCommand
	}
	return id, nil
}

// argCommands are the commands that only make sense with args.
var argCommands = map[string]bool{
	"runCommands": true, "type": true, "vscode.open": true, "workbench.action.terminal.sendSequence": true,
}

// checkBinding reports what is wrong with a command and its args before any key runs it.
func (e *Editor) checkBinding(name string, raw json.RawMessage) error {
	switch name {
	case "runCommands":
		steps, err := chainSteps(raw)
		if err != nil {
			return err
		}
		for _, st := range steps {
			if err := e.checkBinding(st.name, st.args); err != nil {
				return fmt.Errorf(i18n.T("%s in runCommands: %w"), st.name, err)
			}
		}
		return nil
	case "type", "workbench.action.terminal.sendSequence":
		if textArg(raw) == "" {
			return errors.New(i18n.T("args.text is required"))
		}
		return nil
	case "vscode.open":
		var path string
		if json.Unmarshal(raw, &path) != nil || path == "" {
			return errors.New(i18n.T("args must give a file path"))
		}
		return nil
	}
	if a := strings.TrimSpace(string(raw)); a != "" && a != "null" {
		return errors.New(i18n.T("Yan doesn't support bindings with args yet"))
	}
	if _, ok := e.commands[e.commandOf(name)]; !ok {
		return errUnknownCommand
	}
	return nil
}

// chainStep is one command of runCommands.
type chainStep struct {
	name string
	args json.RawMessage
}

func chainSteps(raw json.RawMessage) ([]chainStep, error) {
	var a struct {
		Commands []json.RawMessage `json:"commands"`
	}
	if err := json.Unmarshal(raw, &a); err != nil || len(a.Commands) == 0 {
		return nil, errors.New(i18n.T("runCommands requires args.commands"))
	}
	steps := make([]chainStep, 0, len(a.Commands))
	for _, c := range a.Commands {
		var st chainStep
		if json.Unmarshal(c, &st.name) != nil {
			var obj struct {
				Command string          `json:"command"`
				Args    json.RawMessage `json:"args"`
			}
			if json.Unmarshal(c, &obj) != nil || obj.Command == "" {
				return nil, errors.New(i18n.T("Each item of runCommands must give command"))
			}
			st.name, st.args = obj.Command, obj.Args
		}
		steps = append(steps, st)
	}
	return steps, nil
}

func textArg(raw json.RawMessage) string {
	var t struct {
		Text string `json:"text"`
	}
	json.Unmarshal(raw, &t)
	return t.Text
}

// runBinding runs a command of keybindings.json with its args.
func (e *Editor) runBinding(name string, raw json.RawMessage) error {
	switch name {
	case "runCommands":
		steps, err := chainSteps(raw)
		if err != nil {
			return err
		}
		e.chain = append(steps, e.chain...)
		e.chainFails = e.fails
		e.pumpChain()
		return nil
	case "type":
		if e.whenHolds("terminalFocus") {
			return e.sendTerminal(textArg(raw))
		}
		e.handleText(TextInput{Text: textArg(raw)})
		return nil
	case "workbench.action.terminal.sendSequence":
		return e.sendTerminal(textArg(raw))
	case "vscode.open":
		var path string
		json.Unmarshal(raw, &path)
		return e.openPath(e.bindingPath(path))
	}
	id := e.commandOf(name)
	c, ok := e.commands[id]
	if !ok {
		return fmt.Errorf(i18n.T("%s: %w"), name, errUnknownCommand)
	}
	if err := c.Run(e); err != nil {
		return err
	}
	e.guideSaw(id)
	return nil
}

// bindingPath resolves a vscode.open path: file:// and ~/ as VS Code takes them, and a
// relative path inside the open folder.
func (e *Editor) bindingPath(p string) string {
	if u, err := url.Parse(p); err == nil && u.Scheme == "file" {
		p = filepath.FromSlash(u.Path)
	}
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, filepath.FromSlash(rest))
		}
	} else if !filepath.IsAbs(p) && e.root != "" {
		p = filepath.Join(e.root, filepath.FromSlash(p))
	}
	return p
}

func (e *Editor) sendTerminal(s string) error {
	if h, ok := e.host.(interface{ SendTerminal(string) bool }); ok && h.SendTerminal(s) {
		return nil
	}
	return errors.New(i18n.T("No open terminal"))
}

// pumpChain runs the commands of runCommands, each once those before it are done: their Git
// commands finished and the status read again. A command that fails stops the rest, also when
// it fails later, in the background.
func (e *Editor) pumpChain() {
	for len(e.chain) > 0 {
		if e.fails > e.chainFails {
			e.chain = nil
			return
		}
		if e.gitPending > 0 {
			return // the last of gitDo's callbacks runs pumpChain again
		}
		st := e.chain[0]
		e.chain = e.chain[1:]
		if err := e.runBinding(st.name, st.args); err != nil {
			e.status = err.Error()
			e.chain = nil
			return
		}
	}
}

// whenGuard is a binding whose condition only the key press can tell.
type whenGuard struct {
	when     *keymap.When
	cmd      string // runs while the condition holds
	fallback string // the key's earlier command, "" for none
}

// resolveGuard returns the command a guard's key runs now, "" when none: the key then
// does what it would do unbound, as typing a character.
func (e *Editor) resolveGuard(id string) string {
	for range 64 { // guards chain through their fallbacks; the bound only stops a cycle
		g, ok := e.guards[id]
		if !ok {
			return id
		}
		if g.when.Eval(e.whenHolds) {
			id = g.cmd
		} else {
			id = g.fallback
		}
		if id == "" {
			return ""
		}
	}
	return ""
}

// runtimeNames reports whether w names a context only the key press can tell.
func runtimeNames(w *keymap.When) bool {
	return slices.ContainsFunc(w.Names(), func(n string) bool { return slices.Contains(runtimeWhen, n) })
}

// whenHolds tells whether a context name holds at this moment.
func (e *Editor) whenHolds(name string) bool {
	if h, known := systemKey(e.profile, name); known {
		return h
	}
	t := e.cur()
	ctx := e.context()
	terminalTab := t != nil && t.term != nil
	findInput := e.ov == nil && e.find.open && e.findFocus != fieldNone
	editorText := ctx == keymap.CtxEditor && t != nil && !terminalTab && !t.image && !findInput
	terminal := ctx == keymap.CtxEditor && terminalTab && !findInput
	if h, ok := e.host.(interface{ TerminalFocused() bool }); ok && h.TerminalFocused() {
		terminal, editorText, findInput = true, false, false
	}
	searchInput := ctx == keymap.CtxTree && e.side == sideSearch && e.fs.focus != sfResults
	messageInput := ctx == keymap.CtxTree && e.side == sideSCM && e.repository().scm.focusMsg
	switch name {
	case "editorTextFocus", "editorFocus":
		return editorText
	case "textInputFocus", "inputFocus":
		return editorText || findInput || searchInput || messageInput || ctx == keymap.CtxOverlay && !terminal
	case "editorHasSelection":
		return editorText && !t.doc.buf.Selections().Main().Empty()
	case "editorHasMultipleSelections":
		return editorText && len(t.doc.buf.Selections().Ranges) > 1
	case "editorReadonly":
		return t != nil && !terminalTab && t.doc.buf.Snapshot().Meta().ReadOnly
	case "editorIsOpen", "activeEditor":
		return t != nil
	case "terminalFocus":
		return terminal
	case "terminalIsOpen":
		return terminal || e.hasTerminalTab()
	case "isInDiffEditor", "inDiffEditor":
		return t != nil && t.diff != nil
	case "imagePreviewFocus":
		return ctx == keymap.CtxEditor && t != nil && t.image
	case "findWidgetVisible":
		return e.find.open
	case "findInputFocussed":
		return findInput
	case "sideBarFocus", "listFocus":
		return !terminal && ctx == keymap.CtxTree
	case "sideBarVisible":
		return e.sidebar
	case "explorerViewletFocus", "filesExplorerFocus":
		return !terminal && ctx == keymap.CtxTree && e.side == sideExplorer
	case "explorerViewletVisible":
		return e.sidebar && e.side == sideExplorer
	case "searchViewletFocus":
		return !terminal && ctx == keymap.CtxTree && e.side == sideSearch
	case "searchViewletVisible":
		return e.sidebar && e.side == sideSearch
	case "scmFocus":
		return !terminal && ctx == keymap.CtxTree && e.side == sideSCM
	case "commitMessageFocus":
		return !terminal && messageInput
	case "inQuickOpen":
		return !terminal && ctx == keymap.CtxOverlay
	}
	return false
}

func (e *Editor) hasTerminalTab() bool {
	for _, c := range e.cols {
		for _, t := range c.tabs {
			if t.term != nil {
				return true
			}
		}
	}
	return false
}

// openKeybindings 在编辑器里打开 keybindings.json（不存在时先放好说明，保存时才写文件）。
func (e *Editor) openKeybindings() error {
	p := config.KeybindingsPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := e.openPath(p); err != nil {
		return err
	}
	if t := e.cur(); t != nil && t.doc.buf.Snapshot().Len() == 0 {
		t.doc.buf.Insert(keybindingsTemplate(i18n.Current()), edit.OriginCommand)
		e.status = i18n.T("New keybindings.json: takes effect as soon as it is saved")
	}
	return nil
}

// keybindingsTemplate is a new keybindings.json, its comments in the interface language l.
func keybindingsTemplate(l i18n.Locale) string {
	if l == i18n.SimplifiedChinese {
		return keybindingsTemplateZh
	}
	return keybindingsTemplateEn
}

const keybindingsTemplateEn = `// Yan's keyboard shortcuts, in VS Code's keybindings.json format: paste those of VS Code or Cursor.
// "key" is a key or a chord such as "cmd+k cmd+s"; "command" is a command ID (VS Code's, found in
// the Command Palette); "when" is where it applies: editorTextFocus (editor), sideBarFocus
// (sidebar), inQuickOpen (overlays), terminalFocus (terminal), editorHasSelection (a selection),
// isInDiffEditor (diff view), isMac / isWindows / isLinux, evaluated as the key is pressed.
// "args" supports runCommands, type, vscode.open and workbench.action.terminal.sendSequence.
// A "-" before a command removes that key from it. Takes effect when saved, after [keys] in config.toml.
[
	// { "key": "cmd+k cmd+s", "command": "workbench.action.files.saveAll" },
]
`

const keybindingsTemplateZh = `// 砚的键盘快捷方式，VS Code 的 keybindings.json 格式：VS Code、Cursor 里的可以直接粘贴过来。
// "key" 是按键或连按，如 "cmd+k cmd+s"；"command" 是命令 ID（与 VS Code 相同，命令面板里能找到）；
// "when" 是在哪里生效：editorTextFocus（编辑器）、sideBarFocus（侧边栏）、inQuickOpen（浮层）、terminalFocus（终端）、
// editorHasSelection（有选区）、isInDiffEditor（改动视图）、isMac / isWindows / isLinux；按下键的那一刻求值。
// "args" 支持 runCommands、type、vscode.open、workbench.action.terminal.sendSequence。
// 命令前加 "-" 是拿掉那个命令的这个键。保存后立即生效；在 config.toml 的 [keys] 之后生效。
[
	// { "key": "cmd+k cmd+s", "command": "workbench.action.files.saveAll" },
]
`
