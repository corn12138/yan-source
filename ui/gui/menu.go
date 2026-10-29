package gui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"

	"github.com/corn12138/yan-source/core/app"
)

// menu 是原生菜单栏：每一项执行一个内核命令，快捷键取自内核的键位（改了键位，菜单跟着变）。
// 撤销和重做不占快捷键：⌘Z 交给有焦点的控件——工作台交给内核，快速打开的输入框自己撤销。
func (f *Frontend) menu() *mygo.Menu {
	item := func(id, label string) *mygo.MenuItem {
		if app.CommandUnreleased(id) { // not in the official release yet (core/app/features.json)
			label += unreleasedSuffix
		}
		it := &mygo.MenuItem{ID: id, Label: label, Click: func(_ *mygo.MenuItem, w *mygo.Window) {
			g := f
			if o := windows[w]; o != nil { // 交给有焦点的窗口
				g = o
			}
			g.do(func() { g.ed.Run(id) })
		}}
		if ch, ok := f.ed.Binding(id); ok {
			if acc, ok := uiKey(ch); ok {
				it.Accelerator = acc
			}
		}
		return it
	}
	plain := func(id, label string) *mygo.MenuItem {
		it := item(id, label)
		it.Accelerator = ""
		return it
	}
	sep := mygo.Separator
	theme, appearance, _, _ := f.ed.Settings()
	radio := func(id, label string, on bool) *mygo.MenuItem {
		it := plain(id, label)
		it.Type, it.Checked = mygo.MenuItemRadio, on
		return it
	}
	autoSave := plain("workbench.action.toggleAutoSave", "自动保存")
	autoSave.Type, autoSave.Checked = mygo.MenuItemCheckbox, f.ed.AutoSave()
	vimItem := plain("yan.toggleVim", "Vim 模式")
	vimItem.Type, vimItem.Checked = mygo.MenuItemCheckbox, f.ed.VimMode()
	tabsItem := plain("yan.projectTabs.toggle", "项目标签")
	tabsItem.Type, tabsItem.Checked = mygo.MenuItemCheckbox, f.ed.ProjectTabs()
	wrapItem := item("editor.action.toggleWordWrap", "自动换行")
	wrapItem.Type, wrapItem.Checked = mygo.MenuItemCheckbox, f.ed.WordWrap()
	terminalLocation := plain("view.terminalLocation", "默认在编辑区新建终端")
	terminalLocation.Type, terminalLocation.Checked = mygo.MenuItemCheckbox, f.ed.TerminalLocation() == "editor"
	effects := plain("yan.toggleThemeEffects", "装饰效果")
	effects.Type, effects.Checked = mygo.MenuItemCheckbox, f.ed.ThemeEffects()
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: "文件", Submenu: []*mygo.MenuItem{
			item("workbench.action.files.newUntitledFile", "新建未命名文件"),
			item("workbench.action.files.newFile", "新建文件…"),
			item("workbench.action.newWindow", "新建窗口"),
			item("yan.project.close", "关闭项目"),
			item("workbench.action.files.openFolder", "打开文件夹…"),
			item("yan.openFolderNewWindow", "在新窗口中打开文件夹…"),
			{Label: "打开最近", Submenu: f.recentMenu(plain)},
			sep(),
			item("workbench.action.files.save", "保存"),
			item("workbench.action.files.saveAs", "另存为…"),
			item("workbench.action.files.saveAll", "全部保存"),
			autoSave,
			item("yan.files.overwrite", "覆盖磁盘上的版本"),
			item("workbench.action.files.revert", "还原为磁盘上的版本"),
			sep(),
			item("workbench.action.closeActiveEditor", "关闭标签"),
			item("workbench.action.reopenClosedEditor", "重新打开关闭的标签"),
			sep(),
			item("yan.installCLI", "安装 yan 命令…"),
			item("yan.config.open", "打开用户配置文件"),
			item("workbench.action.openGlobalKeybindingsFile", "打开键盘快捷方式（keybindings.json）"),
		}},
		{Label: "编辑", Submenu: []*mygo.MenuItem{
			plain("undo", "撤销"), plain("redo", "重做"), sep(),
			{Role: mygo.RoleCut}, {Role: mygo.RoleCopy}, {Role: mygo.RolePaste}, {Role: mygo.RoleSelectAll}, sep(),
			item("actions.find", "查找"),
			item("editor.action.startFindReplaceAction", "替换"),
			item("editor.action.nextMatchFindAction", "查找下一个"),
			item("editor.action.previousMatchFindAction", "查找上一个"),
		}},
		{Label: "选择", Submenu: []*mygo.MenuItem{
			item("expandLineSelection", "选中行"),
			item("editor.action.addSelectionToNextFindMatch", "选中下一个匹配项"),
			sep(),
			item("editor.action.commentLine", "切换行注释"),
			item("editor.action.indentLines", "缩进行"),
			item("editor.action.outdentLines", "反缩进行"),
			sep(),
			item("editor.action.moveLinesUpAction", "向上移动行"),
			item("editor.action.moveLinesDownAction", "向下移动行"),
			item("editor.action.copyLinesUpAction", "向上复制行"),
			item("editor.action.copyLinesDownAction", "向下复制行"),
			sep(),
			item("editor.action.insertLineAfter", "在下面插入行"),
			item("editor.action.insertLineBefore", "在上面插入行"),
			item("editor.action.deleteLines", "删除行"),
			sep(),
			item("editor.action.sortLinesAscending", "升序排列行"),
			item("editor.action.sortLinesDescending", "降序排列行"),
			item("editor.action.transformToCamelcase", "转换为驼峰命名"),
			{Label: "缩进方式", Submenu: []*mygo.MenuItem{
				plain("editor.action.indentUsingTabs", "使用制表符"),
				plain("yan.indentUsingSpaces2", "使用 2 个空格"),
				plain("yan.indentUsingSpaces4", "使用 4 个空格"),
				plain("yan.indentUsingSpaces8", "使用 8 个空格"),
			}},
		}},
		{Label: "显示", Submenu: []*mygo.MenuItem{
			item("workbench.action.showCommands", "命令面板…"),
			item("workbench.action.toggleSidebarVisibility", "切换侧边栏"),
			item("workbench.action.toggleSidebarPosition", "移动侧边栏到另一侧"),
			item("workbench.view.explorer", "资源管理器"),
			sep(),
			item("workbench.action.splitEditor", "向右拆分"),
			item("workbench.action.focusNextGroup", "聚焦到下一列"),
			item("workbench.action.focusPreviousGroup", "聚焦到上一列"),
			item("workbench.action.moveEditorToNextGroup", "把标签移到下一列"),
			item("workbench.action.moveEditorToPreviousGroup", "把标签移到上一列"),
			plain("workbench.action.toggleMaximizeEditorGroup", "最大化／还原编辑区"),
			sep(),
			wrapItem,
			tabsItem,
			vimItem,
			sep(),
			item("workbench.action.terminal.toggleTerminal", "切换终端"),
			item("workbench.action.terminal.new", "新建终端"),
			item("workbench.action.createTerminalEditor", "在编辑区新建终端"),
			terminalLocation,
			{Label: "终端面板位置", Submenu: []*mygo.MenuItem{
				radio("workbench.action.positionPanelBottom", "底部", f.ed.PanelPosition() == ""),
				radio("workbench.action.positionPanelLeft", "左侧", f.ed.PanelPosition() == "left"),
				radio("workbench.action.positionPanelRight", "右侧", f.ed.PanelPosition() == "right"),
			}},
			sep(),
			item("workbench.action.zoomIn", "放大"),
			item("workbench.action.zoomOut", "缩小"),
			item("workbench.action.zoomReset", "实际大小"),
			{Label: "字体", Submenu: []*mygo.MenuItem{
				item("yan.font.family", "代码字体…"),
				item("yan.font.size", "字号…"),
				item("yan.font.lineHeight", "行高…"),
			}},
			{Label: "主题", Submenu: []*mygo.MenuItem{
				item("workbench.action.selectTheme", "颜色主题…"),
				sep(),
				radio("yan.appearance.system", "跟随系统", appearance == ""),
				radio("yan.appearance.light", "浅色", appearance == "light"),
				radio("yan.appearance.dark", "深色", appearance == "dark"),
				sep(),
				radio("yan.theme.mayoi", "Mayoi", theme == ""),
				radio("yan.theme.matrix", "The Matrix", theme == "matrix"),
				radio("yan.theme.claude", "Claude", theme == "claude"),
				sep(), effects,
			}},
			sep(),
			{Role: mygo.RoleToggleFullScreen},
		}},
		{Label: "转到", Submenu: []*mygo.MenuItem{
			item("workbench.action.quickOpen", "快速打开…"),
			item("workbench.action.gotoLine", "转到行…"),
			item("editor.action.jumpToBracket", "跳到配对的括号"),
			item("editor.action.revealDefinition", "转到定义"),
			sep(),
			item("workbench.action.navigateBack", "后退"),
			item("workbench.action.navigateForward", "前进"),
			sep(),
			item("workbench.action.nextEditor", "下一个标签"),
			item("workbench.action.previousEditor", "上一个标签"),
			sep(),
			item("yan.project.next", "下一个项目"),
			item("yan.project.previous", "上一个项目"),
		}},
		{Role: mygo.RoleWindowMenu},
		{Label: "Git", Submenu: []*mygo.MenuItem{
			item("git.checkout", "切换分支…"),
			item("git.renameBranch", "重命名当前分支…"),
			item("git.deleteBranch", "删除本地分支…"),
			sep(),
			item("git.fetch", "抓取"),
			item("git.pull", "拉取"),
			item("git.push", "推送"),
			sep(),
			{Label: "远程仓库", Submenu: []*mygo.MenuItem{
				plain("git.addRemote", "添加…"),
				plain("yan.git.setRemoteURL", "修改地址…"),
				plain("git.removeRemote", "删除…"),
				plain("yan.git.pruneRemote", "清理已删除的分支…"),
			}},
			sep(),
			plain("yan.diff.compareFiles", "比较两个文件…"),
			plain("yan.diff.whitespace", "改动视图比较空白…"),
			plain("yan.diff.highlight", "改动视图标出改动的方式…"),
			plain("yan.diff.context", "改动视图上下文行数…"),
			plain("toggle.diff.renderSideBySide", "改动视图换排法"),
			plain("yan.diff.toggleSyncScroll", "并排两边一起滚动（开关）"),
			plain("yan.diff.toggleAlignedRows", "并排两边的行对齐（开关）"),
			sep(),
			{Label: "合并冲突", Submenu: []*mygo.MenuItem{
				plain("yan.merge.files", "冲突文件列表…"),
				plain("yan.merge.open", "打开三方合并"),
				plain("yan.merge.previousConflict", "上一处冲突"),
				plain("yan.merge.nextConflict", "下一处冲突"),
				sep(),
				plain("yan.merge.acceptOurs", "这处用我们的"),
				plain("yan.merge.acceptTheirs", "这处用他们的"),
				plain("yan.merge.ignoreOurs", "这处忽略我们的"),
				plain("yan.merge.ignoreTheirs", "这处忽略他们的"),
				plain("yan.merge.resolveOurs", "全部用我们的"),
				plain("yan.merge.resolveTheirs", "全部用他们的"),
				plain("yan.merge.applyChanges", "应用两边不冲突的改动"),
				plain("yan.merge.toggleAutoApply", "打开时自动应用不冲突的改动"),
				sep(),
				plain("yan.merge.markResolved", "标记为已解决"),
				plain("yan.merge.applyAndNext", "应用并打开下一个冲突文件"),
				plain("yan.merge.saveAndClose", "保存并关闭"),
				plain("yan.merge.restoreConflict", "恢复成冲突…"),
			}},
		}},
		{Label: "评审", Submenu: []*mygo.MenuItem{
			item("review.comment", "添加或编辑意见"), plain("review.list", "查看全部意见"),
			plain("review.next", "下一条意见"), plain("review.previous", "上一条意见"), sep(),
			plain("review.copyExcerpt", "复制代码片段与位置"), plain("review.copy", "复制全部意见"), plain("review.send", "发送意见到终端"), sep(),
			plain("review.clear", "清空意见"), plain("review.undo", "撤销删除或清空"),
		}},
		{Label: "帮助", Submenu: []*mygo.MenuItem{
			item("yan.guide.open", "功能导览（学习）"),
			{Label: "关于砚", Click: func(*mygo.MenuItem, *mygo.Window) { go f.about() }},
			f.updateMenu(),
			f.automaticUpdatesMenu(),
			{Label: "第三方声明", Click: func(*mygo.MenuItem, *mygo.Window) { f.showNotices() }},
		}},
	})
}

// recentMenu 是「打开最近」：最近的文件夹，和找它们的浮层（⌃R）。
func (f *Frontend) recentMenu(plain func(id, label string) *mygo.MenuItem) []*mygo.MenuItem {
	var items []*mygo.MenuItem
	home, _ := os.UserHomeDir()
	for _, dir := range f.ed.RecentFolders() {
		label := dir
		if home != "" && strings.HasPrefix(dir, home+string(filepath.Separator)) {
			label = "~" + dir[len(home):]
		}
		d := dir
		items = append(items, &mygo.MenuItem{Label: label, Click: func(_ *mygo.MenuItem, w *mygo.Window) {
			current := f
			if active := windows[w]; active != nil {
				current = active
			}
			current.do(func() {
				if err := current.openFolder(d, false); err != nil {
					current.ed.Notify(err.Error())
				}
			})
		}})
	}
	if len(items) > 0 {
		items = append(items, mygo.Separator())
	}
	more := plain("workbench.action.openRecent", "更多…")
	return append(items, more)
}
