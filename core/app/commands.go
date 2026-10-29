package app

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
)

// Register 注册（或覆盖）一个命令。
func (e *Editor) Register(c Command) { e.commands[c.ID] = c }

// defaultBindings 是内置的 VS Code 预设，加上 Mayoi 的键位（列、标签、行操作）。
// "mod" 在 Terminal.app 里是 Ctrl，在支持 kitty 键盘协议的终端（Ghostty）里同时是 Cmd 和 Ctrl，
// 在 GUI 里是 Cmd。键位与 docs/DESIGN.md §5.5 保持一致。
func defaultBindings() map[keymap.Context]map[string]string {
	g := map[string]string{
		"ctrl+q": "workbench.action.quit", "cmd+q": "workbench.action.quit",
		"mod+s":  "workbench.action.files.save",
		"mod+p":  "workbench.action.quickOpen",
		"ctrl+r": "workbench.action.openRecent",
		"ctrl+g": "workbench.action.gotoLine", "mod+shift+o": "workbench.action.gotoSymbol", "ctrl+k o": "workbench.action.gotoSymbol",
		// VS Code 在 macOS 上的后退、前进；Windows、Linux 上是 Alt+←→（见 platform.go）
		"ctrl+-": "workbench.action.navigateBack", "ctrl+shift+-": "workbench.action.navigateForward",
		// 命令面板：Terminal.app 分不清 Ctrl+Shift+P 和 Ctrl+P，所以回退键是 Ctrl+K P
		"ctrl+k p": "workbench.action.showCommands", "f1": "workbench.action.showCommands",
		"cmd+shift+p": "workbench.action.showCommands",
		"mod+b":       "workbench.action.toggleSidebarVisibility",
		"ctrl+k e":    "workbench.view.explorer", "cmd+shift+e": "workbench.view.explorer",
		"mod+w":    "workbench.action.closeActiveEditor",
		"ctrl+k ]": "workbench.action.nextEditor", "ctrl+k [": "workbench.action.previousEditor",
		"ctrl+pagedown": "workbench.action.nextEditor", "ctrl+pageup": "workbench.action.previousEditor",
		"cmd+shift+]": "workbench.action.nextEditor", "cmd+shift+[": "workbench.action.previousEditor",
		"ctrl+tab": "workbench.action.nextEditor", "ctrl+shift+tab": "workbench.action.previousEditor",
		// Mayoi：重新打开关掉的标签、向右拆分、在列之间移动焦点
		"mod+shift+t": "workbench.action.reopenClosedEditor",
		"mod+shift+v": "yan.svg.togglePreview",
		"mod+shift+m": "review.comment", "ctrl+k m": "review.comment",
		"alt+z": "yan.toggleTabWordWrap", "cmd+shift+n": "workbench.action.newWindow",
		"cmd+alt+o": "yan.openFolderNewWindow",
		"cmd+o":     "workbench.action.files.openFolder", "cmd+n": "workbench.action.files.newUntitledFile",
		"cmd+shift+s": "workbench.action.files.saveAs",
		"cmd+=":       "workbench.action.zoomIn", "cmd+-": "workbench.action.zoomOut", "cmd+0": "workbench.action.zoomReset",
		"ctrl+k t": "workbench.action.terminal.toggleTerminal",
		"mod+\\":   "workbench.action.splitEditor", "ctrl+k \\": "workbench.action.splitEditor",
		"ctrl+shift+g": "workbench.view.scm", "ctrl+k g": "workbench.view.scm",
		"mod+shift+f": "workbench.action.findInFiles", "ctrl+k f": "workbench.action.findInFiles",
		"mod+shift+h": "workbench.action.replaceInFiles",
		"mod+enter":   "git.commit", "ctrl+k c": "git.commit",
		"cmd+alt+right": "workbench.action.focusNextGroup", "cmd+alt+left": "workbench.action.focusPreviousGroup",
		"ctrl+k right": "workbench.action.focusNextGroup", "ctrl+k left": "workbench.action.focusPreviousGroup",
		// VS Code 是 Ctrl+`；Terminal.app 里 Ctrl+` 可能发不出来，所以另备 Ctrl+K T。
		"ctrl+`": "workbench.action.terminal.toggleTerminal", "ctrl+shift+`": "workbench.action.terminal.new",
	}
	projectBindings(g)
	// Cmd+1…9：像浏览器一样切到第几个标签（9 是最后一个）。
	for i := 1; i <= 9; i++ {
		g["cmd+"+strconv.Itoa(i)] = "workbench.action.openEditorAtIndex" + strconv.Itoa(i)
	}
	return map[keymap.Context]map[string]string{
		keymap.CtxGlobal: g,
		keymap.CtxEditor: {
			"left": "cursorLeft", "right": "cursorRight", "up": "cursorUp", "down": "cursorDown",
			"shift+left": "cursorLeftSelect", "shift+right": "cursorRightSelect",
			"shift+up": "cursorUpSelect", "shift+down": "cursorDownSelect",
			"home": "cursorHome", "end": "cursorEnd", "shift+home": "cursorHomeSelect", "shift+end": "cursorEndSelect",
			"cmd+left": "cursorHome", "cmd+right": "cursorEnd", "shift+cmd+left": "cursorHomeSelect", "shift+cmd+right": "cursorEndSelect",
			"ctrl+home": "cursorTop", "ctrl+end": "cursorBottom", "cmd+up": "cursorTop", "cmd+down": "cursorBottom",
			"shift+cmd+up": "cursorTopSelect", "shift+cmd+down": "cursorBottomSelect",
			"pageup": "cursorPageUp", "pagedown": "cursorPageDown",
			"shift+pageup": "cursorPageUpSelect", "shift+pagedown": "cursorPageDownSelect",
			// Terminal.app 默认把 Option+←/→ 发成 ESC b / ESC f，tcell 报告为 Alt+B / Alt+F。
			"alt+left": "cursorWordLeft", "alt+right": "cursorWordEndRight",
			"alt+b": "cursorWordLeft", "alt+f": "cursorWordEndRight",
			"shift+alt+left": "cursorWordLeftSelect", "shift+alt+right": "cursorWordEndRightSelect",
			"backspace": "deleteLeft", "delete": "deleteRight", "alt+backspace": "deleteWordLeft",
			"cmd+backspace": "deleteAllLeft",
			"enter":         "yan.edit.newline", "tab": "tab", "shift+tab": "outdent",
			// Mayoi 的行操作（与 VS Code 相同的键位）
			"mod+]": "editor.action.indentLines", "mod+[": "editor.action.outdentLines",
			"mod+/":  "editor.action.commentLine",
			"alt+up": "editor.action.moveLinesUpAction", "alt+down": "editor.action.moveLinesDownAction",
			"shift+alt+up": "editor.action.copyLinesUpAction", "shift+alt+down": "editor.action.copyLinesDownAction",
			"mod+shift+k": "editor.action.deleteLines",
			"mod+enter":   "editor.action.insertLineAfter", "mod+shift+enter": "editor.action.insertLineBefore",
			"mod+l":        "expandLineSelection",
			"mod+shift+\\": "editor.action.jumpToBracket",
			"f12":          "editor.action.revealDefinition",
			"esc":          "removeSecondaryCursors",
			"mod+z":        "undo", "ctrl+y": "redo", "mod+shift+z": "redo",
			"mod+a": "editor.action.selectAll",
			"mod+c": "editor.action.clipboardCopyAction",
			"mod+x": "editor.action.clipboardCutAction",
			"mod+v": "editor.action.clipboardPasteAction",
			"mod+d": "editor.action.addSelectionToNextFindMatch",
			// 查找（Ctrl+G 留给转到行，所以下一个匹配用 Cmd+G 或 F3）
			"mod+f": "actions.find", "alt+mod+f": "editor.action.startFindReplaceAction",
			"cmd+g": "editor.action.nextMatchFindAction", "cmd+shift+g": "editor.action.previousMatchFindAction",
			"f3": "editor.action.nextMatchFindAction", "shift+f3": "editor.action.previousMatchFindAction",
			"alt+mod+c": "toggleFindCaseSensitive", "alt+mod+w": "toggleFindWholeWord", "alt+mod+r": "toggleFindRegex",
			"alt+mod+enter": "editor.action.replaceAll",
			"alt+f5":        "workbench.action.editor.nextChange", "shift+alt+f5": "workbench.action.editor.previousChange",
			"alt+f3": "editor.action.dirtydiff.next", "shift+alt+f3": "editor.action.dirtydiff.previous",
			"ctrl+k r": "git.revertSelectedRanges", "ctrl+k s": "git.stageSelectedRanges",
		},
		keymap.CtxTree: {
			"up": "list.focusUp", "down": "list.focusDown", "left": "list.collapse", "right": "list.expand",
			"enter": "list.select", "space": "list.toggleExpand", "esc": "list.clear",
			"mod+c": "filesExplorer.copy", "mod+v": "filesExplorer.paste",
			"pageup": "list.focusPageUp", "pagedown": "list.focusPageDown",
			"tab": "list.focusMessage", "delete": "list.delete", "mod+backspace": "list.delete", "f2": "renameFile",
			"mod+a": "list.selectAll", "shift+up": "list.expandSelectionUp", "shift+down": "list.expandSelectionDown",
		},
	}
}

