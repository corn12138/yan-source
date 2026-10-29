# Quiet UI feature parity

The Quiet UI redesign changes how yan looks, not what it can do. This file tracks every
capability of the baseline through the redesign: where it lives now, which rules it keeps, and
what evidence shows it still works.

- **Baseline**: `a09d29bcadac6996814ee6cbde74bf8d51f9769f` (v0.1.0-alpha.30). That includes untitled
  documents, Save As, Mermaid previews and untitled-document recovery, which is newer than the
  alpha.29 the task description assumed.
- **Design**: Quiet Editor canvas, revision `1793083258-409c` (2026-10-27), artboards `Yan.dc.html`
  (light, interactive), `YanDark.dc.html` and `Spec.dc.html`. A dated copy and the review screenshots
  are kept with the build evidence, outside the repository.
- **Statuses**: `Pending`, `Implemented—unverified`, `Verified`, `Blocked`, `Pre-existing limitation`.
  Each row names automated evidence (Auto) and native evidence (Native) separately. A screenshot or a
  label is not evidence that behavior works.

## Automated gates

| Gate | What it guarantees |
| --- | --- |
| `core/app` `TestCommandsBaselineKept` with `core/app/testdata/commands-baseline.tsv` | All 297 baseline commands still exist, keep their command-palette visibility and keep their default keys on the macOS, Windows and Linux window profiles and in the terminal. |
| `ui/gui` `TestMenuBaselineKept` with `ui/gui/testdata/menu-baseline.tsv` | All 129 commands the menu bar offered are still reachable from it (a menu may change). |
| `core/style` `TestDefaultThemeContrast` | The default theme's text and status colors stay readable: 4.5:1 for body and secondary text, 3:1 for line numbers, comments, accent and Git colors. |
| `ui/gui` `TestContextMenuBaselineKept` with `ui/gui/testdata/context-menu-baseline.tsv` | The 50 entries of five context menus at `a09d29b` are still offered in the same menus. |
| `ui/gui` `TestQuietUIShots` (with `SHOTS=dir`) | Renders the real frontend at 1440×900 and 1100×720, light and dark: workspace, split, Source Control and command palette. These are pictures, not behavior. |
| Whole suite (`make check`) | The 543 existing tests, race detection, architecture boundaries and the size gate. |

Regenerating a baseline file (`UPDATE_COMMAND_BASELINE=1`, `UPDATE_MENU_BASELINE=1`) is only for an
intentional change, and that change must be recorded below.

## Design findings G01–G12

