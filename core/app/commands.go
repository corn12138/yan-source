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
	"github.com/corn12138/yan-source/core/i18n"
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
		"cmd+,":       cmdOpenSettings,
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
		{"workbench.action.files.save", i18n.T("File: Save"), (*Editor).save},
		{cmdOverwrite, i18n.T("File: Overwrite the Version on Disk"), (*Editor).overwrite},
		{cmdRevert, i18n.T("File: Revert to the Version on Disk"), (*Editor).revert},
		{"workbench.action.files.saveAll", i18n.T("File: Save All"), (*Editor).saveAll},
		{"workbench.action.quit", i18n.T("Quit"), (*Editor).quitCmd},
		{"undo", i18n.T("Undo"), func(e *Editor) error { return e.history(true) }},
		{"redo", i18n.T("Redo"), func(e *Editor) error { return e.history(false) }},
		{"editor.action.selectAll", i18n.T("Select All"), func(e *Editor) error {
			return e.withDoc(func(b *edit.Buffer) {
				b.SetSelections(edit.Single(edit.Range{Anchor: 0, Head: b.Snapshot().Len(), GoalX: -1}))
			})
		}},
		{"removeSecondaryCursors", i18n.T("Keep Only the Primary Cursor"), func(e *Editor) error {
			if e.closePeek() {
				return nil // Esc：先关掉改动的框
			}
			if t := e.cur(); t != nil && len(t.doc.buf.Selections().Ranges) == 1 && e.find.open {
				e.closeFind() // Esc：没有多光标时关掉查找框
				return nil
			}
			return e.withDoc(func(b *edit.Buffer) { b.SetSelections(edit.Single(b.Selections().Main())) })
		}},
		{"actions.find", i18n.T("Find"), func(e *Editor) error { return e.openFind(false) }},
		{"editor.action.startFindReplaceAction", i18n.T("Replace"), func(e *Editor) error { return e.openFind(true) }},
		{"editor.action.nextMatchFindAction", i18n.T("Find Next"), func(e *Editor) error { return e.step(1) }},
		{"editor.action.previousMatchFindAction", i18n.T("Find Previous"), func(e *Editor) error { return e.step(-1) }},
		{"toggleFindCaseSensitive", i18n.T("Find: Match Case"), func(e *Editor) error { return e.toggleFindOption(&e.find.matchCase) }},
		{"toggleFindWholeWord", i18n.T("Find: Match Whole Word"), func(e *Editor) error { return e.toggleFindOption(&e.find.word) }},
		{"toggleFindRegex", i18n.T("Find: Use Regular Expression"), func(e *Editor) error { return e.toggleFindOption(&e.find.regex) }},
		{"editor.action.replaceOne", i18n.T("Replace Current Match"), (*Editor).replaceOne},
		{"editor.action.replaceAll", i18n.T("Replace All"), (*Editor).replaceAll},
		{"yan.find.close", i18n.T("Close Find"), func(e *Editor) error { e.closeFind(); return nil }},
		{"workbench.action.editor.nextChange", i18n.T("Git: Next Change"), func(e *Editor) error { return e.stepChange(1) }},
		{"workbench.action.editor.previousChange", i18n.T("Git: Previous Change"), func(e *Editor) error { return e.stepChange(-1) }},
		{"yan.git.toggleBlame", i18n.T("Git: Toggle Inline Blame"), func(e *Editor) error {
			e.cfg.Blame = !e.cfg.Blame
			e.repository().git.blame = blameState{}
			return nil
		}},
		{"yan.edit.newline", i18n.T("New Line"), func(e *Editor) error {
			if t := e.cur(); t != nil && t.diff != nil {
				return e.diffOpen(t) // 改动视图里回车：展开折叠，或打开文件跳到这一行
			}
			return e.newline()
		}},
		{"git.openChange", i18n.T("Git: Open Changes"), func(e *Editor) error {
			t := e.cur()
			if t == nil {
				return errors.New(i18n.T("No open file"))
			}
			if t.diff != nil {
				return e.openPath(t.diff.path)
			}
			return e.openChange(t.doc.buf.Path(), false)
		}},
		{"tab", i18n.T("Indent"), (*Editor).tabKey},
		{"outdent", i18n.T("Outdent"), func(e *Editor) error { return e.indentLines(true) }},
		{"editor.action.indentLines", i18n.T("Lines: Indent"), func(e *Editor) error { return e.indentLines(false) }},
		{"editor.action.outdentLines", i18n.T("Lines: Outdent"), func(e *Editor) error { return e.indentLines(true) }},
		{"editor.action.commentLine", i18n.T("Toggle Line Comment"), (*Editor).toggleComment},
		{"editor.action.moveLinesUpAction", i18n.T("Move Line Up"), func(e *Editor) error { return e.moveLines(false) }},
		{"editor.action.moveLinesDownAction", i18n.T("Move Line Down"), func(e *Editor) error { return e.moveLines(true) }},
		{"editor.action.copyLinesUpAction", i18n.T("Copy Line Up"), func(e *Editor) error { return e.copyLines(false) }},
		{"editor.action.copyLinesDownAction", i18n.T("Copy Line Down"), func(e *Editor) error { return e.copyLines(true) }},
		{"editor.action.deleteLines", i18n.T("Delete Line"), (*Editor).deleteLines},
		{"editor.action.insertLineAfter", i18n.T("Insert Line Below"), func(e *Editor) error { return e.insertLine(false) }},
		{"editor.action.insertLineBefore", i18n.T("Insert Line Above"), func(e *Editor) error { return e.insertLine(true) }},
		{"expandLineSelection", i18n.T("Select Line"), (*Editor).selectLines},
		{"editor.action.sortLinesAscending", i18n.T("Lines: Sort Ascending"), func(e *Editor) error { return e.sortLines(false) }},
		{"editor.action.sortLinesDescending", i18n.T("Lines: Sort Descending"), func(e *Editor) error { return e.sortLines(true) }},
		{"editor.action.transformToCamelcase", i18n.T("Transform to camelCase"), (*Editor).camelCase},
		{"yan.indent.choose", i18n.T("Select Indentation…"), (*Editor).chooseIndent},
		{"yan.eol.choose", i18n.T("Change End of Line Sequence…"), (*Editor).chooseEOL},
		{"yan.eol.lf", i18n.T("End of Line Sequence: LF"), func(e *Editor) error { return e.setEOL(text.EOLLF) }},
		{"yan.eol.crlf", i18n.T("End of Line Sequence: CRLF"), func(e *Editor) error { return e.setEOL(text.EOLCRLF) }},
		{"editor.action.indentUsingTabs", i18n.T("Indentation: Use Tabs"), func(e *Editor) error { return e.setIndent(indentStyle{tabs: true, size: max(e.cfg.TabWidth, 1)}) }},
		{"yan.indentUsingSpaces2", i18n.T("Indentation: Use 2 Spaces"), func(e *Editor) error { return e.setIndent(indentStyle{size: 2}) }},
		{"yan.indentUsingSpaces4", i18n.T("Indentation: Use 4 Spaces"), func(e *Editor) error { return e.setIndent(indentStyle{size: 4}) }},
		{"yan.indentUsingSpaces8", i18n.T("Indentation: Use 8 Spaces"), func(e *Editor) error { return e.setIndent(indentStyle{size: 8}) }},
		{"editor.action.jumpToBracket", i18n.T("Go to Matching Bracket"), (*Editor).jumpToBracket},
		{"deleteLeft", i18n.T("Delete Left"), func(e *Editor) error {
			if t := e.cur(); t != nil && e.writable(t) && e.deletePair(t) {
				e.reveal()
				return nil
			}
			return e.editDoc(func(b *edit.Buffer) { b.DeleteBackward() })
		}},
		{"deleteRight", i18n.T("Delete Right"), func(e *Editor) error {
			return e.editDoc(func(b *edit.Buffer) { b.DeleteForward() })
		}},
		{"deleteWordLeft", i18n.T("Delete Word Left"), func(e *Editor) error {
			return e.editDoc(func(b *edit.Buffer) {
				e.moveBy(b, true, func(s text.Snapshot, r edit.Range) int { return edit.WordLeft(s, r.Head) })
				b.DeleteBackward()
			})
		}},
		{"deleteAllLeft", i18n.T("Delete to Line Start"), func(e *Editor) error {
			return e.editDoc(func(b *edit.Buffer) {
				e.moveBy(b, true, func(s text.Snapshot, r edit.Range) int { return s.LineStart(s.LineAt(r.Head)) })
				b.DeleteBackward()
			})
		}},
		{"editor.action.clipboardCopyAction", i18n.T("Copy"), func(e *Editor) error { return e.copy(false) }},
		{"editor.action.clipboardCutAction", i18n.T("Cut"), func(e *Editor) error { return e.copy(true) }},
		{"editor.action.clipboardPasteAction", i18n.T("Paste"), (*Editor).paste},
		{"editor.action.addSelectionToNextFindMatch", i18n.T("Add Selection to Next Match"), (*Editor).addNextMatch},
		{"workbench.action.terminal.toggleTerminal", i18n.T("Terminal: Toggle Terminal"), (*Editor).shell},
		{"workbench.action.terminal.new", i18n.T("Terminal: New Terminal"), func(e *Editor) error {
			return e.openTerminal(e.terminalDir())
		}},
		{"view.terminalLocation", i18n.T("Terminal: Create New Terminals in the Editor Area"), (*Editor).toggleTerminalLocation},
		{"openInIntegratedTerminal", i18n.T("Explorer: Open in Terminal"), (*Editor).openTreeTerminal},
		{"workbench.action.createTerminalEditor", i18n.T("Terminal: New Terminal in Editor Area"), (*Editor).newTerminalEditor},
		{"workbench.action.quickOpen", i18n.T("Go to File…"), func(e *Editor) error { return e.openQuickOpen("") }},
		{"workbench.action.gotoLine", i18n.T("Go to Line…"), func(e *Editor) error { return e.openQuickOpen(":") }},
		{"workbench.action.gotoSymbol", i18n.T("Go to Symbol…"), func(e *Editor) error { return e.openQuickOpen("@") }},
		{"editor.action.revealDefinition", i18n.T("Go to Definition"), (*Editor).revealDefinition},
		{"workbench.action.navigateBack", i18n.T("Go Back"), func(e *Editor) error { return e.navigate(true) }},
		{"workbench.action.navigateForward", i18n.T("Go Forward"), func(e *Editor) error { return e.navigate(false) }},
		{"workbench.action.openRecent", i18n.T("Open Recent Folder…"), (*Editor).openRecentPicker},
		{"explorer.newFile", i18n.T("Explorer: New File…"), func(e *Editor) error { return e.newEntry(false) }},
		{"explorer.newFolder", i18n.T("Explorer: New Folder…"), func(e *Editor) error { return e.newEntry(true) }},
		{"renameFile", i18n.T("Explorer: Rename…"), (*Editor).renameEntry},
		{"deleteFile", i18n.F("Explorer: Move to %s", e.trashName()), (*Editor).trashEntry},
		{"filesExplorer.copy", i18n.T("Explorer: Copy"), (*Editor).copyFiles},
		{"filesExplorer.paste", i18n.T("Explorer: Paste"), (*Editor).pasteFiles},
		{"copyFilePath", i18n.T("Copy Path"), func(e *Editor) error { return e.copyPath(false) }},
		{"copyRelativeFilePath", i18n.T("Copy Relative Path"), func(e *Editor) error { return e.copyPath(true) }},
		{"revealFileInOS", i18n.F("Reveal in %s", FileManager(e.profile.OS)), (*Editor).revealInOS},
		{"markdown.showPreviewToSide", i18n.T("Open Preview to the Side (SVG, Markdown, Mermaid)"), (*Editor).previewToSide},
		{"yan.svg.togglePreview", i18n.T("Toggle Preview (SVG, Markdown, Mermaid)"), (*Editor).togglePreview},
		{"yan.toggleVim", i18n.T("Toggle Vim Mode"), (*Editor).toggleVim},
		{"editor.action.toggleWordWrap", i18n.T("Toggle Word Wrap"), (*Editor).toggleWordWrap},
		{"yan.toggleTabWordWrap", i18n.T("Toggle Word Wrap for This Tab"), (*Editor).toggleTabWrap},
		{"workbench.action.newWindow", i18n.T("New Window"), func(e *Editor) error {
			if h, ok := e.host.(interface{ NewWindow() }); ok {
				h.NewWindow()
				return nil
			}
			return errors.New(i18n.T("The terminal version has one window: run yan in another terminal"))
		}},
		{"yan.installCLI", i18n.T("Install yan Command…"), func(e *Editor) error {
			if h, ok := e.host.(interface{ InstallCLI() }); ok {
				h.InstallCLI()
				return nil
			}
			return errors.New(i18n.T("Install the command from the File menu of the window version"))
		}},
		{"yan.openFolderNewWindow", i18n.T("Open Folder in New Window…"), func(e *Editor) error {
			if h, ok := e.host.(interface{ OpenFolderNewWindow() }); ok {
				h.OpenFolderNewWindow()
				return nil
			}
			return errors.New(i18n.T("Run yan with a folder in another terminal"))
		}},
		{"workbench.action.files.openFolder", i18n.T("Open Folder…"), func(e *Editor) error {
			if h, ok := e.host.(interface{ OpenFolderDialog() }); ok {
				h.OpenFolderDialog()
				return nil
			}
			return errors.New(i18n.T("Open a folder with \"yan folder\" in the terminal; Ctrl+R lists recent folders"))
		}},
		{"workbench.action.files.newUntitledFile", i18n.T("New Untitled File"), (*Editor).newUntitled},
		{cmdMaximize, i18n.T("Maximize / Restore Editor Area (the merge view's three panes together)"), (*Editor).toggleMaximize},
		{"workbench.action.files.saveAs", i18n.T("Save As…"), (*Editor).saveAs},
		{"workbench.action.files.newFile", i18n.T("New File…"), func(e *Editor) error {
			if h, ok := e.host.(interface{ NewFileDialog(dir string) }); ok {
				dir := e.root
				if t := e.cur(); t != nil && t.diff == nil && t.doc.buf.Path() != "" {
					dir = filepath.Dir(t.doc.buf.Path())
				}
				h.NewFileDialog(dir)
				return nil
			}
			return errors.New(i18n.T("Outside Quick Open, create a file from the terminal: yan filename"))
		}},
		{"workbench.action.showCommands", i18n.T("Show All Commands"), (*Editor).openPalette},
		{"workbench.action.toggleSidebarPosition", i18n.T("Move Sidebar to the Other Side"), (*Editor).toggleSidebarPosition},
		{"workbench.action.toggleSidebarVisibility", i18n.T("Toggle Sidebar Visibility"), (*Editor).toggleSidebar},
		{"workbench.view.explorer", i18n.T("Show Explorer"), (*Editor).focusExplorer},
		{"workbench.action.focusActiveEditorGroup", i18n.T("Focus Editor"), func(e *Editor) error { e.area = areaEditor; return nil }},
		{"workbench.action.closeActiveEditor", i18n.T("Close Editor"), (*Editor).closeEditor},
		{"workbench.action.closeOtherEditors", i18n.T("Close Other Editors"), func(e *Editor) error {
			c := e.column()
			return e.closeTabsIn(c, c.tab())
		}},
		{"workbench.action.closeEditorsInGroup", i18n.T("Close All Editors in This Column"), func(e *Editor) error { return e.closeTabsIn(e.column(), nil) }},
		{"workbench.action.reopenClosedEditor", i18n.T("Reopen Closed Editor"), (*Editor).reopenClosed},
		{"view.lockColumn", i18n.T("Lock / Unlock This Column"), (*Editor).toggleColumnLock},
		{"yan.guide.open", i18n.T("Learn: Feature Tour"), (*Editor).openGuide},
		{"yan.guide.skip", i18n.T("Learn: Skip This Step"), (*Editor).skipStep},
		{"yan.guide.nextTip", i18n.T("Learn: Next Tip"), (*Editor).nextTip},
		{"yan.guide.reset", i18n.T("Learn: Reset Lesson Progress"), (*Editor).resetGuide},
		{"yan.guide.back", i18n.T("Learn: Previous Step"), (*Editor).backStep},
		{"yan.guide.restart", i18n.T("Learn: Restart Lesson"), (*Editor).restartLesson},
		{"yan.guide.leave", i18n.T("Learn: Leave Practice Folder"), (*Editor).leavePracticeFolder},
		{"yan.guide.dismissOffer", i18n.T("Learn: Dismiss Tour Offer"), (*Editor).dismissGuideOffer},
		{"yan.project.customize", i18n.T("Customize Project Badge and Color…"), (*Editor).customizeProject},
		{"workbench.action.positionPanelBottom", i18n.T("Terminal Panel at the Bottom"), func(e *Editor) error { return e.setPanelPosition("") }},
		{"workbench.action.positionPanelLeft", i18n.T("Terminal Panel on the Left"), func(e *Editor) error { return e.setPanelPosition("left") }},
		{"workbench.action.positionPanelRight", i18n.T("Terminal Panel on the Right"), func(e *Editor) error { return e.setPanelPosition("right") }},
		{"yan.config.importVSCode", i18n.T("Import Settings and Shortcuts from VS Code / Cursor"), (*Editor).importVSCode},
		{"yan.config.importSettingsFile", i18n.T("Import Settings from File (settings.json)"), (*Editor).importSettingsFile},
		{"view.terminalLockRight", i18n.T("Terminal: Move to the Locked Column on the Right"), (*Editor).moveTerminalToLockedRight},
		{"workbench.action.nextEditor", i18n.T("Open Next Editor"), func(e *Editor) error { return e.cycleTab(1) }},
		{"workbench.action.previousEditor", i18n.T("Open Previous Editor"), func(e *Editor) error { return e.cycleTab(-1) }},
		{"workbench.action.splitEditor", i18n.T("Split Editor Right"), (*Editor).splitRight},
		{"workbench.action.focusNextGroup", i18n.T("Focus Next Column"), func(e *Editor) error { return e.focusColumnBy(1) }},
		{"workbench.action.focusPreviousGroup", i18n.T("Focus Previous Column"), func(e *Editor) error { return e.focusColumnBy(-1) }},
		{"workbench.action.moveEditorToNextGroup", i18n.T("Move Editor to Next Column"), func(e *Editor) error { return e.moveTabBy(1) }},
		{"workbench.action.moveEditorToPreviousGroup", i18n.T("Move Editor to Previous Column"), func(e *Editor) error { return e.moveTabBy(-1) }},
		{"workbench.action.keepEditor", i18n.T("Keep Editor (Pin the Preview Tab)"), func(e *Editor) error {
			if t := e.cur(); t != nil {
				e.keep(t)
			}
			return nil
		}},
		{"workbench.files.action.refreshFilesExplorer", i18n.T("Refresh Explorer"), func(e *Editor) error {
			e.checkDisk()
			e.reindex()
			return nil
		}},
		{"workbench.files.action.showActiveFileInExplorer", i18n.T("Reveal Current File in Explorer"), func(e *Editor) error {
			if e.ws == nil {
				return errors.New(i18n.T("No folder open"))
			}
			e.sidebar, e.area = true, areaTree
			e.revealInTree()
			return nil
		}},
		{cmdOpenSettings, i18n.T("Preferences: Open Settings"), (*Editor).openSettings},
		{"yan.settings.language", i18n.T("Preferences: Interface Language…"), (*Editor).openSettings},
		{"yan.config.open", i18n.T("Open User Configuration File"), (*Editor).openConfig},
		{cmdOpenKeybindings, i18n.T("Preferences: Open Keyboard Shortcuts (keybindings.json)"), (*Editor).openKeybindings},
		{"yan.toggleThemeEffects", i18n.T("Theme: Toggle Decorations"), (*Editor).toggleThemeEffects},
		{"workbench.action.selectTheme", i18n.T("Preferences: Color Theme…"), (*Editor).openThemePicker},
		{"yan.theme.mayoi", i18n.T("Theme: Mayoi"), func(e *Editor) error { return e.setTheme("") }},
		{"yan.theme.matrix", i18n.T("Theme: The Matrix"), func(e *Editor) error { return e.setTheme("matrix") }},
		{"yan.theme.claude", i18n.T("Theme: Claude"), func(e *Editor) error { return e.setTheme("claude") }},
		{"yan.appearance.system", i18n.T("Appearance: Follow System"), func(e *Editor) error { return e.setAppearance("") }},
		{"yan.appearance.light", i18n.T("Appearance: Light"), func(e *Editor) error { return e.setAppearance("light") }},
		{"yan.appearance.dark", i18n.T("Appearance: Dark"), func(e *Editor) error { return e.setAppearance("dark") }},
		{"yan.font.family", i18n.T("Font: Select Code Font…"), (*Editor).chooseFont},
		{"yan.font.size", i18n.T("Font: Code Font Size…"), (*Editor).chooseFontSize},
		{"yan.font.lineHeight", i18n.T("Font: Line Height…"), (*Editor).chooseLineHeight},
		{"workbench.action.zoomIn", i18n.T("Zoom In"), func(e *Editor) error { return e.zoom(1) }},
		{"workbench.action.zoomOut", i18n.T("Zoom Out"), func(e *Editor) error { return e.zoom(-1) }},
		{"workbench.action.zoomReset", i18n.T("Reset Zoom"), func(e *Editor) error { return e.zoom(0) }},
		{"workbench.action.toggleAutoSave", i18n.T("Toggle Auto Save"), (*Editor).toggleAutoSave},
		{"workbench.action.findInFiles", i18n.T("Search in Files"), func(e *Editor) error { return e.showSearch(false) }},
		{"workbench.action.replaceInFiles", i18n.T("Replace in Files"), func(e *Editor) error { return e.showSearch(true) }},
		{"workbench.view.search", i18n.T("Show Search"), func(e *Editor) error { return e.showSearch(false) }},
		{"search.action.replaceAll", i18n.T("Search: Replace All"), func(e *Editor) error { return e.replaceSearch(true) }},
		{"workbench.view.scm", i18n.T("Show Source Control"), func(e *Editor) error {
			if e.ws == nil {
				return errors.New(i18n.T("No folder open"))
			}
			e.showSide(sideSCM)
			return nil
		}},
		{"git.commit", i18n.T("Git: Commit"), func(e *Editor) error { return e.commit(false) }},
		{"git.commitAndPush", i18n.T("Git: Commit and Push"), func(e *Editor) error { return e.commit(true) }},
		{"git.commitAmend", i18n.T("Git: Amend Last Commit"), (*Editor).startAmend},
		{"git.undoCommit", i18n.T("Git: Undo Last Commit"), (*Editor).undoCommit},
		{"editor.action.dirtydiff.next", i18n.T("Git: View Next Change"), func(e *Editor) error { return e.stepPeek(1) }},
		{"editor.action.dirtydiff.previous", i18n.T("Git: View Previous Change"), func(e *Editor) error { return e.stepPeek(-1) }},
		{"git.revertSelectedRanges", i18n.T("Git: Revert This Change"), (*Editor).revertHunk},
		{"review.comment", i18n.T("Review: Add or Edit Comment"), (*Editor).addReviewComment},
		{"review.save", i18n.T("Review: Save Comment"), (*Editor).saveReview},
		{"review.cancel", i18n.T("Review: Cancel Editing"), (*Editor).cancelReview},
		{"review.delete", i18n.T("Review: Delete Comment"), (*Editor).deleteReview},
		{"review.clear", i18n.T("Review: Clear Comments"), (*Editor).clearReview},
		{"review.undo", i18n.T("Review: Undo Delete or Clear"), (*Editor).undoReview},
		{"review.copy", i18n.T("Review: Copy All Comments"), (*Editor).copyReview},
		{"review.copyExcerpt", i18n.T("Review: Copy Code Excerpt and Location"), (*Editor).copyReviewExcerpt},
		{"review.send", i18n.T("Review: Send Comments to Terminal"), (*Editor).sendReview},
		{"review.list", i18n.T("Review: Show All Comments"), (*Editor).reviewList},
		{"review.next", i18n.T("Review: Next Comment"), func(e *Editor) error { return e.stepReview(1) }},
		{"review.previous", i18n.T("Review: Previous Comment"), func(e *Editor) error { return e.stepReview(-1) }},
		{"yan.git.exportPatch", i18n.T("Git: Export Commit Patch…"), func(e *Editor) error {
			return e.commitTarget(func(e *Editor, c git.Commit) error { return e.commitPatch(c, true) })
		}},
		{"yan.git.copyPatch", i18n.T("Git: Copy Commit Patch"), func(e *Editor) error {
			return e.commitTarget(func(e *Editor, c git.Commit) error { return e.commitPatch(c, false) })
		}},
		{"yan.git.compareHeadToCommit", i18n.T("Git: Compare HEAD → Selected Commit"), func(e *Editor) error {
			return e.commitTarget(func(e *Editor, c git.Commit) error { return e.compareRevisions("HEAD", c.Hash) })
		}},
		{"yan.git.compareCommitToHead", i18n.T("Git: Compare Selected Commit → HEAD"), func(e *Editor) error {
			return e.commitTarget(func(e *Editor, c git.Commit) error { return e.compareRevisions(c.Hash, "HEAD") })
		}},
		{"yan.git.selectCompare", i18n.T("Git: Select Commit as Comparison Start"), func(e *Editor) error {
			return e.commitTarget(func(e *Editor, c git.Commit) error { return e.selectComparison(c) })
		}},
		{"yan.git.compareSelected", i18n.T("Git: Compare with Selected Start"), func(e *Editor) error {
			return e.commitTarget(func(e *Editor, c git.Commit) error { return e.compareSelectedCommit(c) })
		}},
		{"yan.git.openCommitRemote", i18n.T("Git: Open Commit on Remote"), func(e *Editor) error {
			return e.commitTarget(func(e *Editor, c git.Commit) error { return e.commitRemote(c) })
		}},
		{"git.toggleCommitFiles", i18n.T("Git: Show or Hide the Commit's File List"), (*Editor).toggleCommitFiles},
		{"git.openAllChanges", i18n.T("Git: Open All Changes"), (*Editor).openAllChanges},
		{"toggle.diff.renderSideBySide", i18n.T("Diff View: Change Layout (Auto, Side by Side, Inline)"), (*Editor).toggleDiffLayout},
		{"yan.diff.whitespace", i18n.T("Diff View: Compare Whitespace (All, Ignore Line Ends, Ignore All, Ignore Blank Lines)…"), (*Editor).chooseDiffWhitespace},
		{"yan.diff.compareFiles", i18n.T("Compare Two Files…"), (*Editor).compareFiles},
		{"yan.diff.editBeside", i18n.T("Diff View: Edit the Working Tree Side Alongside (i)"), func(e *Editor) error {
			if t := e.cur(); t != nil && t.diff != nil {
				return e.diffEditBeside(t)
			}
			return errors.New(i18n.T("Open a diff view first"))
		}},
		{"yan.diff.toggleSyncScroll", i18n.T("Diff View: Scroll Both Sides Together / Separately"), (*Editor).toggleSyncScroll},
		{"yan.diff.toggleAlignedRows", i18n.T("Diff View: Align Rows of Both Sides / Lay Out Separately"), (*Editor).toggleAlignedRows},
		{"yan.diff.toggleFullFiles", i18n.T("Diff View: Show Whole File / Only Near Changes"), (*Editor).toggleFullFiles},
		{"yan.merge.open", i18n.T("Merge: Open Three-Way Merge (Conflicted File)"), (*Editor).openMergeCurrent},
		{"yan.merge.acceptOurs", i18n.T("Merge: Use Ours for This Conflict"), func(e *Editor) error {
			return e.mergeCommand(func(e *Editor, mv *mergeView, k int) { e.mergeTake(mv, k, false) })
		}},
		{"yan.merge.acceptTheirs", i18n.T("Merge: Use Theirs for This Conflict"), func(e *Editor) error {
			return e.mergeCommand(func(e *Editor, mv *mergeView, k int) { e.mergeTake(mv, k, true) })
		}},
		{"yan.merge.ignoreOurs", i18n.T("Merge: Ignore Ours for This Conflict"), func(e *Editor) error {
			return e.mergeCommand(func(e *Editor, mv *mergeView, k int) { e.mergeIgnore(mv, k, false) })
		}},
		{"yan.merge.ignoreTheirs", i18n.T("Merge: Ignore Theirs for This Conflict"), func(e *Editor) error {
			return e.mergeCommand(func(e *Editor, mv *mergeView, k int) { e.mergeIgnore(mv, k, true) })
		}},
		{"yan.merge.resolveOurs", i18n.T("Merge: Use Ours for All Conflicts"), func(e *Editor) error {
			return e.mergeCommand(func(e *Editor, mv *mergeView, _ int) { e.mergeResolveUsing(mv, false) })
		}},
		{"yan.merge.resolveTheirs", i18n.T("Merge: Use Theirs for All Conflicts"), func(e *Editor) error {
			return e.mergeCommand(func(e *Editor, mv *mergeView, _ int) { e.mergeResolveUsing(mv, true) })
		}},
		{"yan.merge.nextConflict", i18n.T("Merge: Next Conflict"), func(e *Editor) error { return e.mergeStepConflict(1) }},
		{"yan.merge.previousConflict", i18n.T("Merge: Previous Conflict"), func(e *Editor) error { return e.mergeStepConflict(-1) }},
		{"yan.merge.markResolved", i18n.T("Merge: Mark as Resolved (Save and Stage)"), (*Editor).mergeMarkResolved},
		{"yan.merge.restoreConflict", i18n.T("Merge: Restore Conflict…"), (*Editor).mergeRestore},
		{"yan.merge.saveAndClose", i18n.T("Merge: Save and Close Three-Way Merge"), (*Editor).mergeClose},
		{"yan.merge.files", i18n.T("Merge: Conflicted Files…"), (*Editor).showMergeFiles},
		{"yan.merge.applyChanges", i18n.T("Merge: Apply Non-Conflicting Changes from Both Sides"), func(e *Editor) error { return e.mergeApplySide(0) }},
		{"yan.merge.applyOursChanges", i18n.T("Merge: Apply Our Non-Conflicting Changes"), func(e *Editor) error { return e.mergeApplySide(1) }},
		{"yan.merge.applyTheirsChanges", i18n.T("Merge: Apply Their Non-Conflicting Changes"), func(e *Editor) error { return e.mergeApplySide(2) }},
		{"yan.merge.toggleAutoApply", i18n.T("Merge: Apply Non-Conflicting Changes on Open (Toggle)"), (*Editor).toggleMergeAutoApply},
		{"yan.merge.applyAndNext", i18n.T("Merge: Apply (Mark Resolved) and Open Next Conflicted File"), (*Editor).mergeApplyAndNext},
		{"yan.diff.revertBlock", i18n.T("Diff View: Revert This Change (Comparing Files: Write Left into Right)"), func(e *Editor) error { return e.diffBlockAction(blockRevert) }},
		{"yan.diff.appendBlock", i18n.T("Diff View: Append the Other Side's Lines After This Change"), func(e *Editor) error { return e.diffBlockAction(blockAppend) }},
		{"yan.diff.acceptRight", i18n.T("Compare Files: Write This Right-Side Change into the Left"), func(e *Editor) error { return e.diffBlockAction(blockAcceptRight) }},
		{"yan.diff.highlight", i18n.T("Diff View: How Changes Are Highlighted (Word, Character, Line)…"), (*Editor).chooseDiffHighlight},
		{"yan.diff.context", i18n.T("Diff View: Context Lines (0–10)…"), (*Editor).chooseDiffContext},
		{"workbench.action.compareEditor.nextChange", i18n.T("Diff View: Next Change"), func(e *Editor) error { return e.diffTabNav((*Editor).diffStepChange, 1) }},
		{"workbench.action.compareEditor.previousChange", i18n.T("Diff View: Previous Change"), func(e *Editor) error { return e.diffTabNav((*Editor).diffStepChange, -1) }},
		{"yan.diff.nextFile", i18n.T("Diff View: Next File"), func(e *Editor) error { return e.diffTabNav((*Editor).diffStepFile, 1) }},
		{"yan.diff.previousFile", i18n.T("Diff View: Previous File"), func(e *Editor) error { return e.diffTabNav((*Editor).diffStepFile, -1) }},
		{"selectForCompare", i18n.T("Explorer: Select for Compare"), (*Editor).selectForCompare},
		{"compareFiles", i18n.T("Explorer: Compare with Selected"), (*Editor).compareWithSelected},
		{"git.init", i18n.T("Git: Initialize Repository"), (*Editor).initRepo},
		{"yan.scm.moreCommits", i18n.T("Git: Show More Commits"), (*Editor).moreCommits},
		{"yan.scm.chooseRepository", i18n.T("Git: Select Repository…"), (*Editor).chooseRepository},
		{"yan.scm.refreshRepositories", i18n.T("Git: Refresh Repository List"), func(e *Editor) error { e.gitChanged(); return nil }},
		{"yan.scm.toggleTree", i18n.T("Git: View Changes as Tree"), (*Editor).toggleSCMTree},
		{"yan.scm.commitActions", i18n.T("Git: Selected Commit… (Checkout, Merge, Rebase, Reset, Revert, Cherry-Pick)"), func(e *Editor) error {
			if i, ok := e.scmSelectedCommit(); ok {
				return e.openCommit(i)
			}
			return errors.New(i18n.T("Select a commit in Source Control's history first"))
		}},
		{"git.stageSelectedRanges", i18n.T("Git: Stage This Change"), (*Editor).stageHunk},
		{"yan.hunk.close", i18n.T("Git: Close the Change Peek"), func(e *Editor) error { e.closePeek(); return nil }},
		{"yan.scm.toggleCommits", i18n.T("Git: Fold or Unfold Commit History"), func(e *Editor) error { e.SCMToggleSection("commits"); return nil }},
		{"yan.scm.toggleRepositoryPanel", i18n.T("Git: Show or Hide Repository Panel (Branches, Remotes)"), (*Editor).toggleRepoPanel},
		{"yan.scm.toggleAllBranches", i18n.T("Git: Show All Branches in Commit History"), (*Editor).toggleAllBranches},
		{"yan.git.continueOp", i18n.T("Git: Continue Merge, Rebase, Cherry-Pick or Revert"), (*Editor).continueOp},
		{"yan.git.abortOp", i18n.T("Git: Abort Merge, Rebase, Cherry-Pick or Revert"), (*Editor).abortOp},
		{"git.stage", i18n.T("Git: Stage Current File"), func(e *Editor) error { return e.stageCurrent(false) }},
		{"git.unstage", i18n.T("Git: Unstage Current File"), func(e *Editor) error { return e.stageCurrent(true) }},
		{"git.stageAll", i18n.T("Git: Stage All Changes"), func(e *Editor) error {
			if e.repository().git.repo == nil {
				return errors.New(i18n.T("Not in a Git repository"))
			}
			return e.stage([]string{e.repository().git.repo.Root}, false)
		}},
		{"git.unstageAll", i18n.T("Git: Unstage All"), func(e *Editor) error {
			if e.repository().git.repo == nil {
				return errors.New(i18n.T("Not in a Git repository"))
			}
			return e.stage([]string{e.repository().git.repo.Root}, true)
		}},
		{"git.push", i18n.T("Git: Push"), (*Editor).push},
		{"git.pull", i18n.T("Git: Pull"), (*Editor).pull},
		{"git.sync", i18n.T("Git: Sync (Pull then Push; Publish the Branch without Upstream)"), (*Editor).gitSync},
		{"git.fetch", i18n.T("Git: Fetch"), (*Editor).fetch},
		{"git.checkout", i18n.T("Git: Switch Branch…"), (*Editor).openBranchPicker},
		{"git.renameBranch", i18n.T("Git: Rename Current Branch…"), (*Editor).renameBranch},
		{"git.deleteBranch", i18n.T("Git: Delete Local Branch…"), (*Editor).deleteBranch},
		{"git.addRemote", i18n.T("Git: Add Remote…"), (*Editor).addRemote},
		{"git.removeRemote", i18n.T("Git: Remove Remote…"), (*Editor).removeRemote},
		{"yan.git.setRemoteURL", i18n.T("Git: Change Remote URL…"), (*Editor).setRemoteURL},
		{"yan.git.pruneRemote", i18n.T("Git: Prune Deleted Remote Branches…"), (*Editor).pruneRemote},
		{"git.refresh", i18n.T("Git: Refresh"), func(e *Editor) error { e.gitChanged(); return nil }},
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
		return errors.New(i18n.T("No folder open: start with \"yan folder\" to get the Explorer"))
	}
	e.sidebar = !e.sidebar
	if !e.sidebar {
		e.area = areaEditor
	}
	return nil
}

