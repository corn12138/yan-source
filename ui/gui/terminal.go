package gui

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/internal/termsession"
)

// 终端面板照着 Mayoi（cmd/mayoi/panel.go）：编辑区下面的面板里跑 Ghostty 的终端模拟器（libghostty-vt，
// 由 MyGo 的 terminal 插件运行时加载，不用 cgo；mygo build 会把它打进 .app，go run 时第一次用会下载到缓存）。
// 可以开几个终端；颜色跟着主题；输出里的文件路径 ⌘ 点击在编辑器里打开。

const panelHeadH = 34

type termSession struct {
	t          *terminal.Terminal
	title      string
	reviewRoot string // Project at creation; a folder switch must not redirect new reviews into this shell.
	dir        string
	view       ui.Handle
	failed     bool
	closed     bool
	id         int // 在编辑区的标签里时是那个标签的号（内核给的）；在面板里是 0
}

// termTheme 是终端的颜色：底色和面板一致。
func termTheme(p *style.Palette) *terminal.Theme {
	th := &terminal.Theme{Foreground: col(p.Text), Background: col(p.Panel), Cursor: col(p.Accent), Selection: col(p.Selection)}
	for i, c := range p.ANSI {
		th.Palette[i] = col(c)
	}
	return th
}

func (f *Frontend) termThemes() (light, dark *terminal.Theme) {
	return termTheme(style.ThemeFor(f.opts.Theme, false)), termTheme(style.ThemeFor(f.opts.Theme, true))
}

func (f *Frontend) termFont() terminal.Font {
	return terminal.Font{Family: f.opts.FontFamily, Size: f.opts.FontSize - 0.5}
}

// newTerminal 在 dir 里开一个终端，显示在面板里并把键盘交给它。
func (f *Frontend) newTerminal(dir string) {
	s := f.startTerminal(dir)
	if s.failed {
		return
	}
	f.terms = append(f.terms, s)
	f.termActive = len(f.terms) - 1
	f.panelOpen = true
	f.focusTerm = true
}

// 启动失败也保留状态，避免界面每帧重试并重复弹出错误。
func (f *Frontend) startTerminal(dir string) *termSession {
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	s := &termSession{dir: dir, reviewRoot: f.ed.Root(), title: filepath.Base(os.Getenv("SHELL"))}
	if s.title == "." || s.title == "" {
		switch runtime.GOOS {
		case "darwin":
			s.title = "zsh"
		case "windows":
			s.title = "PowerShell"
		default:
			s.title = "sh"
		}
	}
	t, err := terminal.New(f.terminalOptions(s))
	if err != nil {
		f.ed.Notify(i18n.F("Couldn't open a terminal: %s", err.Error()))
		s.failed = true
		return s
	}
	s.t = t
	return s
}

func (f *Frontend) terminalOptions(s *termSession) terminal.Options {
	light, dark := f.termThemes()
	return terminal.Options{
		Dir: s.dir, Font: f.termFont(), Theme: light, DarkTheme: dark, Cursor: terminal.CursorBar,
		// 选中就复制，「全选」也复制，同 Ghostty、iTerm2（照着 Mayoi v0.8.2，MyGo 的 CopyOnSelect）；
		// 改了配置，开着的终端也跟着改（view 里）。剪贴板里只有图片时 ⌘V 发空的括起来的粘贴，Claude Code 就去读截图。
		CopyOnSelect: f.terminalCopyOnSelect(),
		OnTitle: func(title string) {
			f.do(func() {
				if s.closed {
					return
				}
				s.title = title
				if s.id != 0 {
					f.ed.SetTerminalTitle(s.id, title)
				}
			})
		},
		// 项目不在眼前时，终端响铃或者发通知（构建、agent 跑完了）给项目的方块做个记号（照着 Mayoi v0.8.1）。
		OnBell: func() { f.do(func() { f.markAttention(i18n.F("Terminal bell: %s", s.title)) }) },
		OnNotify: func(title, body string) {
			f.do(func() { f.markAttention(i18n.F("Terminal notification: %s", strings.TrimSpace(title+" "+body))) })
		},
		OnExit: func(int) { f.do(func() { f.terminalExited(s) }) },
		// ⌃` 在终端里也是切换面板；其余 ⌘ 组合键交给菜单。
		OnKey: func(m ui.Modifiers, k ui.Key) bool {
			if m == ui.Ctrl && k == ui.KeyBackquote {
				f.do(f.hidePanel)
				return true
			}
			return false
		},
		OpenLink: func(link string) bool { return f.openTerminalFile(s, link) },
		OnPaste:  func(transfer.Data) bool { return f.pasteTerminalFiles(s) },
	}
}

// terminalExited 是 shell 退出了：拿掉它的标签，或者从面板里拿掉。
func (f *Frontend) terminalExited(s *termSession) {
	if s.closed {
		return
	}
	if s.id != 0 {
		id := s.id
		delete(f.tabTerms, id)
		s.close()
		f.ed.TerminalExited(id)
		return
	}
	f.removeTerminal(s)
}

