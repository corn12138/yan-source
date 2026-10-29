package gui

import (
	"github.com/corn12138/yan-source/core/view"
	"github.com/egoist/mygo/ui"
)

func (h host) OpenRepository(path string) error {
	if err := h.f.openFolder(path, false); err != nil {
		return err
	}
	if target := h.f.folderProject(path); target != nil {
		target.ed.ShowSidebarView(int(view.SideSCM))
	}
	return nil
}

func (f *Frontend) nestedRepositories(c *ui.Context, s *view.SCM) bool {
	if !s.RepositoryLoading && s.RepositoryError == "" && len(s.Repositories) == 0 {
		return false
	}
	p := f.pal
	ui.Column(c).Grow(1).MinHeight(0).Padding(12, 10).Gap(8).Children(func() {
		ui.Text(c, "文件夹中的仓库").FontSize(13).FontWeight(600).TextColor(p.text)
		ui.Text(c, "打开一个项目，管理它的改动和提交").FontSize(11.5).TextColor(p.muted)
		if s.RepositoryLoading {
			ui.Text(c, "正在查找仓库…").FontSize(12).TextColor(p.muted)
		}
		if s.RepositoryError != "" {
			ui.Text(c, s.RepositoryError).FontSize(11.5).TextColor(p.muted)
		}
		refresh := ui.Row(c.Key("refresh-repositories")).Padding(4, 6).Radius(4).Label("刷新仓库列表")
		if refresh.Hovered() {
			refresh.Background(p.hover)
		}
		refresh.Children(func() { ui.Text(c, "刷新仓库列表").FontSize(12).TextColor(p.accent) })
		if refresh.Clicked() {
			f.ed.Run("yan.scm.refreshRepositories")
		}
		ui.Scroll(c.Key("nested-repositories")).Grow(1).Children(func() {
			for i, repo := range s.Repositories {
				row := ui.Row(c.Key(repo.Path)).Height(32).Gap(8).Padding(0, 8).Radius(4).AlignItems(ui.Center).
					Label("打开仓库 " + repo.Name).Tooltip(repo.Path)
				if i == s.Selected && f.frame.Focus == view.FocusSidebar {
					row.Background(p.active)
				} else if row.Hovered() {
					row.Background(p.hover)
				}
				row.Children(func() {
					ui.Icon(c, iconBranch).FontSize(14).TextColor(p.muted)
					ui.Text(c, repo.Name).SingleLine().Shrink(1).Grow(1).FontSize(12.5).TextColor(p.text)
					ui.Icon(c, iconChevronRight).FontSize(12).TextColor(p.muted)
				})
				if row.Clicked() {
					f.ed.SCMOpenRepository(repo.Path)
				}
			}
		})
	})
	return true
}