func (e *Editor) focusExplorer() error {
	if e.ws == nil {
		return errors.New(i18n.T("No folder open: start with \"yan folder\" to get the Explorer"))
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
		t.doc.buf.Insert(configTemplate(i18n.Current()), edit.OriginCommand)
		e.status = i18n.T("New configuration file: takes effect when saved (read again at next launch)")
	}
	return nil
}

// configTemplate is a new config.toml, its comments in the interface language l.
func configTemplate(l i18n.Locale) string {
	if l == i18n.SimplifiedChinese {
		return configTemplateZh
	}
	return configTemplateEn
}

const configTemplateZh = `# 砚 yan 的用户配置。所有项都可以省略；项目里的 .yan.toml 会覆盖这里。

[ui]
# 界面语言：system（跟随系统，默认）、zh-Hans（简体中文）、en（English）。
# 整个应用共用，项目的 .yan.toml 里写了不算；「设置 › 通用」里选过的优先。改了以后重启砚生效。
# language = "system"

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

const configTemplateEn = `# Yan's user configuration. Every key is optional; a project's .yan.toml overrides these.

[ui]
# Interface language: system (follow the system, the default), zh-Hans (简体中文), en (English).
# It is the whole application's: a project's .yan.toml can't set it, and a choice made in
# Settings › General wins. A change applies when Yan restarts.
# language = "system"