func (f *Frontend) removeTerminal(s *termSession) {
	i := slices.Index(f.terms, s)
	if i < 0 {
		return
	}
	s.close()
	f.terms = slices.Delete(f.terms, i, i+1)
	f.termActive = max(min(f.termActive, len(f.terms)-1), 0)
	if len(f.terms) == 0 {
		f.hidePanel()
	}
}

func (f *Frontend) hidePanel() {
	f.panelOpen, f.panelMax = false, false
	f.focusRoot = true
}

// toggleTerminal 是 ⌃`：面板关着就打开（没有终端时开一个）；开着而键盘在终端里就收起；否则把键盘交给终端。
func (f *Frontend) toggleTerminal(dir string) {
	switch {
	case !f.panelOpen:
		f.panelOpen = true
		if len(f.terms) == 0 {
			f.newTerminal(dir)
		}
		f.focusTerm = true
	case f.termFocused:
		f.hidePanel()
	default:
		f.focusTerm = true
	}
}

// dragTerm 是拖着的标签里那个终端的号；拖的是文件的标签时是 0。
func (f *Frontend) dragTerm(d tabDrag) int {
	if d.f != f || d.s != nil || d.match != nil || d.col < 0 || d.col >= len(f.frame.Columns) || d.at < 0 || d.at >= len(f.frame.Columns[d.col].Tabs) {
		return 0
	}
	return f.frame.Columns[d.col].Tabs[d.at].Terminal
}

// restyleTerminals 在主题、字体变了以后让面板里和标签里的终端都跟着变。
func (f *Frontend) restyleTerminals() {
	theme := f.opts.Theme
	font := f.termFont()
	if theme == f.termTheme && font.Size == f.termFontSize && font.Family == f.termFontFamily {
		return
	}
	f.termTheme, f.termFontSize, f.termFontFamily = theme, font.Size, font.Family
	light, dark := f.termThemes()
	for _, s := range f.allTerminals() {
		if s.t == nil { // A failed tab keeps its error view until the user closes it.
			continue
		}
		s.t.SetTheme(light, dark)
		s.t.SetFont(f.termFont())
	}
}

// minPanelW is the narrowest a side panel gets.
const minPanelW = 160

// panel 是终端面板：标题栏上每个终端一个标签，新建、最大化、关闭；下面是终端。side 为 "" 时在编辑区下面，
// "left"、"right" 时在窗口那一边的最外侧（同侧有侧边栏时在它外面，照着 Mayoi），宽度单独记，分隔条在朝编辑区的一边。
// 换位置只换外框，终端会话（termSession）不变，shell 照常运行。
func (f *Frontend) panel(c *ui.Context, side string) {
	p := f.pal
	f.termFocused = false
	// 松开指针后的那一帧也保留落点，才能接到 Drop。
	dragging := f.dragTermFrames > 0
	if dragging {
		f.dragTermFrames--
	}
	if (!f.panelOpen || len(f.terms) == 0) && !dragging {
		return
	}
	f.restyleTerminals()
	box := ui.Column(c.Key("panel")).Shrink(0).BorderColor(p.border).Background(p.panel)
	ww, wh := c.Size()
	switch side {
	case "left":
		box.BorderWidth(0, 1, 0, 0)
	case "right":
		box.BorderWidth(0, 0, 0, 1)
	default:
		box.BorderWidth(1, 0, 0, 0)
	}
	// A side panel leaves the editors 320 points beside the sidebar, however narrow the window.
	room := ww - 320
	if f.frame.Sidebar.Visible {
		room -= f.opts.SidebarWidth
	}
	room = max(room, minPanelW)
	switch {
	case !f.panelOpen || len(f.terms) == 0: // only a drop target while a terminal tab is dragged
		if side == "" {
			box.Height(panelHeadH)
		} else {
			box.Width(minPanelW)
		}
	case f.panelMax:
		box.Grow(1)
	case side == "":
		box.Height(f.panelH)
	default:
		box.Width(min(max(f.panelW, minPanelW), room))
	}
	box.Children(func() {
		head := ui.Row(c).Height(panelHeadH).Padding(0, 8, 0, 12).Gap(4).AlignItems(ui.Center)
		// 编辑区里的终端标签拖到标题栏上：挪回面板
		if d, over := ui.DragOver[tabDrag](head); over && d.f == f && f.dragTerm(d) != 0 {
			head.Background(p.accent.Alpha(0.12))
		}
		if d, ok := ui.Drop[tabDrag](head); ok && d.f == f {
			if id := f.dragTerm(d); id != 0 {
				// 这一帧可能已经在编辑区画过它；下一轮再换所有者，避免绑定两次。
				f.do(func() { f.columnToPanel(id) })
			}
		}
		head.Children(func() {
			ui.Text(c, i18n.T("Terminal")).FontSize(11).FontWeight(600).LetterSpacing(0.7).TextColor(p.muted).Padding(0, 8, 0, 0)
			for i, s := range f.terms {
				on := i == f.termActive
				tab := ui.Row(c.Key(s)).Height(24).Padding(0, 8).Gap(6).Radius(5).AlignItems(ui.Center).Label(s.title).
					Drag(tabDrag{f: f, s: s}) // 拖到编辑区的标签条上：挪进那一列
				tab.ContextMenu(func(m *ui.Menu) {
					if m.Item(i18n.T("Move to Editor Area")).Chosen() {
						f.panelToColumn(s, f.frame.Active, len(f.frame.Columns[f.frame.Active].Tabs))
					}
					if m.Item(i18n.T("Kill This Terminal")).Chosen() {
						f.removeTerminal(s)
					}
				})
				if on {
					tab.Background(p.active)
				} else if tab.Hovered() {
					tab.Background(p.hover)
				}
				tab.Children(func() {
					ui.Icon(c, iconTerminal).FontSize(13).TextColor(p.muted)
					t := ui.Text(c, s.title).FontSize(12).SingleLine().MaxWidth(160)
					if on {
						t.TextColor(p.text)
					} else {
						t.TextColor(p.muted)
					}
				})
				if tab.Clicked() {
					f.termActive, f.focusTerm = i, true
				}
			}
			ui.Spacer(c)
			if f.iconButton(c, iconPlus, i18n.T("New Terminal"), false).Clicked() {
				f.newTerminal(f.ed.Root())
			}
			icon := iconMaximize
			if f.panelMax {
				icon = iconMinimize
			}
			if f.iconButton(c, icon, i18n.T("Maximize Panel"), f.panelMax).Clicked() {
				f.panelMax = !f.panelMax
			}
			if len(f.terms) > 0 && f.iconButton(c, iconTrash, i18n.T("Kill This Terminal"), false).Clicked() {
				f.removeTerminal(f.terms[f.termActive])
			}
			if f.iconButton(c, iconX, i18n.F("Hide Panel (%s)", f.ed.KeyHint("workbench.action.terminal.toggleTerminal")), false).Clicked() {
				f.hidePanel()
			}
		})
		if !f.panelOpen || len(f.terms) == 0 {
			return
		}
		s := f.terms[f.termActive]
		tv := terminal.View(c.Key(s), s.t).Fill().Bind(&s.view)
		f.takePaths(c, tv, s)
		f.termFocused = s.view.FocusWithin(c)
		if f.termFocused {
			f.lastReviewTerm = s
		}
		if f.focusTerm {
			f.focusTerm = false
			s.view.Focus(c)
		}
		// 拖面板朝编辑区的那条边改大小
		switch side {
		case "left":
			f.splitter(c, edgeRight, func(d float32) { f.panelW = min(max(f.panelW+d, minPanelW), room) })
		case "right":
			f.splitter(c, edgeLeft, func(d float32) { f.panelW = min(max(f.panelW-d, minPanelW), room) })
		default:
			f.splitter(c, edgeTop, func(dy float32) { f.panelH = max(min(f.panelH-dy, wh-200), 80) })
		}
	})
}

