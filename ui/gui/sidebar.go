package gui

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/view"
)

// treeRowH 是资源管理器一行的高度（DIP）。
const treeRowH = 27

// sidebar 是资源管理器：标题，文件夹名，下面是文件树。单击用预览标签打开文件，双击转正。
func (f *Frontend) sidebar(c *ui.Context) {
	p := f.pal
	sb := &f.frame.Sidebar
	focused := f.frame.Focus == view.FocusSidebar
	ui.Column(c).Fill().Background(p.chrome).Children(func() {
		ui.Row(c).Height(34).Padding(0, 6, 0, 8).Gap(2).AlignItems(ui.Center).Children(func() {
			f.sidebarSwitch(c, "文件", "资源管理器 ("+f.ed.KeyHint("workbench.view.explorer")+")", sb.View == view.SideExplorer, 0, view.SideExplorer)
			f.sidebarSwitch(c, "搜索", "搜索 ("+f.ed.KeyHint("workbench.view.search")+")", sb.View == view.SideSearch, 0, view.SideSearch)
			f.sidebarSwitch(c, "改动", "源代码管理 ("+f.ed.KeyHint("workbench.view.scm")+")", sb.View == view.SideSCM, sb.Changes, view.SideSCM)
			f.sidebarSwitch(c, "学习", "学习：功能导览", sb.View == view.SideLearn, 0, view.SideLearn)
			ui.Spacer(c)
			switch sb.View {
			case view.SideSCM:
				// Four named views leave room for one button: Source Control's actions share a menu.
				f.iconButton(c, iconMore, "更多源代码管理操作", false).Menu(func(m *ui.Menu) {
					if m.Item("以树的形式查看（再按回到列表）").Chosen() {
						f.ed.Run("yan.scm.toggleTree")
					}
					if m.Item("刷新").Chosen() {
						f.ed.Run("git.refresh")
					}
				})
			}
		})
		switch sb.View {
		case view.SideLearn:
			f.learnView(c)
			return
		case view.SideSCM:
			f.scmView(c)
			return
		case view.SideSearch:
			f.searchView(c)
			return
		}
		// The folder's row carries the Explorer's actions, as the Quiet UI design has them.
		ui.Row(c).Height(26).Padding(0, 6, 0, 12).Gap(2).AlignItems(ui.Center).Children(func() {
			ui.Text(c, strings.ToUpper(sb.Title)).FontSize(11).FontWeight(600).LetterSpacing(0.4).TextColor(p.muted).SingleLine().Shrink(1)
			if sb.Busy {
				ui.Text(c, "索引中…").FontSize(11).TextColor(p.faint).SingleLine()
			}
			ui.Spacer(c)
			for _, a := range []struct {
				icon      *ui.SVG
				label, id string
			}{
				{iconFilePlus, "新建文件…", "explorer.newFile"},
				{iconFolderPlus, "新建文件夹…", "explorer.newFolder"},
				{iconLocate, "在资源管理器中显示当前文件", "workbench.files.action.showActiveFileInExplorer"},
				{iconRefresh, "刷新资源管理器", "workbench.files.action.refreshFilesExplorer"},
			} {
				if f.iconButton(c, a.icon, a.label, false).Clicked() {
					f.ed.Run(a.id)
				}
			}
		})
		if len(sb.Rows) == 0 && !sb.Busy { // 空文件夹：说一声，给新建的按钮（照着 Mayoi v0.8.1）
			ui.Column(c).Padding(10, 16).Gap(8).Children(func() {
				ui.Text(c, "这个文件夹是空的").FontSize(12.5).TextColor(p.muted)
				ui.Row(c).Gap(6).Children(func() {
					for _, b := range []struct{ label, cmd string }{{"新建文件…", "explorer.newFile"}, {"新建文件夹…", "explorer.newFolder"}} {
						btn := ui.Box(c.Key("empty:"+b.cmd)).Height(24).Padding(0, 10).Radius(5).Justify(ui.Center).AlignItems(ui.Center).
							Background(p.surface).Label(b.label)
						if btn.Hovered() {
							btn.Background(p.surfaceHover)
						}
						btn.Children(func() { ui.Text(c, b.label).FontSize(12).TextColor(p.text) })
						if btn.Clicked() {
							f.ed.Run(b.cmd)
						}
					}
				})
			})
			return
		}
		if sb.Selected != f.treeAt {
			f.treeAt = sb.Selected
			if focused && sb.Selected >= 0 {
				f.tree.ScrollIntoView(sb.Selected) // 键盘移动了选中行：滚过去
			}
		}
		end, rowClicked := float32(-1), false // 最后一行的下沿：点在它下面就是没点在文件上
		list := ui.List(c.Key("tree"), &f.tree, len(sb.Rows), func(i int) {
			n := &sb.Rows[i]
			row := ui.Row(c).Height(treeRowH).Padding(0, 8, 0, 8+float32(n.Depth)*14).Gap(6).AlignItems(ui.Center).Radius(4).Margin(0, 6)
			if i == len(sb.Rows)-1 {
				if r := row.Bounds(); r.H > 0 {
					end = r.Y + r.H
				}
			}
			row.DragDataFrom(func() transfer.Data { // 拖到一列上打开，拖到访达里复制；开始拖时才问路径
				data, _ := transfer.FileData(f.ed.TreePath(i))
				return data
			})
			switch {
			case i == sb.Selected && focused:
				row.Background(p.active)
			case i == sb.Selected:
				row.Background(p.hover.Alpha(1.6))
			case row.Hovered():
				row.Background(p.hover)
			}
			if row.DoubleClicked() {
				f.ed.ClickTree(i, true)
				rowClicked = true
			} else if row.Clicked() {
				f.ed.ClickTree(i, false)
				rowClicked = true
			}
			row.ContextMenu(func(m *ui.Menu) { f.treeMenu(m, i, n) })
			row.Children(func() {
				chev := ui.Box(c).Size(12, 12)
				if n.Dir {
					chev.Children(func() {
						icon := iconChevronRight
						if n.Expanded {
							icon = iconChevronDown
						}
						ui.Icon(c, icon).FontSize(12).TextColor(p.faint)
					})
				}
				icon, tint := iconFolder, p.accent
				switch {
				case n.Dir && n.Expanded:
					icon = iconFolderOpen
				case !n.Dir:
					icon, tint = fileIcon(p, n.Name)
				}
				if n.Ignored {
					tint = tint.Alpha(0.5)
				}
				ui.Icon(c, icon).FontSize(14).TextColor(tint)
				label := ui.Text(c, n.Name).FontSize(13).SingleLine().Shrink(1).TextColor(p.text)
				switch {
				case n.Modified:
					label.TextColor(p.modified)
				case n.Ignored:
					label.TextColor(p.faint)
				case n.Git != 0:
					label.TextColor(gitColor(p, n.Git))
				}
				if n.Git == 0 {
					return
				}
				ui.Spacer(c)
				if n.Dir { // 下面有改动的目录：一个小圆点
					ui.Box(c).Size(6, 6).Radius(3).Margin(0, 6, 0, 0).Background(gitColor(p, n.Git).Alpha(0.8))
					return
				}
				ui.Text(c, string(n.Git)).FontSize(11.5).FontWeight(600).TextColor(gitColor(p, n.Git)).Width(14).TextAlign(ui.Center)
			})
		}).Grow(1).Padding(2, 0, 8, 0)
		// 文件下面的空白（照着 Mayoi v0.6.0）：点一下什么都不选，右键是打开的文件夹的菜单。
		below := false
		if end >= 0 {
			_, y, over := list.PointerPosition()
			below = over && list.Bounds().Y+y > end
		}
		if list.Clicked() && !rowClicked && below {
			f.ed.ClearTreeSelection()
		}
		list.ContextMenu(func(m *ui.Menu) {
			if !below {
				return
			}
			f.ed.ClearTreeSelection()
			f.workspaceMenu(m)
		})
	})
}