| ID | Requirement | Where it stands | Verification | Status |
| --- | --- | --- | --- | --- |
| G01 | Inventory the real baseline. | Command, menu-bar and context-menu inventories are generated from the running code of `a09d29b` and enforced as gates. The context-menu gate covers Explorer file, Explorer folder, Source Control commit, editor tab and terminal tab (50 entries). Not yet gated: the repository panel's branch/remote rows, the repositories header, panel terminal tabs, project squares and the tree's blank area. | Auto: `TestCommandsBaselineKept`, `TestMenuBaselineKept`, `TestContextMenuBaselineKept`. Native: n/a. | Implemented—unverified (5 context menus not gated) |
| G02 | Keep diff Append next to Revert. | Unchanged: `yan.diff.appendBlock`, `yan.diff.revertBlock`, the gutter arrows with their modifiers, shared undo. | Auto: `TestDiffBlockRevertAndAppend`, `TestSplitArrowRevertsAndGlows`. Native: pending walkthrough 7. | Pending |
| G03 | Independent scrolling and line alignment. | Unchanged: `[diff] sync_scroll`/`yan.diff.toggleSyncScroll` and `[diff] aligned_rows`/`yan.diff.toggleAlignedRows`; panes scroll on their own when sync is off. | Auto: `TestSplitDiffPanes`, `TestSplitDiffInTheWindow`. Native: pending walkthrough 7. | Pending |
| G04 | Side-specific diff selection and copying. | Unchanged. | Auto: `TestSplitCopyOneSide`. Native: pending walkthrough 7. | Pending |
| G05 | Bracket matching and pairing. | Unchanged renderer; the bracket highlight is visible in the new palette (`TestQuietUIShots`, split shots). | Auto: existing bracket and pairing tests, `TestShotDecorations`. Native: pending walkthrough 2. | Pending |
| G06 | Draw and operate every caret. | Unchanged native editor renderer (no textarea). | Auto: `TestMultiCursorAddNextMatch`, `TestSplitRightKeepsCaretsApart`, `TestEOLKeepsSplitCaretsBeforeNewline`. Native: pending walkthrough 2. | Pending |
| G07 | Sticky scopes, overview, gutter and word highlights while editing. | Unchanged; visible while editing in the shots (gutter bars, overview marks, current line, brackets). | Auto: `TestShotSticky`, `TestStickyCrumbsAndSymbols`, `TestOverview*`. Native: pending walkthrough 2. | Pending |
| G08 | Real windows and folder routing. | Unchanged. | Auto: window and project tests in `ui/gui`. Native: pending walkthrough 1. | Pending |
| G09 | Keep yan's Vim behavior as is. | Unchanged; no Vim features added or removed. | Auto: `TestVim*` (12 tests). Native: pending. | Pending |
| G10 | A merge beside a terminal needs room: minimum widths, compact overflow, a reversible maximize. | New command `workbench.action.toggleMaximizeEditorGroup` (**显示 › 最大化／还原编辑区**, the palette, a title-bar button, and a button on the focused merge column). A merge maximizes as its three columns, otherwise the current column. The sidebar and terminal panel are not drawn, and the shown columns share the width evenly. Nothing is moved or closed: hidden columns stay in the frame, so their terminals keep running in the same session, and restoring brings back the exact widths, locks, carets and scroll. The maximize ends by itself when focus moves to a hidden column or the sidebar, when a shown column closes, or when the merge ends. Conflict buttons that don't fit already go into "⋯" (Mayoi #63). The merge has no header to crowd: its summary is a single status-line message. The TUI skips hidden columns too. | Auto: `TestMaximizeRestoresTheLayout`, `TestMaximizeEndsWhenFocusLeaves`, `TestMaximizeMergeKeepsTheTerminal`, `TestMaximizeMergeBesideATerminal` (same `termSession`, columns of 38+ characters), `TestLayoutSkipsHiddenColumns`; shots `maximize-merge*`. Native (quiet-UI walkthrough, below): a merge opened from Source Control beside an editor terminal at 1100×720; a live shell answered before maximizing; maximized in light and dark; restored with the same session; the same shell answered again with its earlier output still on screen. | Verified (maximize/restore); resolving conflicts natively is still walkthrough 8 |
| G11 | Real configuration formats and paste/update semantics. | Unchanged: `config.toml`, `.yan.toml`, `keybindings.json`, `state.json`; image-only paste; real updater. | Auto: configuration, paste and updater tests. Native: pending walkthrough 11. | Pending |
| G12 | Include alpha.29+ features. | Present in the baseline inventory: `workbench.action.files.newUntitledFile`, `workbench.action.files.saveAs`, `workbench.action.files.newFile`, Mermaid with `[ui] preview_mermaid`, and untitled-document recovery (alpha.30). | Auto: `TestUntitled*`, `TestSaveAs*`, `TestMermaid*`, `TestKeepAndRestoreDrafts` and the other recovery tests. Native: recovery has its own disposable-bundle proof from its PR; the rest are pending. | Pending |

## Capabilities

The rows below follow the task's inventory sections A–L. "Unchanged" means the batch touched no code
of that capability; its evidence is the existing regression suite passing on the redesigned tree.

