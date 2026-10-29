# UP-62 formula preview acceptance

UP-62 adapts Mayoi `1010308b542c4bfa25e15f84f4992914fac04223` to Yan's Markdown preview. The feature uses the same MyGo fork revision, `4bd079439ee7`, for inline object layout. It does not incorporate the separate UP-66 upgrade.

## Contract and evidence

| Contract | Evidence |
| --- | --- |
| Inline `$…$`, display `$$…$$`, and fenced `math` | `TestRenderMath`, `TestMarkdownMath`, `TestMathPreview`; fractions, roots, integrals, matrices, AMS alignment and bold symbols |
| Currency, escaped dollars and code stay literal | `TestMarkdownMath`, including empty and unmatched delimiters |
| Existing prose, headings, tables, lists, footnotes and Mermaid | Full GUI suite, including the existing Markdown, live editing and Mermaid suites |
| Theme, wrapping and table alignment | `TestMathPlaces`, `TestMathTableAlignmentAndWidth`, `TestMathLongInlineResizes`; light/dark and narrow-window captures inspected |
| Source, copy, edit and undo | `TestMathPreview`, `TestMathPreviewEditUndo`; both display delimiters and math fences survive editing and undo |
| Default on; user/project configuration; live reload | `TestPreviewMathLayers`, `TestMathPreview`; native watcher tested off and on |
| Cache reuse, eviction and cancelled parsing | `TestMathCacheAndCancellation` |
| Formula isolation and bounded evaluation | `TestMathIsolationAndRecovery`: document macros do not leak, dynamic packages and host APIs are unavailable, oversized input is rejected, a timed-out runtime is discarded and the next formula renders |
| Terminal compatibility after dependency change | `TestNativeTerminal*`: links, file/image/text paste, selection copy, panel movement and retained sessions; actual shell test confirms command output, resize, panel/editor movement and Ctrl+C |
| Real macOS window | `YAN_SELFTEST_MODE=math`: 1440×900 and 1100×720, light/dark, larger font, adjacent source/preview columns, source edits/undo and project config reload; all checks passed |

The headless MyGo tester now has a deterministic clock. The rain lifecycle test advances that clock explicitly; its pause/resume assertions are retained. Terminal tests use the current Sidebar accessibility label and actual image clipboard data.

## Resources and limits

- MathJax 3.2.2 is bundled as SVG path output with base, AMS, newcommand and boldsymbol packages. It runs without host filesystem or network bindings; TeX is passed as a function argument.
- The compressed asset is 577,088 bytes. Rebuilding with `scripts/mathjax.sh` produces the same SHA-256. Apache-2.0 text is included in the asset banner and `packaging/licenses/mathjax.txt`, and the notice generator includes it in distributed licenses. Mayoi attribution remains in `THIRD_PARTY_NOTICES.md`.
- Each formula accepts at most 32 KiB and has a five-second render deadline. A document parse limits new formulas to 256 within a ten-second budget. Formulas beyond those limits retain source and an explanation.
- Unsupported glyphs, including Chinese inside `\text{…}`, retain source; surrounding Chinese Markdown is unaffected. This is a math preview, not a full TeX document engine.
- Windows/Linux native windows have not been accepted. Linux CI and cross-compilation are recorded separately from macOS native evidence.

## Local release gates

Both the development source and the independently maintained public source pass complete `make check`: formatting, vet, all tests, race detection, architecture boundaries and the TUI size gate. TUI sizes are 10,436,978 and 10,453,490 bytes respectively, below 10,485,760 bytes. The public source retains its session/crash-log permission hardening and MIT notices.

Native captures use MyGo's capture interface to advance content when macOS marks the unattended test window as occluded; ordinary hidden windows keep the framework's redraw suspension. Actual shell tests run against the packaged libghostty-vt.
