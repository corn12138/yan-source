// Package gui 是图形前端：一个原生窗口，用 MyGo 在 GPU 上画，没有 webview、没有 Electron，也没有 cgo。
//
// 和终端前端一样，它只做三件事：把窗口事件翻译成内核的 Input、提供按格排版的 LineLayouter、
// 把 view.Frame 画出来。外壳（标题栏、资源管理器、标签条、状态栏、快速打开）的样子照着 Mayoi，
// 代码窗格按格画：一格宽是代码字体里 0 的宽度，一行高是字号乘行高，中文占两格。
//
// 所有者 goroutine 就是 MyGo 的主线程：视图函数、输入回调、菜单回调都在主线程上跑，
// 后台 Post 的闭包经 Window.Update 送到主线程执行。
package gui

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/view"
	"github.com/corn12138/yan-source/core/view/cell"
	"github.com/corn12138/yan-source/internal/launch"
	"github.com/corn12138/yan-source/internal/process"
	"github.com/corn12138/yan-source/internal/termsession"
	"github.com/corn12138/yan-source/internal/version"
)

// Options 是 GUI 的外观设置。
type Options struct {
	Theme        string  // ""（Mayoi）、"matrix"、"claude"
	Appearance   string  // ""（跟随系统）、"light"、"dark"
	FontFamily   string  // 代码字体，逗号分隔的列表依次回退
	FontSize     float32 // 代码字号
	LineHeight   float32 // 行高是字号的几倍
	SidebarWidth float32 // 资源管理器的宽度（DIP）
	TabWidth     int
}

// DefaultOptions 与 Mayoi 的默认设置一致。
func DefaultOptions() Options {
	return Options{FontFamily: "monospace", FontSize: 14, LineHeight: 1.6, SidebarWidth: 240, TabWidth: 4}
}

// Frontend 是图形前端。
type Frontend struct {
	opts          Options
	ed            *app.Editor
	win           *mygo.Window
	frame         view.Frame
	font          codeFont
	pal           *palette
	paletteTarget *palette
	paletteFade   paletteFade
	nativeBar     ui.TitleBar // the room the window's own controls took in the last frame
	dark          bool        // 系统外观（Appearance 为空时跟随它）

	blurred bool
	mascot  mascotState
	rain    rainState
	glyphs  glyphCache
	cols    []colState // 每列的界面状态（窗格的位置、滚动余量）
	tree    ui.ListState
	treeAt  int // 上一帧键盘选中的资源管理器行：变了才滚过去
	// 输入
	preedit       string          // 输入法正在组的字
	preeditAt     int             // 组字光标在第几个字
	composeTarget app.InputTarget // 开始组字时键盘所在的标签
	badge         string          // 状态栏和窗口标题里的版本标签（预览版 v…）
	quitting      bool            // 已经请应用退出了（内核的退出命令）
	swallowText   bool            // 刚把一个打字键当快捷键用了：丢掉它随后到来的文字
	pressCol      int             // 左键按在第几列的代码窗格；-1 表示没有
	glow          float32         // 正在画的这一列里，块操作刚写进去的行亮到多少（0 是不亮）
	// 浮层
	reviewID       int
	reviewInput    string
	lastReviewTerm *termSession
	ovSel          int    // 上一帧的选中项：键盘移动后把它滚进视野
	ovInput        string // 输入框里的字
	ovEditing      bool   // 这一帧是用户在输入框里改的字（内核还没回显）
	root           ui.Handle
	services       ui.Services
	hasServices    bool
	menuPaths      map[string]string // command → its place in the menu bar, for the guide (learn.go)
	settingsGen    uint64
	// 查找框
	findQuery, findWith     string
	findEditing             bool // 这一帧是用户在输入框里改的字
	findFocus               view.FindField
	findSelectGen           uint64
	findQueryIn, findWithIn ui.Handle
	// 源代码管理的提交说明
	scmStates map[uint64]*scmInputState
	// 在文件中搜索
	searchText    [4]string // 查询、替换、包含、排除
	searchEditing int       // 这一帧用户改了第几个输入框（从 1 数；0 表示没改）
	searchFocus   view.SearchField
	searchIn      [4]ui.Handle
	searchList    ui.ListState
	searchSel     int
	// 终端面板
	terms               []*termSession
	tabTerms            map[int]*termSession // 编辑区标签里的终端，按标签的号（termtabs.go）
	shownTerm           int                  // 上一帧键盘交给了哪个标签里的终端（换了才再交一次）
	termActive          int
	panelOpen, panelMax bool
	panelH              float32
	panelW              float32       // the terminal panel's width at the window's left or right
	focusTerm           bool          // 下一帧把键盘交给终端
	focusRoot           bool          // 下一帧把键盘交给工作台（在视图外面要焦点时：项目挪窗口时控件可能同时绑在两个窗口里）
	termFocused         bool          // 上一帧键盘在终端里
	middle              any           // the key of the tab a middle press is on, until it is let go of
	mdClickTaken        bool          // a link or button in a Markdown preview took this frame's click
	clipImage           func() []byte // the clipboard image for tests; nil reads the system's
	crumbs              crumbsPicker  // the breadcrumbs' folder list, while open
	termTheme           string
	termFontSize        float32
	termFontFamily      string
	dragTermFrames      int
	// 看图
	images      map[string]*imageEntry
	markdowns   map[string]*markdownState // Markdown 预览，按文件
	copiedFiles []string                  // 不开窗口时资源管理器复制的文件
	pans        map[string][2]float32
	panFrom     [2]float32
	// 项目：所在的窗口（换窗口时后台通知跟着 winp 走），关项目时停掉通知的 done
	g    *group
	winp atomic.Pointer[mygo.Window]
	done chan struct{}
	// attention 是项目不在眼前时它的终端响铃、发通知的原因：方块描上强调色，提示里说为什么，换过去就清掉
	attention string
	logo      projectLogo // the logo found in this project's folder, for its square on the rail
	// 状态栏、命令中心上一帧的宽度：窄的时候少写几项
	statusW, centerW float32
}

