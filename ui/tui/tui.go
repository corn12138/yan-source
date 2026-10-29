// Package tui 是 v0.1 的终端前端：tcell 屏幕生命周期、事件循环、事件翻译与绘制。
// 它只做三件事：把 tcell 事件翻译成 app.Input；提供单元格 LineLayouter；把 view.Frame 画出来。
package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v3"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/view"
	"github.com/corn12138/yan-source/core/view/cell"
	"github.com/corn12138/yan-source/platform/clipboard"
	"github.com/corn12138/yan-source/platform/trash"
)

// Frontend 是终端前端。
type Frontend struct {
	// SidebarWidth 是资源管理器的宽度（列）。
	SidebarWidth int

	screen      tcell.Screen
	pal         *palette
	frame       view.Frame
	tabWidth    int
	title       string
	lay         layout
	tabHits     []tabHit
	sideLines   []sideLine // 侧边栏每一行是什么
	settingsGen uint64
	blockCaret  bool
	press       press // 左键按下时落在哪里：拖动跟着它走
	lastTree    struct {
		at  time.Time
		row int
	}
	lastTab struct {
		at       time.Time
		col, idx int
	}

	text     strings.Builder // 连续的普通字符，合并后一次插入
	pasting  bool
	pasteBuf strings.Builder
}

// New 创建终端前端（还没有接管终端）。
func New(tabWidth int) *Frontend {
	return &Frontend{pal: newPalette(style.ThemeFor("", true)), tabWidth: tabWidth, SidebarWidth: 30}
}

// SetTheme 换主题：name 是 ""（Mayoi）、"matrix" 或 "claude"，dark 选深色或浅色。
func (f *Frontend) SetTheme(name string, dark bool) { f.pal = newPalette(style.ThemeFor(name, dark)) }

// Layouter 返回单元格布局器。
func (f *Frontend) Layouter() view.LineLayouter { return cell.Layouter{} }

// Host 返回平台能力的实现。
func (f *Frontend) Host() app.Host { return host{f} }

// Profile 返回键位环境：始终同时编译 Ctrl 与 Cmd 两套 mod 绑定。
// Terminal.app 永远收不到 Cmd，所以多出来的那套无害；Ghostty 走 kitty 协议时 Cmd 直接可用，
// 这样就不必等终端异步回报键盘协议。
func (f *Frontend) Profile() keymap.Profile {
	return keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl, keymap.ModSuper}}
}

// Run 接管终端并运行事件循环，直到编辑器请求退出。它就是所有者 goroutine。
func (f *Frontend) Run(ctx context.Context, ed *app.Editor) (err error) {
	s := f.screen // 测试会预先放一个接在虚拟终端上的 screen
	if s == nil {
		if s, err = tcell.NewScreen(); err != nil {
			return err
		}
	}
	if err := s.Init(); err != nil {
		return err
	}
	f.screen = s
	defer func() {
		// 无论如何先把终端还给用户，否则 panic 信息会被吞掉、终端停在 raw 模式。
		r := recover()
		s.Fini()
		if r != nil {
			panic(r)
		}
	}()
	s.SetStyle(f.pal.theme[style.Text])
	s.EnableMouse(tcell.MouseButtonEvents, tcell.MouseDragEvents)
	s.EnablePaste()
	s.EnableFocus() // 切回终端窗口时检查磁盘上的修改
	s.SetCursorStyle(tcell.CursorStyleBlinkingBar)

	events := s.EventQ()
	for !ed.Quit() {
		f.draw(ed)
		select {
		case <-ctx.Done():
			return nil
		case <-ed.Wake():
			ed.RunPending()
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			f.dispatch(ed, ev)
			// 把已经到达的事件一次处理完再画：输入法一次提交的多个字合并成一次插入。
		drain:
			for range 512 {
				select {
				case ev, ok := <-events:
					if !ok {
						return nil
					}
					f.dispatch(ed, ev)
				default:
					break drain
				}
			}
			f.flushText(ed)
		}
	}
	return nil
}

