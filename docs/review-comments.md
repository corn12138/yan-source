# Review comments and terminal handoff

Historical PR and CI links below refer to the original development repository; they are not part
of the clean public source history. Current validation is described in CONTRIBUTING.md.

This batch follows Mayoi's review workflow (introduced in 7be5390a752e, b4fff084971c,
461d0138915f; reference snapshot 8708da764770273c59a056d4ce20c416833e7c46).
Yan retains its shared core, semantic frames and existing GUI/TUI split.

## Acceptance

- Comment on a selected range or caret line in a saved file, working diff, staged diff or commit.
- Preserve full original lines, old/new-side coordinates and the diff excerpt, including text clipped
  by the split view. A selection ending at a line start excludes that line.
- Show saved comments at their source and provide multiline editing, cancel, delete, clear, undo,
  next/previous and a project comment list. Empty comments are rejected.
- Follow unchanged quoted lines after edits; flag missing quotes as outdated without replacing them.
  Persist unsent saved comments with their project, keeping project state isolated.
- Copy all comments or the selected diff excerpt with relative paths and collision-safe code fences.
- Paste a review into the most recently focused terminal in the current project, including editor
  terminal tabs, without submitting it. No usable terminal means clipboard fallback and retained notes.
  Clear comments only after a successful handoff; retain notes when it fails.
- Use platform shortcut profiles and test Windows/Linux hooks. No cloud agent or provider dependency.
- Run focused regressions, full make check, native terminal validation and one Linux CI before merge;
  then publish the next signed macOS preview and verify the public download/update feed.

## Scope and evidence

Implementation, local verification and one Linux PR CI are complete. [PR 12](https://github.com/corn12138/yan/pull/12) merged after [CI 39187995871](https://github.com/corn12138/yan/actions/runs/39187995871) passed. Tagged publication is the remaining delivery gate. New upstream SSH, settings sync and AI gateway
features are recorded as candidates, not added to this batch. Qoder/Qoduck remain excluded.
The previous alpha.11 release completed all gates documented in the saved release checkpoint.

## Verified behavior

- `core/review`: quoted-line relocation, outdated/reappearing quotes, obsolete persisted coordinates,
  removed/staged/commit references and collision-safe Markdown fences.
- `core/app`: blank rejection, multiline input, source selection boundaries, full split-diff excerpts,
  inline-card editing, staged isolation, restored commit navigation, deleted-source access, project
  isolation, delete/clear undo, clipboard fallback, failed/successful handoff, and all three OS shortcuts.
- `ui/gui`: actual tester clicks and typing through create/edit/cancel/delete/undo; source text is unchanged
  by comment entry and regains focus afterward. Rendered dialog and diff-card screenshots were inspected.
- `-tags selftest` terminal checks: actual Ghostty emulator and connection receive one bracketed paste,
  no submit keystroke or escaped paste terminator; the last focused panel/tab wins, and closed, foreign
  or previous-project terminals are excluded. Existing terminal drag/focus checks also pass.
- Native macOS window: a real shell in an editor terminal tab received multiline feedback without
  executing it; the temporary execution marker stayed absent and undo restored the saved note.
  The fixture used separate configuration and an independent app bundle.

Local evidence is saved in the managed checkout's ignored `build/review-validation/` directory.
The final full local check passed (TUI 8,845,218 bytes; GUI 20,064,562 bytes), including races and architecture boundaries. Windows core test cross-compilation also passed. PR CI and release publication are recorded separately at delivery; rendered
checks do not claim manual IME, native Windows/Linux or notarization acceptance.

## Use

Use the Review menu or `mod+shift+m` (Command on macOS, Ctrl on Windows/Linux) on a file range.
The line-side button opens the same editor. Save/cancel buttons work in the GUI; Ctrl+S saves a TUI
draft and Escape cancels it. In diffs, `c`, `y`, `n`/`N`, Shift+J/K and Delete act on comments or their
source selection. The project comment list can reopen saved commit feedback, including after restart.

Sending falls back to copying when the receiving terminal has not requested bracketed paste, so a
plain shell input mode cannot interpret the review's newlines as commands. The success state means
bytes were handed to MyGo's terminal input queue; it does not claim an agent accepted or acted on them.
