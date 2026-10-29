// Package app 是编辑器的所有者：输入、命令、job、帧构建。
//
// 并发规则只有一条：Editor 的所有方法都只能在所有者 goroutine（前端事件循环）
// 上调用，唯一的例外是 Post——后台 goroutine 用它把结果「寄」回所有者。
package app

import (
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
	"github.com/corn12138/yan-source/core/workspace"
)

// Input 是前端送进内核的事件；isInput 未导出，外部包无法新增实现。
type Input interface{ isInput() }

type (
	// KeyInput 是一次按键。
	KeyInput struct{ Chord keymap.Chord }
	// TextInput 是要插入的文本：普通按键、输入法提交的整句、或一次粘贴。
	TextInput struct {
		Text  string
		Paste bool
		// IME 表示输入法组完字提交的一段：一次组字是一步撤销。Target 是开始组字时键盘所在的标签
		// （前端在组字开始时用 Editor.InputTarget 取）。
		IME    bool
		Target InputTarget
	}
	// MouseInput 是代码窗格里的鼠标事件；坐标是窗格内的 Unit（x 含行号栏，y 是第几行）。
	MouseInput struct {
		Column int
		X, Y   view.Unit
		Button MouseButton
		Mods   keymap.Mod
		Kind   MouseKind
	}
	// ScrollInput 滚动一列的代码窗格（Sidebar 为真时滚动资源管理器）。DY 的单位是行。
	ScrollInput struct {
		Column  int
		Sidebar bool
		DX, DY  view.Unit
	}
	FocusInput  struct{ Gained bool }
	MouseButton uint8
	MouseKind   uint8
)

const (
	MouseLeft MouseButton = iota
	MouseMiddle
	MouseRight
)

const (
	MousePress MouseKind = iota
	MouseDrag
	MouseRelease
)

func (KeyInput) isInput()    {}
func (TextInput) isInput()   {}
func (MouseInput) isInput()  {}
func (ScrollInput) isInput() {}
func (FocusInput) isInput()  {}

// Host 是内核需要、由前端用 platform/* 实现的平台能力（依赖倒置：core 不导入 platform）。
type Host interface {
	SetClipboard(text string) error
	Clipboard() (string, error)
	// Shell 让出整个界面，在 dir 里运行用户的交互式 shell，退出后恢复编辑器
	// （VS Code「终端」的 v0.1 形态；内嵌终端面板在 v0.2）。
	Shell(dir string) error
	// Trash 把文件移到废纸篓（放弃未跟踪文件的修改时）。
	Trash(path string) error
}

// Command 是唯一的功能入口：快捷键、命令面板、菜单、鼠标都调用 Run。
type Command struct {
	ID     string // 与 VS Code 同名，如 "workbench.action.files.save"
	Title  string // 命令面板标题，可以是中文
	Run    func(e *Editor) error
	Hidden bool // 不在命令面板里列出（光标移动这类）
}