func registerBuiltins(e *Editor) {
	cmds := []struct {
		id, title string
		run       func(*Editor) error
	}{
		{"workbench.action.files.save", "文件：保存", (*Editor).save},
		{cmdOverwrite, "文件：覆盖磁盘上的版本", (*Editor).overwrite},
		{cmdRevert, "文件：还原为磁盘上的版本", (*Editor).revert},
		{"workbench.action.files.saveAll", "文件：全部保存", (*Editor).saveAll},
		{"workbench.action.quit", "退出", (*Editor).quitCmd},
		{"undo", "撤销", func(e *Editor) error { return e.history(true) }},
		{"redo", "重做", func(e *Editor) error { return e.history(false) }},
		{"editor.action.selectAll", "全选", func(e *Editor) error {
			return e.withDoc(func(b *edit.Buffer) {
				b.SetSelections(edit.Single(edit.Range{Anchor: 0, Head: b.Snapshot().Len(), GoalX: -1}))
			})
		}},
		{"removeSecondaryCursors", "只保留主光标", func(e *Editor) error {
			if e.closePeek() {
				return nil // Esc：先关掉改动的框
			}
			if t := e.cur(); t != nil && len(t.doc.buf.Selections().Ranges) == 1 && e.find.open {
				e.closeFind() // Esc：没有多光标时关掉查找框
				return nil
			}
			return e.withDoc(func(b *edit.Buffer) { b.SetSelections(edit.Single(b.Selections().Main())) })
		}},
		{"actions.find", "查找", func(e *Editor) error { return e.openFind(false) }},
		{"editor.action.startFindReplaceAction", "替换", func(e *Editor) error { return e.openFind(true) }},
		{"editor.action.nextMatchFindAction", "查找下一个", func(e *Editor) error { return e.step(1) }},
		{"editor.action.previousMatchFindAction", "查找上一个", func(e *Editor) error { return e.step(-1) }},
		{"toggleFindCaseSensitive", "查找：区分大小写", func(e *Editor) error { return e.toggleFindOption(&e.find.matchCase) }},
		{"toggleFindWholeWord", "查找：全字匹配", func(e *Editor) error { return e.toggleFindOption(&e.find.word) }},
		{"toggleFindRegex", "查找：使用正则表达式", func(e *Editor) error { return e.toggleFindOption(&e.find.regex) }},
		{"editor.action.replaceOne", "替换当前匹配", (*Editor).replaceOne},
		{"editor.action.replaceAll", "全部替换", (*Editor).replaceAll},
		{"yan.find.close", "关闭查找框", func(e *Editor) error { e.closeFind(); return nil }},
		{"workbench.action.editor.nextChange", "Git：下一个改动", func(e *Editor) error { return e.stepChange(1) }},
		{"workbench.action.editor.previousChange", "Git：上一个改动", func(e *Editor) error { return e.stepChange(-1) }},
		{"yan.git.toggleBlame", "Git：切换行内 blame", func(e *Editor) error {
			e.cfg.Blame = !e.cfg.Blame
			e.repository().git.blame = blameState{}
			return nil
		}},
		{"yan.edit.newline", "换行", func(e *Editor) error {
			if t := e.cur(); t != nil && t.diff != nil {
				return e.diffOpen(t) // 改动视图里回车：展开折叠，或打开文件跳到这一行
			}
			return e.newline()
		}},
		{"git.openChange", "Git：打开改动", func(e *Editor) error {
			t := e.cur()
			if t == nil {
				return errors.New("没有打开的文件")
			}
			if t.diff != nil {
				return e.openPath(t.diff.path)
			}
			return e.openChange(t.doc.buf.Path(), false)
		}},
		{"tab", "缩进", (*Editor).tabKey},
		{"outdent", "反缩进", func(e *Editor) error { return e.indentLines(true) }},
		{"editor.action.indentLines", "行：缩进", func(e *Editor) error { return e.indentLines(false) }},
		{"editor.action.outdentLines", "行：反缩进", func(e *Editor) error { return e.indentLines(true) }},
		{"editor.action.commentLine", "切换行注释", (*Editor).toggleComment},
		{"editor.action.moveLinesUpAction", "向上移动行", func(e *Editor) error { return e.moveLines(false) }},
		{"editor.action.moveLinesDownAction", "向下移动行", func(e *Editor) error { return e.moveLines(true) }},
		{"editor.action.copyLinesUpAction", "向上复制行", func(e *Editor) error { return e.copyLines(false) }},
		{"editor.action.copyLinesDownAction", "向下复制行", func(e *Editor) error { return e.copyLines(true) }},
		{"editor.action.deleteLines", "删除行", (*Editor).deleteLines},
		{"editor.action.insertLineAfter", "在下面插入行", func(e *Editor) error { return e.insertLine(false) }},
		{"editor.action.insertLineBefore", "在上面插入行", func(e *Editor) error { return e.insertLine(true) }},
		{"expandLineSelection", "选中行", (*Editor).selectLines},
		{"editor.action.sortLinesAscending", "行：升序排列", func(e *Editor) error { return e.sortLines(false) }},
		{"editor.action.sortLinesDescending", "行：降序排列", func(e *Editor) error { return e.sortLines(true) }},
		{"editor.action.transformToCamelcase", "转换为驼峰命名", (*Editor).camelCase},
		{"yan.indent.choose", "选择缩进方式…", (*Editor).chooseIndent},
		{"yan.eol.choose", "转换行尾序列…", (*Editor).chooseEOL},
		{"yan.eol.lf", "行尾序列：LF", func(e *Editor) error { return e.setEOL(text.EOLLF) }},
		{"yan.eol.crlf", "行尾序列：CRLF", func(e *Editor) error { return e.setEOL(text.EOLCRLF) }},
		{"editor.action.indentUsingTabs", "缩进：使用制表符", func(e *Editor) error { return e.setIndent(indentStyle{tabs: true, size: max(e.cfg.TabWidth, 1)}) }},
		{"yan.indentUsingSpaces2", "缩进：使用 2 个空格", func(e *Editor) error { return e.setIndent(indentStyle{size: 2}) }},
		{"yan.indentUsingSpaces4", "缩进：使用 4 个空格", func(e *Editor) error { return e.setIndent(indentStyle{size: 4}) }},
		{"yan.indentUsingSpaces8", "缩进：使用 8 个空格", func(e *Editor) error { return e.setIndent(indentStyle{size: 8}) }},
		{"editor.action.jumpToBracket", "跳到配对的括号", (*Editor).jumpToBracket},
		{"deleteLeft", "向左删除", func(e *Editor) error {
			if t := e.cur(); t != nil && e.writable(t) && e.deletePair(t) {
				e.reveal()
				return nil
			}
			return e.editDoc(func(b *edit.Buffer) { b.DeleteBackward() })
		}},
		{"deleteRight", "向右删除", func(e *Editor) error {
			return e.editDoc(func(b *edit.Buffer) { b.DeleteForward() })
		}},
		{"deleteWordLeft", "向左删除单词", func(e *Editor) error {
			return e.editDoc(func(b *edit.Buffer) {
				e.moveBy(b, true, func(s text.Snapshot, r edit.Range) int { return edit.WordLeft(s, r.Head) })
				b.DeleteBackward()
			})
		}},
		{"deleteAllLeft", "删除到行首", func(e *Editor) error {
			return e.editDoc(func(b *edit.Buffer) {
				e.moveBy(b, true, func(s text.Snapshot, r edit.Range) int { return s.LineStart(s.LineAt(r.Head)) })
				b.DeleteBackward()
			})
		}},
		{"editor.action.clipboardCopyAction", "复制", func(e *Editor) error { return e.copy(false) }},
		{"editor.action.clipboardCutAction", "剪切", func(e *Editor) error { return e.copy(true) }},
		{"editor.action.clipboardPasteAction", "粘贴", (*Editor).paste},
		{"editor.action.addSelectionToNextFindMatch", "选中下一个匹配项", (*Editor).addNextMatch},
		{"workbench.action.terminal.toggleTerminal", "终端：切换终端", (*Editor).shell},
		{"workbench.action.terminal.new", "终端：新建终端", func(e *Editor) error {
			return e.openTerminal(e.terminalDir())
		}},
		{"view.terminalLocation", "终端：默认在编辑区新建", (*Editor).toggleTerminalLocation},
		{"openInIntegratedTerminal", "资源管理器：在终端中打开", (*Editor).openTreeTerminal},
		{"workbench.action.createTerminalEditor", "终端：在编辑区新建终端", (*Editor).newTerminalEditor},
		{"workbench.action.quickOpen", "转到文件…", func(e *Editor) error { return e.openQuickOpen("") }},
		{"workbench.action.gotoLine", "转到行…", func(e *Editor) error { return e.openQuickOpen(":") }},
		{"workbench.action.gotoSymbol", "转到符号…", func(e *Editor) error { return e.openQuickOpen("@") }},
		{"editor.action.revealDefinition", "转到定义", (*Editor).revealDefinition},
		{"workbench.action.navigateBack", "后退", func(e *Editor) error { return e.navigate(true) }},
		{"workbench.action.navigateForward", "前进", func(e *Editor) error { return e.navigate(false) }},
		{"workbench.action.openRecent", "打开最近的文件夹…", (*Editor).openRecentPicker},
		{"explorer.newFile", "资源管理器：新建文件…", func(e *Editor) error { return e.newEntry(false) }},
		{"explorer.newFolder", "资源管理器：新建文件夹…", func(e *Editor) error { return e.newEntry(true) }},
		{"renameFile", "资源管理器：改名…", (*Editor).renameEntry},
		{"deleteFile", "资源管理器：移到" + e.trashName(), (*Editor).trashEntry},
		{"filesExplorer.copy", "资源管理器：复制", (*Editor).copyFiles},
		{"filesExplorer.paste", "资源管理器：粘贴", (*Editor).pasteFiles},
		{"copyFilePath", "复制路径", func(e *Editor) error { return e.copyPath(false) }},
		{"copyRelativeFilePath", "复制相对路径", func(e *Editor) error { return e.copyPath(true) }},
		{"revealFileInOS", "在" + FileManager(e.profile.OS) + "中显示", (*Editor).revealInOS},
		{"markdown.showPreviewToSide", "在右侧打开预览（SVG、Markdown、Mermaid）", (*Editor).previewToSide},
		{"yan.svg.togglePreview", "切换预览（SVG、Markdown、Mermaid）", (*Editor).togglePreview},
		{"yan.toggleVim", "切换 Vim 模式", (*Editor).toggleVim},
		{"editor.action.toggleWordWrap", "切换自动换行", (*Editor).toggleWordWrap},
		{"yan.toggleTabWordWrap", "切换这个标签的自动换行", (*Editor).toggleTabWrap},
		{"workbench.action.newWindow", "新建窗口", func(e *Editor) error {
			if h, ok := e.host.(interface{ NewWindow() }); ok {
				h.NewWindow()
				return nil
			}
			return errors.New("终端版只有一个窗口：在另一个终端里运行 yan")
		}},
		{"yan.installCLI", "安装 yan 命令…", func(e *Editor) error {
			if h, ok := e.host.(interface{ InstallCLI() }); ok {
				h.InstallCLI()
				return nil
			}
			return errors.New("请在窗口版的文件菜单安装命令")
		}},
		{"yan.openFolderNewWindow", "在新窗口中打开文件夹…", func(e *Editor) error {
			if h, ok := e.host.(interface{ OpenFolderNewWindow() }); ok {
				h.OpenFolderNewWindow()
				return nil
			}
			return errors.New("在另一个终端里运行 yan 目录")
		}},
		{"workbench.action.files.openFolder", "打开文件夹…", func(e *Editor) error {
			if h, ok := e.host.(interface{ OpenFolderDialog() }); ok {
				h.OpenFolderDialog()
				return nil
			}
			return errors.New("在终端里用「yan 目录」打开文件夹；最近打开的文件夹按 Ctrl+R")
		}},
		{"workbench.action.files.newUntitledFile", "新建未命名文件", (*Editor).newUntitled},
		{cmdMaximize, "最大化／还原编辑区（合并视图三栏一起）", (*Editor).toggleMaximize},
		{"workbench.action.files.saveAs", "另存为…", (*Editor).saveAs},
		{"workbench.action.files.newFile", "新建文件…", func(e *Editor) error {
			if h, ok := e.host.(interface{ NewFileDialog(dir string) }); ok {
				dir := e.root
				if t := e.cur(); t != nil && t.diff == nil && t.doc.buf.Path() != "" {
					dir = filepath.Dir(t.doc.buf.Path())
				}
				h.NewFileDialog(dir)
				return nil
			}
			return errors.New("在终端里用快速打开以外的方式新建：yan 文件名")
		}},
		{"workbench.action.showCommands", "显示所有命令", (*Editor).openPalette},
		{"workbench.action.toggleSidebarPosition", "移动侧边栏到另一侧", (*Editor).toggleSidebarPosition},
		{"workbench.action.toggleSidebarVisibility", "切换侧边栏可见性", (*Editor).toggleSidebar},
		{"workbench.view.explorer", "显示资源管理器", (*Editor).focusExplorer},
		{"workbench.action.focusActiveEditorGroup", "聚焦到编辑器", func(e *Editor) error { e.area = areaEditor; return nil }},
		{"workbench.action.closeActiveEditor", "关闭编辑器", (*Editor).closeEditor},
		{"workbench.action.closeOtherEditors", "关闭其他编辑器", func(e *Editor) error {
			c := e.column()
			return e.closeTabsIn(c, c.tab())
		}},
		{"workbench.action.closeEditorsInGroup", "关闭这一列的所有编辑器", func(e *Editor) error { return e.closeTabsIn(e.column(), nil) }},
		{"workbench.action.reopenClosedEditor", "重新打开关闭的编辑器", (*Editor).reopenClosed},
		{"view.lockColumn", "锁定/解锁这一列", (*Editor).toggleColumnLock},
		{"yan.guide.open", "学习：功能导览", (*Editor).openGuide},
		{"yan.guide.skip", "学习：跳过这一步", (*Editor).skipStep},
		{"yan.guide.nextTip", "学习：下一条提示", (*Editor).nextTip},
		{"yan.guide.reset", "学习：清空课程进度", (*Editor).resetGuide},
		{"yan.guide.back", "学习：回到上一步", (*Editor).backStep},
		{"yan.guide.restart", "学习：重新开始这一课", (*Editor).restartLesson},
		{"yan.guide.leave", "学习：离开练习文件夹，回到原来的文件夹", (*Editor).leavePracticeFolder},
		{"yan.guide.dismissOffer", "学习：不再提示功能导览", (*Editor).dismissGuideOffer},
		{"yan.project.customize", "自定义项目标识与颜色…", (*Editor).customizeProject},
		{"workbench.action.positionPanelBottom", "终端面板放在底部", func(e *Editor) error { return e.setPanelPosition("") }},
		{"workbench.action.positionPanelLeft", "终端面板放在左侧", func(e *Editor) error { return e.setPanelPosition("left") }},
		{"workbench.action.positionPanelRight", "终端面板放在右侧", func(e *Editor) error { return e.setPanelPosition("right") }},
		{"yan.config.importVSCode", "导入 VS Code / Cursor 的设置和快捷键", (*Editor).importVSCode},
		{"yan.config.importSettingsFile", "从文件导入设置（settings.json）", (*Editor).importSettingsFile},
		{"view.terminalLockRight", "终端：移到右侧锁定列", (*Editor).moveTerminalToLockedRight},
		{"workbench.action.nextEditor", "打开下一个编辑器", func(e *Editor) error { return e.cycleTab(1) }},
		{"workbench.action.previousEditor", "打开上一个编辑器", func(e *Editor) error { return e.cycleTab(-1) }},
		{"workbench.action.splitEditor", "向右拆分编辑器", (*Editor).splitRight},
		{"workbench.action.focusNextGroup", "聚焦到下一列", func(e *Editor) error { return e.focusColumnBy(1) }},
		{"workbench.action.focusPreviousGroup", "聚焦到上一列", func(e *Editor) error { return e.focusColumnBy(-1) }},
		{"workbench.action.moveEditorToNextGroup", "把编辑器移到下一列", func(e *Editor) error { return e.moveTabBy(1) }},
		{"workbench.action.moveEditorToPreviousGroup", "把编辑器移到上一列", func(e *Editor) error { return e.moveTabBy(-1) }},
		{"workbench.action.keepEditor", "保留编辑器（预览标签转正）", func(e *Editor) error {
			if t := e.cur(); t != nil {
				e.keep(t)
			}
			return nil
		}},
		{"workbench.files.action.refreshFilesExplorer", "刷新资源管理器", func(e *Editor) error {
			e.checkDisk()
			e.reindex()
			return nil
		}},
		{"workbench.files.action.showActiveFileInExplorer", "在资源管理器中显示当前文件", func(e *Editor) error {
			if e.ws == nil {
				return errors.New("没有打开文件夹")
			}
			e.sidebar, e.area = true, areaTree
			e.revealInTree()
			return nil
		}},
		{"yan.config.open", "打开用户配置文件", (*Editor).openConfig},
		{cmdOpenKeybindings, "首选项：打开键盘快捷方式（keybindings.json）", (*Editor).openKeybindings},
		{"yan.toggleThemeEffects", "主题：切换装饰效果", (*Editor).toggleThemeEffects},
		{"workbench.action.selectTheme", "首选项：颜色主题…", (*Editor).openThemePicker},
		{"yan.theme.mayoi", "主题：Mayoi", func(e *Editor) error { return e.setTheme("") }},
		{"yan.theme.matrix", "主题：The Matrix", func(e *Editor) error { return e.setTheme("matrix") }},
		{"yan.theme.claude", "主题：Claude", func(e *Editor) error { return e.setTheme("claude") }},
		{"yan.appearance.system", "外观：跟随系统", func(e *Editor) error { return e.setAppearance("") }},
		{"yan.appearance.light", "外观：浅色", func(e *Editor) error { return e.setAppearance("light") }},
		{"yan.appearance.dark", "外观：深色", func(e *Editor) error { return e.setAppearance("dark") }},
		{"yan.font.family", "字体：选择代码字体…", (*Editor).chooseFont},
		{"yan.font.size", "字体：代码字号…", (*Editor).chooseFontSize},
		{"yan.font.lineHeight", "字体：行高…", (*Editor).chooseLineHeight},
		{"workbench.action.zoomIn", "放大", func(e *Editor) error { return e.zoom(1) }},
		{"workbench.action.zoomOut", "缩小", func(e *Editor) error { return e.zoom(-1) }},
		{"workbench.action.zoomReset", "重置缩放", func(e *Editor) error { return e.zoom(0) }},
		{"workbench.action.toggleAutoSave", "切换自动保存", (*Editor).toggleAutoSave},
		{"workbench.action.findInFiles", "在文件中搜索", func(e *Editor) error { return e.showSearch(false) }},
		{"workbench.action.replaceInFiles", "在文件中替换", func(e *Editor) error { return e.showSearch(true) }},
		{"workbench.view.search", "显示搜索", func(e *Editor) error { return e.showSearch(false) }},
		{"search.action.replaceAll", "搜索：全部替换", func(e *Editor) error { return e.replaceSearch(true) }},
		{"workbench.view.scm", "显示源代码管理", func(e *Editor) error {
			if e.ws == nil {
				return errors.New("没有打开文件夹")
			}
			e.showSide(sideSCM)
			return nil
		}},
		{"git.commit", "Git：提交", func(e *Editor) error { return e.commit(false) }},
		{"git.commitAndPush", "Git：提交并推送", func(e *Editor) error { return e.commit(true) }},
		{"git.commitAmend", "Git：修改上次提交", (*Editor).startAmend},
		{"git.undoCommit", "Git：撤销上次提交", (*Editor).undoCommit},
		{"editor.action.dirtydiff.next", "Git：看下一处改动", func(e *Editor) error { return e.stepPeek(1) }},
		{"editor.action.dirtydiff.previous", "Git：看上一处改动", func(e *Editor) error { return e.stepPeek(-1) }},
		{"git.revertSelectedRanges", "Git：还原这处改动", (*Editor).revertHunk},
		{"review.comment", "评审：添加或编辑意见", (*Editor).addReviewComment},
		{"review.save", "评审：保存意见", (*Editor).saveReview},
		{"review.cancel", "评审：取消编辑", (*Editor).cancelReview},
		{"review.delete", "评审：删除意见", (*Editor).deleteReview},
		{"review.clear", "评审：清空意见", (*Editor).clearReview},
		{"review.undo", "评审：撤销删除或清空", (*Editor).undoReview},
		{"review.copy", "评审：复制全部意见", (*Editor).copyReview},
		{"review.copyExcerpt", "评审：复制代码片段与位置", (*Editor).copyReviewExcerpt},
		{"review.send", "评审：发送意见到终端", (*Editor).sendReview},
		{"review.list", "评审：查看全部意见", (*Editor).reviewList},
		{"review.next", "评审：下一条意见", func(e *Editor) error { return e.stepReview(1) }},
		{"review.previous", "评审：上一条意见", func(e *Editor) error { return e.stepReview(-1) }},
		{"yan.git.exportPatch", "Git：导出提交补丁…", func(e *Editor) error {
			return e.commitTarget(func(e *Editor, c git.Commit) error { return e.commitPatch(c, true) })
		}},
		{"yan.git.copyPatch", "Git：复制提交改动补丁", func(e *Editor) error {
			return e.commitTarget(func(e *Editor, c git.Commit) error { return e.commitPatch(c, false) })
		}},
		{"yan.git.compareHeadToCommit", "Git：比较 HEAD → 选中提交", func(e *Editor) error {
			return e.commitTarget(func(e *Editor, c git.Commit) error { return e.compareRevisions("HEAD", c.Hash) })
		}},
		{"yan.git.compareCommitToHead", "Git：比较选中提交 → HEAD", func(e *Editor) error {
			return e.commitTarget(func(e *Editor, c git.Commit) error { return e.compareRevisions(c.Hash, "HEAD") })
		}},
		{"yan.git.selectCompare", "Git：选中提交作为比较起点", func(e *Editor) error {
			return e.commitTarget(func(e *Editor, c git.Commit) error { return e.selectComparison(c) })
		}},
		{"yan.git.compareSelected", "Git：与选定起点比较", func(e *Editor) error {
			return e.commitTarget(func(e *Editor, c git.Commit) error { return e.compareSelectedCommit(c) })
		}},
		{"yan.git.openCommitRemote", "Git：在远端打开提交", func(e *Editor) error {
			return e.commitTarget(func(e *Editor, c git.Commit) error { return e.commitRemote(c) })
		}},
		{"git.toggleCommitFiles", "Git：显示或隐藏提交文件列表", (*Editor).toggleCommitFiles},
		{"git.openAllChanges", "Git：打开所有改动", (*Editor).openAllChanges},
		{"toggle.diff.renderSideBySide", "改动视图：换排法（自动、并排、一栏）", (*Editor).toggleDiffLayout},
		{"yan.diff.whitespace", "改动视图：比较空白（全部、忽略行尾、忽略全部、忽略空行）…", (*Editor).chooseDiffWhitespace},
		{"yan.diff.compareFiles", "比较两个文件…", (*Editor).compareFiles},
		{"yan.diff.editBeside", "改动视图：在旁边编辑工作区这一边（i）", func(e *Editor) error {
			if t := e.cur(); t != nil && t.diff != nil {
				return e.diffEditBeside(t)
			}
			return errors.New("先打开一个改动视图")
		}},
		{"yan.diff.toggleSyncScroll", "改动视图：并排两边一起滚／各自滚", (*Editor).toggleSyncScroll},
		{"yan.diff.toggleAlignedRows", "改动视图：并排两边的行对齐／各自排", (*Editor).toggleAlignedRows},
		{"yan.diff.toggleFullFiles", "改动视图：显示整个文件／只显示改动附近", (*Editor).toggleFullFiles},
		{"yan.merge.open", "合并：打开三方合并（冲突文件）", (*Editor).openMergeCurrent},
		{"yan.merge.acceptOurs", "合并：这处冲突用我们的", func(e *Editor) error {
			return e.mergeCommand(func(e *Editor, mv *mergeView, k int) { e.mergeTake(mv, k, false) })
		}},
		{"yan.merge.acceptTheirs", "合并：这处冲突用他们的", func(e *Editor) error {
			return e.mergeCommand(func(e *Editor, mv *mergeView, k int) { e.mergeTake(mv, k, true) })
		}},
		{"yan.merge.ignoreOurs", "合并：这处冲突忽略我们的", func(e *Editor) error {
			return e.mergeCommand(func(e *Editor, mv *mergeView, k int) { e.mergeIgnore(mv, k, false) })
		}},
		{"yan.merge.ignoreTheirs", "合并：这处冲突忽略他们的", func(e *Editor) error {
			return e.mergeCommand(func(e *Editor, mv *mergeView, k int) { e.mergeIgnore(mv, k, true) })
		}},
		{"yan.merge.resolveOurs", "合并：全部冲突用我们的", func(e *Editor) error {
			return e.mergeCommand(func(e *Editor, mv *mergeView, _ int) { e.mergeResolveUsing(mv, false) })
		}},
		{"yan.merge.resolveTheirs", "合并：全部冲突用他们的", func(e *Editor) error {
			return e.mergeCommand(func(e *Editor, mv *mergeView, _ int) { e.mergeResolveUsing(mv, true) })
		}},
		{"yan.merge.nextConflict", "合并：下一处冲突", func(e *Editor) error { return e.mergeStepConflict(1) }},
		{"yan.merge.previousConflict", "合并：上一处冲突", func(e *Editor) error { return e.mergeStepConflict(-1) }},
		{"yan.merge.markResolved", "合并：标记为已解决（保存并暂存）", (*Editor).mergeMarkResolved},
		{"yan.merge.restoreConflict", "合并：恢复成冲突…", (*Editor).mergeRestore},
		{"yan.merge.saveAndClose", "合并：保存并关闭三方合并", (*Editor).mergeClose},
		{"yan.merge.files", "合并：冲突文件列表…", (*Editor).showMergeFiles},
		{"yan.merge.applyChanges", "合并：应用两边不冲突的改动", func(e *Editor) error { return e.mergeApplySide(0) }},
		{"yan.merge.applyOursChanges", "合并：应用我们这边不冲突的改动", func(e *Editor) error { return e.mergeApplySide(1) }},
		{"yan.merge.applyTheirsChanges", "合并：应用他们那边不冲突的改动", func(e *Editor) error { return e.mergeApplySide(2) }},
		{"yan.merge.toggleAutoApply", "合并：打开时自动应用不冲突的改动（开关）", (*Editor).toggleMergeAutoApply},
		{"yan.merge.applyAndNext", "合并：应用（标记已解决）并打开下一个冲突文件", (*Editor).mergeApplyAndNext},
		{"yan.diff.revertBlock", "改动视图：还原这处改动（比较两个文件时把左边写进右边）", func(e *Editor) error { return e.diffBlockAction(blockRevert) }},
		{"yan.diff.appendBlock", "改动视图：在这处改动后追加对侧的行", func(e *Editor) error { return e.diffBlockAction(blockAppend) }},
		{"yan.diff.acceptRight", "比较两个文件：把右边这处写进左边", func(e *Editor) error { return e.diffBlockAction(blockAcceptRight) }},
		{"yan.diff.highlight", "改动视图：标出改动的方式（按词、按字、整行）…", (*Editor).chooseDiffHighlight},
		{"yan.diff.context", "改动视图：上下文行数（0～10）…", (*Editor).chooseDiffContext},
		{"workbench.action.compareEditor.nextChange", "改动视图：下一处改动", func(e *Editor) error { return e.diffTabNav((*Editor).diffStepChange, 1) }},
		{"workbench.action.compareEditor.previousChange", "改动视图：上一处改动", func(e *Editor) error { return e.diffTabNav((*Editor).diffStepChange, -1) }},
		{"yan.diff.nextFile", "改动视图：下一个文件", func(e *Editor) error { return e.diffTabNav((*Editor).diffStepFile, 1) }},
		{"yan.diff.previousFile", "改动视图：上一个文件", func(e *Editor) error { return e.diffTabNav((*Editor).diffStepFile, -1) }},
		{"selectForCompare", "资源管理器：选为比较对象", (*Editor).selectForCompare},
		{"compareFiles", "资源管理器：与已选文件比较", (*Editor).compareWithSelected},
		{"git.init", "Git：初始化仓库", (*Editor).initRepo},
		{"yan.scm.moreCommits", "Git：显示更多提交", (*Editor).moreCommits},
		{"yan.scm.chooseRepository", "Git：选择仓库…", (*Editor).chooseRepository},
		{"yan.scm.refreshRepositories", "Git：刷新仓库列表", func(e *Editor) error { e.gitChanged(); return nil }},
		{"yan.scm.toggleTree", "Git：以树的形式查看改动", (*Editor).toggleSCMTree},
		{"yan.scm.commitActions", "Git：对选中的提交…（检出、合并、变基、重置、还原、拣选）", func(e *Editor) error {
			if i, ok := e.scmSelectedCommit(); ok {
				return e.openCommit(i)
			}
			return errors.New("先在源代码管理的提交历史里选中一个提交")
		}},
		{"git.stageSelectedRanges", "Git：暂存这处改动", (*Editor).stageHunk},
		{"yan.hunk.close", "Git：关掉改动的框", func(e *Editor) error { e.closePeek(); return nil }},
		{"yan.scm.toggleCommits", "Git：折起或展开提交历史", func(e *Editor) error { e.SCMToggleSection("commits"); return nil }},
		{"yan.scm.toggleRepositoryPanel", "Git：显示或隐藏仓库面板（分支、远程）", (*Editor).toggleRepoPanel},
		{"yan.scm.toggleAllBranches", "Git：提交历史显示所有分支", (*Editor).toggleAllBranches},
		{"yan.git.continueOp", "Git：继续合并、变基、拣选或还原", (*Editor).continueOp},
		{"yan.git.abortOp", "Git：中止合并、变基、拣选或还原", (*Editor).abortOp},
		{"git.stage", "Git：暂存当前文件", func(e *Editor) error { return e.stageCurrent(false) }},
		{"git.unstage", "Git：取消暂存当前文件", func(e *Editor) error { return e.stageCurrent(true) }},
		{"git.stageAll", "Git：暂存全部更改", func(e *Editor) error {
			if e.repository().git.repo == nil {
				return errors.New("不在 git 仓库里")
			}
			return e.stage([]string{e.repository().git.repo.Root}, false)
		}},
		{"git.unstageAll", "Git：取消暂存全部", func(e *Editor) error {
			if e.repository().git.repo == nil {
				return errors.New("不在 git 仓库里")
			}
			return e.stage([]string{e.repository().git.repo.Root}, true)
		}},
		{"git.push", "Git：推送", (*Editor).push},
		{"git.pull", "Git：拉取", (*Editor).pull},
		{"git.sync", "Git：同步（拉取后推送；没有上游时发布分支）", (*Editor).gitSync},
		{"git.fetch", "Git：抓取", (*Editor).fetch},
		{"git.checkout", "Git：切换分支…", (*Editor).openBranchPicker},
		{"git.renameBranch", "Git：重命名当前分支…", (*Editor).renameBranch},
		{"git.deleteBranch", "Git：删除本地分支…", (*Editor).deleteBranch},
		{"git.addRemote", "Git：添加远程仓库…", (*Editor).addRemote},
		{"git.removeRemote", "Git：删除远程仓库…", (*Editor).removeRemote},
		{"yan.git.setRemoteURL", "Git：修改远程仓库地址…", (*Editor).setRemoteURL},
		{"yan.git.pruneRemote", "Git：清理远程已删除的分支…", (*Editor).pruneRemote},
		{"git.refresh", "Git：刷新", func(e *Editor) error { e.gitChanged(); return nil }},
	}
	for _, c := range cmds {
		e.Register(Command{ID: c.id, Title: c.title, Run: c.run})
	}
	for i := 1; i <= 9; i++ {
		n := i
		e.Register(Command{ID: "workbench.action.openEditorAtIndex" + strconv.Itoa(i), Hidden: true, Run: func(e *Editor) error {
			c := e.column()
			at := n - 1
			if n == 9 {
				at = len(c.tabs) - 1
			}
			e.switchTo(at)
			return nil
		}})
	}
	tree := []struct {
		id  string
		run func(*Editor) error
	}{
		{"list.focusUp", func(e *Editor) error { return e.sideMove(-1) }},
		{"list.selectAll", (*Editor).scmSelectAll},
		{"list.expandSelectionUp", func(e *Editor) error { return e.scmExtend(-1) }},
		{"list.expandSelectionDown", func(e *Editor) error { return e.scmExtend(1) }},
		{"list.focusDown", func(e *Editor) error { return e.sideMove(1) }},
		{"list.focusPageUp", func(e *Editor) error { return e.sideMove(-max(e.sideRows-1, 1)) }},
		{"list.focusPageDown", func(e *Editor) error { return e.sideMove(max(e.sideRows-1, 1)) }},
		{"list.collapse", func(e *Editor) error {
			if e.side == sideSCM {
				return e.scmCommitExpand(false)
			}
			return e.sideOnly(sideExplorer, (*Editor).treeLeft)
		}},
		{"list.expand", func(e *Editor) error {
			if e.side == sideSCM {
				return e.scmCommitExpand(true)
			}
			return e.sideOnly(sideExplorer, (*Editor).treeRight)
		}},
		{"list.select", func(e *Editor) error {
			switch e.side {
			case sideSCM:
				return e.scmOpen(true)
			}
			return e.treeOpen(true)
		}},
		{"list.toggleExpand", func(e *Editor) error {
			switch e.side {
			case sideSCM:
				return e.scmToggleStage()
			}
			return e.treeOpen(false)
		}},
		{"list.focusMessage", func(e *Editor) error {
			if e.side == sideSCM && e.repository().git.repo != nil {
				e.repository().scm.focusMsg = true
			}
			return nil
		}},
		{"git.clean", func(e *Editor) error { return e.sideOnly(sideSCM, (*Editor).scmDiscardSelected) }},
		{"list.clear", (*Editor).clearOrLeaveTree},
		{"list.delete", func(e *Editor) error {
			switch e.side {
			case sideSCM:
				return e.scmDiscardSelected()
			case sideExplorer:
				return e.trashEntry()
			}
			return nil
		}},
	}
	for _, c := range tree {
		e.Register(Command{ID: c.id, Run: c.run, Hidden: true})
	}
	registerMoves(e)
	registerProjects(e)
}

