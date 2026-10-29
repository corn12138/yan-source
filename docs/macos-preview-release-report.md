# macOS preview release report — v0.1.0-alpha.1

Date: 2026-10-13. Host: Apple M3 Max, macOS 27.2 (26B5101f), Go 1.27.1 darwin/arm64, MyGo v0.4.1.
Current release workflow: [CONTRIBUTING.md](../CONTRIBUTING.md).

## Release

| | |
|---|---|
| Release | https://github.com/corn12138/yan/releases/tag/v0.1.0-alpha.1 (pre-release, not draft, not "latest") |
| Repository | `corn12138/yan`, **private**, created for this release (the account `corn12138` is the authenticated `gh` user; the repo did not exist) |
| Tag | `v0.1.0-alpha.1` (annotated) → commit `47e3f2dac46014bdd29c312985754cf62b88955d` on branch `codex/macos-preview` |
| Version shown by the app | `yan-app v0.1.0-alpha.1（47e3f2dac460）` (`--version`, Help › About) |
| Signing | **ad-hoc**, no Team ID, **not notarized** (no Developer ID identity on this Mac: `security find-identity -v -p codesigning` → 0) |
| Architecture / minimum OS | darwin/arm64 only; `LSMinimumSystemVersion` 13.0; executable and `libghostty-vt.dylib` both `minos 13.0`. Tested on macOS 27.2 only |

| Asset | Bytes | SHA-256 |
|---|---|---|
| `yan-v0.1.0-alpha.1-macos-arm64.dmg` (recommended) | 5,582,223 | `0cdb2e693e0e1808ebbd048290c9269be8788e9f9797d7ee0c81345b3db42734` |
| `yan-v0.1.0-alpha.1-macos-arm64.app.zip` | 7,332,514 | `645b0cc07c7588c8d99ccb90c740aaadd262d8b01f3c94f51011c676e5ccd793` |
| `THIRD_PARTY_LICENSES.txt` | 108,762 | `2bd707427b1cfdac596a23a8bedfb6a6a213e182dde6b6e9cc2aebbbd3e74525` |
| `SHA256SUMS` | 297 | — |

`yan.app` is 18.9 MB: the executable is 17.6 MB, `libghostty-vt.dylib` is 1.9 MB, plus the icon and the notices. The terminal build `bin/yan` is 8,457,042 bytes, under the 10 MB gate.

## Findings and fixes

| ID | Fix | Regression tests |
|---|---|---|
| R1 auto-save overwrote external changes | `edit.Buffer` records the disk stamp (mtime + size) at load, save and reload. `Save` returns `ErrConflict` when the file changed, or was created after a buffer of a then-missing file; a deleted file is recreated only by an explicit save, never by auto-save. New commands `yan.files.overwrite` and `workbench.action.files.revert` (undoable), also in the File menu. Tab switch now reports failures (it used to ignore them). Close falls back to the "press again to discard" prompt. Save All saves what it can and reports the first error. | `core/edit/edit_test.go` (5 tests); `core/app/safety_test.go`: blur, timer, tab switch, close, folder switch, Save All, pre-Git stage, explicit save, overwrite, revert + undo, delete/recreate, clean reload, unwritable directory, auto-save off |
| R2 search and Git callbacks crossed folders | Workspace epoch; `postHere` fences search debounce, results, `gitDo`, amend, undo-commit, branch picker, blame and HEAD reads. The search generation survives folder switches; old workers and timers are cancelled. Replacement refuses paths outside the open folder. | `TestStaleSearchResultsIgnoredAfterSwitchFolder` holds A's real posted results, switches to B, searches B, then delivers A's results: they are ignored, A is untouched and B is replaced. Mutation-checked: with the fence removed it fails. Also `TestGitCallbackIgnoredAfterSwitchFolder` and `TestReplaceRejectsResultsOutsideWorkspace` |
| R3 single replacement ignored matched text | Results record their source version (document snapshot version, or disk stamp). A single replacement requires an unchanged query (not mid-debounce), the same version and the same matched text; otherwise the search refreshes and the status says why. | `TestReplacementsChecksMatchedText` (the audit probe), `TestReplaceOneRejectsStaleOpenBuffer` (same-length cat→dog), `TestReplaceOneRejectsStaleUnopenedFile`, `TestReplaceWaitsForQueryChange` |
| R4 native focus never reached the core | `Window.OnFocus` / `OnBlur` → `FocusInput`, on the owner thread. | `TestWindowFocusReloadsAndKeepsConflicts` (headless) plus the native runs below |
| R5 race failures hidden | `make check` runs `make race`, with no `||` fallback. A machine that cannot build `runtime/cgo` with `-race` gets "竞态检测：没跑（BLOCKED）" and exit 2 (verified with `CC=/nonexistent/cc`). Race timeout is 45 min. | Gate run below |
| R6 notices not shipped | `internal/tools/notices` reads the build info of the shipped binaries and writes the repository notices, Go runtime license, and every linked module's LICENSE/NOTICE (25 modules). For libghostty-vt it adds Ghostty `befcdfd2c3a1`, simdutf, Highway (`66486a10`), uucode (`9d555245`, with its Unicode and Hoehrmann licenses) and Zig 0.16.0 std. Symbols for simdutf and Highway were confirmed in the dylib. The file sits in `Contents/Resources` and is a release asset. The release script checks that the bundled executable links the same modules as the binary the notices came from. Attribution of ported Mayoi packages corrected: highlight has a nil guard, git has a submodule fix, symbols was adapted to `text.Snapshot` and byte columns; linediff and glob are unchanged apart from package names. | Release-script checks |
| R7 native editing / IME | IME commits use `OriginIME`, one undo step each, and carry the tab where composition started. If the user switched tabs mid-composition, the text goes into that original tab; if it was closed, the text is shown in the status bar. | `TestInputMethodComposition` (headless MyGo composition events: no edit while composing, commit, two commits = two undos, cancel, replace selection, switch tab mid-composition; mutation-checked) and `TestIMECommitGoesToComposingTab` |