// workspaceMenu 是在资源管理器的空白处右键的菜单：对打开的文件夹做的事。
func (f *Frontend) workspaceMenu(m *ui.Menu) {
	if m.Item("新建文件…").Chosen() {
		f.ed.Run("explorer.newFile")
	}
	if m.Item("新建文件夹…").Chosen() {
		f.ed.Run("explorer.newFolder")
	}
	m.Separator()
	if m.Item("在" + app.FileManager(hostOS) + "中显示").Chosen() {
		mygo.Shell.ShowItemInFolder(f.ed.Root())
	}
	if m.Item("在终端中打开").Chosen() {
		f.ed.ClearTreeSelection()
		f.ed.Run("openInIntegratedTerminal")
	}
	m.Separator()
	if m.Item("粘贴").Disabled(!f.ed.CanPaste()).Chosen() {
		f.ed.Run("filesExplorer.paste")
	}
}

// gitColor 是 git 字母的颜色：修改、新增、删除、冲突（照着 Mayoi）。
func gitColor(p *palette, letter byte) ui.Color {
	switch letter {
	case 'A', 'U':
		return p.added
	case 'D':
		return p.removed
	case 'C':
		return p.danger
	}
	return p.modified
}

// treeMenu 是资源管理器里一行的右键菜单（照着 Mayoi）。
func (f *Frontend) treeMenu(m *ui.Menu, i int, n *view.TreeRow) {
	run := func(id string) {
		f.ed.SelectTree(i)
		f.ed.Run(id)
	}
	if m.Item("新建文件…").Chosen() {
		run("explorer.newFile")
	}
	if m.Item("新建文件夹…").Chosen() {
		run("explorer.newFolder")
	}
	m.Separator()
	if m.Item("在" + app.FileManager(hostOS) + "中显示").Chosen() {
		run("revealFileInOS")
	}
	if m.Item("在终端中打开").Chosen() {
		run("openInIntegratedTerminal")
	}
	if !n.Dir && n.Git != 0 && m.Item("打开改动").Chosen() {
		f.ed.ClickTree(i, true)
		f.ed.Run("git.openChange")
	}
	if !n.Dir {
		m.Separator()
		if m.Item("选为比较对象").Chosen() {
			run("selectForCompare")
		}
		if m.Item("与已选文件比较").Disabled(!f.ed.CanCompareWithSelected()).Chosen() {
			run("compareFiles")
		}
	}
	m.Separator()
	if m.Item("复制").Chosen() {
		run("filesExplorer.copy")
	}
	if m.Item("粘贴").Disabled(!f.ed.CanPaste()).Chosen() {
		run("filesExplorer.paste")
	}
	m.Separator()
	if m.Item("复制路径").Chosen() {
		run("copyFilePath")
	}
	if m.Item("复制相对路径").Chosen() {
		run("copyRelativeFilePath")
	}
	m.Separator()
	if m.Item("改名…").Chosen() {
		run("renameFile")
	}
	if m.Item("移到" + app.TrashName(hostOS)).Chosen() {
		f.ed.SelectTree(i)
		f.confirmTrash(n.Name, i)
	}
}

// confirmTrash 先问一声，再把资源管理器里的第 i 项移到废纸篓。
func (f *Frontend) confirmTrash(name string, i int) {
	trash := app.TrashName(hostOS)
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent: f.win, Type: mygo.MessageWarning, Message: "把 " + name + " 移到" + trash + "吗？",
			Detail: "可以从" + trash + "里放回来。", Buttons: []string{"移到" + trash, "取消"}, DefaultButton: 0, CancelButton: 1,
		})
		if err == nil && res.Button == 0 {
			f.do(func() {
				if err := f.ed.TrashPath(f.ed.TreePath(i)); err != nil {
					f.ed.Notify(err.Error())
				}
			})
		}
	}()
}