// enterChord 是浮层里鼠标点击候选项时模拟的回车。
var enterChord = keymap.Chord{Key: keymap.KeyEnter}

// keyHints 为每个命令挑一个「在当前前端真的按得出来」的键，给命令面板和菜单显示。
// 终端里优先不含 Cmd 的绑定（Terminal.app 收不到 Cmd），mod 显示成 Ctrl；
// macOS 的窗口版里 mod 是 Cmd，用 macOS 的符号显示；Windows、Linux 的窗口版写成 Ctrl+Shift+P。
func (e *Editor) keyHints(bindings map[keymap.Context]map[string]string, p keymap.Profile) {
	mod := keymap.ModCtrl
	if len(p.Mods) > 0 && p.GUI {
		mod = p.Mods[0]
	}
	own := "" // the platform's own modifier as keymap.Symbols writes it, on macOS windows
	if p.GUI && p.MacKeys() {
		own = "⌘"
	}
	for _, m := range bindings {
		for seq, id := range m {
			if !p.GUI && strings.Contains(seq, "cmd+") {
				continue
			}
			chords, err := keymap.Parse(seq, mod)
			if err != nil {
				continue
			}
			h := chordString(chords)
			if p.GUI && p.MacKeys() {
				h = keymap.Symbols(chords)
			}
			if old, ok := e.keyHint[id]; !ok || betterHint(h, old, own) {
				e.keyHint[id] = h
			}
		}
	}
}