Race-gate investigation: the audit's 600 s timeout was runtime cost, not a hang. Under `-race` each relex subtest finished in 40–250 s, 588 s in total.

The same run also exposed a real failure that the old gate's fallback would have hidden. Chroma sets a fixed 250 ms `MatchTimeout` on every lexer regexp (`chroma/v2@v2.27.0/regexp.go:375`), and under the detector long comment scans exceed it and lex differently. With a scratch copy of Chroma using a 60 s timeout, the unchanged 30-copy test passed under `-race` (1,186 s). The race build now uses 6 copies of each sample.

That smaller workload hit a deterministic counterexample, with or without `-race`. A JS template literal's `${…}` leaves the lexer in a state no token shows, so relex settles early. relex then reports `exact=false`, as its contract allows, and the highlighter re-lexes everything after 300 ms of quiet. The test now enforces that contract: exact results must equal a full lex, and inexact guesses must stay under 2%. The TUI end-to-end test now polls for redraws instead of using fixed sleeps.

## Command results (release commit, before tagging; tests unchanged since)

| Command | Result |
|---|---|
| `gofmt -l .` | no output |
| `CGO_ENABLED=0 go vet ./...` | pass |
| `CGO_ENABLED=0 go test -count=1 ./...` | pass (13 packages with tests) |
| `CGO_ENABLED=1 go test -race -count=1 -timeout 45m ./...` (`DEVELOPER_DIR=/Library/Developer/CommandLineTools`) | pass; highlight 284.6 s, others < 25 s |
| `make check` | exit 0, 5 min 40 s |
| `make race` without a usable C compiler | prints BLOCKED, exit 2 |
| `scripts/release-macos.sh v0.1.0-alpha.1` (`make notices` + `go tool mygo build`) | pass. Checks: arm64 executable and dylib; dylib SHA-256 `6f385d32…` matches MyGo's manifest; notices bundled; module sets equal; version string; `codesign --verify --deep --strict`; `hdiutil verify` |
| DMG contents | `yan.app` + `Applications` → `/Applications`; signature valid inside the mounted image |
| zip | no `__MACOSX`; the extracted app passes `codesign --verify --deep --strict` |
| `codesign -dvv` | `Signature=adhoc`, `TeamIdentifier=not set` |
| `spctl --assess --type execute` | **rejected** (expected for ad-hoc), both with and without a quarantine attribute |
| `syspolicy_check distribution` | "Adhoc Signed App" distribution error (expected) |
| `gh release download` into an empty directory, then `shasum -a 256 -c SHA256SUMS` | all three OK; downloaded `SHA256SUMS` identical to local; GitHub's asset digests match |
| Install from the downloaded DMG into a test Applications folder | signature valid; `--version` = `v0.1.0-alpha.1（47e3f2dac460）`; launched via LaunchServices (`open -n`, cwd `/`, isolated `XDG_CONFIG_HOME`) and became frontmost; 116 MB RSS at 6 s |

