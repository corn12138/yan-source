package gui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/i18n"
)

// menu 是原生菜单栏：每一项执行一个内核命令，快捷键取自内核的键位（改了键位，菜单跟着变）。
// 撤销和重做不占快捷键：⌘Z 交给有焦点的控件——工作台交给内核，快速打开的输入框自己撤销。
func (f *Frontend) menu() *mygo.Menu {
	item := func(id, label string) *mygo.MenuItem {
		if app.CommandUnreleased(id) { // not in the official release yet (core/app/features.json)
			label += unreleasedSuffix()
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
	autoSave := plain("workbench.action.toggleAutoSave", i18n.T("Auto Save"))
	autoSave.Type, autoSave.Checked = mygo.MenuItemCheckbox, f.ed.AutoSave()
	vimItem := plain("yan.toggleVim", i18n.T("Vim Mode"))
	vimItem.Type, vimItem.Checked = mygo.MenuItemCheckbox, f.ed.VimMode()
	tabsItem := plain("yan.projectTabs.toggle", i18n.T("Project Tabs"))
	tabsItem.Type, tabsItem.Checked = mygo.MenuItemCheckbox, f.ed.ProjectTabs()
	wrapItem := item("editor.action.toggleWordWrap", i18n.T("Word Wrap"))
	wrapItem.Type, wrapItem.Checked = mygo.MenuItemCheckbox, f.ed.WordWrap()
	terminalLocation := plain("view.terminalLocation", i18n.T("Create New Terminals in the Editor Area"))
	terminalLocation.Type, terminalLocation.Checked = mygo.MenuItemCheckbox, f.ed.TerminalLocation() == "editor"
	effects := plain("yan.toggleThemeEffects", i18n.T("Decorations"))
	effects.Type, effects.Checked = mygo.MenuItemCheckbox, f.ed.ThemeEffects()
	settings := item("yan.settings.open", i18n.T("Settings…"))
	fileSettings := []*mygo.MenuItem{settings}
	if nativeMenuBar() {
		fileSettings = nil // macOS keeps Settings in the application menu
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		f.appMenu(settings),
		{Label: i18n.T("File"), Submenu: append([]*mygo.MenuItem{
			item("workbench.action.files.newUntitledFile", i18n.T("New Untitled File")),
			item("workbench.action.files.newFile", i18n.T("New File…")),
			item("workbench.action.newWindow", i18n.T("New Window")),
			item("yan.project.close", i18n.T("Close Project")),
			item("workbench.action.files.openFolder", i18n.T("Open Folder…")),
			item("yan.openFolderNewWindow", i18n.T("Open Folder in New Window…")),
			{Label: i18n.T("Open Recent"), Submenu: f.recentMenu(plain)},
			sep(),
			item("workbench.action.files.save", i18n.T("Save")),
			item("workbench.action.files.saveAs", i18n.T("Save As…")),
			item("workbench.action.files.saveAll", i18n.T("Save All")),
			autoSave,
			item("yan.files.overwrite", i18n.T("Overwrite the Version on Disk")),
			item("workbench.action.files.revert", i18n.T("Revert to the Version on Disk")),
			sep(),
			item("workbench.action.closeActiveEditor", i18n.T("Close Tab")),
			item("workbench.action.reopenClosedEditor", i18n.T("Reopen Closed Tab")),
			sep(),
			item("yan.installCLI", i18n.T("Install yan Command…")),
		}, append(fileSettings,
			item("yan.config.open", i18n.T("Open User Configuration File")),
			item("workbench.action.openGlobalKeybindingsFile", i18n.T("Open Keyboard Shortcuts (keybindings.json)")),
		)...)},
		{Label: i18n.T("Edit"), Submenu: []*mygo.MenuItem{
			plain("undo", i18n.T("Undo")), plain("redo", i18n.T("Redo")), sep(),
			{Role: mygo.RoleCut, Label: i18n.T("Cut")}, {Role: mygo.RoleCopy, Label: i18n.T("Copy")},
			{Role: mygo.RolePaste, Label: i18n.T("Paste")}, {Role: mygo.RoleSelectAll, Label: i18n.T("Select All")}, sep(),
			item("actions.find", i18n.T("Find")),
			item("editor.action.startFindReplaceAction", i18n.T("Replace")),
			item("editor.action.nextMatchFindAction", i18n.T("Find Next")),
			item("editor.action.previousMatchFindAction", i18n.T("Find Previous")),
		}},
		{Label: i18n.T("Selection"), Submenu: []*mygo.MenuItem{
			item("expandLineSelection", i18n.T("Select Line")),
			item("editor.action.addSelectionToNextFindMatch", i18n.T("Add Selection to Next Match")),
			sep(),
			item("editor.action.commentLine", i18n.T("Toggle Line Comment")),
			item("editor.action.indentLines", i18n.T("Indent Lines")),
			item("editor.action.outdentLines", i18n.T("Outdent Lines")),
			sep(),
			item("editor.action.moveLinesUpAction", i18n.T("Move Line Up")),
			item("editor.action.moveLinesDownAction", i18n.T("Move Line Down")),
			item("editor.action.copyLinesUpAction", i18n.T("Copy Line Up")),
			item("editor.action.copyLinesDownAction", i18n.T("Copy Line Down")),
			sep(),
			item("editor.action.insertLineAfter", i18n.T("Insert Line Below")),
			item("editor.action.insertLineBefore", i18n.T("Insert Line Above")),
			item("editor.action.deleteLines", i18n.T("Delete Line")),
			sep(),
			item("editor.action.sortLinesAscending", i18n.T("Sort Lines Ascending")),
			item("editor.action.sortLinesDescending", i18n.T("Sort Lines Descending")),
			item("editor.action.transformToCamelcase", i18n.T("Transform to camelCase")),
			{Label: i18n.T("Indentation"), Submenu: []*mygo.MenuItem{
				plain("editor.action.indentUsingTabs", i18n.T("Use Tabs")),
				plain("yan.indentUsingSpaces2", i18n.T("Use 2 Spaces")),
				plain("yan.indentUsingSpaces4", i18n.T("Use 4 Spaces")),
				plain("yan.indentUsingSpaces8", i18n.T("Use 8 Spaces")),
			}},
		}},
		{Label: i18n.T("View"), Submenu: []*mygo.MenuItem{
			item("workbench.action.showCommands", i18n.T("Command Palette…")),
			item("workbench.action.toggleSidebarVisibility", i18n.T("Toggle Sidebar")),
			item("workbench.action.toggleSidebarPosition", i18n.T("Move Sidebar to the Other Side")),
			item("workbench.view.explorer", i18n.T("Explorer")),
			sep(),
			item("workbench.action.splitEditor", i18n.T("Split Right")),
			item("workbench.action.focusNextGroup", i18n.T("Focus Next Column")),
			item("workbench.action.focusPreviousGroup", i18n.T("Focus Previous Column")),
			item("workbench.action.moveEditorToNextGroup", i18n.T("Move Tab to Next Column")),
			item("workbench.action.moveEditorToPreviousGroup", i18n.T("Move Tab to Previous Column")),
			plain("workbench.action.toggleMaximizeEditorGroup", i18n.T("Maximize / Restore Editor Area")),
			sep(),
			wrapItem,
			tabsItem,
			vimItem,
			sep(),
			item("workbench.action.terminal.toggleTerminal", i18n.T("Toggle Terminal")),
			item("workbench.action.terminal.new", i18n.T("New Terminal")),
			item("workbench.action.createTerminalEditor", i18n.T("New Terminal in Editor Area")),
			terminalLocation,
			{Label: i18n.T("Terminal Panel Position"), Submenu: []*mygo.MenuItem{
				radio("workbench.action.positionPanelBottom", i18n.T("Bottom"), f.ed.PanelPosition() == ""),
				radio("workbench.action.positionPanelLeft", i18n.T("Left"), f.ed.PanelPosition() == "left"),
				radio("workbench.action.positionPanelRight", i18n.T("Right"), f.ed.PanelPosition() == "right"),
			}},
			sep(),
			item("workbench.action.zoomIn", i18n.T("Zoom In")),
			item("workbench.action.zoomOut", i18n.T("Zoom Out")),
			item("workbench.action.zoomReset", i18n.T("Actual Size")),
			{Label: i18n.T("Font"), Submenu: []*mygo.MenuItem{
				item("yan.font.family", i18n.T("Code Font…")),
				item("yan.font.size", i18n.T("Font Size…")),
				item("yan.font.lineHeight", i18n.T("Line Height…")),
			}},
			{Label: i18n.T("Theme"), Submenu: []*mygo.MenuItem{
				item("workbench.action.selectTheme", i18n.T("Color Theme…")),
				sep(),
				radio("yan.appearance.system", i18n.T("Follow System"), appearance == ""),
				radio("yan.appearance.light", i18n.T("Light"), appearance == "light"),
				radio("yan.appearance.dark", i18n.T("Dark"), appearance == "dark"),
				sep(),
				radio("yan.theme.mayoi", "Mayoi", theme == ""),
				radio("yan.theme.matrix", "The Matrix", theme == "matrix"),
				radio("yan.theme.claude", "Claude", theme == "claude"),
				sep(), effects,
			}},
			sep(),
			{Role: mygo.RoleToggleFullScreen, Label: i18n.T("Toggle Full Screen")},
		}},
		{Label: i18n.T("Go"), Submenu: []*mygo.MenuItem{
			item("workbench.action.quickOpen", i18n.T("Quick Open…")),
			item("workbench.action.gotoLine", i18n.T("Go to Line…")),
			item("editor.action.jumpToBracket", i18n.T("Go to Matching Bracket")),
			item("editor.action.revealDefinition", i18n.T("Go to Definition")),
			sep(),
			item("workbench.action.navigateBack", i18n.T("Go Back")),
			item("workbench.action.navigateForward", i18n.T("Go Forward")),
			sep(),
			item("workbench.action.nextEditor", i18n.T("Next Tab")),
			item("workbench.action.previousEditor", i18n.T("Previous Tab")),
			sep(),
			item("yan.project.next", i18n.T("Next Project")),
			item("yan.project.previous", i18n.T("Previous Project")),
		}},
		{Role: mygo.RoleWindowMenu, Label: i18n.T("Window"), Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleMinimize, Label: i18n.T("Minimize")}, {Role: mygo.RoleZoom, Label: i18n.T("Zoom")},
			sep(), {Role: mygo.RoleFront, Label: i18n.T("Bring All to Front")},
		}},
		{Label: "Git", Submenu: []*mygo.MenuItem{
			item("git.checkout", i18n.T("Switch Branch…")),
			item("git.renameBranch", i18n.T("Rename Current Branch…")),
			item("git.deleteBranch", i18n.T("Delete Local Branch…")),
			sep(),
			item("git.fetch", i18n.T("Fetch")),
			item("git.pull", i18n.T("Pull")),
			item("git.push", i18n.T("Push")),
			sep(),
			{Label: i18n.T("Remote Repositories"), Submenu: []*mygo.MenuItem{
				plain("git.addRemote", i18n.T("Add…")),
				plain("yan.git.setRemoteURL", i18n.T("Change URL…")),
				plain("git.removeRemote", i18n.T("Remove…")),
				plain("yan.git.pruneRemote", i18n.T("Prune Deleted Branches…")),
			}},
			sep(),
			plain("yan.diff.compareFiles", i18n.T("Compare Two Files…")),
			plain("yan.diff.whitespace", i18n.T("Whitespace in Diff Views…")),
			plain("yan.diff.highlight", i18n.T("How Diff Views Highlight Changes…")),
			plain("yan.diff.context", i18n.T("Diff View Context Lines…")),
			plain("toggle.diff.renderSideBySide", i18n.T("Change Diff View Layout")),
			plain("yan.diff.toggleSyncScroll", i18n.T("Scroll Both Sides Together (Toggle)")),
			plain("yan.diff.toggleAlignedRows", i18n.T("Align Rows of Both Sides (Toggle)")),
			sep(),
			{Label: i18n.T("Merge Conflicts"), Submenu: []*mygo.MenuItem{
				plain("yan.merge.files", i18n.T("Conflicted Files…")),
				plain("yan.merge.open", i18n.T("Open Three-Way Merge")),
				plain("yan.merge.previousConflict", i18n.T("Previous Conflict")),
				plain("yan.merge.nextConflict", i18n.T("Next Conflict")),
				sep(),
				plain("yan.merge.acceptOurs", i18n.T("Use Ours Here")),
				plain("yan.merge.acceptTheirs", i18n.T("Use Theirs Here")),
				plain("yan.merge.ignoreOurs", i18n.T("Ignore Ours Here")),
				plain("yan.merge.ignoreTheirs", i18n.T("Ignore Theirs Here")),
				plain("yan.merge.resolveOurs", i18n.T("Use Ours for All")),
				plain("yan.merge.resolveTheirs", i18n.T("Use Theirs for All")),
				plain("yan.merge.applyChanges", i18n.T("Apply Non-Conflicting Changes from Both Sides")),
				plain("yan.merge.toggleAutoApply", i18n.T("Apply Non-Conflicting Changes on Open")),
				sep(),
				plain("yan.merge.markResolved", i18n.T("Mark as Resolved")),
				plain("yan.merge.applyAndNext", i18n.T("Apply and Open Next Conflicted File")),
				plain("yan.merge.saveAndClose", i18n.T("Save and Close")),
				plain("yan.merge.restoreConflict", i18n.T("Restore Conflict…")),
			}},
		}},
		{Label: i18n.T("Review"), Submenu: []*mygo.MenuItem{
			item("review.comment", i18n.T("Add or Edit Comment")), plain("review.list", i18n.T("Show All Comments")),
			plain("review.next", i18n.T("Next Comment")), plain("review.previous", i18n.T("Previous Comment")), sep(),
			plain("review.copyExcerpt", i18n.T("Copy Code Excerpt and Location")), plain("review.copy", i18n.T("Copy All Comments")), plain("review.send", i18n.T("Send Comments to Terminal")), sep(),
			plain("review.clear", i18n.T("Clear Comments")), plain("review.undo", i18n.T("Undo Delete or Clear")),
		}},
		{Label: i18n.T("Help"), Submenu: []*mygo.MenuItem{
			item("yan.guide.open", i18n.T("Feature Tour (Learn)")),
			{Label: i18n.T("About Yan"), Click: func(*mygo.MenuItem, *mygo.Window) { go f.about() }},
			f.updateMenu(),
			f.automaticUpdatesMenu(),
			{Label: i18n.T("Third-Party Notices"), Click: func(*mygo.MenuItem, *mygo.Window) { f.showNotices() }},
		}},
	})
}

// appMenu is macOS's application menu, with Yan's labels in the interface language: the menu
// roles would otherwise give them in English.
func (f *Frontend) appMenu(settings *mygo.MenuItem) *mygo.MenuItem {
	name := mygo.App.Name()
	sep := mygo.Separator
	return &mygo.MenuItem{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
		{Role: mygo.RoleAbout, Label: i18n.F("About %s", name)},
		sep(), settings, sep(),
		{Role: mygo.RoleServices, Label: i18n.T("Services")}, sep(),
		{Role: mygo.RoleHide, Label: i18n.F("Hide %s", name)},
		{Role: mygo.RoleHideOthers, Label: i18n.T("Hide Others")},
		{Role: mygo.RoleUnhide, Label: i18n.T("Show All")}, sep(),
		{Role: mygo.RoleQuit, Label: i18n.F("Quit %s", name)},
	}}
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
	more := plain("workbench.action.openRecent", i18n.T("More…"))
	return append(items, more)
}
