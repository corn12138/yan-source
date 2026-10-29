package gui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/view"
)

// unreleasedSuffix marks a menu item the official release does not have yet.
const unreleasedSuffix = "（未发布）"

// learnView is the feature guide in the sidebar: the lessons by module, the steps of the one
// running with the key, palette name and menu path for each, and a tip. The core decides when
// a step is done.
func (f *Frontend) learnView(c *ui.Context) {
	p := f.pal
	lg := &f.frame.Sidebar.Learn
	ui.Scroll(c.Key("learn")).Grow(1).Children(func() {
		ui.Column(c).Padding(8, 12, 12, 12).Gap(10).Children(func() {
			ui.Text(c, fmt.Sprintf("已完成 %d / %d 课", lg.Done, len(lg.Lessons))).FontSize(12).TextColor(p.muted)
			if lg.Home != "" {
				home := ui.Row(c.Key("learn-home")).Gap(6).Padding(6, 10).Radius(6).AlignItems(ui.Center).Background(p.surface).
					Label("回到 " + lg.Home).Cursor(ui.CursorPointer)
				home.Children(func() {
					ui.Icon(c, iconFolder).FontSize(14).TextColor(p.accent)
					ui.Text(c, "练习文件夹 · 回到 "+lg.Home).FontSize(12.5).TextColor(p.accent).Grow(1).SingleLine()
				})
				if home.Clicked() {
					f.ed.Run("yan.guide.leave")
				}
			}
			if lg.Active >= 0 && lg.Active < len(lg.Lessons) {
				f.learnSteps(c, lg)
			}
			ui.Column(c.Key("lessons")).Gap(1).Children(func() { f.learnLessons(c, lg) })
			ui.Row(c).Margin(6, 0, 0, 0).Padding(10).Gap(8).Radius(6).Background(p.surface).AlignItems(ui.Start).Children(func() {
				ui.Icon(c, iconTip).FontSize(15).TextColor(p.accent)
				ui.Text(c, lg.Tip).FontSize(12.5).TextColor(p.text).Grow(1).MinWidth(0)
				if f.iconButton(c, iconChevronRight, "下一条提示", false).Clicked() {
					f.ed.Run("yan.guide.nextTip")
				}
			})
		})
	})
}

// unreleasedBadge says a feature is not in the official release yet; since is its version.
func (f *Frontend) unreleasedBadge(c *ui.Context) ui.Element {
	p := f.pal
	return ui.Text(c, "未发布").FontSize(10.5).TextColor(p.modified).Padding(0, 5).Radius(4).
		BorderWidth(1).BorderColor(p.modified).Shrink(0).SingleLine().
		Tooltip("正式版里还没有：这一项写在 CHANGELOG 的 Unreleased 下")
}

// learnLessons lists the lessons by module; a click starts one.
func (f *Frontend) learnLessons(c *ui.Context, lg *view.Learn) {
	p := f.pal
	module := ""
	for i, l := range lg.Lessons {
		if l.Module != module {
			module = l.Module
			ui.Text(c, module).FontSize(11).FontWeight(600).LetterSpacing(0.6).TextColor(p.faint).Margin(8, 0, 2, 0)
		}
		row := ui.Row(c.Key(fmt.Sprint("lesson:", i))).Height(26).Padding(0, 8).Gap(8).Radius(5).AlignItems(ui.Center).
			Label("课程 " + l.Title)
		switch {
		case i == lg.Active:
			row.Background(p.active)
		case i == lg.Selected && f.frame.Focus == view.FocusSidebar, row.Hovered():
			row.Background(p.hover)
		}
		row.Children(func() {
			icon, col := iconCircle, p.faint
			if l.Done {
				icon, col = iconCircleCheck, p.added
			}
			ui.Icon(c, icon).FontSize(14).TextColor(col)
			ui.Text(c, l.Title).FontSize(13).TextColor(p.text).Grow(1).SingleLine()
			if l.Unreleased {
				f.unreleasedBadge(c)
			}
			if l.Folder {
				ui.Icon(c, iconFolder).FontSize(12).TextColor(p.faint).Tooltip("在练习文件夹里做，不动你的项目")
			}
			ui.Text(c, fmt.Sprintf("%d 步", l.Steps)).FontSize(11).TextColor(p.faint)
		})
		if row.Clicked() {
			f.ed.StartLesson(i)
			f.focusRoot = true // the practice file has the keyboard
		}
	}
}