## Native smoke matrix

Ways each item was checked:

- **Native, release**: the downloaded, installed release app.
- **Native, self-test**: the app built from the tagged commit with `-tags selftest`. This only adds `cmd/yan-app/selftest.go` and `ui/gui/selftest.go`, which drive the core API in place of the keyboard. The app was launched from a test Applications folder with isolated config, and focus changed by really switching apps with `open -a`.
- **Headless**: MyGo's `ui.Tester`.

Default is arm64, macOS 27.2.

| Check | Result | How |
|---|---|---|
| Install from DMG, launch from Applications via LaunchServices, no repo cwd | pass | native, release |
| Open folder; quit and relaunch restores two-column session | pass | native, self-test |
| Blur → auto-save writes the edit | pass | native, self-test |
| External change + dirty buffer → blur and idle timer do not overwrite; conflict message shown | pass | native, self-test |
| Refocus → clean file reloads; dirty buffer kept | pass | native, self-test |
| Terminal with the bundled libghostty-vt: command output, resize 100×30, Ctrl+C interrupts `sleep 30`, close, quit with a terminal open | pass | native, self-test |
| Terminal library missing (`MYGO_GHOSTTY_VT=/nonexistent`) → terminal does not open, app keeps running, edit + save works | pass | native, self-test |
| Typing, save/undo, LF/CRLF and BOM preservation, multi-cursor, dirty close prompt, unwritable save | pass | headless + core tests |
| IME: pre-edit not committed, commit, Escape cancel, replace selection, one undo per composition, tab switch mid-composition | pass | headless (simulated MyGo composition events) |
| Find/replace, cross-file search, stale-result rejection, folder switch during search | pass | core + headless |
| Git in a temp repo: stage/unstage/discard/commit of tracked files (`core/git`: discard restores from the index; not recoverable from Trash), stage → commit → branch switch and diff view (`core/app`). Moving untracked files to Trash is not separately tested | pass | core tests |
| Images, SVG preview, split, preview and reopened tabs, menus | pass | headless |
| Idle sample, opened on a folder | ~127 MB RSS, 60 MB physical footprint (`footprint`), 0% CPU at 30 s | native, pre-tag build |
| **Real keyboard typing and system Simplified Chinese Pinyin in the live window** (candidate position, Quick Open / find / search fields) | **not tested** | — |
| **"Save before closing?" dialog in the live window** | **not tested** | — |
| **Gatekeeper "Open Anyway" in System Settings** | **not tested** | — |
| Other input methods (WeChat, Sogou, Rime), Intel Macs, macOS < 27 | not tested | — |

The three bold rows are blocked by the same missing prerequisite: this session has no Accessibility or Screen Recording permission (`osascript is not allowed assistive access`; `screencapture` fails). Without those it cannot send keystrokes or clicks or see the screen, and granting them is a security setting only the owner may change. The installation steps follow Apple's current procedure ([support.apple.com/102445](https://support.apple.com/en-us/102445)). On first launch the owner should confirm that the Open Anyway button appears and works, then type Pinyin in a scratch file.

## Known limitations

- Ad-hoc signed, not notarized: first launch needs Open Anyway; `spctl` rejects it. Fixing this needs an Apple Developer ID plus notarization credentials, which this Mac doesn't have.
- arm64 only. No automatic updates. The repository is private, so downloads need a signed-in GitHub account with access.
- `CFBundleShortVersionString` is `0.1.0`: Apple requires numbers only. The full version is in `--version` and Help › About.
- Deferred features: word wrap, commit history, gutter hunk actions, tab/file dragging, multiple windows, Vim macros and Ex commands, find in selection, LSP.
- Yan's own code has no license chosen yet. This release ships only the third-party notices.