// betterHint reports whether hint h shows a command better than old: one with the platform's
// own modifier (⌘Q rather than ^Q on macOS), else the shorter, else the first in order.
func betterHint(h, old, own string) bool {
	if own != "" && strings.Contains(h, own) != strings.Contains(old, own) {
		return strings.Contains(h, own)
	}
	return len(h) < len(old) || len(h) == len(old) && h < old
}

// hintOr 返回命令的快捷键提示；没有绑定时返回 fallback。
func (e *Editor) hintOr(id, fallback string) string {
	if h := e.keyHint[id]; h != "" {
		return h
	}
	return fallback
}

func (e *Editor) toggleSidebar() error {
	if e.ws == nil {
		return errors.New("没有打开文件夹：用 yan 目录 启动才有资源管理器")
	}
	e.sidebar = !e.sidebar
	if !e.sidebar {
		e.area = areaEditor
	}
	return nil
}

func (e *Editor) focusExplorer() error {
	if e.ws == nil {
		return errors.New("没有打开文件夹：用 yan 目录 启动才有资源管理器")
	}
	e.showSide(sideExplorer) // from Search or Source Control too: the sidebar shows the Explorer
	e.revealInTree()
	return nil
}

// closeEditor 关闭当前标签；有未保存修改时第一次只提醒，再按一次才放弃修改。
func (e *Editor) closeEditor() error {
	c := e.column()
	if c.tab() == nil {
		return nil
	}
	return e.closeTab(c, c.active, false)
}

