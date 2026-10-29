# Mayoi 命令入口索引

上游 `46990a1ca9945de7b90b94104c32f2d7770461f5`，来源 `cmd/mayoi/commands.go`。这是静态注册项的完整提取；循环生成项和上下文菜单另见功能台账。ID不同不代表功能缺失，同名也不代表行为等价。逐项内化时按功能台账验收，不能把此索引当通过报告。

共 133 个静态入口。

| 上游命令 ID | 标签表达式 |
| --- | --- |
| `file.new` | `t.CmdFileNew()` |
| `file.newWindow` | `t.CmdFileNewWindow()` |
| `file.openFolder` | `t.CmdFileOpenFolder()` |
| `file.openFolderWindow` | `t.CmdFileOpenFolderWindow()` |
| `file.openRecent` | `t.CmdFileOpenRecent()` |
| `file.clearRecent` | `t.CmdFileClearRecent()` |
| `remote.connect` | `t.CmdRemoteConnect()` |
| `remote.openFolder` | `t.CmdRemoteOpenFolder()` |
| `remote.reconnect` | `t.CmdRemoteReconnect()` |
| `remote.disconnect` | `t.CmdRemoteDisconnect()` |
| `remote.forwardPort` | `t.CmdRemoteForwardPort()` |
| `file.save` | `t.CmdFileSave()` |
| `file.saveAs` | `t.CmdFileSaveAs()` |
| `file.saveAll` | `t.CmdFileSaveAll()` |
| `file.autoSave` | `t.CmdFileAutoSave()` |
| `file.settings` | `t.CmdFileSettings()` |
| `file.keybindings` | `t.CmdFileKeybindings()` |
| `file.copyPath` | `t.CmdFileCopyPath()` |
| `file.copyRelativePath` | `t.CmdFileCopyRelativePath()` |
| `file.reveal` | `revealActiveLabel(t)` |
| `file.close` | `t.CmdFileClose()` |
| `file.reopenClosed` | `t.CmdFileReopenClosed()` |
| `file.closeFolder` | `t.CmdFileCloseFolder()` |
| `file.closeProject` | `t.CmdFileCloseProject()` |
| `account.signIn` | `t.CmdAccountSignIn()` |
| `account.show` | `t.CmdAccountShow()` |
| `account.syncNow` | `t.CmdAccountSyncNow()` |
| `account.compareKeybindings` | `t.CmdAccountCompareKeybindings()` |
| `account.manage` | `t.CmdAccountManage()` |
| `account.signOut` | `t.CmdAccountSignOut()` |
| `file.installCLI` | `t.CmdFileInstallCLI()` |
| `file.uninstallCLI` | `t.CmdFileUninstallCLI()` |
| `edit.undo` | `t.CmdEditUndo()` |
| `edit.redo` | `t.CmdEditRedo()` |
| `edit.cut` | `t.CmdEditCut()` |
| `edit.find` | `t.CmdEditFind()` |
| `edit.replace` | `t.CmdEditReplace()` |
| `edit.findNext` | `t.CmdEditFindNext()` |
| `edit.findPrev` | `t.CmdEditFindPrev()` |
| `edit.findInFiles` | `t.CmdEditFindInFiles()` |
| `edit.replaceInFiles` | `t.CmdEditReplaceInFiles()` |
| `sel.comment` | `t.CmdSelComment()` |
| `sel.indent` | `t.CmdSelIndent()` |
| `sel.outdent` | `t.CmdSelOutdent()` |
| `sel.moveUp` | `t.CmdSelMoveUp()` |
| `sel.moveDown` | `t.CmdSelMoveDown()` |
| `sel.copyUp` | `t.CmdSelCopyUp()` |
| `sel.copyDown` | `t.CmdSelCopyDown()` |
| `sel.deleteLine` | `t.CmdSelDeleteLine()` |
| `sel.lineBelow` | `t.CmdSelLineBelow()` |
| `sel.lineAbove` | `t.CmdSelLineAbove()` |
| `sel.line` | `t.CmdSelLine()` |
| `sel.indentTabs` | `t.CmdSelIndentTabs()` |
| `sel.indent2` | `t.CmdSelIndentSpaces(2)` |
| `sel.indent4` | `t.CmdSelIndentSpaces(4)` |
| `sel.indent8` | `t.CmdSelIndentSpaces(8)` |
| `sel.camelCase` | `t.CmdSelCamelCase()` |
| `sel.sortAsc` | `t.CmdSelSortAsc()` |
| `sel.sortDesc` | `t.CmdSelSortDesc()` |
| `go.bracket` | `t.CmdGoBracket()` |
| `go.quickOpen` | `t.CmdGoQuickOpen()` |
| `go.commands` | `t.CmdGoCommands()` |
| `go.line` | `t.CmdGoLine()` |
| `go.symbol` | `t.CmdGoSymbol()` |
| `go.nextTab` | `t.CmdGoNextTab()` |
| `go.prevTab` | `t.CmdGoPrevTab()` |
| `go.nextProject` | `t.CmdGoNextProject()` |
| `go.prevProject` | `t.CmdGoPrevProject()` |
| `view.sidebar` | `t.CmdViewSidebar()` |
| `view.sidebarRight` | `t.CmdViewSidebarRight()` |
| `view.explorer` | `t.CmdViewExplorer()` |
| `view.search` | `t.CmdViewSearch()` |
| `view.scm` | `t.CmdViewScm()` |
| `git.commit` | `t.CmdGitCommit()` |
| `git.commitPush` | `t.CmdGitCommitPush()` |
| `git.amend` | `t.CmdGitAmend()` |
| `git.writeMessage` | `t.CmdGitWriteMessage()` |
| `git.openChanges` | `t.CmdGitOpenChanges()` |
| `git.openAllChanges` | `t.CmdGitOpenAllChanges()` |
| `git.nextChange` | `t.CmdGitNextChange()` |
| `git.prevChange` | `t.CmdGitPrevChange()` |
| `git.showChange` | `t.CmdGitShowChange()` |
| `git.revertChange` | `t.CmdGitRevertChange()` |
| `git.stageChange` | `t.CmdGitStageChange()` |
| `git.stageFile` | `t.CmdGitStageFile()` |
| `git.unstageFile` | `t.CmdGitUnstageFile()` |
| `git.stageAll` | `t.CmdGitStageAll()` |
| `git.undoCommit` | `t.CmdGitUndoCommit()` |
| `git.continue` | `t.CmdGitContinue()` |
| `git.abort` | `t.CmdGitAbort()` |
| `git.switchBranch` | `t.CmdGitSwitchBranch()` |
| `git.createBranch` | `t.CmdGitCreateBranch()` |
| `git.blame` | `t.CmdGitBlame()` |
| `git.fetch` | `t.CmdGitFetch()` |
| `git.pull` | `t.CmdGitPull()` |
| `git.push` | `t.CmdGitPush()` |
| `git.refresh` | `t.CmdGitRefresh()` |
| `review.comment` | `t.CmdReviewComment()` |
| `review.next` | `t.CmdReviewNext()` |
| `review.previous` | `t.CmdReviewPrevious()` |
| `review.copy` | `t.CmdReviewCopy()` |
| `review.send` | `t.CmdReviewSend()` |
| `review.clear` | `t.CmdReviewClear()` |
| `view.terminal` | `t.CmdViewTerminal()` |
| `view.newTerminal` | `t.CmdViewNewTerminal()` |
| `view.terminalTab` | `t.CmdViewTerminalTab()` |
| `view.terminalMove` | `t.CmdViewTerminalMove()` |
| `view.terminalLockRight` | `t.CmdViewTerminalLockRight()` |
| `view.panelPosition.bottom` | `t.CmdViewPanelBottom()` |
| `view.panelPosition.left` | `t.CmdViewPanelLeft()` |
| `view.panelPosition.right` | `t.CmdViewPanelRight()` |
| `view.lockColumn` | `t.CmdViewLockColumn()` |
| `view.splitRight` | `t.CmdViewSplitRight()` |
| `view.nextColumn` | `t.CmdViewNextColumn()` |
| `view.prevColumn` | `t.CmdViewPrevColumn()` |
| `view.moveTabNext` | `t.CmdViewMoveTabNext()` |
| `view.moveTabPrev` | `t.CmdViewMoveTabPrev()` |
| `view.wordWrap` | `t.CmdViewWordWrap()` |
| `view.vim` | `t.CmdViewVim()` |
| `view.projectTabs` | `t.CmdViewProjectTabs()` |
| `view.wordWrapTab` | `t.CmdViewWordWrapTab()` |
| `view.togglePreview` | `t.CmdViewTogglePreview()` |
| `view.previewToSide` | `t.CmdViewPreviewToSide()` |
| `view.zoomIn` | `t.CmdViewZoomIn()` |
| `view.zoomOut` | `t.CmdViewZoomOut()` |
| `view.zoomReset` | `t.CmdViewZoomReset()` |
| `view.appearance.system` | `t.CmdViewAppearanceSystem()` |
| `view.appearance.light` | `t.CmdViewAppearanceLight()` |
| `view.appearance.dark` | `t.CmdViewAppearanceDark()` |
| `view.theme.mayoi` | `"Mayoi"` |
| `view.theme.matrix` | `t.CmdViewThemeMatrix()` |
| `view.theme.claude` | `"Claude"` |
| `view.theme.qoder` | `"Qoder"` |

动态入口：`view.locale.system` 与系统注册的语言命令、`file.recent:<folder>`。SSH 命令在不支持的平台移除；菜单中的上下文动作以功能台账对应条目跟踪。

## 2026-10-22 增量入口（abf7db6）

原始 133 项表仍保留上述固定版本；新入口映射如下，完成情况以功能台账为准。

| 上游命令 ID | 砚入口/开放项 |
| --- | --- |
| `view.terminalLocation` | 同名命令，UP-29；设置新终端的默认位置 |
| `view.newTerminal` | `workbench.action.terminal.new`，遵循默认位置 |
| Explorer Open in Terminal | `openInIntegratedTerminal`，按选中文件/目录或工作区选择目录 |
| `git.toggleCommitFiles` | UP-43，已实现提交文件树与 SCM 文件展开 |
| `view.selectTheme` / `workbench.action.selectTheme` | `workbench.action.selectTheme`，UP-41，窗口内预览、搜索、确认与取消 |