// takePaths 让拖到终端上的文件（访达、资源管理器里拖来的）把路径打进终端，加好 shell 的引号，
// 后面留一个空格，同 macOS 的「终端」（照着 Mayoi）；拖在上面时描上强调色的边。
func (f *Frontend) takePaths(c *ui.Context, tv ui.Element, s *termSession) bool {
	files := transfer.DropOptions{Formats: []transfer.Format{transfer.FileList}}
	_, over := ui.DataDragOver(tv, files)
	if over {
		tv.Border(2, f.pal.accent)
	}
	if drop, ok := ui.DropData(tv, files); ok {
		if paths, err := drop.Data.Files(); err == nil && len(paths) > 0 {
			s.t.Paste(quotePaths(paths))
			s.view.Focus(c)
		}
	}
	return over
}

// quotePaths 把路径加上这个系统的 shell 的引号，用空格连起来，末尾一个空格。
func quotePaths(paths []string) string {
	q := make([]string, len(paths))
	for i, p := range paths {
		q[i] = app.ShellQuote(runtime.GOOS, p)
	}
	return strings.Join(q, " ") + " "
}

// closeTerminals 在窗口关掉时结束所有终端。
func (f *Frontend) closeTerminals() {
	for _, s := range f.terms {
		s.close()
	}
	f.terms = nil
	f.closeTabTerminals()
}

// Close 产生的退出回调可能晚于新终端创建；先标记释放，回调不得再修改标签。
// MyGo 挂断 shell 之后，接住或忽略挂断的作业（在自己的进程组里）还会在后台跑，再也看不到；
// 过了同样的两秒，把 shell 那个会话里剩下的都结束掉（internal/termsession）。
func (s *termSession) close() {
	if s.closed {
		return
	}
	s.closed = true
	if s.t != nil {
		pid := shellPid(s.t)
		s.t.Close()
		termsession.End(pid, 2*time.Second)
	}
}

func (f *Frontend) allTerminals() []*termSession {
	sessions := slices.Clone(f.terms)
	for _, s := range f.tabTerms {
		if !s.failed {
			sessions = append(sessions, s)
		}
	}
	return sessions
}

func (f *Frontend) terminalCopyOnSelect() terminal.CopyOnSelect {
	if f.ed.TerminalCopyOnSelect() {
		return terminal.CopyOnSelectClipboard
	}
	return terminal.CopyOnSelectNone
}