// openConfig 在编辑器里打开用户配置文件（不存在时先建目录，保存时才写文件）。
func (e *Editor) openConfig() error {
	p := config.UserPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := e.openPath(p); err != nil {
		return err
	}
	if t := e.cur(); t != nil && t.doc.buf.Snapshot().Len() == 0 {
		t.doc.buf.Insert(configTemplate, edit.OriginCommand)
		e.status = "新的配置文件：保存后生效（下次启动时读取）"
	}
	return nil
}

const configTemplate = `# 砚 yan 的用户配置。所有项都可以省略；项目里的 .yan.toml 会覆盖这里。

[editor]
tab_width = 4
line_numbers = true

[explorer]
width = 30

[terminal]
# panel：面板；editor：编辑区。菜单里保存的选择优先。
location = "panel"

# 按键：上下文可用 global、editor、explorer、overlay；命令 ID 与 VS Code 相同（见命令面板）。
# "mod" 在 Terminal.app 里是 Ctrl，在 Ghostty 里 Cmd 和 Ctrl 都行，在窗口版里是 Cmd。值为 "" 表示解绑。
# 也可以用 VS Code 的格式写在旁边的 keybindings.json 里（命令面板：首选项：打开键盘快捷方式），它在这里之后生效。
[keys.editor]
# "ctrl+k k" = "editor.action.deleteLines"
`

