# UP-50: submodule Source Control acceptance

Date: 2026-11-02. Baseline: Yan v0.1.0-alpha.37. Upstream: Mayoi
[#76](https://github.com/sorrycc/Mayoi/pull/76), merged as
`20e7968c3d63132d5525c38003253402dfc57779`.

## Behavior

Submodule rows distinguish changed commit references from internal uncommitted files.
The window app's **Open Submodule Changes** action selects the child's panel and retains
the workspace, parent diff and independent drafts. Parent discard excludes gitlinks even
when Git enables recursive submodule restore; parent path actions reject child-owned files.

Clean repositories start collapsed. Header arrows change expansion; names choose the
history and command target. Keyboard entry into a commit draft expands that repository.
Native input focus relinquishes the old draft before it can select its repository again.
Background status insertion preserves the selected file and selection anchor by identity.

## Evidence

- Core regression fixtures use real parent/child commits and cover commit-reference/content
  states, recursive restore protection, path ownership, unavailable submodules, retained
  diffs/drafts, keyboard expansion, and selection retention after inserted rows.
- GUI regressions cover context-menu navigation, deferred navigation from a focused parent
  draft, collapsed child input focus, arrow/name semantics, and Chinese/dark plus English/light
  rendering at normal and narrow widths.
- Real macOS `submodule-scm` self-tests pass in both Chinese and English. Actual action
  handlers and native input controls retain both drafts across repository changes; keyboard
  focus unfolds the selected child. Light/dark screenshots at 1100 and 760 logical pixels
  were captured; Chinese narrow/dark and English wide/light were visually inspected.

- Full `GOMAXPROCS=2 GOFLAGS="-buildvcs=false -p=1" make check` passes, including
  race, architecture and size gates. The synchronized public source passes the same gate.
- Windows/Linux amd64 window apps cross-compile. `go test -tags selftest ./ui/gui
  -run TestNativeTerminal` passes with the pinned native terminal library.

## Limits

UP-50 remains partial: clearing selection when its row disappears, the continuous list,
sticky repository/group headers, independent history scrolling and saved divider are open.
Native automation invokes the navigation handlers; it is not a physical mouse-click test.
Native Windows/Linux desktop behavior and long-session soak testing are not claimed.
All native fixtures and application settings are isolated from existing user projects.