// colState 是一列在界面上的状态。
type colState struct {
	commitList  commitListState
	reviewHover int
	dropFrames  int
	pane        ui.Handle  // 代码窗格：前端用它的位置换算鼠标坐标和输入法锚点
	scroll      float32    // 触控板滚动不足一行的余量（DIP）
	scrollX     float32    // 横向滚动不足一格的余量（DIP）
	peek        ui.Rect    // 改动的框在窗格里占的地方（没开时是零）
	size        ui.Rect    // 窗格上一帧画在哪里（宽高用来认出点在了右边那一条上）
	strip       bool       // 左键按在右边那一条上：拖动是在滚文件，不是选文字
	split       splitState // 并排两窗格的改动视图：滑动、发光、沟槽里指着的箭头（splitdiff.go）
	link        codeLink   // 按住链接修饰键时指着的名字（点它转到定义）
}

// New 创建图形前端。
func New(opts Options) *Frontend {
	d := DefaultOptions()
	if opts.FontFamily == "" {
		opts.FontFamily = d.FontFamily
	}
	if opts.FontSize <= 0 {
		opts.FontSize = d.FontSize
	}
	if opts.LineHeight <= 0 {
		opts.LineHeight = d.LineHeight
	}
	if opts.SidebarWidth <= 0 {
		opts.SidebarWidth = d.SidebarWidth
	}
	if opts.TabWidth <= 0 {
		opts.TabWidth = d.TabWidth
	}
	return &Frontend{opts: opts, pressCol: -1, treeAt: -1, ovSel: -1, panelH: 240, panelW: 400, termTheme: "-", badge: version.Label()}
}

// Layouter 返回按格排版的布局器（与终端前端相同）。
func (f *Frontend) Layouter() view.LineLayouter { return cell.Layouter{} }