// learnSteps shows the steps of the lesson running.
func (f *Frontend) learnSteps(c *ui.Context, lg *view.Learn) {
	p := f.pal
	ui.Column(c.Key("learn-steps")).Padding(10).Gap(6).Radius(6).Background(p.surface).Children(func() {
		ui.Text(c, lg.Lessons[lg.Active].Title).FontSize(13).FontWeight(600).TextColor(p.text)
		for i, s := range lg.Steps {
			ui.Row(c.Key(fmt.Sprint("step:", i))).Gap(8).AlignItems(ui.Start).Children(func() {
				icon, col := iconCircle, p.faint
				switch {
				case s.Done:
					icon, col = iconCircleCheck, p.added
				case s.Current:
					col = p.accent
				}
				ui.Icon(c, icon).FontSize(14).TextColor(col).Margin(1, 0, 0, 0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Start).Children(func() {
						txt := ui.Text(c, s.Text).FontSize(12.5).Grow(1).MinWidth(0)
						switch {
						case s.Done || s.WindowOnly:
							txt.TextColor(p.muted)
						default:
							txt.TextColor(p.text)
						}
						if s.Unreleased {
							f.unreleasedBadge(c)
						}
					})
					if s.Current {
						f.learnHow(c, s)
					}
					if s.WindowOnly {
						ui.Text(c, "窗口版才有，已跳过").FontSize(11).TextColor(p.faint)
					}
				})
			})
		}
		done := len(lg.Steps) > 0 && lg.Steps[len(lg.Steps)-1].Done
		ui.Row(c).Gap(14).Margin(4, 0, 0, 0).Children(func() {
			link := func(label, cmd string) {
				if ui.Text(c.Key(cmd), label).FontSize(12).TextColor(p.accent).Label(label).Cursor(ui.CursorPointer).Clicked() {
					f.ed.Run(cmd)
				}
			}
			link("上一步", "yan.guide.back")
			if !done {
				link("跳过这一步", "yan.guide.skip")
			}
			link("重新开始", "yan.guide.restart")
		})
		if done {
			ui.Text(c, "完成了！在下面选下一课").FontSize(12).TextColor(p.added)
		}
	})
}

// learnHow says how to do the current step: its keys in the user's keymap, its name in the
// command palette, and where the menus have it.
func (f *Frontend) learnHow(c *ui.Context, s view.LearnStep) {
	p := f.pal
	if s.Keys != "" || s.Times > 1 {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			if s.Keys != "" {
				ui.Text(c, s.Keys).Font(f.opts.FontFamily).FontSize(11.5).TextColor(p.text).
					Padding(1, 6).Radius(4).Background(p.chrome).BorderWidth(1).BorderColor(p.border)
			}
			if s.Times > 1 {
				ui.Text(c, fmt.Sprintf("%d / %d", s.Count, s.Times)).FontSize(11).TextColor(p.muted)
			}
		})
	}
	if s.Title != "" {
		ui.Text(c, "命令面板：「"+s.Title+"」").FontSize(11).TextColor(p.muted)
	}
	if where := f.menuPath(s.Command); where != "" {
		ui.Text(c, "菜单："+where).FontSize(11).TextColor(p.muted)
	}
}

// menuPath is where the menu bar has command id, as "显示 › 最大化／还原编辑区"; "" when no
// menu has it. The paths are read from the menu itself once, so they never go stale in text.
func (f *Frontend) menuPath(id string) string {
	if id == "" {
		return ""
	}
	if f.menuPaths == nil {
		f.menuPaths = map[string]string{}
		var walk func(items []*mygo.MenuItem, path string)
		walk = func(items []*mygo.MenuItem, path string) {
			for _, it := range items {
				label := strings.TrimSuffix(it.Label, unreleasedSuffix)
				where := label
				if path != "" {
					where = path + " › " + label
				}
				if _, seen := f.menuPaths[it.ID]; it.ID != "" && !seen {
					f.menuPaths[it.ID] = where
				}
				walk(it.Submenu, where)
			}
		}
		walk(f.menu().Items(), "")
	}
	return f.menuPaths[id]
}

// guideOfferView is the one-time offer of the guide on a first launch, in a corner.
func (f *Frontend) guideOfferView(c *ui.Context) {
	p := f.pal
	o := f.frame.Guide
	ui.Overlay(c, func() {
		card := f.floating(c, ui.Column(c.Key("guide-offer")).Absolute().Right(20).Bottom(statusBarH+18).Width(330).
			Padding(14, 16).Gap(10).Radius(10)).Role(ui.RoleDialog).Label("功能导览")
		card.Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconLearn).FontSize(16).TextColor(p.accent)
				ui.Text(c, "第一次用砚？").FontSize(13.5).FontWeight(600).TextColor(p.text)
			})
			ui.Text(c, fmt.Sprintf("功能导览有 %d 节小课：多光标、锁定列、最大化编辑区、转到定义、三方合并……每一步都在练习文件里真的做一遍，不动你的项目。", o.Lessons)).
				FontSize(12.5).TextColor(p.muted)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c.Key("guide-offer-no"), "不用了").Label("不用了").Clicked() {
					f.ed.Run("yan.guide.dismissOffer")
				}
				if ui.Button(c.Key("guide-offer-yes"), "开始导览").Label("开始导览").Background(p.accent).TextColor(p.editor).Clicked() {
					f.ed.Run("yan.guide.open")
				}
			})
		})
	})
}