[editor]
tab_width = 4
line_numbers = true

[explorer]
width = 30

[terminal]
# panel: the panel; editor: the editor area. A choice saved from the menus wins.
location = "panel"

# Keys: contexts are global, editor, explorer and overlay; command IDs are VS Code's (see the
# Command Palette). "mod" is Ctrl in Terminal.app, Cmd or Ctrl in Ghostty, and Cmd in the window
# version. An empty value unbinds a key.
# keybindings.json beside this file takes VS Code's format too (Command Palette: Preferences:
# Open Keyboard Shortcuts) and applies after these.
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
		{"cursorLeft", i18n.T("Cursor Left"), func(s text.Snapshot, r edit.Range) int { return edit.PrevBoundary(s, r.Head) }},
		{"cursorRight", i18n.T("Cursor Right"), func(s text.Snapshot, r edit.Range) int { return edit.NextBoundary(s, r.Head) }},
		{"cursorWordLeft", i18n.T("Word Left"), func(s text.Snapshot, r edit.Range) int { return edit.WordLeft(s, r.Head) }},
		{"cursorWordEndRight", i18n.T("Word Right"), func(s text.Snapshot, r edit.Range) int { return edit.WordRight(s, r.Head) }},
		{"cursorHome", i18n.T("Line Start"), func(s text.Snapshot, r edit.Range) int { return edit.SmartHome(s, r.Head) }},
		{"cursorEnd", i18n.T("Line End"), func(s text.Snapshot, r edit.Range) int {
			_, end := s.LineRange(s.LineAt(r.Head))
			return end
		}},
		{"cursorTop", i18n.T("File Start"), func(text.Snapshot, edit.Range) int { return 0 }},
		{"cursorBottom", i18n.T("File End"), func(s text.Snapshot, _ edit.Range) int { return s.Len() }},
	}
	for _, m := range horizontal {
		for _, sel := range []bool{false, true} {
			id, title := m.id, m.title
			if sel {
				id, title = id+"Select", i18n.F("%s and Select", title)
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
		{"cursorUp", i18n.T("Cursor Up"), func(*Editor) int { return -1 }},
		{"cursorDown", i18n.T("Cursor Down"), func(*Editor) int { return 1 }},
		{"cursorPageUp", i18n.T("Page Up"), func(e *Editor) int { return -max(e.pageLines()-1, 1) }},
		{"cursorPageDown", i18n.T("Page Down"), func(e *Editor) int { return max(e.pageLines()-1, 1) }},
	}
	for _, m := range vertical {
		for _, sel := range []bool{false, true} {
			id, title := m.id, m.title
			if sel {
				id, title = id+"Select", i18n.F("%s and Select", title)
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
		return errors.New(i18n.T("No open file"))
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
		return errors.New(i18n.T("No open file"))
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
		return errors.New(i18n.T("No open file"))
	}
	if t.doc.untitled() {
		return e.saveAs()
	}
	if err := e.saveDoc(t.doc); err != nil {
		return err
	}
	e.gitChanged()
	e.keep(t)
	e.status = i18n.F("Saved %s", filepath.Base(t.doc.buf.Path()))
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
		return errors.New(i18n.T("Read-only file: can't save"))
	}
	err := d.buf.Save()
	if errors.Is(err, edit.ErrConflict) {
		return e.conflictError(d)
	}
	if err != nil {
		return fmt.Errorf(i18n.T("Couldn't save %s: %w"), d.name(), err)
	}
	return nil
}

// conflictError 说明文档和磁盘冲突了、该怎么办。
func (e *Editor) conflictError(d *document) error {
	return userError(i18n.F("%s was changed on disk by another program and was not saved: use \"%s\" to write your changes, or \"%s\" to discard them", filepath.Base(d.buf.Path()), e.hintOr(cmdOverwrite, i18n.T("File: Overwrite the Version on Disk")), e.hintOr(cmdRevert, i18n.T("File: Revert to the Version on Disk"))))
}

const (
	cmdOverwrite = "yan.files.overwrite"
	cmdRevert    = "workbench.action.files.revert"
)

// overwrite 用编辑器里的内容覆盖磁盘上的文件（磁盘上被改过时，用户明确选择覆盖）。
func (e *Editor) overwrite() error {
	t := e.cur()
	if t == nil {
		return errors.New(i18n.T("No open file"))
	}
	d := t.doc
	if d.untitled() {
		return e.saveAs()
	}
	if d.buf.Snapshot().Meta().ReadOnly {
		return errors.New(i18n.T("Read-only file: can't save"))
	}
	if err := d.buf.Overwrite(); err != nil {
		return fmt.Errorf(i18n.T("Couldn't save %s: %w"), filepath.Base(d.buf.Path()), err)
	}
	e.gitChanged()
	e.keep(t)
	e.status = i18n.F("Overwrote %s with the editor's content", filepath.Base(d.buf.Path()))
	return nil
}

// revert 放弃未保存的修改，换成磁盘上现在的内容。以差异的形式应用，还能撤销回来。
func (e *Editor) revert() error {
	t := e.cur()
	if t == nil {
		return errors.New(i18n.T("No open file"))
	}
	d := t.doc
	if d.untitled() {
		return userError(i18n.F("%s has never been saved: there is no version on disk to revert to", d.title))
	}
	if !d.buf.Exists() {
		return userError(i18n.F("%s does not exist on disk: nothing to revert to", filepath.Base(d.buf.Path())))
	}
	s, err := text.Load(d.buf.Path())
	if err != nil {
		return err
	}
	e.reloading = true
	d.buf.Reload(s)
	e.reloading = false
	e.status = i18n.T("Reverted to the version on disk (can be undone)")
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
	e.status = i18n.F("Saved %d files", n)
	if untitled > 0 {
		e.status += i18n.F("; %d untitled files need \"%s\" to choose a location", untitled, e.hintOr("workbench.action.files.saveAs", i18n.T("Save As")))
	}
	return nil
}

func (e *Editor) quitCmd() error {
	const id = "workbench.action.quit"
	e.saveDirty()
	kept := e.KeepForRestart()
	if kept != nil {
		e.status = i18n.F("Untitled files couldn't be kept for next time: %s", kept.Error())
	}
	for _, d := range e.docs {
		if d.buf.Dirty() && (kept != nil || !d.untitled()) && e.armed != id {
			e.armed = id
			e.status = i18n.F("Unsaved changes: %s to save, or press %s again to discard changes and quit", e.hintOr("workbench.action.files.save", i18n.T("Save")), e.hintOr(id, i18n.T("Quit")))
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
			e.status = map[bool]string{true: i18n.T("Nothing to undo"), false: i18n.T("Nothing to redo")}[undo]
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
		return errors.New(i18n.T("Clipboard unavailable"))
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
		return fmt.Errorf(i18n.T("Couldn't write the clipboard: %w"), err)
	}
	if cut {
		return e.editDoc(func(b *edit.Buffer) { b.Apply(edit.Transaction{Changes: cs, Origin: edit.OriginCommand}) })
	}
	return nil
}

func (e *Editor) paste() error {
	if e.host == nil {
		return errors.New(i18n.T("Clipboard unavailable"))
	}
	s, err := e.host.Clipboard()
	if err != nil {
		return fmt.Errorf(i18n.T("Couldn't read the clipboard: %w"), err)
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
				e.status = i18n.T("All matches are already selected")
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
		return errors.New(i18n.T("This frontend doesn't support terminals"))
	}
	if err := e.host.Shell(dir); err != nil {
		return fmt.Errorf(i18n.T("Couldn't open the terminal: %w"), err)
	}
	if h, ok := e.host.(interface{ EmbeddedTerminal() bool }); ok && h.EmbeddedTerminal() {
		return nil // 终端在窗口里的面板中：界面没让出去
	}
	e.status = i18n.T("Returned from the terminal")
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
		return errors.New(i18n.T("No open file"))
	}
	path := t.doc.buf.Path()
	if dt := t.diff; dt != nil {
		if dt.commit != nil || dt.fixed {
			return errors.New(i18n.T("Historical changes can't be staged"))
		}
		if dt.peer != "" {
			return errors.New(i18n.T("This compares two files: nothing to stage"))
		}
		path = dt.path
		if dt.changes {
			line := t.doc.buf.Snapshot().LineAt(t.doc.buf.Selections().Main().Head)
			if line < 0 || line >= len(dt.lines) || dt.lines[line].sec < 0 {
				return errors.New(i18n.T("Select a changed file first"))
			}
			path = dt.pathOf(dt.lines[line])
		}
	}
	return e.stage([]string{path}, unstage)
}