// Editor 归一个 goroutine 所有；只有 Post 是线程安全的。
type Editor struct {
	armedRepository uint64
	armedPath       string

	repositories repositoryState
	repoScope    *repositoryContext

	mu      sync.Mutex // 只保护 pending
	pending []func(*Editor)
	spare   []func(*Editor)
	wake    chan struct{} // 容量 1：多次 Post 合并成一次唤醒

	cfg              *config.Config
	state            *config.State   // 菜单里选的设置、会话；nil 表示不记
	langPref         i18n.Preference // the interface language chosen without a state file (tests)
	profile          keymap.Profile
	allBranches      bool // 提交历史显示所有分支和标签（换文件夹也留着）
	repoPanel        bool // 源代码管理下面的仓库面板开着
	repoRemotes      bool // 仓库面板显示远程（否则分支）
	mergeManual      bool // 三方合并打开时不自动应用两边不冲突的改动
	commitFilesOpen  bool
	commitFilesWidth float64
	scmFolded        map[string]bool // 源代码管理里折起来的分组（scmfold.go）
	copied           []string        // 资源管理器复制的文件（前端没有能放文件的剪贴板时）
	compareLeft      string          // 资源管理器里「选为比较对象」的文件
	movedFrom        string          // 会话是从这个（已经不在的）文件夹搬来的：下次保存时去掉它的记录

	settingsGen uint64 // 外观设置变化时递增，前端据此重新取主题、字号
	autoSave    bool
	autoSaveGen uint64
	host        Host
	lay         view.LineLayouter
	keys        *keymap.Keymap
	commands    map[string]Command
	keyHint     map[string]string // 命令 ID → 命令面板里显示的快捷键
	docs        []*document
	cols        []*column
	col         int  // 有键盘焦点（或最后有）的列
	loaded      *tab // 选区已装进 buffer 的标签（见 sync）
	nextTerm    int  // 上一个终端标签的号（terminals.go）
	closed      []closedTab
	root        string         // 项目根目录（终端的工作目录）
	ws          *ws            // 打开的文件夹；只开单个文件时为 nil
	wsEpoch     uint64         // 每打开一个文件夹加一：后台任务带着它，换了文件夹就丢掉旧结果（见 postHere）
	def         defState       // 在文件夹里找着的定义（definition.go）
	rec         recoveryState  // 未命名文档的备份（recovery.go）
	maximized   *maximizeState // 最大化编辑区时画出来的列（maximize.go）
	nav         navHistory     // 转到定义前后的位置，后退、前进走它（navigation.go）
	// guards are the bindings of keybindings.json whose when only the key press can tell; chain
	// is what runCommands still has to run, stopped by a failure counted in fails after chainFails;
	// gitPending counts gitDo commands still running in the background.
	guide                         guideState         // the feature guide (guide.go)
	fileCol                       *column            // the last unlocked column implicit opens used (columnlock.go)
	configProblem                 string             // the last reload's first warning, kept in the status bar
	cfgWatcher                    *workspace.Watcher // the user's configuration folder
	guards                        map[string]whenGuard
	chain                         []chainStep
	fails, chainFails, gitPending int
	reloading                     bool // checkDisk 正在用磁盘内容重载文档（不算编辑，预览标签不转正）
	sidebar                       bool
	side                          sideView // 侧边栏显示哪个视图
	scm                           scmState
	fs                            fileSearch // 在文件中搜索
	vimReg                        vimRegister
	sideRows                      int // 前端给的侧边栏可见行数
	area                          area
	review                        reviewState
	ov                            *overlay
	find                          findState
	git                           gitState
	findFocus                     findField // 查找框里有键盘焦点的输入框；fieldNone 表示键盘在别处
	recent                        []string  // 最近打开的文件（绝对路径），快速打开空查询时列出
	recentCmds                    []string

	seq     []keymap.Chord // 正在输入的连按序列，如 Ctrl+K 之后
	status  string         // 状态栏左侧的临时消息
	action  statusAction   // 跟着某条消息的按钮：消息换了就不显示
	armed   string         // 需要按两次才生效的命令（有未保存修改时的退出、关闭标签）
	running string         // 正在执行的命令：要按两次的操作用它做 armed 的键
	quit    bool
	mouse   mouseState

	scratch view.LineLayout
	wrapped []view.Row // 正在出帧的那一行折成的视觉行
	vis     []vrow     // 正在出帧的窗格里每一行是哪个视觉行
	lineBuf []byte
	arena   []byte
}

type mouseState struct {
	dragging  bool
	lastClick time.Time
	lastPos   int
	clicks    int
}