// registerMoves 注册所有光标移动命令及其 …Select 版本。
func registerMoves(e *Editor) {
	type mv struct {
		id, title string
		fn        func(s text.Snapshot, r edit.Range) int
	}
	horizontal := []mv{
		{"cursorLeft", "光标左移", func(s text.Snapshot, r edit.Range) int { return edit.PrevBoundary(s, r.Head) }},
		{"cursorRight", "光标右移", func(s text.Snapshot, r edit.Range) int { return edit.NextBoundary(s, r.Head) }},
		{"cursorWordLeft", "按词左移", func(s text.Snapshot, r edit.Range) int { return edit.WordLeft(s, r.Head) }},
		{"cursorWordEndRight", "按词右移", func(s text.Snapshot, r edit.Range) int { return edit.WordRight(s, r.Head) }},
		{"cursorHome", "行首", func(s text.Snapshot, r edit.Range) int { return edit.SmartHome(s, r.Head) }},
		{"cursorEnd", "行尾", func(s text.Snapshot, r edit.Range) int {
			_, end := s.LineRange(s.LineAt(r.Head))
			return end
		}},
		{"cursorTop", "文件首", func(text.Snapshot, edit.Range) int { return 0 }},
		{"cursorBottom", "文件尾", func(s text.Snapshot, _ edit.Range) int { return s.Len() }},
	}
	for _, m := range horizontal {
		for _, sel := range []bool{false, true} {
			id, title := m.id, m.title
			if sel {
				id, title = id+"Select", title+"并选择"
			}
			e.Register(Command{ID: id, Title: title, Hidden: true, Run: func(e *Editor) error {
				return e.withDoc(func(b *edit.Buffer) {
					collapse := !sel && (m.id == "cursorLeft" || m.id == "cursorRight")
					e.moveBy(b, sel, func(s text.Snapshot, r edit.Range) int {
						if collapse && !r.Empty() { // 有选区时左右键先收起选区（VS Code 行为）
							if m.id == "cursorLeft" {
								return r.From()
							}
							return r.To()
						}
						return m.fn(s, r)
					})
				})
			}})
		}
	}
	vertical := []struct {
		id, title string
		lines     func(e *Editor) int
	}{
		{"cursorUp", "光标上移", func(*Editor) int { return -1 }},
		{"cursorDown", "光标下移", func(*Editor) int { return 1 }},
		{"cursorPageUp", "上一页", func(e *Editor) int { return -max(e.pageLines()-1, 1) }},
		{"cursorPageDown", "下一页", func(e *Editor) int { return max(e.pageLines()-1, 1) }},
	}
	for _, m := range vertical {
		for _, sel := range []bool{false, true} {
			id, title := m.id, m.title
			if sel {
				id, title = id+"Select", title+"并选择"
			}
			e.Register(Command{ID: id, Title: title, Hidden: true, Run: func(e *Editor) error {
				return e.withDoc(func(b *edit.Buffer) {
					n := m.lines(e)
					e.moveVertical(b, n, sel)
					if m.id == "cursorPageUp" || m.id == "cursorPageDown" {
						e.scrollTab(e.cur(), n)
					}
				})
			}})
		}
	}
}