// Profile 返回键位环境：macOS 上 mod 就是 Cmd，快捷键提示用 macOS 的符号；
// Windows、Linux 上 mod 是 Ctrl，键位换成 VS Code 在那里的（见 core/app/platform.go）。
func (f *Frontend) Profile() keymap.Profile {
	return guiProfile(hostOS)
}

// hostOS is the system the window runs on, for its keys and its names for the file manager and
// the trash; tests set it to check another platform's labels.
var hostOS = runtime.GOOS

func guiProfile(goos string) keymap.Profile {
	p := keymap.Profile{Mods: []keymap.Mod{keymap.ModSuper}, GUI: true, OS: goos}
	if !p.MacKeys() {
		p.Mods = []keymap.Mod{keymap.ModCtrl}
	}
	return p
}

// Host 返回平台能力的实现。
func (f *Frontend) Host() app.Host { return host{f} }

// windows 是开着的窗口和它们的前端（只在主线程上用）：菜单、Dock 菜单按它找到该交给哪个窗口。
var windows = map[*mygo.Window]*Frontend{}

// makeWindow 为「新建窗口」造一个新的编辑器和前端（cmd/yan-app 给出）；nil 表示不能新建窗口。
var makeWindow func() (*app.Editor, *Frontend)

// SetWindowMaker 让「新建窗口」（⇧⌘N、Dock 菜单）能用：fn 像启动时那样造一个编辑器和前端，只是什么都不打开。
// 在 Run 之前调用。
func SetWindowMaker(fn func() (*app.Editor, *Frontend)) { makeWindow = fn }

// PrimaryInstance hands secondary launches to the running GUI before they load or rotate user state.
func PrimaryInstance() bool {
	mygo.App.SetName("yan")
	return mygo.App.RequestSingleInstanceLock()
}

// Run opens the first window and owns the native event loop; call it on the main goroutine.
func (f *Frontend) Run(ed *app.Editor, extra ...launch.Target) error {
	mygo.App.SetName("yan")
	updateChecks.Store(ed.UpdateChecks())
	setupUpdates()
	// Native appearance is app-wide; editor preferences and previews are window-local.
	// Keep the desktop signal intact and apply explicit palettes in each window.
	mygo.Theme.SetSource(mygo.ThemeSystem)
	mygo.App.OnSecondInstance(func(args []string, wd string) {
		current := f
		if g := focusedGroup(); g != nil {
			current = g.cur()
		}
		current.do(func() { current.openRequests(args, wd) })
	})
	mygo.App.OnOpenFile(func(path string) {
		current := f
		if g := focusedGroup(); g != nil {
			current = g.cur()
		}
		current.do(func() {
			if err := current.openTarget(launch.ResolvePath(path), true); err != nil {
				current.ed.Notify(err.Error())
			}
		})
	})
	// 窗口都关了（终端都挂断了）再退出：等终端里留下的程序结束，不然进程一退出就没人收拾它们了。
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		// A development relaunch can exit inside MyGo before main's defer runs.
		if err := process.StopLoginShellPath(); err != nil {
			fmt.Fprintln(os.Stderr, "yan-app:", err)
		}
		termsession.Wait(3 * time.Second)
	})
	mygo.App.WhenReady(func() {
		g := f.open(ed, 0, 0)
		g.restoreProjects()
		g.restoreDrafts()
		f.refreshDock()
		for _, t := range extra {
			if err := f.openTarget(t, true); err != nil {
				f.ed.Notify(err.Error())
			}
		}
	})
	return mygo.App.Run()
}

// spawn 在主线程上、当前这一轮输入处理完之后再开一个窗口（交给有焦点的窗口去排队；f 是后备）。
func (f *Frontend) spawn() {
	g := f
	if o := windows[mygo.FocusedWindow()]; o != nil {
		g = o
	} else {
		for _, o := range windows {
			g = o
			break
		}
	}
	g.do(newWindow)
}