// New 创建编辑器。profile 决定 "mod" 解析成 Ctrl 还是 Cmd（由前端探测终端后给出）。
func New(cfg *config.Config, lay view.LineLayouter, host Host, profile keymap.Profile) (*Editor, []string) {
	e := &Editor{
		cfg:      cfg,
		host:     host,
		lay:      lay,
		wake:     make(chan struct{}, 1),
		commands: map[string]Command{},
		keyHint:  map[string]string{},
		cols:     []*column{newColumn()},
		sideRows: 20,
	}
	e.profile = profile
	e.autoSave = profile.GUI // 窗口版照着 Mayoi 默认自动保存，终端版默认不
	if cfg.AutoSave != nil {
		e.autoSave = *cfg.AutoSave
	}
	e.guide.active = -1
	registerBuiltins(e)
	warns := e.compileKeys(cfg)
	e.watchConfig()
	return e, warns
}

// compileKeys 依次叠加内置键位、配置文件的 [keys]、keybindings.json，编出键位和快捷键提示；返回告警。
func (e *Editor) compileKeys(cfg *config.Config) []string {
	bindings := platformBindings(defaultBindings(), e.profile)
	var warns []string
	for ctx, m := range cfg.Keys {
		c, ok := keymap.ContextNames[ctx]
		if !ok {
			warns = append(warns, i18n.F("Unknown key context [keys.%s] (available: global, editor, explorer, overlay)", ctx))
			continue
		}
		if bindings[c] == nil {
			bindings[c] = map[string]string{}
		}
		for seq, cmd := range m {
			bindings[c][seq] = cmd
		}
	}
	warns = append(warns, e.applyKeybindings(bindings, cfg.Keybindings)...)
	keys, kw := keymap.Compile(bindings, e.profile)
	e.keys, warns = keys, append(warns, kw...)
	e.keyHint = map[string]string{}
	hints := bindings
	if len(e.guards) > 0 { // a guarded key is hinted for the command it binds
		hints = map[keymap.Context]map[string]string{}
		for c, m := range bindings {
			hints[c] = map[string]string{}
			for seq, id := range m {
				if g, ok := e.guards[id]; ok {
					id = g.cmd
				}
				hints[c][seq] = id
			}
		}
	}
	e.keyHints(hints, e.profile)
	for _, m := range bindings {
		for seq, id := range m {
			if _, ok := e.commands[id]; !ok && id != "" {
				warns = append(warns, i18n.F("Key %q is bound to unknown command %q", seq, id))
			}
		}
	}
	return warns
}

// Notify 在状态栏显示一条消息。
func (e *Editor) Notify(msg string) { e.status = msg }

// SetRoot 设置项目根目录（单文件模式：没有资源管理器，但 git 照样看）。
func (e *Editor) SetRoot(dir string) {
	e.root = dir
	e.startGit(dir)
}

// Root 返回项目根目录。
func (e *Editor) Root() string { return e.root }

// OpenFile 打开（或切换到）一个文件，并记入最近打开。
func (e *Editor) OpenFile(path string) error {
	defer e.sync()
	return e.openPath(path)
}

// Post 把 fn 排进所有者的待办队列；任意 goroutine 都可调用。
func (e *Editor) Post(fn func(*Editor)) {
	e.mu.Lock()
	e.pending = append(e.pending, fn)
	e.mu.Unlock()
	select {
	case e.wake <- struct{}{}:
	default: // 已经有一个唤醒在路上了
	}
}

// postHere 返回一个只在还是同一个文件夹时才把 fn 交给所有者的 Post：后台的搜索、git 操作
// 结束时，用户可能已经换了文件夹，旧文件夹的结果不能落到新文件夹的状态、文档上。仅所有者调用。
func (e *Editor) postHere() func(func(*Editor)) {
	epoch := e.wsEpoch
	return func(fn func(*Editor)) {
		e.Post(func(e *Editor) {
			if e.wsEpoch == epoch {
				fn(e)
			}
		})
	}
}

// Wake 返回唤醒通道：前端事件循环 select 它，收到后调用 RunPending。
func (e *Editor) Wake() <-chan struct{} { return e.wake }

