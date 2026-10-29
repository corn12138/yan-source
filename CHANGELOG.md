# Changelog

What changed for people using yan, newest first. `scripts/bump.sh` turns `## Unreleased` into the next
version's section, and that section becomes the "What's new" of the release notes in `docs/releases/`.

## Unreleased

- Yan's own source is licensed under MIT; see `LICENSE` for reuse terms and `THIRD_PARTY_NOTICES.md` for included third-party code. To build from source, follow the clone and `make build` steps in `README.md`.
- On macOS and Linux, saved sessions (including recent paths and review notes) and crash logs are now readable only by your account. Existing session permissions are tightened when the session is next saved; crash-log permissions are tightened when the app next starts.

## 0.1.0-alpha.35

- Files containing a line longer than 64 KiB are refused before text layout, with an explanation, so minified bundles and source maps cannot stall the editor when opened or restored. Session restore skips these files and keeps other tabs. If an external change introduces a long line, the current buffer and undo history stay intact; format the file externally before reopening or reloading it. UTF-8 BOM and CRLF line endings do not consume the line budget.

## 0.1.0-alpha.34

- Preview LaTeX formulas in Markdown with `$…$`, `$$…$$`, or fenced `math` blocks. Formulas render locally in the current theme; hover a display formula to copy its TeX or view its source. Unsupported formulas keep their source with an explanation. Set `[ui] preview_math = false` in `config.toml` or `.yan.toml` to show formulas as source.

## 0.1.0-alpha.33

- **Close Other Projects** asks for confirmation before stopping the other projects and their terminals. Unsaved files are handled one project at a time; Cancel or a failed save stops the remaining closes, and the project you keep remains open.

- Sidebar view names wrap onto another row when space is tight, so every view stays accessible in either language even with a large change count.

- **Open Preview to the Side** reuses the nearest unlocked column on the right, or otherwise on the left. It creates a new column only when no other unlocked column is available, keeps the source focused, and reuses an existing tab for the same document.