// newWindow 再开一个空窗口（欢迎页），错开一点放在有焦点的窗口右下方；用项目标签时在前面那个窗口里加一个项目。
// 在主线程上调用。
func newWindow() {
	if makeWindow == nil {
		return
	}
	if g := focusedGroup(); g != nil && g.cur().ed.ProjectTabs() {
		g.add("")
		return
	}
	ed, f := makeWindow()
	x, y := 0, 0
	if w := mygo.FocusedWindow(); w != nil && windows[w] != nil {
		x, y = w.Position()
		x, y = x+28, y+28
	}
	f.open(ed, x, y)
}

// open 为编辑器 ed 开一个窗口（x、y 都是 0 时放在屏幕中间）。在主线程上调用。
func (f *Frontend) open(ed *app.Editor, x, y int) *group { return f.openWindow(ed, x, y) }

// openWindow 开一个原生窗口，里面先只有项目 f（编辑器 ed）。
func (f *Frontend) openWindow(ed *app.Editor, x, y int) *group {
	g := &group{}
	g.win = mygo.NewWindow(mygo.WindowOptions{
		Title: "yan", Width: 1200, Height: 780, MinWidth: 480, MinHeight: 320, X: x, Y: y,
		TitleBarStyle: mygo.TitleBarHidden,
		Content:       ui.View(g.view),
	})
	g.attach(f, ed)
	g.projects = []*Frontend{f}
	groups[g.win], windows[g.win] = g, f
	g.win.OnClose(func(e *mygo.CloseEvent) {
		if g.moving {
			return
		}
		g.saveProjects()
		if !g.mayClose() {
			e.PreventDefault()
		}
	})
	// 回到窗口时看看文件在外面有没有被改过；离开窗口时自动保存（和终端版的焦点事件一样）。
	// 菜单栏是整个应用的：换到哪个窗口，就换成那个窗口里现在这个项目的（打开最近、勾选的设置跟着它）。
	g.win.OnFocus(func() {
		setAppMenu(g.cur().menu())
		g.cur().refreshDock()
		g.cur().focusChanged(true)
	})
	g.win.OnBlur(func() { g.cur().focusChanged(false) })
	g.win.OnClosed(func() {
		delete(windows, g.win)
		delete(groups, g.win)
		for _, pf := range g.projects { // 挪走的项目已经不在这里
			pf.shutdown()
		}
		if len(windows) == 0 {
			mygo.App.Quit()
		}
	})
	setAppMenu(f.menu())
	return g
}

// do 在主线程上执行 fn（菜单、对话框的回调用它），然后重画。
func (f *Frontend) do(fn func()) {
	if w := f.winp.Load(); w != nil { // 项目可能挪到了别的窗口
		w.Update(fn)
		return
	}
	if f.ed != nil { // 没有窗口（测试）：经内核的 Post 回到所有者
		f.ed.Post(func(*app.Editor) { fn() })
	}
}

// focusChanged 把窗口获得、失去焦点交给内核（在所有者线程上）。
func (f *Frontend) focusChanged(gained bool) {
	f.do(func() {
		f.blurred = !gained
		if !gained {
			f.mascot = mascotState{}
			f.rain.moving = false
		}
		f.ed.Handle(app.FocusInput{Gained: gained})
	})
}

var closeDialog = mygo.Dialog.Message

// mayClose 决定窗口能不能关：开着自动保存的先存；未命名的文件留到下次（留不住就也算未保存）；
// 还有没保存的才问。
func (g *group) mayClose() bool {
	dirty := false
	var lost error // 没能留到下次的未命名文件：它们也要问
	for _, pf := range g.projects {
		if pf.ed.AutoSave() {
			pf.ed.Run("workbench.action.files.saveAll")
		}
		d, kept := pf.ed.Unsaved()
		dirty = dirty || d
		lost = cmp.Or(lost, kept)
	}
	return !dirty || g.confirmClose(lost)
}