// RunPending 在所有者 goroutine 上执行所有已投递的闭包。两个切片轮换使用，稳态零分配。
func (e *Editor) RunPending() {
	e.mu.Lock()
	run := e.pending
	e.pending = e.spare[:0]
	e.mu.Unlock()
	if len(run) == 0 {
		e.spare = run
		return
	}
	e.sync()
	for i, fn := range run {
		fn(e)
		run[i] = nil
	}
	e.spare = run
	e.sync()
}

// Quit 报告编辑器是否请求退出。
func (e *Editor) Quit() bool { return e.quit }

// Pending 报告是否正在等待连按的下一个键（前端据此决定要不要合并文本输入）。
func (e *Editor) Pending() bool { return len(e.seq) > 0 }

// statusAction 是跟着消息 text 的按钮。
type statusAction struct {
	text, label, cmd string
	repositoryID     uint64
}

// tell 在状态栏说 msg，旁边一个按钮 label，按下运行命令 cmd（照着 Mayoi 的 toast，如提交之后的「撤销」）。
func (e *Editor) tell(msg, label, cmd string) {
	e.status = msg
	e.action = statusAction{text: msg, label: label, cmd: cmd, repositoryID: e.repository().id}
}

// Handle 处理一个输入事件。仅所有者调用。
func (e *Editor) Handle(in Input) {
	e.sync()
	defer e.sync()
	defer e.guideCheck()
	if _, ok := in.(KeyInput); ok {
		e.status = ""
	}
	switch in := in.(type) {
	case KeyInput:
		e.handleKey(in.Chord)
	case TextInput:
		e.handleText(in)
	case MouseInput:
		e.handleMouse(in)
	case ScrollInput:
		e.scroll(in)
	case FocusInput:
		if in.Gained {
			e.checkDisk()
			e.gitChanged()
		} else {
			e.saveDirty() // 窗口失去焦点：自动保存
		}
	}
}

// TryKey 处理一次按键，报告内核有没有用它：没有绑定、也不在连按中途时返回 false，
// 前端就让这个键照常打出字来（GUI 里 Option+字母要么是快捷键，要么是特殊字符）。
func (e *Editor) TryKey(c keymap.Chord) bool {
	if e.review.draft != nil {
		e.Handle(KeyInput{Chord: c})
		return true
	}
	if len(e.seq) == 0 && (e.ov == nil || !overlayKey(c)) {
		id, m := e.keys.Resolve(e.context(), []keymap.Chord{c})
		if m == keymap.MatchNone || m == keymap.MatchExact && e.resolveGuard(id) == "" {
			return false // unbound, or a guard that does not hold: the key types as usual
		}
	}
	e.Handle(KeyInput{Chord: c})
	return true
}

// context 返回当前焦点对应的按键上下文。
func (e *Editor) context() keymap.Context {
	switch {
	case e.ov != nil:
		return keymap.CtxOverlay
	case e.area == areaTree && e.sidebar && e.ws != nil:
		return keymap.CtxTree
	}
	return keymap.CtxEditor
}

func (e *Editor) handleKey(c keymap.Chord) {
	if e.reviewKey(c) {
		return
	}
	if c.Key == keymap.KeyDelete && e.ov == nil && e.area == areaEditor && !(e.find.open && e.findFocus != fieldNone) && e.cur() != nil && e.cur().diff != nil {
		e.deleteReview()
		return
	}
	if e.ov != nil && len(e.seq) == 0 && e.handleOverlayKey(c) {
		return
	}
	if len(e.seq) == 0 && c.Key == keymap.KeyRune && c.Mods&^keymap.ModShift == 0 && (e.diffKey(c.Rune) || e.mergeKey(c.Rune)) {
		return // 改动视图、三方合并两边的翻页器键
	}
	if len(e.seq) == 0 && e.vimKey(c) {
		return
	}
	if e.ov == nil && e.find.open && e.findFocus != fieldNone && len(e.seq) == 0 && e.handleFindKey(c) {
		return
	}
	if e.ov == nil && e.area == areaTree && e.side == sideSCM && e.repository().scm.focusMsg && len(e.seq) == 0 && e.handleMessageKey(c) {
		return
	}
	if e.ov == nil && e.area == areaTree && e.side == sideSearch && len(e.seq) == 0 && e.handleSearchKey(c) {
		return
	}
	if e.ov == nil && e.area == areaTree && e.side == sideGuide && len(e.seq) == 0 && e.handleGuideKey(c) {
		return
	}
	e.seq = append(e.seq, c)
	id, m := e.keys.Resolve(e.context(), e.seq)
	switch m {
	case keymap.MatchPrefix:
		e.status = i18n.F("%s pressed, waiting for the next key…", chordString(e.seq))
		return
	case keymap.MatchExact:
		e.seq = e.seq[:0]
		e.run(id)
		return
	}
	if len(e.seq) > 1 {
		e.status = i18n.F("%s is not bound to a command", chordString(e.seq))
	}
	e.seq = e.seq[:0]
}