// withDoc 对当前标签的 buffer 执行 fn（buffer 里装的就是这个标签的选区），然后让光标保持可见。
func (e *Editor) withDoc(fn func(b *edit.Buffer)) error {
	t := e.cur()
	if t == nil {
		return errors.New("没有打开的文件")
	}
	e.sync()
	fn(t.doc.buf)
	e.reveal()
	return nil
}

// editDoc 与 withDoc 相同，但先检查可写。
func (e *Editor) editDoc(fn func(b *edit.Buffer)) error {
	t := e.cur()
	if t == nil {
		return errors.New("没有打开的文件")
	}
	if !e.writable(t) {
		return nil
	}
	e.armed = ""
	return e.withDoc(fn)
}

// moveBy 移动每个选择区的 Head；extend 为 false 时 Anchor 跟着走（收起选区）。
func (e *Editor) moveBy(b *edit.Buffer, extend bool, target func(text.Snapshot, edit.Range) int) {
	s := b.Snapshot()
	sels := b.Selections()
	out := edit.Selections{Ranges: make([]edit.Range, len(sels.Ranges)), Primary: sels.Primary}
	for i, r := range sels.Ranges {
		h := target(s, r)
		a := h
		if extend {
			a = r.Anchor
		}
		out.Ranges[i] = edit.Range{Anchor: a, Head: h, GoalX: -1}
	}
	b.SetSelections(out)
	b.BreakGroup()
}

// moveVertical 上下移动 n 个视觉行，记住目标 x（GoalX），穿过短行后回到原来的列。
func (e *Editor) moveVertical(b *edit.Buffer, n int, extend bool) {
	s := b.Snapshot()
	w := e.wrapWidth(e.column(), e.cur(), s)
	sels := b.Selections()
	out := edit.Selections{Ranges: make([]edit.Range, len(sels.Ranges)), Primary: sels.Primary}
	for i, r := range sels.Ranges {
		at, hx := e.rowAt(s, w, r.Head)
		x := r.GoalX
		if x < 0 {
			x = int32(hx)
		}
		var h int
		switch to, left := e.stepRows(s, w, at, n); {
		case left < 0:
			h = 0
		case left > 0:
			h = s.Len()
		default:
			ls, _ := e.layoutWrapped(s, to.line, w)
			h = ls + e.scratch.RowByteAt(to.sub, view.Unit(x))
		}
		a := h
		if extend {
			a = r.Anchor
		}
		out.Ranges[i] = edit.Range{Anchor: a, Head: h, GoalX: x}
	}
	b.SetSelections(out)
	// SetSelections 可能合并区间；保住 GoalX 才能连续上下移动。
	b.BreakGroup()
}

func (e *Editor) save() error {
	t := e.cur()
	if t == nil {
		return errors.New("没有打开的文件")
	}
	if t.doc.untitled() {
		return e.saveAs()
	}
	if err := e.saveDoc(t.doc); err != nil {
		return err
	}
	e.gitChanged()
	e.keep(t)
	e.status = "已保存 " + filepath.Base(t.doc.buf.Path())
	if e.isConfigFile(t.doc.buf.Path()) {
		e.reloadConfig()
	}
	return nil
}

// saveDoc 保存文档。磁盘上的文件在编辑器上次读写之后被别的程序改过时不保存（自动保存也一样），
// 两边的内容都留着，等用户选「覆盖」或「还原」。
func (e *Editor) saveDoc(d *document) error {
	if d.untitled() {
		return errUntitled
	}
	if d.buf.Snapshot().Meta().ReadOnly {
		return errors.New("只读文件，不能保存")
	}
	err := d.buf.Save()
	if errors.Is(err, edit.ErrConflict) {
		return e.conflictError(d)
	}
	if err != nil {
		return fmt.Errorf("保存 %s 失败：%w", d.name(), err)
	}
	return nil
}

// conflictError 说明文档和磁盘冲突了、该怎么办。
func (e *Editor) conflictError(d *document) error {
	return userError(filepath.Base(d.buf.Path()) + " 在磁盘上被别的程序改过了，没有保存：用「" +
		e.hintOr(cmdOverwrite, "文件：覆盖磁盘上的版本") + "」写入你的修改，或用「" +
		e.hintOr(cmdRevert, "文件：还原为磁盘上的版本") + "」放弃你的修改")
}

const (
	cmdOverwrite = "yan.files.overwrite"
	cmdRevert    = "workbench.action.files.revert"
)

// overwrite 用编辑器里的内容覆盖磁盘上的文件（磁盘上被改过时，用户明确选择覆盖）。
func (e *Editor) overwrite() error {
	t := e.cur()
	if t == nil {
		return errors.New("没有打开的文件")
	}
	d := t.doc
	if d.untitled() {
		return e.saveAs()
	}
	if d.buf.Snapshot().Meta().ReadOnly {
		return errors.New("只读文件，不能保存")
	}
	if err := d.buf.Overwrite(); err != nil {
		return fmt.Errorf("保存 %s 失败：%w", filepath.Base(d.buf.Path()), err)
	}
	e.gitChanged()
	e.keep(t)
	e.status = "已用编辑器里的内容覆盖 " + filepath.Base(d.buf.Path())
	return nil
}

// revert 放弃未保存的修改，换成磁盘上现在的内容。以差异的形式应用，还能撤销回来。
func (e *Editor) revert() error {
	t := e.cur()
	if t == nil {
		return errors.New("没有打开的文件")
	}
	d := t.doc
	if d.untitled() {
		return userError(d.title + " 还没存过：没有磁盘上的版本可以还原")
	}
	if !d.buf.Exists() {
		return userError(filepath.Base(d.buf.Path()) + " 在磁盘上不存在：没有可还原的内容")
	}
	s, err := text.Load(d.buf.Path())
	if err != nil {
		return err
	}
	e.reloading = true
	d.buf.Reload(s)
	e.reloading = false
	e.status = "已还原为磁盘上的版本（可以撤销）"
	return nil
}