// MyGo's modal wait pumps native events. Returning the decision within OnClose
// preserves the pending relaunch instead of cancelling it before the user answers.
// lost 不为 nil 时未命名的文件没能留到下次：它们也算未保存，「全部保存」存不了它们。
func (g *group) confirmClose(lost error) bool {
	if g.confirming {
		return false
	}
	g.confirming = true
	defer func() { g.confirming = false }()
	detail := "不保存的话，修改会丢失。"
	if lost != nil {
		detail = "未命名的文件没能留到下次（" + lost.Error() + "）：先另存为它们，或者选「不保存」放弃。" + detail
	}
	res, err := closeDialog(mygo.MessageOptions{
		Parent: g.win, Type: mygo.MessageWarning,
		Message: "有文件还没保存，要先保存吗？",
		Detail:  detail,
		Buttons: []string{"全部保存", "不保存", "取消"}, DefaultButton: 0, CancelButton: 2,
	})
	if err != nil || res.Button == 2 {
		return false
	}
	for _, pf := range g.projects {
		if res.Button == 0 {
			pf.ed.Run("workbench.action.files.saveAll")
			if dirty, _ := pf.ed.Unsaved(); dirty {
				return false
			}
		}
		pf.ed.SaveSession()
	}
	return true
}

// host 用 MyGo 实现 app.Host。
type host struct{ f *Frontend }

// 剪贴板经视图的 Services（照着 Mayoi）：窗口里和不开窗口的测试里都能用。
func (h host) SetClipboard(t string) error {
	if h.f.hasServices {
		h.f.services.WriteClipboard(t)
		return nil
	}
	mygo.Clipboard.WriteText(t)
	return nil
}

func (h host) Clipboard() (string, error) {
	if h.f.hasServices {
		return h.f.services.ReadClipboard(), nil
	}
	return mygo.Clipboard.ReadText(), nil
}
func (h host) Trash(path string) error { return mygo.Shell.TrashItem(path) }

// CopyFiles 把文件放进系统剪贴板（访达、别的窗口都能粘贴）；不开窗口的测试里记在前端。
func (h host) CopyFiles(paths []string) error {
	if h.f.win == nil {
		h.f.copiedFiles = slices.Clone(paths)
		return nil
	}
	return mygo.Clipboard.WriteFiles(paths...)
}

// ClipboardFiles 是剪贴板里的文件（访达里复制的也算）。
func (h host) ClipboardFiles() []string {
	if h.f.win == nil {
		return h.f.copiedFiles
	}
	files, _ := mygo.Clipboard.ReadFiles()
	return files
}

// Shell 是 ⌃`：打开、收起编辑区下面的终端面板。
func (h host) Shell(dir string) error {
	h.f.toggleTerminal(dir)
	return nil
}

// EmbeddedTerminal 告诉内核终端在窗口里（不会让出界面，回来后不用重读磁盘）。
func (h host) EmbeddedTerminal() bool { return true }

// NewTerminal 在面板里新开一个终端（⌃⇧`）。
func (h host) NewTerminal(dir string) { h.f.newTerminal(dir) }

// SendTerminal writes text, as keybindings.json's sendSequence and type give it, to the terminal
// that last had the keyboard in this folder. It reports false when no terminal is open.
func (h host) SendTerminal(text string) bool {
	s := h.f.reviewTerminal()
	if s == nil || text == "" {
		return false
	}
	s.t.Send([]byte(text))
	return true
}

// MovePanelTerminal moves the panel's active terminal into column col at at, keeping its shell.
func (h host) MovePanelTerminal(col, at int) bool {
	f := h.f
	if f.termActive < 0 || f.termActive >= len(f.terms) {
		return false
	}
	f.panelToColumn(f.terms[f.termActive], col, at)
	return true
}

// TerminalFocused reports whether the panel's terminal has the keyboard; a terminal tab's
// focus the core already knows from the active tab.
func (h host) TerminalFocused() bool { return h.f.termFocused }

// NewWindow 再开一个窗口（⇧⌘N）。
func (h host) NewWindow() { h.f.spawn() }

// Reveal 在访达里显示文件。
func (h host) Reveal(path string) { mygo.Shell.ShowItemInFolder(path) }