func chordString(seq []keymap.Chord) string {
	parts := make([]string, len(seq))
	for i, c := range seq {
		parts[i] = c.String()
	}
	return strings.Join(parts, " ")
}

// Run 按 ID 执行命令（菜单、按钮用它）；失败信息显示在状态栏。
func (e *Editor) Run(id string) {
	e.sync()
	defer e.sync()
	defer e.guideCheck() // a step can wait for the state a menu item or a button brings
	e.run(id)
}

func (e *Editor) run(id string) {
	c, ok := e.commands[id]
	if !ok {
		e.status = i18n.F("Unknown command: %s", id)
		return
	}
	if id != e.armed {
		e.armed = ""
	}
	e.running = id
	if err := c.Run(e); err != nil {
		e.status = err.Error()
		return
	}
	e.guideSaw(id)
}

// Commands 返回命令面板会列出的命令 ID（菜单、帮助用）。
func (e *Editor) Commands() []Command {
	out := make([]Command, 0, len(e.commands))
	for _, c := range e.commands {
		out = append(out, c)
	}
	return out
}

// KeyHint 返回命令的快捷键提示（命令面板、菜单显示用）；没有绑定时返回 ""。
func (e *Editor) KeyHint(id string) string { return e.keyHint[id] }

// Bindings 返回某个命令在给定上下文里的单键绑定（GUI 菜单的快捷键用）。
func (e *Editor) Binding(id string) (keymap.Chord, bool) { return e.keys.First(id) }

func (e *Editor) handleText(in TextInput) {
	if e.review.draft != nil {
		e.review.input += in.Text
		return
	}
	if r, n := utf8.DecodeRuneInString(in.Text); n > 0 && n == len(in.Text) && !in.IME && (e.diffKey(r) || e.mergeKey(r)) {
		return // 改动视图里打字：翻页器的键
	}
	if len(e.seq) > 0 { // 连按的第二个键是普通字母，例如 Ctrl+K 之后的 V
		r := []rune(in.Text)
		e.handleKey(keymap.Chord{Key: keymap.KeyRune, Rune: toLower(r[0])})
		if rest := string(r[1:]); rest != "" {
			e.handleText(TextInput{Text: rest, Paste: in.Paste})
		}
		return
	}
	if in.IME && in.Target.t != nil && in.Target != e.InputTarget() {
		e.commitInto(in.Target.t, in)
		return
	}
	if e.ov != nil { // 浮层的输入框：中文输入法可以直接搜中文文件名
		e.setOverlayInput(e.ov.input + strings.NewReplacer("\n", "", "\r", "", "\t", "").Replace(in.Text))
		return
	}
	if e.area == areaTree {
		switch {
		case e.side == sideSCM && e.repository().scm.focusMsg:
			e.repository().scm.message += in.Text
		case e.side == sideSearch && e.fs.focus != sfResults:
			f := e.searchField()
			*f += strings.NewReplacer("\n", "", "\r", "").Replace(in.Text)
			if e.fs.focus != sfReplace {
				e.searchSoon(searchPause)
			}
		}
		return
	}
	if e.find.open && e.findFocus != fieldNone {
		e.findText(in.Text)
		return
	}
	if !in.Paste && e.vimText(in.Text) {
		return
	}
	e.insertText(e.cur(), in)
}