// flushText 把累积的普通字符作为一次 TextInput 交给内核。
func (f *Frontend) flushText(ed *app.Editor) {
	if f.text.Len() > 0 {
		ed.Handle(app.TextInput{Text: f.text.String()})
		f.text.Reset()
	}
}

func (f *Frontend) dispatch(ed *app.Editor, ev tcell.Event) {
	switch ev := ev.(type) {
	case *tcell.EventResize:
		f.screen.Sync()
	case *tcell.EventPaste:
		if ev.Start() {
			f.flushText(ed)
			f.pasting = true
			f.pasteBuf.Reset()
		} else if ev.End() {
			f.pasting = false
			ed.Handle(app.TextInput{Text: f.pasteBuf.String(), Paste: true})
		}
	case *tcell.EventKey:
		if !ev.Pressed() { // kitty 协议会报告松键，忽略
			return
		}
		if f.pasting {
			f.pasteBuf.WriteString(pasteText(ev))
			return
		}
		if txt, ok := plainText(ev); ok && !ed.Pending() {
			f.text.WriteString(txt)
			return
		}
		f.flushText(ed)
		if c, ok := translateKey(ev); ok {
			ed.Handle(app.KeyInput{Chord: c})
		}
	case *tcell.EventMouse:
		f.flushText(ed)
		f.mouse(ed, ev)
	case *tcell.EventFocus:
		f.flushText(ed)
		ed.Handle(app.FocusInput{Gained: ev.Focused})
	}
}

func pasteText(ev *tcell.EventKey) string {
	switch ev.Key() {
	case tcell.KeyRune:
		return ev.Str()
	case tcell.KeyEnter, tcell.KeyLF:
		return "\n"
	case tcell.KeyTab:
		return "\t"
	}
	return ""
}

// plainText 报告这个按键是不是「打字」：可打印字符，没有 Ctrl/Alt/Cmd。
func plainText(ev *tcell.EventKey) (string, bool) {
	if ev.Key() != tcell.KeyRune || ev.Str() == "" {
		return "", false
	}
	if ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt|tcell.ModMeta) != 0 {
		return "", false
	}
	return ev.Str(), true
}

var namedKeys = map[tcell.Key]keymap.Key{
	tcell.KeyEnter: keymap.KeyEnter, tcell.KeyEsc: keymap.KeyEsc, tcell.KeyTab: keymap.KeyTab,
	tcell.KeyBackspace: keymap.KeyBackspace, tcell.KeyDelete: keymap.KeyDelete,
	tcell.KeyUp: keymap.KeyUp, tcell.KeyDown: keymap.KeyDown, tcell.KeyLeft: keymap.KeyLeft, tcell.KeyRight: keymap.KeyRight,
	tcell.KeyHome: keymap.KeyHome, tcell.KeyEnd: keymap.KeyEnd, tcell.KeyPgUp: keymap.KeyPgUp, tcell.KeyPgDn: keymap.KeyPgDn,
	tcell.KeyF1: keymap.KeyF1, tcell.KeyF2: keymap.KeyF2, tcell.KeyF3: keymap.KeyF3, tcell.KeyF4: keymap.KeyF4,
	tcell.KeyF5: keymap.KeyF5, tcell.KeyF6: keymap.KeyF6, tcell.KeyF7: keymap.KeyF7, tcell.KeyF8: keymap.KeyF8,
	tcell.KeyF9: keymap.KeyF9, tcell.KeyF10: keymap.KeyF10, tcell.KeyF11: keymap.KeyF11, tcell.KeyF12: keymap.KeyF12,
}