// OpenFolderDialog 用系统的对话框选一个文件夹，在这个窗口里打开（⌘O）。
func (h host) OpenFolderDialog()            { h.chooseFolder(false) }
func (h host) OpenFolderNewWindow()         { h.chooseFolder(true) }
func (h host) OpenFolder(path string) error { return h.f.openFolder(path, false) }
func (h host) chooseFolder(newWindow bool) {
	f := h.f
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: f.win, Title: "打开文件夹", ButtonLabel: "打开", Directory: true, CreateDirectories: true,
		})
		if err == nil && len(paths) > 0 {
			f.do(func() {
				if err := f.openFolder(paths[0], newWindow); err != nil {
					f.ed.Notify(err.Error())
				}
			})
		}
	}()
}

// CompareFilesDialog chooses the two files of a comparison with the system's dialog: two at once,
// or one and then the other.
func (h host) CompareFilesDialog(dir string) {
	f := h.f
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: f.win, Title: "比较两个文件", Message: "选两个文件（或先选左边的一个）", ButtonLabel: "比较", DefaultPath: dir, Multiple: true,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		if len(paths) == 1 {
			more, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
				Parent: f.win, Title: "比较两个文件", Message: "和 " + filepath.Base(paths[0]) + " 比较的文件", ButtonLabel: "比较", DefaultPath: filepath.Dir(paths[0]),
			})
			if err != nil || len(more) == 0 {
				return
			}
			paths = append(paths, more[0])
		}
		f.do(func() {
			if err := f.ed.CompareFiles(paths[0], paths[1]); err != nil {
				f.ed.Notify(err.Error())
			}
		})
	}()
}

// NewFileDialog 用保存对话框选新文件的位置，建好后打开（「新建文件…」）。
func (h host) NewFileDialog(dir string) {
	f := h.f
	go func() {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent: f.win, Title: "新建文件", DefaultPath: filepath.Join(dir, "未命名.txt"), CreateDirectories: true,
		})
		if err == nil && path != "" {
			f.do(func() {
				if err := f.ed.NewFile(path); err != nil {
					f.ed.Notify(err.Error())
				}
			})
		}
	}()
}

// saveAsDialog 是「另存为」的系统保存对话框（测试里换掉）。它自己问要不要替换已有的文件。
var saveAsDialog = mygo.Dialog.Save

func (h host) SaveAsDialog(path string, done func(string, error)) {
	go func() {
		path, err := saveAsDialog(mygo.SaveDialogOptions{Parent: h.f.win, Title: "另存为", DefaultPath: path, CreateDirectories: true})
		done(path, err)
	}()
}

// paletteFor 返回窗口当前该用的配色。
func (f *Frontend) paletteFor(dark bool) *palette {
	switch f.opts.Appearance {
	case "dark":
		dark = true
	case "light":
		dark = false
	}
	p := style.ThemeFor(f.opts.Theme, dark)
	if f.paletteTarget == nil || f.paletteTarget.src != p {
		f.paletteTarget = newPalette(p)
	}
	return f.paletteTarget
}

// about 显示版本（「关于砚」）。
func (f *Frontend) about() {
	mygo.Dialog.Message(mygo.MessageOptions{
		Parent: f.win, Type: mygo.MessageInfo,
		Message: "砚 " + version.String(),
		Detail:  "macOS 预览版。第三方代码的版权与许可见「帮助 › 第三方声明」。",
		Buttons: []string{"好"},
	})
}

// showNotices 用系统默认的程序打开随应用附带的第三方声明。
func (f *Frontend) showNotices() {
	dir, err := mygo.App.Path(mygo.PathResources)
	p := filepath.Join(dir, "THIRD_PARTY_LICENSES.txt")
	if err == nil {
		_, err = os.Stat(p)
	}
	if err == nil {
		err = mygo.Shell.OpenPath(p)
	}
	if err != nil {
		f.do(func() { f.ed.Notify("找不到第三方声明：开发构建里没有，打包的 yan.app 里有") })
	}
}