// insertText 把文字打进标签 t 的文档。
func (e *Editor) insertText(t *tab, in TextInput) {
	if t == nil || !e.writable(t) {
		return
	}
	e.armed = ""
	buf := t.doc.buf
	s := in.Text
	if !in.Paste && !in.IME && utf8.RuneCountInString(s) == 1 {
		if r, _ := utf8.DecodeRuneInString(s); e.typeRune(t, r) {
			e.reveal()
			return
		}
	}
	origin := edit.OriginTyping
	switch {
	case in.Paste:
		origin = edit.OriginPaste
		s = normalizeEOL(s, buf.Snapshot().Meta().EOL)
	case in.IME:
		origin = edit.OriginIME
	}
	buf.Insert(s, origin)
	if in.Paste || in.IME {
		buf.BreakGroup()
	}
	e.reveal()
}

// InputTarget 记下键盘现在在哪个标签的代码里（不在代码里时为空）。
type InputTarget struct{ t *tab }

// InputTarget 返回键盘现在所在的标签：前端在输入法开始组字时取，提交时随 TextInput 带回来。
func (e *Editor) InputTarget() InputTarget {
	if e.ov != nil || e.area != areaEditor || e.find.open && e.findFocus != fieldNone {
		return InputTarget{}
	}
	return InputTarget{e.cur()}
}

// commitInto 把输入法提交的字放进开始组字时的那个标签：组字途中用户换了标签、点了别处，
// 字不能落进别的文件，也不能丢。那个标签已经关了时，在状态栏里留下这段字。
func (e *Editor) commitInto(t *tab, in TextInput) {
	for _, c := range e.cols {
		i := slices.Index(c.tabs, t)
		if i < 0 {
			continue
		}
		back, backAt, area := e.column(), e.column().active, e.area
		e.activate(c, i)
		e.insertText(t, in)
		e.activate(back, backAt)
		e.area = area
		return
	}
	e.status = i18n.F("Text \"%s\" from the input method was not inserted: the tab where composition began is closed", in.Text)
}

func normalizeEOL(s string, eol text.EOL) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if eol == text.EOLCRLF {
		s = strings.ReplaceAll(s, "\n", "\r\n")
	}
	return s
}

func toLower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 'a' - 'A'
	}
	return r
}

func (e *Editor) writable(t *tab) bool {
	if t.doc.buf.Snapshot().Meta().ReadOnly {
		e.status = i18n.T("Read-only file: can't edit")
		return false
	}
	if t.svg || t.markdown { // 看不见的文字不能改
		e.status = i18n.F("Can't edit in the preview: press %s to return to the text", e.hintOr("yan.svg.togglePreview", "⇧⌘V"))
		return false
	}
	return true
}

// GotoLine 把光标移到第 n 行（从 1 数）行首，并让它可见。
func (e *Editor) GotoLine(n int) {
	e.GotoLocation(n, 1)
}

// GotoLocation uses one-based Unicode columns, independent of UTF-8 bytes,
// tab width and the frontend's visual wrapping. Missing columns mean line start.
func (e *Editor) GotoLocation(line, column int) {
	e.sync()
	defer e.sync()
	if t := e.cur(); t != nil {
		buf := t.doc.buf
		s := buf.Snapshot()
		row := max(line, 1) - 1
		from, to := s.LineRange(row)
		at := from
		if column > 1 {
			at = to
			col := 1
			for off := range s.String(from, to) {
				if col == column {
					at = from + off
					break
				}
				col++
			}
		}
		buf.SetSelections(edit.Single(edit.Cursor(at)))
		e.centerLine(t, s.LineAt(at))
		e.reveal()
	}
}
