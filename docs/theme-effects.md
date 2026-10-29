# Theme decorations

This batch ports Mayoi's Clawd drawing, pose timing, and Matrix rain from `cmd/mayoi/` at
`e8cfddd`, under its MIT license. Qoder and Qoduck remain excluded.

| Requirement | Implementation | Acceptance |
| --- | --- | --- |
| Claude welcome and status mascot | Pixel art at two sizes; idle blink/bob, Git and file-search activity | Rendered light/dark screenshots; pose boundaries and activity tests |
| Matrix welcome rain | Deterministic glyph columns behind a readable welcome card; no file overlay | Rendered screenshot; open/close file and theme transition tests |
| Reduce background work | Paint-only delayed frames; static when blurred or system Reduce Motion is set | GUI state and held-frame regressions; native OS preference UI not exercised |
| Disable decorations | View → Theme checkbox and `yan.toggleThemeEffects` command | Menu callback, removal, re-enable and saved-state tests |
| Configuration precedence | `[ui] theme_effects` defaults true; project overrides user config, saved menu choice overrides both | Layering and persisted-false regressions |
| Platform seam | No new shortcut; MyGo preferences/focus APIs and core config | Portable implementation; Windows/Linux native apps not accepted |

The effects are GUI-only. Search activity is read from the core even when its sidebar is hidden.
Animations use the painter's delayed invalidation rather than a goroutine or continuously ticking
editor refresh. A new view drops callbacks for removed effects; a blurred inactive project also clears
its activity flags when focus changes. Reduced motion preserves a static decoration, while the explicit
checkbox removes decorations entirely. The default Mayoi theme has neither decoration.

## Validation boundary

Targeted core/config/GUI tests passed. The in-memory renderer produced Claude light/dark and Matrix
welcome screenshots, inspected for mascot placement, readable hints and unclipped status controls.
The system preference panel and native animation cadence were not manually exercised. Full local `make check` passed, including races, vet, architecture boundaries and the 10 MiB
TUI gate (8,794,658 bytes; GUI 19,608,962 bytes). PR #10 passed Linux CI run 38846902027 and merged. Version v0.1.0-alpha.10 was published
from 6be50553ecab, and the downloaded release artifacts passed SHA-256 checks. Apple signing/notarization and Windows/Linux
packaging are separate from this feature.

## Color theme picker

The picker follows Mayoi `d81bfc4`, adapted to Yan's headless core and shared Frame. Open
**View → Theme → Color Theme…**, use the welcome page, or run `workbench.action.selectTheme`.
Up/Down previews a theme. Left/Right changes appearance only with an empty search; a query
retains ordinary text navigation and IME composition. Search accepts abbreviated theme names.
Enter or clicking a theme confirms; Escape, an outside click or another overlay cancels.
Matrix always renders dark and disables appearance controls without losing the previous preference.

Preview is separate from config/state, so saving a session or zoom setting cannot persist it.
Cancellation exposes the latest configuration, including external changes received during preview.
Each editor owns its preview. Native dialogs and system controls follow the desktop appearance;
each editor draws its selected palette, so another window's explicit light/dark setting cannot
replace the system signal. This also keeps System accurate when leaving Matrix's forced dark theme.
GUI palettes transition for 160 ms, continue from the current colors when interrupted, and stop
immediately with Reduce Motion or window blur. TUI changes the same palette without animation.
Qoder/Qoduck remain excluded; existing direct-theme commands remain available.

Targeted core and GUI tests cover confirmation, rollback, fuzzy filtering, text navigation, composition,
separate windows, concurrent configuration changes, persistence and interrupted/reduced-motion fades.
Rendered light/dark and 480-pixel-wide screenshots were inspected. Native OS theme switching,
system IME candidate windows and Windows/Linux desktop execution remain separate acceptance checks.
