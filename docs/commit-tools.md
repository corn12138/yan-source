# Commit tools from Mayoi

Reference: Mayoi `8708da764770273c59a056d4ce20c416833e7c46`, MIT. Its
`internal/git/commitactions.go` and tests are retained in `core/git`; interaction is rewritten
in the shared Yan core, using the existing GUI/TUI diff renderer and platform host.

## Use

In Source Control, right-click a commit for:

- **导出补丁… / 复制改动补丁**: changes from its first parent; a root commit uses the empty tree.
  Patches preserve binary data and disable terminal colors, external diff tools and text conversions.
  The GUI uses the system save dialog (including overwrite confirmation). The TUI uses a path prompt
  and refuses an existing path. Cancelling leaves files unchanged.
- **比较 HEAD → 此提交 / 比较此提交 → HEAD**: the arrow is old to new. Both revisions resolve to
  full hashes before opening, so later checkouts/commits do not change an open comparison.
- **选为比较起点**, then **与选定起点比较** on another commit: selection belongs to that repository.
- **在远端打开提交**: public GitHub, GitLab or Bitbucket HTTPS pages, with credentials/query/fragment
  removed. Prefer a unique remote decoration, current upstream, origin, then the first remote.
  Unknown hosting services return an error instead of guessing. TUI copies the link.

The same actions are available under **Git** in the command palette, using the focused commit tab
or the selected history row. A comparison tab's target commit is used for commit-specific actions.
Press Enter or double-click the origin/target metadata row to copy its complete hash. Existing
inline/split layout controls, file folding and pager keys work in comparisons. Historical tabs
cannot stage changes or edit a working file through historical line numbers.

Commit and comparison tabs restore with each folder's session, including column, active tab,
preview flag, cursor and scroll position. Repository discovery may finish after tabs restore;
placeholders wait for that result. Missing objects or a removed repository show an error tab.
Review comments use both endpoints as their identity, retain original excerpts, and do not appear
in the reverse comparison or an unrelated base. “Show all branches” is saved in state.json.

## Validation

- Git fixtures: root/merge/binary patches, reverse applicability, comparison direction, forced colors,
  invalid revisions, credential-free provider links and remote selection.
- Core fixtures: renamed/deleted/binary files, immutable revisions after HEAD moves, copied hashes,
  staging refusal, review isolation, multi-column session restore, missing objects/repository,
  TUI export, cancellation and stale save callbacks after switching projects.
- MyGo headless UI: actual right-click menu selection, clipboard patch, two-commit comparison,
  provider URL dispatch, and saving through an injected system-dialog result. A screenshot records
  the rendered comparison. This does not claim a manual native save-dialog/browser acceptance run.
- Full `make check` and the single Linux CI are the merge gates. Windows core test binaries are
  cross-compiled; native Windows/Linux installers remain future work.

Local result: final `make check` passed, including all tests and race detection; TUI 9,000,578 bytes,
GUI 20,115,074 bytes. Windows core test cross-compilation and the rendered GUI interaction
checks passed. The Linux PR result and final release verification are recorded in the delivery checkpoint.

Validation logs/screenshots are kept under ignored `build/commit-tools-validation` and copied to
an external evidence directory before packaging, which replaces build output.

## Next candidates

Continue with search-in-selection and editor workflow gaps, then evaluate multi-repository/submodule
SCM separately. SSH workspaces, settings sync and AI-provider integration need their own scope;
Qoder/Qoduck remain excluded. Release binaries use the separate public distribution repository.
