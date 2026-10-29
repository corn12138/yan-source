# UP-56: disk-input long-line protection

Selected from the Mayoi feature ledger after alpha.34. The upstream reference is `6d98a4981121aef441498c6cbe9e8f15085e608a`. UP-50 submodule actions, UP-49 merge layout, CLI and MyGo upgrades remain separate batches.

## Behavior and boundary

The previous loader accepted a 26 MiB single-line source map. Both frontends use a whole-line layout with caret stops for every grapheme. The new disk-input guard refuses lines containing more than 65,536 bytes before creating text/layout structures. A leading UTF-8 BOM and LF/CRLF terminators are excluded. A boundary-length line remains valid, including Chinese byte boundaries. The file is never truncated or changed.

Open and preview failures retain the current tab. Watcher and focus reloads retain the buffer, undo history and saved-disk baseline; explicit Revert returns the same error. A later Save still refuses an external conflict. Session restore skips refused files while keeping safe tabs, reports the skipped count, and excludes refused tabs from its next saved session. A corrected external file can reload normally, and that reload remains undoable.

This protects loading from disk, not arbitrary pasted/generated text, recovered unsaved drafts, Git revision/diff content, or image decoding. Those paths retain their existing behavior. The existing 8 MiB read-only and 256 MiB file-size limits remain separate. Windows/Linux desktop operation is not claimed from cross-compilation.

## Verification

- `core/text/longline_test.go`: 26 MiB fixture; exact/excess limits, middle/final lines, Unicode bytes, BOM and CRLF round trips, large multiline read-only files.
- `core/app/longline_test.go`: open/preview without losing a dirty tab; watcher/focus reload, Revert, conflict-safe Save, undo and valid reload recovery; mixed session restore and saved-session cleanup.
- `ui/gui/longline_test.go`: refused 26 MiB input never enters GUI layout, error stays visible, dirty text and undo remain usable.
- Native `YAN_SELFTEST_MODE=long-line`: independent app/config and disposable files; error capture, editing, external change, Revert, undo, normal reload and disk preservation.
- Existing asynchronous Git tests wait for repository readiness and operation completion before asserting; the formula resize fixture uses a wide rule to isolate geometry from renderer cost under race instrumentation. Production rendering timeouts remain unchanged.
- Release gates: complete `make check`, Windows cross-compilation, exact-head Linux CI, public-source tree/history scans, packaged-app verification and anonymous signed update installation. Delivery evidence is recorded separately with actual release and commit IDs.

## Continuation contract

Read AGENTS.md, this record and the feature ledger first. Preserve disk-input refusal and all existing save-conflict/undo semantics. Future support for oversized lines must bound layout costs rather than silently truncating content or weakening conflict checks. Keep native platform results separate from compilation checks.