- yan now speaks English as well as Simplified Chinese. Choose the interface language under **Settings → General → Interface Language** (设置 → 通用 → 界面语言): open Settings with Cmd+, on macOS (Ctrl+, on Windows and Linux), from the application menu, or with **Preferences: Open Settings** / **Preferences: Interface Language…** in the command palette. The default, **Follow System**, takes the first of your system's preferred languages that yan supports and otherwise uses English; the panel shows which language that is. Each language keeps its own name in the list, so you can always find your way back.
- A language change is saved at once and applies the next time yan starts. Until then every window keeps its current language, and Settings says **Restart Yan to apply the language change**. yan never quits or restarts on its own, so unsaved work and terminals are safe. On macOS, a language chosen for yan in System Settings is followed too.
- The language can also be set with `[ui] language = "system"`, `"zh-Hans"` or `"en"` in your user `config.toml`. A choice made in Settings wins over it. A project's `.yan.toml` cannot change the interface language: it is ignored there with a warning, and the project's other settings still apply. The terminal app uses the same choice, and with Follow System it reads `LANGUAGE`, `LC_ALL`, `LC_MESSAGES` and `LANG`.
- Menus, the command palette, Settings, Source Control, diffs and merges, search, previews, the feature tour, Yan’s dialog text and the update window are translated; controls supplied by macOS inside native file panels still follow the system language. File names, document text, terminal output and Git's own messages are shown as they are.
- `workbench.action.openSettings` in your keybindings (VS Code's Cmd+,) now opens Settings. **Open User Configuration File**, **Preferences: Open Keyboard Shortcuts (keybindings.json)** and `workbench.action.openSettingsJson` still open the files, and Settings links to both.


## 0.1.0-alpha.32

- The feature guide (sidebar **学习**, **Help → 功能导览（学习）**, or "Learn: Feature Guide" in the command palette) is now a hands-on tour. New lessons cover going to a definition by key or by link-click, moving editors between columns, maximizing and restoring the editor area, a three-way merge, and Markdown footnotes and Mermaid diagrams. Each step shows the shortcut your keymap has, the command's palette name and its menu path, and completes when you actually do it.
- Source Control and three-way merge lessons run in a throwaway practice repository next to your configuration, never in your project. The window switches to it for the lesson; **回到 <your folder>** in the guide (or "Learn: Leave Practice Folder") brings your project back.
- Go back a step, restart a lesson with a fresh practice file, or skip a step from the guide. An unfinished lesson resumes where you left it the next time you open the guide, even after restarting yan.
- On first launch the window app offers the tour once. **开始导览** opens it; **不用了** hides the offer for good.
- Steps, lessons, menu items and command palette entries for features that are not in an official release yet are marked **未发布** (Unreleased), so people on the release do not look for them.

## 0.1.0-alpha.31

- The Markdown preview shows footnotes as on GitHub. A reference such as `[^1]` becomes a raised number that scrolls to its note. Notes are listed after a rule at the end, slightly smaller, each with a ↩ link back to where it was referenced.
- Bare URLs in the Markdown preview become links after punctuation too, such as the full-width colon in `链接：https://…`, after Chinese text, or inside full-width parentheses. URLs inside code and existing links are left alone.
- Mermaid labels stay readable on nodes and subgraphs whose `style` or `classDef` sets a fill. When the theme's text color would be hard to read on that fill, the label uses the theme's text or background color, whichever contrasts more. Labels with their own `color` keep it.
- A quieter look: the default theme (Mayoi) is now a warm paper background with a restrained terracotta accent, in both light and dark appearances. Claude and Matrix are unchanged. Tabs, breadcrumbs, the project rail and file rows use slightly more compact, evenly spaced sizes, and the focused column's active tab has a thin accent line.
- Code now defaults to 14 pt with slightly more line spacing, and Markdown previews read like a document: larger body text with generous line spacing and a wider reading column, while code blocks keep the code size. Your own font size, line height and zoom still apply, and resetting zoom returns to the new default.
- The sidebar's views are now named tabs: 文件, 搜索, 改动 (with the number of changes) and 学习. The Explorer's buttons (new file, new folder, show the current file, refresh) sit on the folder's row. All commands, menus and shortcuts are unchanged.
- In narrow columns, the breadcrumb's **+ 意见** button no longer wraps onto two lines.
- **View → 最大化／还原编辑区** (also in the command palette, as a title-bar button, and on the focused column of a three-way merge) lets the current column, or a merge's three columns, fill the window without the sidebar or terminal panel. Run it again to restore exactly what was there: column widths, locks, cursors, scroll positions and running terminals are untouched. Moving to another column or the sidebar, closing the column, or finishing the merge restores the layout automatically. It works in the terminal app too.
- Source Control's tree-view and refresh buttons are now in the **⋯** menu at the top of the sidebar.
- **显示 › 资源管理器** (and its shortcut) now switches the sidebar to the Explorer when Search or Source Control is showing; before, it only moved the focus.
- On Windows and Linux, the find bar and the terminal panel name keys the way those systems do (for example **Shift+Enter** and **Ctrl+`**) instead of showing macOS symbols.

## 0.1.0-alpha.30

- Untitled documents now survive quitting, closing a window or project, switching folders and update restarts. yan keeps their text, line endings, column and cursor, and brings them back unsaved the next time that folder opens, or, for windows without a folder, when the window app starts. Closing no longer asks about them. Closing an untitled tab twice still discards it for good. If yan cannot keep them (for example, the disk is full), closing asks as before and says why.
- While yan is running, edits to untitled documents are backed up within about a second, so they also survive a crash. Backups live in the `recovery` folder next to your configuration, readable only by you. Two yan instances on the same folder keep their drafts separately. Untitled documents from the terminal app without a folder come back in the window app.

## 0.1.0-alpha.29

- **New Untitled File** (Cmd+N on macOS, Ctrl+N on Windows and Linux) opens an empty document named 未命名-1, 未命名-2, … without creating a file. The first Save asks where to save it. Save All, autosave and closing never pick a location for you: they leave untitled documents unsaved and tell you how many are left. The old flow that picks the location first is still under **File → 新建文件…**.
- **File → Save As…** (Cmd+Shift+S on macOS, Ctrl+Shift+S on Windows and Linux) saves the current document to a new file and continues editing it there; undo history is kept. yan writes the new file before the document switches to it, so a failed save changes nothing. It does not replace a file that changed after you confirmed, or a file already open in another tab. In the terminal app, Save As asks for the path and confirms before replacing.
- The Markdown preview in the window app draws `mermaid` code blocks as diagrams (flowcharts, sequence, class, state, Gantt, pie, git graphs and more) in the colors of the current light or dark theme, with Chinese and other labels drawn in the app's own font. Diagrams wider than the text column use the full width of the pane. Hover a diagram for **显示源码** (Show Source) and **复制代码** (Copy Code); a block that cannot be drawn stays as code with the reason underneath. Set `[ui] preview_mermaid = false` in `config.toml` or `.yan.toml` to show Mermaid blocks as code.
- `.mmd` and `.mermaid` files can be previewed as their diagram with **切换预览** or **在右侧打开预览**, and the preview redraws as you edit.
- Untitled text is not yet restored after quitting. Closing a window whose untitled documents have changes keeps it open after **Save All**, so you can save them or close their tabs.

## 0.1.0-alpha.28

- Go to a name's definition: press F12, choose **Go → Go to Definition**, or hold your platform's link-click modifier (Cmd on macOS, Ctrl on Windows and Linux) and click the name. While the modifier is held, the name under the pointer is underlined. Yan looks in the current file first (nearest local variable, the enclosing function's parameters, then the file's declarations), then in files of the same language in the open folder, preferring the folder named by a `pkg.` qualifier and then the current folder. When several definitions match, pick one from the list. This is a name-based heuristic without a language server: it does not follow types or imports, so a method shared by several types offers all of them.
- Go back and forward after a jump with **Go → Back / Forward**: Ctrl+- and Ctrl+Shift+- on macOS, Alt+Left and Alt+Right on Windows and Linux (`workbench.action.navigateBack` / `navigateForward`). On Windows and Linux, word movement remains on Ctrl+Left and Ctrl+Right.
- Go to Symbol no longer crashes on a Go line that starts with `func() {`.
- Git and its hooks in the macOS and Linux window app now use the command paths configured by your login shell, so tools such as Node and pnpm can be found when you open yan from the desktop. Restart yan after changing your shell setup. If shell discovery fails or times out, Git keeps the inherited paths.

## 0.1.0-alpha.27

- Open file paths printed in the terminal with your platform's link-click modifier. Relative paths follow the shell's reported working directory after `cd`, with the startup folder as a fallback; `~/` paths and `:line:column` locations are supported. Existing filenames containing colons take precedence, ignored files can open, and dirty buffers and locked columns are preserved.
- Paste copied files into a terminal to insert their full paths with shell quoting, including multiple files and spaces. File clipboard data takes precedence over its text representation; ordinary text and image-only paste retain their existing behavior. Pasting does not press Enter.
- Preview color themes from **View → Theme → Color Theme…**, the welcome page, or `workbench.action.selectTheme`. Use Up/Down to preview, Left/Right to switch appearance when the search is empty, Enter or a theme click to save, and Escape or an outside click to cancel. Search supports abbreviated names. Preview stays local to its window and is not saved by unrelated settings or session updates; reduced motion disables color transitions.
- Each window now keeps its own light/dark palette. **Follow System** reads the desktop appearance even when another project uses Matrix or an explicit light/dark theme; native system dialogs follow the desktop.

## 0.1.0-alpha.26

- macOS update checks and downloads now use the system HTTP, HTTPS or SOCKS proxy when the app is launched from Finder or the Dock without proxy environment variables. Explicit environment proxies and bypass rules still take precedence. Update errors now suggest checking connectivity or using a manual download.
- Switching between Find and Replace reliably focuses and selects the requested input, including while the window is processing background work.

## 0.1.0-alpha.25

- Commit tabs can show a file tree beside the diff: use the file-list button or **Git: Show/Hide Commit Files**. Click a file to jump to its historical changes; Enter pins the preview. The list follows diff scrolling, supports folders or a flat list, remembers its visibility and width, and hides in narrow columns. Expand a commit in Source Control to browse its files with the mouse or arrow keys. Historical added, deleted and renamed files retain their status and original repository.

- Reopening Find, or invoking Find again while its input is focused, selects the query so typing replaces it. Reinvoking Replace selects its current replacement text. Chinese input composition stays in the input until committed, without changing the search or the document midway.

## 0.1.0-alpha.24

- Choose **View → New Terminals in Editor Area** (显示 → 默认在编辑区新建终端), or set `[terminal] location = "editor"` in config.toml, to open new terminals as editor tabs. The Explorer's **Open in Terminal** now works on files and folders as well as its blank area, and follows this preference. Toggle Terminal and the panel's + still use the panel; existing sessions stay where they are. The menu choice is saved and takes precedence over config.toml; turn it off to restore the panel default.
- Changing settings or terminal appearance no longer crashes when an editor terminal failed to start; its error tab remains available to close.

- Closing a terminal now ends what you ran in it. Before, a program that keeps running after its terminal hangs up, such as many dev servers and watchers, stayed alive in the background after you closed the terminal, the window or Yan, with no terminal left to see or stop it. Now Yan gives the program two seconds to exit and then stops it. Programs that put themselves in the background on purpose (for example `docker compose up -d`, or a program started with `setsid`) keep running. To hide the terminal panel without stopping anything, use its × button or Ctrl+`; to stop one terminal, use the trash button or close its tab.
- When a diff or merge column is too narrow for the buttons on a change (for example the three columns of a merge in a small window), the buttons that do not fit go into a "⋯" menu at the end of the row instead of running over the code; the least needed go first.

## 0.1.0-alpha.23

- Empty columns beside an open file no longer repeat the welcome page. They show what the column is for and a "Close this column" button, and their header has the lock button and a close button, so an empty locked column can now be unlocked or closed. With no file open, only one column shows the welcome page.
- In Source Control, click a section's title (Merge Conflicts, Staged Changes, Changes, Untracked Files, Commits) to fold or unfold it; the arrows were there before but did nothing. A folded group still shows its file count, "Stage All" and "Discard All" still act on every file in it, and Yan remembers what you folded. The command palette has "Git: fold or unfold the commit history".
- Choose the code font, its size and the line height in the window: View > Font, or "字体" in the command palette. The font list shows the monospaced fonts installed on this computer, each previewed in itself with Chinese, and marks the ones that keep Chinese aligned; you can also type a comma-separated list of families. The choice applies at once and is remembered; `[ui]` in `config.toml` still sets the starting values.
- The welcome page's shortcuts are now buttons: click a row to do it. With a folder open they show the ways around it — quick open, go to symbol, search in files, Source Control, the command palette, the explorer and the terminal; without one, how to open a folder. On macOS, menus and hints now show the ⌘ shortcut when a command has one (⌘Q to quit, ⇧⌘P for the command palette).

## 0.1.0-alpha.22

- In the two-pane side-by-side diff, hover over a line number on the right to add a review comment there, as in other diffs; lines with comments keep their button. With nothing selected, Copy copies the current line from the side you last clicked, not both sides joined.

- On Windows and Linux, the menus open from a menu button at the left of the title bar instead of a native menu bar that took a whole row. The button opens the same menus, with check marks and shortcuts.
- On Linux, the window buttons follow Yan's theme colors without the desktop theme's shadows, and the title bar's empty labels no longer block clicks on the buttons beside them.
- Choosing Light or Dark appearance now always wins over the desktop's own light or dark setting.

## 0.1.0-alpha.21

- Switching a Markdown file between its text and its preview (⇧⌘V on macOS, Ctrl+Shift+V elsewhere) keeps your place both ways: the preview opens where the text was, and the text comes back where you had read to in the preview.

- Yan checks for a new version every hour instead of once a day, and no longer pops up the update window on its own. When a new version is out, the status bar, the welcome page and the Help menu (“Update to …”) say so; click to open the update window. A version you chose to skip stays hidden. Turn the hourly checks off with “Check for Updates Automatically” in the Help menu.
- The branch list (click the branch in the status bar, or “Switch Branch…”) always starts with “Create Branch…”. It asks for a name, tells you as you type if the name is not allowed or already taken, then creates the branch and switches to it.

## 0.1.0-alpha.21

- Side-by-side diffs in the window now show each side in its own pane, with a gutter between them. Each change is a curved band joining its lines on the left to its lines on the right. Its “»” arrow takes the change back, or appends the original lines when you hold Ctrl or Alt; hovering the arrow highlights the lines it will replace. Lines written by an arrow or a Revert/Append button glow for a moment. Going to the next change or file glides there instead of jumping.
- Two new switches in the Git menu, also kept in `config.toml` under `[diff]`: “Synchronized Scrolling” (`sync_scroll`) and “Line Alignment” (`aligned_rows`). With synchronized scrolling off, the wheel scrolls only the side under the pointer; turn it back on and the sides line up again at the right side's position. With line alignment off, each side shows only its own lines, with no blank rows for the other side.

## 0.1.0-alpha.20

- Edit your working copy from a diff: press `i` on a change, or click “✎ Edit” on its `@@` line, and the file opens in the column beside the diff at that change. As you type there, the diff updates; undo, redo and saving are the file's own. Staged changes and commits stay read-only.

- In the three-way merge view, each change from one side that does not conflict has an “» Apply This” button in its side column while the result does not have it yet. The Git menu's “Merge Conflicts” submenu can apply all of them at once, and can turn off applying them automatically when a merge view opens.

- File headers in All Changes now offer the right whole-file action: “Restore File…” brings a deleted file back, “Delete File…” moves a file Git does not track to the trash, and “Discard Changes…” reverts a changed file. Each asks first.
- Diffs keep their place: unfolding unchanged lines, switching the layout, or resizing the window keeps the same line at the top of the view, and the cursor stays on its line.

- The status bar has a sync button next to the branch. It pulls (fast-forward only) and then pushes the branch to its upstream; for a branch with no upstream yet, it publishes the branch. Click the branch name itself to switch to or create a branch from the same searchable list as Source Control.
- On Linux, changing the menus (opening a folder, changing a setting) no longer risks crashing the window: the menu bar is replaced after the window has finished drawing.

## 0.1.0-alpha.19

- Source Control has a Repository panel under the commit history. Click “Repository” to open it, then pick “Branches” or “Remotes”.
  - Branches lists your local branches, with their upstream, how far ahead or behind they are and when they last changed. Open a branch to see its history, and its unpushed or unpulled commits; long histories load more as you ask.
  - Remotes lists each remote with where it points. Open one to see its fetch and push URLs and its branches.
  - Right-click a branch to check it out, compare it with HEAD or delete it, and a remote to change its URL, prune it or remove it. Click a commit to open it.
  - Nothing is read until you open it, and Yan remembers whether the panel is open and which tab it shows.

- Big files no longer slow down All Changes. A file with more than 2000 changed lines starts folded, and a file over 1 MB is not even read until you unfold it; its header shows its size instead. Once you unfold a file it stays unfolded when the changes refresh.
- Yan notices a repository created or removed in the open folder (for example by `git init` in a terminal) right away, and no longer rescans the folder for repositories every five seconds.

- A long commit message in a commit's tab now shows its first five lines and folds the rest away; press Enter (or double-click) on “⋯ … more lines” to read all of it, and again to fold it.
- Sessions follow a moved folder. Yan now remembers a folder's open tabs relative to the folder, so after you move or rename the folder (keeping its name), opening it from the new place brings the tabs back.

- Choose several files in Source Control: Shift-click (or Shift+↑/↓) takes a run of files, ⌘-click on macOS (Ctrl-click on Windows and Linux) adds or removes one, and ⌘A (Ctrl+A) takes them all. Space, Delete and a chosen row's stage, unstage and discard buttons then act on exactly those files. A plain click goes back to one file.
- When All Changes is open, clicking a file in Source Control scrolls All Changes to it instead of opening another tab (double-click still opens the file's own diff). Scrolling All Changes highlights the file at the top in Source Control.

- All Changes is easier to scan and act on:
  - Each file header shows the file's status letter and a five-cell bar of added (green) and removed (red) lines next to the counts.
  - Each change's `@@` line names the function or block it is in.
  - When you scroll past a file's header, it stays pinned at the top; click it to jump back.
  - File headers have “Open File” and “Stage File” (or “Unstage”) buttons; conflicted files have “Three-Way Merge”.
  - Each change has “Stage”, “Revert” and “Append” buttons (unstaged changes) or “Unstage” (staged ones). Single-file diffs offer “Stage” too.
- “Diff: Show Whole Files” in the command palette shows every unchanged line of the diff instead of folding them; run it again to fold them back.

## 0.1.0-alpha.18

- Resolve merge conflicts in a three-way merge view. Open a conflicted file from Source Control (or press Enter on it in All Changes) and it appears in three columns. “Ours” (your branch) is on the left and “theirs” (the branch being merged in) on the right, both read-only. The editable result sits in the middle. The result starts from the common ancestor, with every change that does not conflict already applied. Unresolved conflicts are highlighted.
  - Each conflict in a side column has “✓ Use This Side” and “✕ Ignore” buttons (or press `a` and `x` there; `]` and `[` move between conflicts). Using both sides keeps both, one after the other. Ignoring a side counts as handled.
  - The Git menu's “Merge Conflicts” submenu and the command palette can also resolve every conflict with one side.
  - You can edit the result like any file. Undo and redo also take back the choices you made.
  - When every conflict is handled, click “Mark as Resolved” in the status bar to save and stage the file; then the merge can continue. “Save and Close” keeps the conflict for later; reopening the file continues from where you left off. “Restore Conflict…” puts the file back in conflict.
- Work through a conflicted merge, rebase, cherry-pick or revert one file at a time. When a command stops for conflicts, Yan lists the conflicted files; reopen the list any time with “Conflicted Files…” in the Git menu's “Merge Conflicts” submenu. Pick a file to open it in the merge view. “Apply and Open Next” marks the file resolved and opens the next one. Files you have resolved stay in the list, ticked, even after you restart Yan, and you can put one back in conflict from there. The list is cleared once the command finishes.

- Revert or extend a single change right from a diff. Each change in a diff of your uncommitted work has “↶ Revert” and “+ Append” buttons on its `@@` line, or press `r` or `a` with the cursor in the change. Revert puts the original lines back. Append adds the original lines after your change. Either way, the edit goes into the file's own tab, which opens behind the diff if needed and shows the file as unsaved. Undo and redo in the diff (or in the file) take it back as one step. When comparing two files, the buttons copy a change into the right file or into the left one (`r` and `R`). Staged changes and commits stay read-only.
- Fixed: undo in a diff tab no longer empties the diff.

- Choose how diffs mark changed lines: by word (as before), by character, or the whole line only. Also choose how many unchanged lines (0 to 10) stay visible around each change; the rest fold away. Both are in the Git menu and the command palette (“Diff Highlight…”, “Diff Context Lines…”). The diff layout you pick (automatic, side by side or inline) is now remembered too. You can also set `highlight = "word" | "char" | "line"`, `context = 0`–`10` and `layout = "auto" | "split" | "inline"` under `[diff]` in `config.toml`.

- Moving between changes in a diff no longer wraps around silently. At the last (or first) change of a file, the status bar says so, and pressing `]` (or `[`) once more moves on to the next file with changes. In a single-file diff, that opens the next changed file from Source Control. At the very end of the diff it says so and stays put. The status bar also shows which change you are on, such as “change 2 of 5”. `.` and `,` say when you are at the first or last file. Set `cross_files = "always"` under `[diff]` in `config.toml` to move on at once, or `"never"` to stay in the file. “Next Change” and “Previous Change” (Alt+F5 and Shift+Alt+F5 by default) now work in diffs too, and the command palette has “Next File” and “Previous File”.
- Pressing Enter on a removed line in a diff now opens the file at that change instead of refusing, and the list of all changes opens with the cursor on the first change.

- Compare any two local files, in a repository or not: choose “Compare Two Files…” in the Git menu or the command palette and pick both files, or right-click a file in the explorer, choose “Select for Compare”, then right-click another and choose “Compare with Selected”. The diff shows the first file as the old side and updates as you edit either one. Press Enter on a line to open that file at the line. Comparing never stages either file.
- Diffs can ignore whitespace: choose “Diff Whitespace…” in the Git menu or the command palette to compare every character, ignore spaces at line ends, ignore all spaces and tabs, or also ignore blank lines. Only what diffs show, count and stage changes; your files keep their text. The choice is remembered, and you can also set `whitespace = "exact" | "eol" | "all" | "blank"` under `[diff]` in `config.toml`.

- Manage branches and remotes from the new Git menu or the command palette. You can rename the current branch, and delete a local branch: one with commits not merged anywhere else is deleted only after a second confirmation. You can also add, re-point or remove a remote, and clean up remote branches that were deleted upstream. All of these act on the repository selected in Source Control.

- New feature guide, like the Learn panel in PyCharm and other JetBrains IDEs. Open it from the graduation-cap button at the top of the sidebar, Help → Feature Guide, the welcome page, or “Learn: Feature Guide” in the command palette. It has seven short lessons: editing basics, multiple cursors, finding files and commands, columns and terminals, Source Control, live Markdown editing and personalising yan.
  - Each lesson opens a practice file of its own, outside your project, and shows step by step what to do, with the shortcut your keymap actually uses.
  - Each step is ticked off when you do it, whether by key, menu or command palette.
  - You can skip a step, and Yan remembers which lessons you have finished.
  - Below the lessons, tips introduce features one at a time.
  - In the terminal edition, steps that only the window version supports are skipped automatically.

- Project squares on the rail show the project's own logo when it has one: a `favicon`, `logo` or `icon` in the folder, `public/`, `app/`, `src/` or `assets/`, or the icon its `index.html` or Next.js layout declares. A mark you have chosen still takes priority.
- Give a project any picture as its icon: right-click its square and choose “Choose Icon…”, or drop a PNG, JPEG, WebP, SVG or ICO file onto it. Yan keeps a copy, so the icon stays even if the original file moves. “Reset Icon” brings back the logo or initials.

## 0.1.0-alpha.17

- Click a folder in the breadcrumbs above a file to list that folder's files in a tree. Expand folders there without changing the explorer, move with the arrow keys, and press Enter or click a file to preview it in the same column. This also works for files outside the open folder.

- Project squares on the rail now tell similar names apart. A one-word folder shows its first two letters, and when two labels would match, each shows the first letter that differs, as in “Dw” and “Dc” for Downloads and Documents. Chinese names show their first character, plus the first character that differs when two would match.
- To give a project its own mark (up to two characters or an emoji) and colour, right-click its square and choose “Customize Mark and Color…”, or run the command from the command palette. Yan remembers your choice for that folder on this computer.

- Drag a search result onto a column, its tab bar or a column edge to open the file there with the match selected, including in a locked column. Drag a changed file from Source Control onto a column, or out to your file manager, as you can from the explorer.

- Put the terminal panel at the bottom, left or right with View → Terminal Panel Position, or set `position` under `[terminal]` in `config.toml`. At a side, the panel spans the window's full height, sits outside a sidebar on the same side, and has its own width; drag its inner edge to resize it. Running terminals keep their sessions when the panel moves. Importing VS Code settings brings over `workbench.panel.defaultLocation`.

- The Markdown edit box now has document-editor keys:
  - Cmd/Ctrl+B, +I, +E and Cmd/Ctrl+Shift+X bold, italicize, code-format or strike through the selection; pressing them again removes the formatting.
  - Cmd/Ctrl+K turns the selection into a link.
  - Enter continues a list, numbered list, task or quote, and Enter on an empty item ends it.
  - Tab and Shift+Tab nest and un-nest list items.
- In the Markdown edit box, type `/` at the start of a line to insert a heading, list, task, quote, code block, table or divider. Narrow the menu by typing English or pinyin, for example `/dai` or `/code`, and choose with the arrow keys and Enter.
- Paste an image into the Markdown edit box to save it in an `assets` folder next to the document and link it where the cursor is. If the clipboard also has text, the text is pasted instead.

- Toggle Line Comment (Cmd+/ on macOS, Ctrl+/ on Windows and Linux) now works in files without a language mode, such as `.gitignore`, `.dockerignore`, `.env` files, `.editorconfig` and `.properties`, using `#`.
- Editing a `.gitignore`, in yan or elsewhere, updates the explorer at once: a folder you un-ignore stops being dimmed and its files appear in quick open and Find in Files. Folders you ignore are dimmed again.
- Import settings from VS Code, Cursor, VS Code Insiders, VSCodium, Windsurf, Trae or Kiro with “Import VS Code / Cursor Settings and Keybindings”, or from any `settings.json` with “Import Settings from File”. Font, font size, line height, tab size, word wrap, line numbers, autosave, light or dark appearance, sidebar side, terminal copy-on-select and Git blame take effect immediately, overriding sizes or themes chosen from the menus. Keybindings are added after your own. Your `config.toml` keeps its comments and other settings, and settings yan has no equivalent for are listed.
- Changing `font_family` or `line_height` in `config.toml` now applies to open windows without a restart. `font_family` accepts a comma-separated fallback list, as in VS Code.

- Edit Markdown right in its preview: double-click a paragraph, heading, list item, quote, code block or table row to edit that block's Markdown in place, with your input method as usual. Every change goes straight into the document, so the source column, unsaved marker, undo, autosave and Git all follow. Press Esc or Cmd/Ctrl+Enter, or click elsewhere, to see it rendered again. A single click or a drag still selects text.

- Lock a column with the lock button in its header, the tab menu or “Lock/Unlock Column” (`view.lockColumn`). Files opened from the explorer, quick open, search, Source Control and history, plus new terminal tabs, then go to the nearest unlocked column. Dragging a tab still puts it in a locked column. A locked column stays when emptied, keeps its width when other columns close, and is restored with the folder session. There is always one unlocked column for files.
- “Terminal: Move to Locked Right Column” (`view.terminalLockRight`, also in a terminal tab's menu) moves the current terminal, or the panel's, into a locked column on the right without restarting its shell.
- Middle-click a tab to close it without switching to it.

- Files changed by other programs now update while yan is open, without waiting for the window to regain focus. Clean documents reload (one undo step brings back your previous text), unsaved documents keep your edits and show a conflict notice, and deleted files keep their text in the tab until you save or close it. Changes that keep the file size and modification time are detected too.
- The explorer, quick open and Find in Files follow files created, changed or deleted on disk. Search results also follow unsaved edits as you type, and results from a folder you have switched away from never reach the new one.
- Saving `config.toml`, `keybindings.json` or a project's `.yan.toml` in another window or another program applies it in every open window; project settings still override user settings. If a reload cannot be read, yan keeps the last settings and keys that worked and shows the error in the status bar until the file is fixed.
- `when` conditions in `keybindings.json` are checked at the moment of the key press, including `editorHasSelection`, `terminalFocus`, `findInputFocussed`, `isInDiffEditor`, the sidebar views and `isMac` / `isWindows` / `isLinux`. When a condition does not hold, the key keeps its previous command, or types its character as usual.
- Bindings can pass arguments to `type`, `vscode.open` (absolute, `~/`, `file://` or folder-relative paths) and `workbench.action.terminal.sendSequence`. `runCommands` accepts these too, waits for each Git command to finish before running the next, and stops at the first failure.

## 0.1.0-alpha.16

- Saving and autosaving now detect external content changes even when a sync tool preserves the file size and modification time. Both versions remain intact until you explicitly choose how to resolve the conflict.
- Search replacements reject outdated single-result actions, re-read files for whole-file replacements, and verify external changes before writing. UTF-8 BOM files now use the same match positions as the editor while preserving their encoding and line endings.

- Open Markdown or SVG beside its source with the breadcrumb button or “Open Preview to the Side” command. Repeated requests reuse the preview, and the source keeps keyboard focus.
- Markdown previews now show GitHub alerts, heading anchors, content-sized table columns, selectable text, reading estimates and code blocks with a Copy Code button. Copies preserve the code's original whitespace.
- Markdown source and preview scroll together by source block without moving the source caret or selection. Preview mode and reading position return with the folder session; the terminal edition continues to open source text.

## 0.1.0-alpha.15

- Manage multiple local repositories in one Source Control sidebar, with collapsible panels and separate commit messages. Use the repository picker for history and Git commands, or choose automatic following of the active file. Right-click a repository header to open a separate project.
- Discover initialized submodules alongside nested projects and linked worktrees. Staging, commits, branch actions, confirmations and undo stay attached to their originating repository, while parent operations preserve unsaved child-repository buffers.
- Commit and comparison tabs keep their repository when you switch panels or restore a session. The terminal edition supports repository selection, per-repository messages and clickable change rows; Windows/Linux commit shortcut hooks use Ctrl.

## 0.1.0-alpha.14

- Open a folder containing several projects and Source Control now lists Git repositories up to two folders deep. Click a repository, or select it and press Enter in the terminal edition, to open its own changes and commit history. Project tabs reuse already-open repositories and retain the parent project's unsaved work.
- The repository list refreshes automatically as projects are cloned or removed, with a manual “Git: Refresh Repository List” command. Linked worktrees are recognized; hidden folders, `node_modules` and symlink children are skipped. An unreadable folder shows an error while retaining the previous list.

## 0.1.0-alpha.13

- Right-click a commit to export or copy its patch, including binary files, root commits and merge changes against their first parent. The terminal edition can save to a new path from the command palette.
- Compare a commit with HEAD in either direction, or select one commit as the starting point and compare it with another. Comparisons stay on those exact revisions, support inline and side-by-side layouts, and restore with the folder session alongside commit tabs and review comments.
- Press Enter or double-click a commit metadata row to copy its full hash. Commit menus can open the corresponding GitHub, GitLab or Bitbucket page; the terminal edition copies the link.
- The commit graph remembers “show all branches” between launches. Historical diffs identify renamed and binary files and show a clear error when a saved revision is no longer available.

## 0.1.0-alpha.12

- **Review code before handing it off.** Select lines in a saved file, working diff, staged diff or
  commit, then use Review → Add/Edit Comment or the line-side button. Comments keep the complete
  original quote, follow unchanged lines after edits, flag outdated quotes, and restore with the project.
- **Read and manage feedback in context.** Diff tabs show inline comment cards; source files and the
  overview strip mark commented lines. Use the Review menu to navigate, edit, copy, delete or clear
  comments. Deletion and clearing can be undone. In diffs, `c` comments, `y` copies the excerpt,
  `n`/`N` visit comments and Shift+J/K extends the selection. TUI drafts save with Ctrl+S.
- **Send all comments to the last focused project terminal**, including editor terminal tabs, without
  submitting them. If no terminal is ready for multiline paste, feedback is copied and retained.
  A successful handoff clears the list with an undo action. Source remains private; no AI service is required.

## 0.1.0-alpha.11

- **Signed automatic updates** are available in Help → 检查更新…. The app checks daily and offers
  release notes, installation and restart; automatic checks can be disabled in the same menu.
  Downloads come from the public `corn12138/yan-releases` repository and need no GitHub login.
- **Preview versions update correctly** from one alpha to the next. Preview and stable builds use
  separate feeds, and corrupt or incorrectly signed archives are rejected before replacing the app.
- **Save before restarting after an update** keeps the restart request active. Cancel or a save
  conflict leaves the current editor open.
- **Windows background helpers** request no console window, including Git commands used by status,
  gutter marks and diffs. Windows client packaging remains in development.

## 0.1.0-alpha.10

- **Claude's Clawd** appears on the welcome page and in the status bar. It blinks and bobs while
  idle and hops during Git operations or file searches.
- **Matrix digital rain** fills the welcome background without covering an open file. Both theme
  effects hold still when the window loses focus or the system requests reduced motion.
- **Turn theme decorations off** in View → Theme → 装饰效果 or with the “主题：切换装饰效果”
  command. The choice is remembered; `[ui] theme_effects = false` is also supported in configuration.

## 0.1.0-alpha.9

- **Open a folder in a new window** from the File menu or with Option+Command+O on macOS.
  An already-open folder is brought forward, including folders opened through a symlink. The
  Windows/Linux binding is Ctrl+K, Shift+O.
- **Recent folders in the Dock menu** open or focus their project window. File → Open Recent and
  the recent-folder picker use the same project routing and keep unsaved buffers intact.
- **Move the sidebar to either side** from the View menu or command palette. The choice is saved;
  `[explorer] right = true` is also supported in configuration, including the terminal UI.
- **Click indentation or LF/CRLF in the status bar.** Indentation changes subsequent editing;
  line endings convert the document in one undo step while retaining encoding and save-conflict checks.
- **Install the GUI `yan` command** from the File menu. It opens folders or files (`yan file:42`)
  in the running app, supports several arguments, and preserves any existing command at the install
  path. Installation is opt-in and does not edit shell configuration.

## 0.1.0-alpha.8

- **A strip along the editor's right edge** shows the whole file: the lines changed since the last commit,
  find's matches, the caret's line and the part in view. Click or drag it to go there.
- **Drag tabs.** Drag a tab along its strip to reorder it, or onto another column's strip to move it
  there; an accent line shows where it lands. A diff or commit tab moves too, caret and scroll with it,
  and a column left empty closes.
- **A tab's right-click menu**: close, close others, close all, split right, move to the next or previous
  column, copy its path or relative path, and reveal it in the file manager. Close others and close all
  leave tabs with unsaved changes open.
- **Alt/Option+Z wraps just this tab.** It turns word wrap on or off for the tab in front only, until the Word Wrap
  setting (View menu or command palette) changes again. The override is restored with the folder session.
- **Terminals in editor tabs.** Use View → 在编辑区新建终端, or move a panel terminal with its
  right-click menu or by dragging it onto a tab strip. Drag it back onto the panel header to return it;
  the same shell keeps running. Splitting opens a new shell in the same folder. Closed terminal tabs
  can be reopened, and folder sessions restore their directories with new shells.
- **Drop on a column edge** to open a new column, for files, editor tabs and panel terminals. Dropping
  files on the tab strip inserts them at that position.
- **Drop files on a terminal** to type their paths into it, quoted for the shell, from the Finder or the
  Explorer.
- **Resize columns and the sidebar.** Drag the border between two columns to move width from one to the
  other, or the sidebar's edge to widen it. Splitting a column gives each half its width, and the
  widths come back with the folder's session.

## 0.1.0-alpha.7

- **Development journal.** Read the 14-part Chinese series in `docs/juejin/README.md`, from yan's
  initial design to its macOS previews, with a timeline and source references for each topic.

- **Changes in the gutter.** Click a change mark beside the line numbers, or press ⌥F3 (⇧⌥F3 for the
  previous one), to open a box under the change with the lines as they are at HEAD. From there, or with
  ⌃K R and ⌃K S at the caret, revert the change, which is a step of undo, or stage it, which puts only
  those lines in the index. Esc closes the box.

- **All changes in one tab.** 打开所有改动, at the top of Source Control or in the command palette, shows
  every changed file in one tab, as Source Control groups them: staged, then changes, then untracked. Each
  file has a header with its path and the lines added and removed; Enter or `z` on it folds it.
- **Diffs read like a pager.** In a diff tab, `j` `k` move, `]` `[` go to the next or previous change, `.`
  `,` to the next or previous file, Space `f` `b` `d` `u` page and half-page, `g` `G` go to the top or the
  bottom, `z` opens folded lines or folds a file, `e` opens the file at the line, and `s` stages the
  change at the caret, or the file on its header (unstages what is staged).

- **Source Control as a tree.** The button by the title (or 以树的形式查看改动 in the command palette) lists
  each group's files under their folders, a folder with only one folder in it as one row, as
  `internal/git`. A click folds a folder, and its buttons stage, unstage or discard every file in it.
- After a commit the status bar says so with an **撤销** (undo) button, which takes the commit back and keeps
  its changes staged and its message in the box.
- **Commits open in a tab.** A click on a commit in the history shows its message, author, date, hash and
  branches, then every file it changed, in a preview tab, read with the same pager keys. What you can do to
  a commit (check out, merge, rebase, reset, revert, cherry-pick, branch, tag) is now on its right-click
  menu; in the terminal, press Space on it.
- **显示更多提交** at the end of the history reads 200 more commits.
- A folder outside a Git repository gets an **初始化仓库** (initialize repository) button in Source Control.

- **Side-by-side diffs.** A diff shows the old lines on the left and the new on the right, a change across
  from what it replaced, once each side has room for 80 columns. The button at the end of a diff's
  breadcrumbs (or the command palette) switches between automatic, always side by side and one column.

## 0.1.0-alpha.6

- **Paste a screenshot into Claude Code in the terminal.** When the clipboard holds only an image, ⌘V in a
  terminal sends an empty paste, as VS Code's terminal does, and Claude Code reads the image itself.
- In the terminal, **Select All copies** what it selects too, while copy on select is on, and changing
  `[terminal] copy_on_select` in the config now reaches terminals already open.

## 0.1.0-alpha.5

- **The terminal copies what you select**, as Ghostty and iTerm2 do: a drag, a double or a triple click
  puts the selection on the clipboard. `[terminal] copy_on_select = false` in the config turns it off
  (for terminals opened after).
- **Project tabs:** drag a square along the rail to reorder the projects. A terminal that rings its bell
  or sends a notification in a project out of sight, as a build or an agent does when it finishes, marks
  the project's square in the accent until the project shows, and its tip says why.
- **Narrow windows:** the status bar leaves out its least needed parts, the encoding first, only once they
  would not fit, and the command center leaves out the folder before it cuts the file's name short.

## 0.1.0-alpha.4

- The changes of a new file in a folder whose name starts with a dot, as `.changeset/x.md`, show in Source
  Control instead of git's "ambiguous argument" error.
- **Project tabs.** 显示 › 项目标签 keeps every folder in one window, with a rail on its left of a square for
  each, its initials in it, and a dot when it has unsaved changes. A click shows a project, as do ⌃⌘] and
  ⌃⌘[ (转到 › 下一个项目, 上一个项目) and ⌃⌘1…9, each with its tabs, terminals, sidebar and panel as it left
  them. Opening a folder, ⇧⌘N, the rail's ＋ or dropping a folder on the rail adds a project; a square's menu
  reveals, copies the path or closes it, as does 文件 › 关闭项目. Turning it on gathers the windows into the
  one in front, and off parts them into a window each again. The projects come back the next time yan
  opens. Window version only.
- **Merge, rebase, reset, revert and cherry-pick** from a commit in Source Control's history: click the
  commit and choose. Reset asks for soft, mixed or hard. When one stops for conflicts, Source Control says
  so above the commit box with 继续 (continue) and 中止 (abort), also in the command palette, and the
  commit box holds the message Git proposes.
- **Copy and paste files in the Explorer.** ⌘C copies the file or folder chosen and ⌘V pastes into the
  folder chosen, also between yan windows and the Finder; a name already taken gets "copy" added, as in the
  Finder. The menu of a file or folder has 复制 and 粘贴.
- **Drag files to open them.** Drag a file from the Explorer, or from the Finder, onto a column to open it
  there; drag one from the Explorer into the Finder to copy it there.
- A click below the files in the Explorer, or Esc, chooses none, so new and pasted files go in the
  folder that is open; a right click there opens a menu for that folder (new file or folder, reveal in the
  Finder, open in a terminal, paste).
- An empty folder's explorer says it is empty, with buttons for a new file and a new folder.
- When the window version crashes, what happened is kept in `~/.config/yan/crash.log`, and the last 10
  crashes in `~/.config/yan/crashes/`, also when yan was opened from the Finder or the Dock.
- **Word wrap.** ⌥Z (or 显示 › 自动换行) wraps long lines at the edge of the pane, breaking after spaces
  and between Chinese characters; ↑/↓, Page Up/Down, clicks and scrolling go by the wrapped rows, and
  continuation rows have no line number. The choice is remembered; `[editor] word_wrap = true` in the
  config turns it on too.
- **New window.** ⇧⌘N (文件 › 新建窗口, or the Dock icon's menu) opens another window, each with its own
  folder, tabs and terminals; the menu bar follows the window in front, and closing a window leaves the
  others open. Window version only.
- **Markdown preview.** ⇧⌘V shows a Markdown file laid out, as it already did for SVG: headings, lists and
  task lists, quotes, code highlighted as in the editor, tables, local images, badges and links, in the
  editor's fonts and colours. Open the same file in another column to edit the text while the preview
  follows. A web link opens in the browser, a local one in yan. Window version only.
- **Shortcuts from VS Code.** 文件 › 打开键盘快捷方式 opens `keybindings.json` in VS Code's format, so
  bindings from VS Code or Cursor can be pasted in: chords such as `cmd+k cmd+s`, `-command` to remove a
  key, `when` for the editor, sidebar or overlay, and `runCommands`. It applies when saved; anything yan
  cannot do is skipped and named in the status bar.
- Typing into a tab that shows a preview (SVG or Markdown) no longer edits the hidden text; the status bar
  says to press ⇧⌘V to go back to it.
- MyGo 0.4.8: shortcuts work on keyboard layouts that type no Latin letters, a selection draws above
  highlights, and long files lay out and draw with less work.

## 0.1.0-alpha.3

- **Commit history.** Source Control shows the last 200 commits below the changes, drawn as a graph: a
  lane and colour per branch, branch and tag badges, HEAD as a ring, merges as hollow dots, and an arrow on
  commits not yet pushed. The branch button in the "提交" header shows all branches and tags. Click a
  commit to list the files it changed, copy its hash, check it out, or create a branch or tag there.
- **Worktree switching.** Switching to a branch checked out in another worktree offers to open that
  worktree in this window; if its folder is gone, yan offers to clean up that worktree and switch here.
- **Editing commands:** sort lines ascending or descending, transform to camel case, and indentation
  (tabs, or 2, 4 or 8 spaces), on every cursor.
- **Trackpad scrolling** follows the swipe's main direction, and slow sideways swipes move the view.
- **New icon:** a text cursor dripping a drop of vermilion ink.

## 0.1.0-alpha.2

- **Version label.** The status bar and the window title show the build: `预览版 v0.1.0-alpha.2` for
  previews, `v1.2.3` for stable releases, `开发版` for development builds.

## 0.1.0-alpha.1

- First downloadable preview for macOS (Apple Silicon): the window version with VS Code-style editing,
  Git (change marks, blame, source control, diffs), a built-in terminal, themes and Vim mode.
- Auto-save is on and never overwrites a file another program changed.