// translateKey 把 tcell 按键规范化成 keymap.Chord。要同时兼容两种编码：
//   - legacy（Terminal.app）：Ctrl+字母报告为 KeyCtrlA…KeyCtrlZ，Ctrl+/ 是 0x1F，Ctrl+` 是 NUL；
//   - kitty（Ghostty）：Ctrl+字母报告为 KeyRune + ModCtrl，Cmd 报告为 ModMeta。
func translateKey(ev *tcell.EventKey) (keymap.Chord, bool) {
	m := ev.Modifiers()
	var mods keymap.Mod
	if m&tcell.ModShift != 0 {
		mods |= keymap.ModShift
	}
	if m&tcell.ModCtrl != 0 {
		mods |= keymap.ModCtrl
	}
	if m&tcell.ModAlt != 0 {
		mods |= keymap.ModAlt
	}
	if m&tcell.ModMeta != 0 {
		mods |= keymap.ModSuper
	}
	k := ev.Key()
	switch {
	case k == tcell.KeyRune:
		r, _ := utf8.DecodeRuneInString(ev.Str())
		if r == utf8.RuneError {
			return keymap.Chord{}, false
		}
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		return keymap.Chord{Mods: mods, Key: keymap.KeyRune, Rune: r}, true
	case k >= tcell.KeyCtrlA && k <= tcell.KeyCtrlZ:
		return keymap.Chord{Mods: mods | keymap.ModCtrl, Key: keymap.KeyRune, Rune: rune('a' + k - tcell.KeyCtrlA)}, true
	case k == tcell.KeyNUL: // Ctrl+` / Ctrl+Space / Ctrl+@ 在 legacy 编码里都是 NUL
		return keymap.Chord{Mods: keymap.ModCtrl, Key: keymap.KeyRune, Rune: '`'}, true
	case k == tcell.KeyUS: // Ctrl+/ 和 Ctrl+_
		return keymap.Chord{Mods: keymap.ModCtrl, Key: keymap.KeyRune, Rune: '/'}, true
	case k == tcell.KeyBacktab:
		return keymap.Chord{Mods: mods | keymap.ModShift, Key: keymap.KeyTab}, true
	}
	if nk, ok := namedKeys[k]; ok {
		return keymap.Chord{Mods: mods, Key: nk}, true
	}
	return keymap.Chord{}, false
}

// press 记下左键按下的位置：代码窗格里的拖动选择要一直发给按下时的那一列。
type press struct {
	down bool
	col  int // 按在第几列的代码窗格；-1 表示按在别处
}

const doubleClick = 400 * time.Millisecond

// mouse 做命中测试：外壳（标签、资源管理器、浮层）上的点击翻译成语义动作，
// 代码窗格里的点击和拖动换算成窗格内坐标交给内核。
func (f *Frontend) mouse(ed *app.Editor, ev *tcell.EventMouse) {
	x, y := ev.Position()
	b := ev.Buttons()
	var mods keymap.Mod
	if ev.Modifiers()&tcell.ModShift != 0 {
		mods |= keymap.ModShift
	}
	ux, uy := view.Unit(x), view.Unit(y)
	switch {
	case b&(tcell.WheelUp|tcell.WheelDown|tcell.WheelLeft|tcell.WheelRight) != 0:
		in := app.ScrollInput{Column: f.columnAt(ux, uy), Sidebar: f.lay.side.Contains(ux, uy)}
		switch {
		case b&tcell.WheelUp != 0:
			in.DY = -3
		case b&tcell.WheelDown != 0:
			in.DY = 3
		case b&tcell.WheelLeft != 0:
			in.DX = -4
		default:
			in.DX = 4
		}
		ed.Handle(in)
	case b&tcell.Button1 != 0:
		if f.press.down { // 拖动
			if f.press.col >= 0 {
				r := f.lay.cols[f.press.col].code
				ed.Handle(app.MouseInput{Column: f.press.col, X: ux - r.X, Y: uy - r.Y, Mods: mods, Kind: app.MouseDrag})
			}
			return
		}
		f.press = press{down: true, col: -1}
		f.click(ed, ux, uy, mods)
	case f.press.down:
		if f.press.col >= 0 {
			ed.Handle(app.MouseInput{Column: f.press.col, Kind: app.MouseRelease})
		}
		f.press = press{}
	}
}

