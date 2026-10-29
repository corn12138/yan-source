# Multi-repository Source Control

Open a folder containing several Git projects, then open Source Control. Changes and commit
messages now live in collapsible repository panels in the same window. The repository picker
chooses the history and window-level Git commands; **自动跟随当前文件** follows the active file or
historical diff. Clicking a panel action explicitly selects its repository. Right-click a repository
header to open it as a separate project if desired.

Each repository retains its message, amend state, tree expansion, history pagination and selected
comparison origin while switching panels. Stage, unstage, commit, fetch, pull, push, branch operations
and undo operate on that repository. Commit and comparison tabs retain their original repository
when another panel is selected, including after restoring the folder session. Review comments retain the repository root,
absolute source paths and immutable revisions to distinguish identical names/hashes in different
worktrees. Destructive confirmations name the repository; retired panel IDs cannot operate on a
replacement at the same path.

The terminal edition renders the same repository panels. Up/Down can move between their rows;
Tab edits the selected repository's message. The command palette's **Git：选择仓库…** selects a
repository or restores automatic following. Clickable rows retain repository identity after scrolling.

## Discovery and ownership

- A folder without its own repository discovers projects up to two levels deep. Hidden folders,
  `node_modules` and symlink children remain excluded.
- Initialized submodules are discovered recursively through `.gitmodules` and tracked gitlinks;
  uninitialized submodules are not mistaken for their parent repository.
- Linked worktrees use their working-directory roots as identities, even when they share a common
  Git directory. The longest matching root owns a file; committing a parent does not save a child's
  unsaved editor buffers. Gitlink changes remain part of the parent's index.
- Discovery refreshes every five seconds and via **Git：刷新仓库列表**. A failed scan retains the
  previous list and reports an error. Removing a repository cancels its watcher and invalidates
  queued callbacks and confirmations.

## Architecture and platform hooks

`core/app/repositories.go` owns stable repository contexts and frame-bound action IDs. Git workers
post results into their captured context on the editor's owner goroutine. GUI and TUI consume
`view.SCM.Panels`; neither frontend owns Git state. Historical tabs store the root in `diffTab` and
`config.SessionTab.Repository`; review notes persist the same ownership. Filesystem operations use `filepath`; visible relative paths use
`filepath.ToSlash`. GUI commit shortcuts use `keymap.Profile.MacKeys()` (Cmd on macOS, Ctrl on
Windows/Linux), with the existing platform menu and file-manager interfaces preserved.

This batch covers local repositories, worktrees and initialized submodules. It does not add SSH
workspaces, settings synchronization, AI providers, Windows/Linux native installers, or Apple
Developer ID notarization.

## Verification

- Real temporary Git repositories: independent drafts, staging/committing B while editing A,
  repository-bound undo, stale dialogs/callbacks, automatic selection, fixed diff ownership,
  session restoration, shared-object worktrees, submodules and parent/child unsaved-buffer isolation.
- GUI tester: two simultaneously visible panels, typing into both messages, staging through a
  retained menu target, collapse/expand and screenshots.
- TUI: viewport hit targets and the selected repository's message caret.
- Cross-platform submit-key tests, full `make check`, and Windows compilation are recorded in the
  delivery evidence. Native Windows/Linux UI acceptance remains future work.

Reference: Mayoi (MIT), `cmd/mayoi/gitcontext.go`, `repositorypicker.go`, and
`internal/git/submodules.go` as present at `d28bc2500f4715c35ab12df289656c170600dc4a`.
