package gui

import (
	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/view"
)

// scmRepositoryPanel is the repository panel under Source Control's history: a header that
// opens and closes it, tabs for Branches and Remotes, and their rows, each read only when
// shown (after Mayoi's repository panel).
func (f *Frontend) scmRepositoryPanel(c *ui.Context, s *view.SCM) {
	ed := repositoryEditor{Editor: f.ed, id: s.RepositoryID}
	p := f.pal
	ui.Row(c.Key("group:repository")).Gap(4).AlignItems(ui.Center).Height(22).Padding(6, 4, 0, 4).Children(func() {
		toggle := ui.Row(c.Key("repository-toggle")).Gap(4).AlignItems(ui.Center).Grow(1).Label("仓库面板").Cursor(ui.CursorPointer)
		toggle.Children(func() {
			icon := iconChevronRight
			if s.Panel {
				icon = iconChevronDown
			}
			ui.Icon(c, icon).FontSize(12).TextColor(p.muted)
			ui.Text(c, "仓库").FontSize(11).FontWeight(600).LetterSpacing(0.4).TextColor(p.muted).SingleLine().Shrink(1)
		})
		if toggle.Clicked() {
			ed.Run("yan.scm.toggleRepositoryPanel")
		}
		for _, tab := range []struct {
			label   string
			remotes bool
		}{{"分支", false}, {"远程", true}} {
			on := s.Panel && s.PanelRemotes == tab.remotes
			b := ui.Box(c.Key("panel-tab-"+tab.label)).Padding(0, 6).Height(18).Radius(4).AlignItems(ui.Center).Label(tab.label)
			col := p.muted
			if on {
				col = p.accent
				b.Background(p.accent.Alpha(0.12))
			} else if b.Hovered() {
				b.Background(p.hover)
				col = p.text
			}
			b.Children(func() { ui.Text(c, tab.label).FontSize(11).TextColor(col) })
			if b.Clicked() {
				ed.act(func(e *app.Editor) { e.SCMPanelTab(tab.remotes) })
			}
		}
	})
	for _, r := range s.PanelRows {
		f.scmPanelRow(c, ed, r)
	}
}

func (f *Frontend) scmPanelRow(c *ui.Context, ed repositoryEditor, r view.SCMPanelRow) {
	p := f.pal
	row := ui.Row(c.Key("panel:"+r.Key)).Gap(5).AlignItems(ui.Center).Height(22).Padding(0, 4, 0, 4+float32(r.Depth)*12).Radius(4).Label(r.Label)
	if r.Detail != "" {
		row.Tooltip(r.Label + "\n" + r.Detail)
	}
	if row.Hovered() && r.Kind != "status" && r.Kind != "url" {
		row.Background(p.hover)
	}
	row.Children(func() {
		switch r.Kind {
		case "branch", "remote", "range":
			icon := iconChevronRight
			if r.Open {
				icon = iconChevronDown
			}
			ui.Icon(c, icon).FontSize(11).TextColor(p.muted)
		}
		if r.Kind == "branch" {
			ui.Icon(c, iconBranch).FontSize(11).TextColor(p.muted)
		}
		col := p.text
		switch r.Kind {
		case "status", "more", "retry", "retryLog", "addRemote":
			col = p.muted
		case "url":
			col = p.faint
		}
		label := ui.Text(c, r.Label).FontSize(12).TextColor(col).SingleLine().Shrink(1)
		if r.Current {
			label.FontWeight(600)
		}
		if r.Detail != "" && r.Kind != "retry" && r.Kind != "retryLog" {
			ui.Text(c, r.Detail).FontSize(11).TextColor(p.faint).SingleLine().Shrink(2)
		}
	})
	if row.Clicked() {
		ed.act(func(e *app.Editor) { e.SCMPanelActivate(r) })
	}
	switch r.Kind {
	case "branch":
		row.ContextMenu(func(m *ui.Menu) {
			if m.Item("检出").Disabled(r.Current).Chosen() {
				ed.act(func(e *app.Editor) { e.SCMPanelBranch(r.Label, r.Remote, "checkout") })
			}
			if m.Item("比较 HEAD → 这个分支").Chosen() {
				ed.act(func(e *app.Editor) { e.SCMPanelBranch(r.Label, r.Remote, "compare") })
			}
			if !r.Remote {
				m.Separator()
				if m.Item("删除分支…").Disabled(r.Current).Chosen() {
					ed.act(func(e *app.Editor) { e.SCMPanelBranch(r.Label, false, "delete") })
				}
			}
		})
	case "remote":
		row.ContextMenu(func(m *ui.Menu) {
			if m.Item("修改地址…").Chosen() {
				ed.act(func(e *app.Editor) { e.SCMPanelRemote(r.Label, "url") })
			}
			if m.Item("清理已删除的分支").Chosen() {
				ed.act(func(e *app.Editor) { e.SCMPanelRemote(r.Label, "prune") })
			}
			m.Separator()
			if m.Item("删除远程仓库…").Chosen() {
				ed.act(func(e *app.Editor) { e.SCMPanelRemote(r.Label, "remove") })
			}
		})
	}
}
