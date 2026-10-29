> This document records the alpha.14 discovery batch. The current same-screen management,
> per-repository actions and initialized submodule support are described in
> [Multi-repository Source Control](multi-repository-scm.md).

# Repositories inside a project folder

Reference: [Mayoi #49, d28bc2500f47](https://github.com/sorrycc/Mayoi/commit/d28bc2500f4715c35ab12df289656c170600dc4a), MIT.
Its bounded repository discovery and depth/exclusion regression are adapted to Yan's local filesystem
and shared core. No remote service or new shortcut is required.

## User flow

1. Open a folder such as `workspace/`, containing `project-a/.git` and `group/project-b/.git`,
   then open Source Control. The parent must not already belong to a Git worktree.
2. Choose a repository to open that project's Source Control. The GUI follows the existing project-tabs
   preference and reuses an already-open project; the terminal edition switches folders. Without project
   tabs, normal save/dirty checks apply before replacing the current workspace.
3. Stage, commit, inspect history or compare commits using the selected project's existing tools.
   The parent list has no implicit target for commits. A commit request there reports that a repository
   must be opened and does not trap the keyboard in an invisible message input.

The list rescans every five seconds and through **Git：刷新仓库列表**. Both `.git` directories and
linked-worktree `.git` files count. Discovery stops at a repository boundary and skips hidden folders,
`node_modules` and directory symlinks. Names use `/` on every platform. An unreadable scan reports an
error and keeps the last successful list; a removed repository cannot silently resolve to its ancestor.
Closed or replaced workspaces discard pending discovery results.

## Scope and verification

- Core Git tests cover two-level limits, exclusions, broken markers, linked worktrees, symlink aliases,
  cancellation and failed scans.
- Editor tests use real repositories to check automatic discovery, additions/removals, selection identity,
  opening, committing to only the chosen repository, root initialization, dirty-buffer protection and
  stale callbacks after a workspace switch.
- GUI harness tests exercise a repository row click and the resulting Source Control. Project routing
  tests check parent draft preservation, child reuse and retained commit messages. Screenshots are
  recorded under ignored `build/nested-validation/shots` when `SHOTS` is set.
- TUI keyboard dispatch is covered by the core tests; the paint test covers a short viewport and row hits.
- These checks do not claim manual native clicks or Windows/Linux runtime acceptance. The shared
  filesystem/path logic and existing platform host/process interfaces remain available for those clients.

Yan opens repositories as separate projects. This batch does **not** aggregate several repositories'
changes, infer a commit target from the active file, recursively enumerate submodules, or introduce
SSH workspaces. Those require a separate multi-repository context design, including diff and review
identity, per-repository messages and asynchronous command routing.