// saveAll 保存所有有修改的文件；有的存不了时其余的照存，报第一个错。
func (e *Editor) saveAll() error {
	n, untitled := 0, 0
	var first error
	for _, d := range e.docs {
		if d.buf.Dirty() && d.untitled() {
			untitled++ // 存到哪里要用户自己选
			continue
		}
		if d.buf.Dirty() {
			if err := e.saveDoc(d); err != nil {
				first = cmp.Or(first, err)
				continue
			}
			n++
		}
	}
	if n > 0 {
		e.gitChanged()
	}
	if first != nil {
		return first
	}
	e.status = fmt.Sprintf("已保存 %d 个文件", n)
	if untitled > 0 {
		e.status += fmt.Sprintf("；%d 个未命名的文件要用「%s」选位置", untitled, e.hintOr("workbench.action.files.saveAs", "另存为"))
	}
	return nil
}

func (e *Editor) quitCmd() error {
	const id = "workbench.action.quit"
	e.saveDirty()
	kept := e.KeepForRestart()
	if kept != nil {
		e.status = "未命名的文件没能留到下次：" + kept.Error()
	}
	for _, d := range e.docs {
		if d.buf.Dirty() && (kept != nil || !d.untitled()) && e.armed != id {
			e.armed = id
			e.status = "有未保存的修改：" + e.hintOr("workbench.action.files.save", "保存") + " 保存，或再按一次 " + e.hintOr(id, "退出") + " 放弃修改并退出"
			return nil
		}
	}
	e.quit = true
	return nil
}

// Dirty 报告有没有未保存的修改（GUI 关窗前问一声）。
func (e *Editor) Dirty() bool {
	return slices.ContainsFunc(e.docs, func(d *document) bool { return d.buf.Dirty() })
}

func (e *Editor) history(undo bool) error {
	if t := e.cur(); t != nil && t.diff != nil {
		e.diffHistory(t, undo)
		return nil
	}
	if mv := mergeOf(e.cur()); mv != nil {
		e.mergeHistory(mv, undo)
		return nil
	}
	return e.withDoc(func(b *edit.Buffer) {
		var ok bool
		if undo {
			_, ok = b.Undo()
		} else {
			_, ok = b.Redo()
		}
		if !ok {
			e.status = map[bool]string{true: "没有可撤销的操作", false: "没有可重做的操作"}[undo]
		}
	})
}

// copy 复制选区；没有选区时复制整行（VS Code 行为）。cut 为真时同时删除。
func (e *Editor) copy(cut bool) error {
	t := e.cur()
	if t != nil && t.diff != nil && t.diff.splitW > 0 && t.doc.buf.Selections().Main().Empty() {
		return e.copySplitLine(t) // 并排的改动视图：复制一边的这一行，不是「左 │ 右」
	}
	if t == nil {
		return nil
	}
	if e.host == nil {
		return errors.New("剪贴板不可用")
	}
	b := t.doc.buf
	s := b.Snapshot()
	sels := b.Selections()
	var parts []string
	var cs text.ChangeSet
	whole := true
	for _, r := range sels.Ranges {
		if !r.Empty() {
			whole = false
		}
	}
	for _, r := range sels.Ranges {
		from, to := r.From(), r.To()
		if whole {
			line := s.LineAt(r.Head)
			from = s.LineStart(line)
			to = s.LineStart(line + 1)
			if line+1 >= s.LineCount() {
				to = s.Len()
			}
		}
		txt := s.String(from, to)
		if whole && !strings.HasSuffix(txt, "\n") {
			txt += "\n"
		}
		parts = append(parts, txt)
		cs = append(cs, text.Change{Start: from, End: to})
	}
	sep := "\n"
	if whole {
		sep = ""
	}
	if err := e.host.SetClipboard(strings.Join(parts, sep)); err != nil {
		return fmt.Errorf("写剪贴板失败：%w", err)
	}
	if cut {
		return e.editDoc(func(b *edit.Buffer) { b.Apply(edit.Transaction{Changes: cs, Origin: edit.OriginCommand}) })
	}
	return nil
}

func (e *Editor) paste() error {
	if e.host == nil {
		return errors.New("剪贴板不可用")
	}
	s, err := e.host.Clipboard()
	if err != nil {
		return fmt.Errorf("读剪贴板失败：%w", err)
	}
	if s != "" {
		e.handleText(TextInput{Text: s, Paste: true})
	}
	return nil
}

// addNextMatch：没有选区时选中光标处的词；有选区时在下一个相同文本处再加一个选区（多光标）。
func (e *Editor) addNextMatch() error {
	return e.withDoc(func(b *edit.Buffer) {
		s := b.Snapshot()
		sels := b.Selections()
		main := sels.Main()
		if main.Empty() {
			a, z := edit.WordAt(s, main.Head)
			if a == z {
				return
			}
			rs := slices.Clone(sels.Ranges) // 不原地改：Ranges 与撤销历史共享底层数组
			rs[sels.Primary] = edit.Range{Anchor: a, Head: z, GoalX: -1}
			b.SetSelections(edit.Selections{Ranges: rs, Primary: sels.Primary})
			return
		}
		needle := s.String(main.From(), main.To())
		hay := s.String(0, s.Len())
		start := main.To()
		for _, r := range sels.Ranges {
			start = max(start, r.To())
		}
		i := strings.Index(hay[start:], needle)
		if i >= 0 {
			i += start
		} else if i = strings.Index(hay, needle); i < 0 {
			return
		}
		for _, r := range sels.Ranges {
			if r.From() == i {
				e.status = "所有匹配项都已选中"
				return
			}
		}
		nr := edit.Range{Anchor: i, Head: i + len(needle), GoalX: -1}
		ranges := append(append([]edit.Range(nil), sels.Ranges...), nr)
		b.SetSelections(edit.Selections{Ranges: ranges, Primary: len(ranges) - 1})
	})
}

// shell 切到项目目录下的交互式 shell；退出 shell 后回到编辑器，状态原样保留。
func (e *Editor) shell() error { return e.shellIn(e.terminalDir()) }

func (e *Editor) shellIn(dir string) error {
	if e.host == nil {
		return errors.New("当前前端不支持终端")
	}
	if err := e.host.Shell(dir); err != nil {
		return fmt.Errorf("打开终端失败：%w", err)
	}
	if h, ok := e.host.(interface{ EmbeddedTerminal() bool }); ok && h.EmbeddedTerminal() {
		return nil // 终端在窗口里的面板中：界面没让出去
	}
	e.status = "已从终端返回"
	e.checkDisk() // 你可能在终端里改了文件、建了新文件
	if e.ws != nil {
		e.reindex()
	}
	return nil
}

// sideMove 在侧边栏当前的列表里上下移动。
func (e *Editor) sideMove(n int) error {
	switch e.side {
	case sideSCM:
		if e.repository().git.repo == nil {
			e.repository().scm.sel = max(0, min(e.repository().scm.sel+n, len(e.repository().git.nested)-1))
			return nil
		}
		e.moveRepositoryRow(n)
		return nil
	}
	return e.treeMove(n)
}

// sideOnly 只在侧边栏显示视图 v 时执行 fn。
func (e *Editor) sideOnly(v sideView, fn func(*Editor) error) error {
	if e.side != v {
		return nil
	}
	return fn(e)
}

// stageCurrent 暂存（或取消暂存）当前标签的文件。
func (e *Editor) stageCurrent(unstage bool) error {
	if t := e.cur(); t != nil {
		if t.diff != nil {
			defer e.bindDiff(t.diff)()
		} else {
			defer e.bindRepository(e.repositoryForPath(t.doc.buf.Path()))()
		}
	}
	t := e.cur()
	if t == nil {
		return errors.New("没有打开的文件")
	}
	path := t.doc.buf.Path()
	if dt := t.diff; dt != nil {
		if dt.commit != nil || dt.fixed {
			return errors.New("历史改动不能暂存")
		}
		if dt.peer != "" {
			return errors.New("这是两个文件的比较：暂存不了")
		}
		path = dt.path
		if dt.changes {
			line := t.doc.buf.Snapshot().LineAt(t.doc.buf.Selections().Main().Head)
			if line < 0 || line >= len(dt.lines) || dt.lines[line].sec < 0 {
				return errors.New("先选择一个改动文件")
			}
			path = dt.pathOf(dt.lines[line])
		}
	}
	return e.stage([]string{path}, unstage)
}
