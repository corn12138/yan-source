package gui

import (
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/view"
)

// 编辑区里的终端标签（照着 Mayoi）：内核记着标签和它的号，终端在这里。第一次画这个标签时在它的文件夹里开 shell；
// 标签没了（关掉、项目关了）就结束它。终端能在面板和编辑区之间挪：拖面板上的终端到标签条上，
// 拖终端标签到面板的标题栏上，或者用右键菜单。

func (f *Frontend) tabTerm(id int, dir string) *termSession {
	if s := f.tabTerms[id]; s != nil {
		return s
	}
	s := f.startTerminal(dir)
	s.id = id
	if f.tabTerms == nil {
		f.tabTerms = map[int]*termSession{}
	}
	f.tabTerms[id] = s
	return s
}

func (f *Frontend) termPane(c *ui.Context, pane ui.Element, ci int) {
	pf := &f.frame.Columns[ci].Pane
	s := f.tabTerm(pf.Terminal, pf.Path)
	pane.Background(f.pal.panel)
	if s.failed {
		pane.Children(func() {
			ui.Text(c, "终端未能启动，请关闭此标签后重试").Padding(24).TextColor(f.pal.muted)
		})
		return
	}
	f.restyleTerminals()
	pane.Children(func() {
		tv := terminal.View(c.Key(s), s.t).Fill().Bind(&s.view)
		if f.takePaths(c, tv, s) {
			f.cols[ci].dropFrames = 2
		}
		if s.view.FocusWithin(c) {
			f.lastReviewTerm = s
		}
		if tv.Pressed() {
			f.ed.FocusColumn(ci)
			f.shownTerm = pf.Terminal
		}
	})
	if ci == f.frame.Active && f.frame.Focus == view.FocusEditor && !f.frame.Overlay.Visible && !f.focusTerm && f.shownTerm != pf.Terminal {
		s.view.Focus(c)
		f.shownTerm = pf.Terminal
	}
}

// 每帧先清理已关闭的标签，晚到的 shell 回调由 closed 标记隔离。
func (f *Frontend) reapTerminals() {
	if len(f.tabTerms) == 0 {
		return
	}
	live := map[int]bool{}
	for _, col := range f.frame.Columns {
		for _, t := range col.Tabs {
			if t.Terminal != 0 {
				live[t.Terminal] = true
			}
		}
	}
	for id, s := range f.tabTerms {
		if !live[id] {
			delete(f.tabTerms, id)
			s.close()
		}
	}
}

// panelToColumn 把面板里的终端挪进第 ci 列的第 at 个位置（shell 不重开）。
func (f *Frontend) panelToColumn(s *termSession, ci, at int) {
	for i, o := range f.terms {
		if o != s {
			continue
		}
		f.terms = append(f.terms[:i:i], f.terms[i+1:]...)
		f.termActive = max(min(f.termActive, len(f.terms)-1), 0)
		if len(f.terms) == 0 {
			f.hidePanel()
		}
		s.id = f.ed.OpenTerminalTab(ci, at, s.dir, s.title)
		if f.tabTerms == nil {
			f.tabTerms = map[int]*termSession{}
		}
		f.tabTerms[s.id] = s
		f.shownTerm, f.focusTerm, f.focusRoot = 0, false, false
		return
	}
}

// columnToPanel 把第 id 号标签里的终端挪到面板里（shell 不重开）。
func (f *Frontend) columnToPanel(id int) {
	s := f.tabTerms[id]
	if s == nil {
		for _, col := range f.frame.Columns {
			for _, t := range col.Tabs {
				if t.Terminal == id {
					s = f.tabTerm(id, t.Path)
				}
			}
		}
	}
	if s == nil || s.failed {
		return
	}
	delete(f.tabTerms, id)
	f.ed.DetachTerminal(id)
	s.id = 0
	f.terms = append(f.terms, s)
	f.termActive = len(f.terms) - 1
	f.panelOpen, f.focusTerm = true, true
}

func (f *Frontend) closeTabTerminals() {
	for id, s := range f.tabTerms {
		delete(f.tabTerms, id)
		s.close()
	}
}
