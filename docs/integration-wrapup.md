# Selected Mayoi integration: delivery audit

This audit follows the agreed priority groups and the subsequent line-review handoff. It does not
turn every newer Mayoi feature into a release requirement. Source is licensed under MIT; installers and signed updates use the separate public binary repository.

| Requested result | Current implementation | Evidence and delivery |
| --- | --- | --- |
| Git changes and commit graph | `hunks.go`, `changes.go`, `diff.go`, `scm.go`, `history.go`, `graph.go`: gutter preview/stage/revert, Changes tab, inline/split layouts, pager, tree mode, more commits, commit tabs, undo commit and init | Core/GUI regressions; merged PR 7; alpha.7 release notes |
| Editor layout | `overview.go`, `tabs.go`, `wrap.go`, `terminals.go`, GUI pane/drop/terminal views: overview, tab dragging, column resizing, per-tab wrap, panel/editor terminal moves and reopening | Core/GUI and native terminal drag/focus regressions; merged PR 8; alpha.8 |
| Windows and file opening | Folder routing and reuse, Dock recents, launcher, sidebar placement, indentation/EOL controls | `window-workflows.md`; merged PR 9; alpha.9; native second-process and hidden-window evidence |
| Optional theme decorations selected earlier | Clawd and Matrix effects with focus/reduced-motion/configuration controls | `theme-effects.md`; merged PR 10; alpha.10 |
| Signed automatic updates | Native updater, full alpha ordering, signatures, save-before-restart, separate public binary distribution | `automatic-updates.md`; merged PR 11; alpha.11; public checksums and native disposable-bundle replacement |
| Line comments and terminal handoff | Shared review model, source/diff/commit comments, retained quotes, relocation/outdated state, project persistence, GUI/TUI editing and copy, last-project-terminal paste with undo/fallback | `review-comments.md`; targeted core, GUI, native emulator and actual-shell evidence; merged PR 12 and Linux CI 39187995871; alpha.12 published; public checksums and native public-feed replacement verified |
| Continued upstream commit tools | Binary-capable patch copy/export, directional comparisons pinned to full hashes, comparison/commit session restore, remote links and remembered graph scope | `commit-tools.md`; merged PR 13; alpha.13; local checks, Linux CI 39190706434 and native public-feed replacement verified |
| Folders containing Git projects | Two-level discovery, recursive initialized submodules, same-screen repository panels and optional independent-project opening; isolated drafts/actions/diffs | `nested-repositories.md`, `multi-repository-scm.md`; same-screen management follows alpha.14 discovery |
| Windows/Linux hooks | OS-profile bindings and names; review shortcut tests for darwin/windows/linux; Windows compile check; platform process and installer seams | `platform.go`, `platform_test.go`, `review_test.go`, `internal/process`, `internal/cli` |
| Preserve chosen differences | Qoder/Qoduck excluded; TOML settings retained; source and installer distribution separated | Theme/configuration sources; separate distribution repository |
| Merge and release with limited CI | Full local `make check`, then one Linux PR CI, fast-forward merge, version script, tagged package and public feed verification | Release-specific logs and checkpoint under ignored `build/review-validation/`; release artifacts are the final delivery evidence |

## Verification boundaries

Local checks cover formatting, vet, all tests, races, architecture boundaries and the 10 MiB TUI
limit. The review tests use actual UI events, real Ghostty transport and an isolated native macOS
window. No test replaces the installed user application or submits review text as a shell command.

Windows/Linux native installers remain future development as agreed; their hooks and compile/keymap
coverage do not claim native OS acceptance. Apple Developer ID/notarization, manual Finder drag/IME
acceptance and the OS preference panel remain separately stated boundaries in the feature reports.

Other entries under `MAYOI.md`'s backlog, plus upstream SSH, settings sync and AI gateway, are future
candidates rather than implicit additions to this closing batch. No provider account or cloud agent
is required for the implemented review workflow.
