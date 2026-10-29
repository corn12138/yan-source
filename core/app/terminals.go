package app

import (
	"errors"
	"path/filepath"
	"slices"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/text"
)

// 编辑区的终端标签（照着 Mayoi）：一个标签里是一个终端，和文件的标签一样能拖、能拆分、能关。
// 内核只记着它是第几号终端、在哪个文件夹里开的、标题是什么；终端本身归窗口版（它第一次画这个窗格时开 shell，
// 标签没了就结束它）。终端版没有内嵌终端，不会有这种标签。

// 每个标签有独立 ID；标题和工作目录相同也不能合并会话。
type termTab struct {
	id    int
	dir   string
	title string
}

func (e *Editor) newTerminalTab(c *column, at int, dir, title string) int {
	e.nextTerm++
	id := e.nextTerm
	if title == "" {
		title = i18n.T("Terminal")
	}
	d := &document{buf: edit.NewBuffer(text.FromString("", text.Meta{ReadOnly: true}))}
	t := &tab{doc: d, sels: edit.Single(edit.Cursor(0)), term: &termTab{id: id, dir: dir, title: title}}
	d.tabs = append(d.tabs, t)
	at = min(max(at, 0), len(c.tabs))
	c.tabs = slices.Insert(c.tabs, at, t)
	if c.active >= at {
		c.active++
	}
	e.activate(c, at)
	e.area = areaEditor
	return id
}

// terminalDir 是新终端开在哪里：打开的文件夹，没有就是当前文件所在的文件夹。
func (e *Editor) terminalDir() string {
	dir := e.root
	if t := e.cur(); dir == "" && t != nil {
		if t.term != nil {
			return t.term.dir
		}
		dir = filepath.Dir(t.doc.buf.Path())
	}
	return dir
}

// TerminalLocation is "editor" for editor tabs, or "" for the panel.
func (e *Editor) TerminalLocation() string { return e.cfg.TerminalLocation }

func (e *Editor) toggleTerminalLocation() error {
	location := "editor"
	if e.cfg.TerminalLocation == location {
		location = ""
	}
	e.cfg.TerminalLocation = location
	if e.state != nil {
		e.state.TerminalLocation = &location
		e.saveState()
	}
	e.settingsGen++
	return nil
}

// Adapted from Mayoi d81abe4: only implicit creation uses this preference.
// Toggle Terminal and the panel's + keep their explicit panel destination;
// TUI hosts still yield to a shell instead of creating an unusable editor tab.
func (e *Editor) openTerminal(dir string) error {
	if h, ok := e.host.(interface{ EmbeddedTerminal() bool }); ok && h.EmbeddedTerminal() && e.cfg.TerminalLocation == "editor" {
		return e.newTerminalEditorIn(dir)
	}
	if h, ok := e.host.(interface{ NewTerminal(dir string) }); ok {
		h.NewTerminal(dir)
		return nil
	}
	return e.shellIn(dir)
}

func (e *Editor) openTreeTerminal() error {
	_, dir, ok := e.treeTarget()
	if !ok {
		dir = e.terminalDir()
	}
	return e.openTerminal(dir)
}

func (e *Editor) newTerminalEditor() error {
	return e.newTerminalEditorIn(e.terminalDir())
}

func (e *Editor) newTerminalEditorIn(dir string) error {
	if h, ok := e.host.(interface{ EmbeddedTerminal() bool }); !ok || !h.EmbeddedTerminal() {
		return errors.New(i18n.T("Terminals in the editor area are only in the window version"))
	}
	c := e.fileColumn()
	e.newTerminalTab(c, c.active+1, dir, "")
	return nil
}

// OpenTerminalTab 把一个终端放进第 col 列的第 at 个位置（从面板挪过来、拖过来），返回它在标签里的号；
// 前端拿这个号认出画哪个终端。
func (e *Editor) OpenTerminalTab(col, at int, dir, title string) int {
	e.sync()
	defer e.sync()
	if col < 0 || col >= len(e.cols) {
		col = e.col
	}
	return e.newTerminalTab(e.cols[col], at, dir, title)
}

func (e *Editor) findTerminal(id int) (*column, int) {
	for _, c := range e.cols {
		for i, t := range c.tabs {
			if t.term != nil && t.term.id == id {
				return c, i
			}
		}
	}
	return nil, -1
}

func (e *Editor) SetTerminalTitle(id int, title string) {
	if c, i := e.findTerminal(id); c != nil && title != "" {
		c.tabs[i].term.title = title
	}
}

// TerminalExited 关掉第 id 号终端的标签（shell 退出了），不进「最近关闭」。
func (e *Editor) TerminalExited(id int) { e.DetachTerminal(id) }

// DetachTerminal 把第 id 号终端的标签拿掉，终端还开着（挪到面板去）；没有这个标签时返回 false。
func (e *Editor) DetachTerminal(id int) bool {
	c, i := e.findTerminal(id)
	if c == nil {
		return false
	}
	e.sync()
	defer e.sync()
	e.dropTab(c, i)
	return true
}

// OpenTerminalInSplit creates a slot beside col; the frontend transfers the existing shell to its ID.
func (e *Editor) OpenTerminalInSplit(col int, before bool, dir, title string) int {
	if col < 0 || col >= len(e.cols) {
		return 0
	}
	e.sync()
	defer e.sync()
	return e.newTerminalTab(e.splitColumn(col, before), 0, dir, title)
}
