# UP-66: MyGo upgrade and font lifecycle acceptance

Date: 2026-10-30. Baseline: Yan v0.1.0-alpha.36. MyGo is pinned to
`github.com/sorrycc/mygo v0.4.9-0.20261029144802-e386308904aa`, with Go 1.27.3.
The fork retains inline math objects, copy-on-select and image-only terminal paste.

## Behavior

Released native fonts are never drawn. Terminals reshape retained rows when the text
system recycles fonts. Yan's cached editor glyphs and Mermaid label layouts also refresh
when a fresh glyph reports a different font identity, even if content and settings are unchanged.
The probe runs once per editor frame or visible diagram; rows retain their normal shaping cache.

On macOS, signed updates are staged while the app runs and replace the bundle after exit.
The running bundle stays in place, preserving its Dock identity. Update verification must
observe the completed replacement from outside the exiting process.

## Regression evidence

- `TestNativeTerminalFontCacheRecovery` reproduces a native Core Text SIGSEGV with the alpha.36
  dependency. With the new pin, three 640-font recycling cycles preserve the exact rendered pixels
  of English/CJK terminal output and subsequent keyboard input still reaches the same connection.
- `TestCodeFontCacheRecovery` and `TestMermaidFontCacheRecovery` compare rendered pixels before
  and after three recycling cycles. Their unchanged-text fixtures detect missing editor glyphs and
  diagram labels; both pass with Yan's cache invalidation.
- MyGo's `TestReleasedFont` and `TestRowsShapedAnewAfterFonts` pass against the pinned module.
- `go test -tags selftest ./ui/gui -run TestNativeTerminal` passes, including clipboard, file links,
  copy-on-select, image-only paste, review handoff, focus, movement and terminal closure.
- Real macOS `terminal-font` self-test: two disposable `/bin/sh` sessions, English/CJK output,
  three recycling cycles, 9–28 point sizes, light/dark appearances, panel/editor movement,
  subsequent commands and clean terminal closure. Captured windows were visually inspected.
- Real macOS `math` self-test: formulas, source edits/undo, two column layouts, light/dark,
  larger text and configuration reload pass. Real `update-restart` opens the updater window,
  asks to save a dirty buffer and restores the saved document in a new process.
- `TestMyGoUpdateIntegration` rejects bad signatures and corrupted archives without changing the
  installed app, confirms the running old bundle remains, then waits for its process to exit and
  verifies the complete new bundle. The public-feed self-test records `update-staged.json` so
  external release verification can inspect the actual signed replacement after exit.
- Real macOS `terminal` self-test executes shell commands, retains the same shell across moves,
  resizes the PTY, interrupts sleep with Ctrl+C and closes the window with a terminal still open.
- Full `GOMAXPROCS=4 GOFLAGS=-p=1 make check` passes, including race, architecture and size gates:
  TUI 10,453,762 bytes; GUI 29,951,186 bytes.
- Windows amd64 and Linux amd64 GUI test binaries cross-compile without the macOS self-test tag.

## Limits

Native Windows/Linux desktop execution, multi-day soak testing, and quantified non-ASCII
performance improvements are not claimed. The accelerated tests exercise font-cache exhaustion
directly; they do not simulate every possible long-running workload. Editor configuration and fixtures live in disposable directories; no installed app or
existing user shell session is modified.
