# Window and file-opening workflows

This batch follows the agreed Mayoi integration order after the layout and terminal batch
(v0.1.0-alpha.8). Qoder and Qoduck are excluded. Windows/Linux client distribution and automatic
updates remain separate work.

| Requirement | Implementation | Acceptance evidence |
| --- | --- | --- |
| Open folder in a new native window | File menu / `yan.openFolderNewWindow`; macOS Option+Command+O; Windows/Linux Ctrl+K, Shift+O | Platform keymap tests; native separate-window check passed |
| Reuse an already-open folder | Compare filesystem identity across native windows and project tabs, including symlink aliases; restore minimized windows | Dirty-buffer and alias regression; native hidden-window focus check passed |
| Recent folders from menu and Dock | Both use shared folder routing; the Dock opens a separate window when needed | Native menu contents and callback passed |
| Sidebar on either side | View → Move Sidebar; persistent state or `[explorer] right = true`; GUI and TUI | GUI position and TUI geometry regression |
| Click status indentation | Choose tabs or 2/4/8 spaces for subsequent editing; existing text is retained | Core and GUI tests |
| Click status EOL | Convert LF/CRLF in one undo step; retain BOM, selections, mixed-ending undo, and conflict safety | Buffer and editor regressions |
| CLI launcher | File → Install yan Command; writes `~/.local/bin/yan` only on explicit action; refuses other files and symlinks | Real shell argument and collision tests; real second-process delivery passed |
| Reopen closed terminals | Already released in alpha.8; no additional implementation needed | Existing terminal session tests |

`yan folder`, `yan file:42`, and several file arguments open in the GUI. `--new-window` chooses a new
folder window; an already-open folder is reused. `--` permits filenames beginning with a dash.
A real filename ending in a colon and number takes precedence over line notation. Direct `yan-app`
invocations share the same request parser. A second process forwards requests to the first instance.

The installed launcher points to the current executable. If the application moves, remove that launcher
and install it again. Installation does not edit shell startup files or require administrator rights.
If `~/.local/bin` is absent from PATH, the application explains how to invoke the full command path.
The installer is separated by platform; Windows installation is deliberately unavailable until its
client packaging is implemented. The existing TUI binary is not replaced.

## Validation gates

Local gates passed: focused tests, `make check` (including races and architecture checks), and the
native macOS workflow with isolated configuration. The TUI binary is 8,794,610 bytes, below the
10 MiB gate; the GUI binary is 19,608,722 bytes. PR #9 passed one Linux CI run (38844867700)
and merged. Version v0.1.0-alpha.9 was published from tag commit 143dddc239fd; the downloaded
archive and disk image passed their published SHA-256 checksums. Passing an in-memory UI test does not
constitute Finder drag, system IME, Gatekeeper, or Windows/Linux native acceptance.

## Observed native behavior

The `window-workflows` self-test used a disposable app bundle and independent configuration. It
passed separate native windows with project tabs enabled, real second-process delivery, hidden-window
restoration and focus, line navigation without losing unsaved text, folder aliases, right-sidebar state,
and Dock menu contents and callbacks. A direct native accessibility/screenshot check also showed the
existing window and line 2 after a command-line request.

Minimization recovery has a platform API hook, but the automated OS minimization state could not be
established reliably in this environment; the repeatable native test explicitly covers hidden windows.
The later screen-capture service failed, so native clicking of the status controls is not claimed;
their click paths and right-sidebar resizing passed the in-memory GUI tester. No launcher was installed
into the user's actual shell directories during validation.

## Remaining integration

Next agreed work: Clawd/Matrix effects, then automatic updates after resolving signing configuration.
Mayoi's newer line comments and review-to-terminal workflow remain tracked in `MAYOI.md` and are not
part of this window batch.

## Continuation brief

Continue from the current branch without touching unrelated work in the primary checkout. Verify
window reuse across project groups and aliases, dirty-buffer preservation, command-line delivery,
right-sidebar hit testing and resizing, and EOL undo/save/conflict behavior. Complete the validation
gates above, update this report with actual outcomes, then merge and publish the next alpha. Use one
PR CI run only after local checks pass; keep Windows/Linux shortcut and installer seams intact.