| Feature / command IDs | Baseline source and behavior | New entry point | State / persistence rules | Verification and evidence | Status |
| --- | --- | --- | --- | --- | --- |
| A. Projects, windows, rail, welcome, recents (`yan.project.*`, `workbench.action.newWindow`, `yan.openFolderNewWindow`, `workbench.action.openRecent`) | `ui/gui/projects.go`, `core/app/projects.go`, `docs/window-workflows.md` | Same places. The rail is 48 px with a 6 px gap, the design's metrics. | Project order, looks and icons in `state.json`; sessions per root. | Auto: window/project tests (79 in the area). Native: pending walkthrough 1. | Pending |
| A. Sessions and untitled recovery (`SaveSession`, `RestoreSession`, `core/recovery`) | alpha.30 recovery (PR #65), `core/recovery`, `core/app/recovery.go` | Unchanged | Checkpoints per session, locks, generation fencing | Auto: `TestKeepAndRestoreDrafts`, `TestRootlessDraftsAreAdopted`, `TestWindowCloseKeepsDrafts`. Native: disposable-bundle proof from PR #65; to repeat with the new UI. | Pending |
| B. Explorer: tree, refresh, new file/folder, rename, trash, copy/paste, paths, reveal, Open in Terminal, drag and drop (`explorer.*`, `workbench.files.action.*`, `copyFilePath`, `copyRelativeFilePath`) | `ui/gui/sidebar.go`, `core/app/files.go` | The sidebar's **文件** tab. The folder row now carries **新建文件…**, **新建文件夹…**, **在资源管理器中显示当前文件** and **刷新资源管理器** as icon buttons; context menus and keys are unchanged. Rows are 27 px. | Unchanged | Auto: explorer and file tests, menu gate. Native: pending. | Implemented—unverified |
| C. New Untitled File, New File…, Save, Save As, Save All, autosave, close/discard, reopen closed | `core/app/untitled.go`, `commands.go`, `recovery.go` | Unchanged | Untitled drafts kept across quits; Save As writes before binding | Auto: `TestUntitled*`, `TestSaveAs*`, `TestNonInteractiveSavesSkipUntitled`, `TestCloseUntitled`. Native: pending walkthrough 3. | Pending |
| C. Editing: multicursor, selections, indentation, comments, line operations, smart Enter, pairing, IME | `core/edit`, `core/app/editing.go` | Unchanged | Shared document, independent carets | Auto: about 70 editing tests. Native: pending walkthrough 2 (real IME). | Pending |
| C. Decorations: syntax, line numbers, guides, bracket/word highlight, gutter changes, overview, sticky scopes, breadcrumbs | `ui/gui/code.go`, `crumbs.go` | Breadcrumbs are 26 px. In narrow columns the review button keeps its label on one line and the scope crumbs shorten instead (it used to wrap). | Unchanged | Auto: `TestShotDecorations`, `TestShotSticky`, overview tests; shots at both sizes. Native: pending. | Implemented—unverified |
| C. Encoding, EOL, reload, conflicts, read-only, large files | `core/text`, `core/edit`, `freshness.go` | Unchanged | Unchanged | Auto: reload/conflict tests. Native: pending walkthrough 3. | Pending |
| D. Quick Open, palette, Go to Line/Symbol, definition, back/forward | `core/app/overlay.go`, `definition.go`, `navigation.go` | Unchanged; the palette shows the design's surface and shadow. | Unchanged | Auto: navigation tests, command gate. Native: pending. | Pending |
| D. Find/Replace and project search | `core/app/find.go`, `search.go`, `ui/gui/find.go`, `search.go` | Sidebar **搜索** tab | Unchanged | Auto: find/search tests (50 in the area). Native: pending walkthrough 4. | Pending |
| E. Tabs: preview/pinned/dirty, close variants, middle-click, reorder, drag, split, locks, column moves | `ui/gui/view.go`, `core/app/tabs.go` | Same strip at 34 px. The focused column's active tab has a 2 px accent line, other columns' active tabs a muted one. Names are 13 px; the dirty dot turns into a 20 px close target. | Unchanged | Auto: tab/column tests (60 in the area). Native: pending. | Implemented—unverified |
| E. Sidebar visibility and side, panel sizes, focus | `view.go`, `core/app` | Unchanged | Unchanged | Auto: layout tests. Native: pending. | Pending |
| F. Terminals in the panel and editor, positions, copy-on-select, links, paste, close lifecycle | `ui/gui/terminal.go`, `termtabs.go`, `internal/termsession` | Unchanged | Unchanged | Auto: terminal tests (35). Native: pending walkthrough 5. | Pending |
| G. Source Control: repositories, groups, multiselect, stage/unstage/discard, commit variants, branches, remotes, history | `ui/gui/scm.go`, `core/app/scm.go` and related files | Sidebar **改动** tab with a count chip (it was an icon with a badge). The header's **以树的形式查看** and **刷新** moved into one **⋯** menu (更多源代码管理操作), so all four view tabs fit in the default sidebar width. | Unchanged | Auto: Git/SCM tests (99). Native: pending walkthrough 6. | Implemented—unverified |
| H. Diffs and All Changes: layouts, highlight, context, whitespace, sync scroll, aligned rows, Append/Revert, stage | `core/app/diff*.go`, `ui/gui/splitdiff.go` | Unchanged | Unchanged | Auto: 46 diff tests. Native: pending walkthrough 7. | Pending |
| I. Three-way merge and conflict flows | `core/app/merge*.go`, `ui/gui/merge*.go` | Unchanged; G10 still to do. | Unchanged | Auto: 22 merge tests. Native: pending walkthrough 8. | Pending |
| J. Review comments and terminal handoff (`review.*`) | `core/review`, `ui/gui/review.go` | Breadcrumb **+ 意见** / **发送意见** buttons: 20 px, single line | Unchanged | Auto: review tests, `TestNativeTerminalReviewHandoff`. Native: pending walkthrough 9. | Implemented—unverified |
| K. Markdown preview and live editing, SVG, Mermaid | `ui/gui/markdown*.go`, `mermaid.go` | The preview reads at the design's proportions, following the user's code size: prose at 8/7 of it with a 1.75 line height (16/28 at the default 14), a reading column of 80 zeros at that size (about 770 px), headings scaled from the prose. Code blocks keep the code size and line height. The block editor uses the prose line height, so entering edit mode doesn't move lines. | `font_size` and `line_height` stay the user's; only defaults changed | Auto: 38 tests including `TestMermaid*`. Native: pending walkthrough 10. | Pending |
| L. Code font defaults | `core/config/config.go`, `ui/gui/gui.go`, `core/app/settings.go` (zoom), `fonts.go` | The default code size is 14 (was 13) with line height 1.6 (22.4 px, was 1.55). Zoom reset (`workbench.action.zoomReset`) returns to the configured default instead of a literal 13. A saved `font_size`, `line_height` or zoom still wins. | `state.json` / `config.toml` unchanged | Auto: zoom, import and theme-preview tests derive their expectations from `config.Default()`. Native: walkthrough captures. | Implemented—unverified |
| L. Themes (Mayoi, Claude, Matrix), appearance, preview/confirm/cancel, effects | `core/style/theme.go`, `ui/gui/theme*.go` | The default **Mayoi** theme is now the design's warm paper with a terracotta accent, light and dark. Claude and Matrix are unchanged. The previous Mayoi colors are gone; that is the redesign's intended change. | Theme choice and appearance in `state.json`, unchanged | Auto: `TestDefaultThemeContrast`, theme tests, before/after shots. Native: pending walkthrough 11. | Implemented—unverified |
| Platform names for keys and places in shared controls | `ui/gui/find.go`, `terminal.go`, `keys.go`; `app.FileManager`, `app.TrashName` | The find bar's previous/next and the terminal panel's collapse button used to show macOS symbols on every system (`⇧↩`, `↩`, ``⌃` ``). They now come from the profile: `Shift+Enter`, `Enter`, the toggle command's own key. File-manager and trash names follow the same `hostOS`. | Respects user keybindings for the panel toggle | Auto: `TestWindowsLabelsUseWindowsKeys`, mutation-checked against the old label. Native Windows/Linux: not available. | Implemented—unverified |
| L. Settings, keybindings, imports, guide, notices, About, updates, Vim, fonts, zoom | `core/config`, `core/app/keybindings.go`, `ui/gui/updates*.go` | Sidebar **学习** tab for the guide; everything else unchanged | Unchanged | Auto: settings tests, command and menu gates. Native: pending walkthrough 11. | Pending |

## Native walkthrough

`native-quiet-ui-proof.py` (kept with the build evidence) builds this tree with `-tags selftest` into a
disposable bundle with its own bundle ID, configuration and runtime directory. It opens a fixture
repository that has a real merge conflict, and runs the `quiet-ui` selftest in the real window.

- **Run 2026-10-27** on `75949fc` plus the batch 3 changes: 22 PASS, 0 FAIL.
  - The window is exactly 1440×900 and 1100×720, in light and dark appearances. The appearance is switched with the real `yan.appearance.*` commands.
  - Captures of the window's own drawing, at 2× (Retina): workspace and Source Control at each size and appearance, plus the merge beside a terminal, maximized (light and dark) and restored.
  - The title bar: macOS reports its window controls as 32 px high and 69 px wide on the left. yan draws its bar at 32, the larger of the native height and the design's 30, so the traffic lights are never clipped. This is a measurement, not a picture: this machine's shell has no Screen Recording permission, so the OS window chrome itself was not captured.
  - Live terminal continuity across maximize/restore, as described for G10.
- **Not covered natively yet**:
  - real keyboard and mouse input, including the Chinese IME (the selftest drives the app through the same commands the input reaches);
  - native save and close dialogs;
  - multi-window routing;
  - Markdown and Mermaid interaction;
  - settings reload;
  - walkthroughs 1–7 and 9–12 beyond the captures.

The walkthrough found two problems, both fixed in batch 3:
- **"显示资源管理器" did not switch views.** `workbench.view.explorer` (⇧⌘E, **显示 › 资源管理器**) left Search or Source Control on screen and only moved the focus. It now shows the Explorer (`TestShowExplorerFromAnotherView`).
- **The selftest's own appearance switch was ignored** until it used the real appearance commands.

## Changes by batch

1. **Tokens and shell.**
   - The default theme takes the design's light and dark colors. Secondary text is `#6A675F`, slightly darker than the design's `#706D65`, because `#706D65` measured 4.42:1 on the sidebar color.
   - Design metrics: title bar 30 (the native title bar height still wins if larger), tabs 34, breadcrumbs 26, rail 48, tree rows 27, control radius 5, and the design's floating shadow.
   - Text view tabs in the sidebar, and Explorer actions on the folder row.
   - The breadcrumb review button no longer wraps.
   - Menu items now carry their command IDs, which the menu gate reads.

2. **Room and platforms.**
   - G10 maximize/restore, described above.
   - Source Control's header actions share a **⋯** menu.
   - Key labels in shared controls come from the platform profile.
   - Merge and terminal shots at 1100×720.

3. **Gates and native evidence.**
   - The context-menu gate.
   - The native quiet-UI walkthrough.
   - `workbench.view.explorer` shows the Explorer from any view.

4. **Typography.**
   - Code defaults 14 / 1.6.
   - Markdown reading proportions.
   - Markdown shots at both sizes.

## Not yet done

- Context menus not yet gated: repository panel rows, the repositories header, panel terminal tabs, project squares, the tree's blank area.
- Native walkthroughs: the native quiet-UI walkthrough covers sizes, appearances, captures and G10. The input-, dialog- and multi-window-heavy walkthroughs (1–7, 9–12) still need real interaction, including a real Chinese IME.
- Matching the design's other surfaces: the Source Control panel, terminal panel header, merge header and Markdown reading width have not been compared row by row yet.