func (f *Frontend) click(ed *app.Editor, x, y view.Unit, mods keymap.Mod) {
	now := time.Now()
	if f.frame.Overlay.Visible {
		b := f.lay.overlay
		if !b.Contains(x, y) {
			ed.CloseOverlay()
			return
		}
		if rows := overlayRows(&f.frame.Overlay); y > b.Y+1 && int(y-b.Y)-2 < len(rows) { // 边框 + 输入行
			if i := rows[int(y-b.Y)-2].item; i >= 0 {
				ed.ChooseOverlay(i)
			}
		}
		return
	}
	for _, h := range f.tabHits {
		if int(y) == h.y && int(x) >= h.x0 && int(x) < h.x1 {
			double := now.Sub(f.lastTab.at) < doubleClick && f.lastTab.col == h.col && f.lastTab.idx == h.idx
			f.lastTab.at, f.lastTab.col, f.lastTab.idx = now, h.col, h.idx
			ed.ClickTab(h.col, h.idx, double)
			return
		}
	}
	if s := f.lay.side; s.Contains(x, y) {
		line := sideLine{}
		if row := int(y - s.Y); row < len(f.sideLines) {
			line = f.sideLines[row]
		}
		if line.kind == sideNone {
			ed.FocusSidebar()
			return
		}
		i := line.index
		double := now.Sub(f.lastTree.at) < doubleClick && f.lastTree.row == i
		f.lastTree.at, f.lastTree.row = now, i
		switch line.kind {
		case sideTree:
			ed.ClickTree(i, double)
		case sideSearchRow:
			ed.SearchClick(i, double)
		case sideLesson:
			ed.StartLesson(i)
		case sideRepository:
			ed.ToggleRepository(line.repositoryID)
		case sideRepositoryMessage:
			ed.WithRepository(line.repositoryID, func(ed *app.Editor) { ed.FocusCommitMessage(true) })
		case sideSCMRow:
			if line.repositoryID != 0 {
				ed.WithRepository(line.repositoryID, func(ed *app.Editor) { ed.SCMClick(i, double) })
				break
			}
			if repositories := f.frame.Sidebar.SCM.Repositories; i < len(repositories) {
				ed.SCMOpenRepository(repositories[i].Path)
			} else {
				ed.SCMClick(i, double)
			}
		}
		return
	}
	for i, c := range f.lay.cols {
		if c.code.Contains(x, y) {
			f.press.col = i
			ed.Handle(app.MouseInput{Column: i, X: x - c.code.X, Y: y - c.code.Y, Mods: mods, Kind: app.MousePress})
			return
		}
	}
}

// columnAt 返回 (x, y) 所在的列；不在任何列里时返回当前列。
func (f *Frontend) columnAt(x, y view.Unit) int {
	for i, c := range f.lay.cols {
		if x >= c.tabs.X && x < c.tabs.X+c.tabs.W {
			return i
		}
	}
	return f.frame.Active
}

// host 用 platform/* 实现 app.Host。
type host struct{ f *Frontend }

func (h host) SetClipboard(t string) error {
	if err := clipboard.Set(t); err != nil {
		h.f.screen.SetClipboard([]byte(t)) // SSH 等没有 pbcopy 的环境：退回 OSC 52
	}
	return nil
}

func (h host) Clipboard() (string, error) { return clipboard.Get() }

func (h host) Trash(path string) error { return trash.Move(path) }

// Shell 暂停编辑器、把终端交给用户的 shell，shell 退出后恢复。编辑器状态全程留在内存里。
func (h host) Shell(dir string) error {
	s := h.f.screen
	if err := s.Suspend(); err != nil {
		return err
	}
	defer func() {
		s.Resume()
		s.Sync()
	}()
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/zsh"
	}
	fmt.Printf(i18n.T("\x1b[2J\x1b[H\x1b[36m── yan terminal · %s ──\x1b[0m\nType exit or press Ctrl+D to return to the editor.\n\n"), dir)
	cmd := exec.Command(sh, "-i")
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), "YAN_SHELL=1")
	_ = cmd.Run() // shell 的退出码不是错误
	return nil
}
