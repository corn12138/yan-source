# Automatic updates and Windows helpers

Historical PR and CI links below refer to the original development repository; they are not part
of the clean public source history. Current validation is described in CONTRIBUTING.md.

Installers and signed updates use `corn12138/yan-releases`. Its release tags and history are
independent of the MIT-licensed source repository.

| Requirement | Implementation | Evidence |
| --- | --- | --- |
| Native update UI | MyGo native updater; Chinese menu and dialogs, hourly checks, persistent off switch | Native menu opened the updater window |
| Signed install | Ed25519 key embedded in MyGo config; signed SHA-256 archive; verification before replacement | Real MyGo download/replacement with wrong signature and corrupted archive rejected |
| Preview updates | Full tagged version at runtime and in manifests; separate preview/stable feed URLs | alpha.9 → alpha.11 detected, equal and stable versions remain current in downloader integration |
| Separate binary distribution | Release scripts upload only explicit artifact paths to distribution repo | alpha.11 public assets and five checksums verified |
| Publish order and retries | Numbered artifacts first, anonymous signature verification, staged channel manifest, retained prior manifests, observed rename outcomes and bounded recovery | Fault tests cover lost responses, upload/activation failure, cancellation, cross-run recovery, concurrency checks and downgrade prevention; read-only idempotent replay against public alpha.27 passed. New rename-based promotion still needs its next real version transition; two API renames are not atomic |
| Preserve unsaved work on restart | Synchronous modal decision within close event; reentrant guard; failed save cancels | Native relaunch into new process saved dirty file; cancel/error/conflict unit regressions |
| Windows helpers | Mayoi console suppression behind internal/process; all three Git process paths routed | Output regression and Windows cross-compilation; no native Windows execution |

The release signing key is stored outside the repository with restricted permissions. It must be
backed up securely; key rotation is not automatic. Update signing authenticates downloaded bytes and
does not provide Apple Developer ID or notarization. Current distribution remains Apple Silicon only.
Delta updates and Windows/Linux installers are not part of this batch.

The native restart test simulated only the save-dialog answer. It exercised actual window creation,
process relaunch and file persistence with isolated settings. The downloader integration replaced a
disposable app directory, never the installed Yan application. Published-feed validation must run
against a disposable self-test bundle after publication, with the real public feed and public key.

## Initial release gates (alpha.11)

Targeted tests, native restart and full `make check` passed (TUI 8,794,658 bytes; GUI 19,980,898 bytes).
Alpha.11 completed one Linux PR CI ([run 38850128399](https://github.com/corn12138/yan/actions/runs/38850128399)),
tagged packaging, public publication, anonymous checksums and native published-feed replacement in a
separate verification bundle. [PR 11](https://github.com/corn12138/yan/pull/11) merged; the source tag
is `1b298ab2f2e9a158e236d8a00b153e3d828f3338`. The installed user application was not replaced by tests.

## Current publication verification (alpha.27)

Alpha.27 was published from `68b9c94` after the theme and terminal batches passed local checks and
their Linux PR jobs. All five public checksums and the signed archive passed verification. A separate
native test bundle confirmed the current version is up to date, an older version discovers the public
release, and the signed download replaces that disposable bundle. The installed user app was untouched.
The recovery publisher described above adds fault-injection tests and a same-version live replay;
its first real rename-based version transition remains to be verified at the next client release.

Continue the open contracts in [the feature ledger](mayoi-feature-ledger.md), including untitled
document recovery. Preserve Windows/Linux interfaces, the Qoder/Qoduck exclusion, and unrelated
local changes.
