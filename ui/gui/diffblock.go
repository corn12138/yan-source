package gui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/view"
)

// diffBlockW is how wide the block buttons at the right of a change's header row are.
const diffBlockW = 330

// diffBlockButtons puts the block actions of each change on its header row of a diff: revert
// and append for the work tree's changes, either side into the other for a comparison (after
// the round arrows of Mayoi PR #50's diff, as buttons that say what they do).
func (f *Frontend) diffBlockButtons(c *ui.Context, ci int, pf *view.PaneFrame) {
	for i, row := range pf.Sticky { // a file's header pinned at the top keeps its buttons
		f.diffBlockBar(c, ci, fmt.Sprintf("diff-sticky-%d", i), i, row)
	}
	for i, row := range pf.Rows {
		if i >= len(pf.Sticky) {
			f.diffBlockBar(c, ci, fmt.Sprintf("diff-block-%d", i), i, row)
		}
	}
}

// diffBlockButtonsOf are the buttons of a row's block kind: label, tooltip, action.
func diffBlockButtonsOf(kind view.DiffBlock) [][3]string {
	switch kind {
	case view.DiffBlockRevert:
		return [][3]string{
			{i18n.T("✎ Edit"), i18n.T("Edit the working tree side alongside (i)"), "edit"},
			{i18n.T("+ Stage"), i18n.T("Stage this change"), "stage"},
			{i18n.T("↶ Revert"), i18n.T("Revert this change: restore the other side's lines (can be undone)"), "revert"},
			{i18n.T("⤓ Append"), i18n.T("Append the other side's lines after this change (can be undone)"), "append"},
		}
	case view.DiffBlockStaged:
		return [][3]string{{i18n.T("− Unstage"), i18n.T("Unstage this change"), "unstage"}}
	case view.DiffBlockCompare:
		return [][3]string{
			{i18n.T("→ Write Right"), i18n.T("Write this left-side change into the right file (can be undone)"), "revert"},
			{i18n.T("← Write Left"), i18n.T("Write this right-side change into the left file (can be undone)"), "acceptRight"},
		}
	case view.DiffBlockMerge:
		return [][3]string{
			{i18n.T("✓ Use This"), i18n.T("Use this side's lines for this conflict (both sides when the other is already used; can be undone)"), "take"},
			{i18n.T("✕ Ignore"), i18n.T("Don't use this side for this conflict: counts as handled (can be undone)"), "ignore"},
		}
	case view.DiffBlockMergeApply:
		return [][3]string{{i18n.T("» Apply This"), i18n.T("Apply this side's non-conflicting change to the result (can be undone)"), "apply"}}
	case view.DiffBlockFile:
		return [][3]string{{i18n.T("Open File"), i18n.T("Open this file at its first change"), "openFile"}, {i18n.T("↶ Discard Changes…"), i18n.T("Discard this file's changes (asks first)"), "discardFile"}, {i18n.T("+ Stage File"), i18n.T("Stage the whole file"), "stageFile"}}
	case view.DiffBlockFileDeleted:
		return [][3]string{{i18n.T("↶ Restore File…"), i18n.T("Restore this deleted file from the index (asks first)"), "discardFile"}, {i18n.T("+ Stage Deletion"), i18n.T("Stage this file's deletion"), "stageFile"}}
	case view.DiffBlockFileUntracked:
		return [][3]string{{i18n.T("Open File"), i18n.T("Open this file"), "openFile"}, {i18n.T("✕ Delete File…"), i18n.F("Move this file to %s (asks first)", app.TrashName(hostOS)), "discardFile"}, {i18n.T("+ Stage File"), i18n.T("Stage the whole file"), "stageFile"}}
	case view.DiffBlockFileStaged:
		return [][3]string{{i18n.T("Open File"), i18n.T("Open this file at its first change"), "openFile"}, {i18n.T("− Unstage"), i18n.T("Unstage the whole file"), "unstageFile"}}
	case view.DiffBlockFileConflict:
		return [][3]string{{i18n.T("Three-Way Merge"), i18n.T("Resolve this file's conflicts in the three-way merge"), "merge"}}
	}
	return nil
}

// diffBlockBar puts row's buttons at the right of its place in the pane.
func (f *Frontend) diffBlockBar(c *ui.Context, ci int, key string, i int, row view.RowFrame) {
	f.diffBlockBarAt(c, ci, key, codePadY+float32(i)*f.font.lineH, row)
}

// diffBlockBarAt puts row's buttons at the right of the pane, at y. In a narrow pane (three
// merge columns, a split window) the buttons that do not fit go into a "⋯" menu at the end of
// the bar, least needed first, as Mayoi #63 does with its merge bars: none is out of reach.
func (f *Frontend) diffBlockBarAt(c *ui.Context, ci int, key string, y float32, row view.RowFrame) {
	buttons := diffBlockButtonsOf(row.Block)
	if len(buttons) == 0 {
		return
	}
	line := int(row.Line)
	act := func(action string) {
		if row.Block == view.DiffBlockMerge {
			f.ed.MergeBlockAt(ci, line, action)
		} else {
			f.ed.DiffBlockAt(ci, line, action)
		}
	}
	shown, more := diffBlockFit(buttons, f.cols[ci].size.W-14-diffBlockLeft, func(s string) float32 { return textWidth(c, s, 11) })
	bar := ui.Row(c.Key(key)).Absolute().Right(14).Top(y).
		Height(f.font.lineH).Gap(4).AlignItems(ui.Center)
	bar.Children(func() {
		for _, b := range shown {
			el := ui.Box(c.Key(b[2])).Padding(0, 6).Height(f.font.lineH - 2).Radius(4).AlignItems(ui.Center).
				Background(f.pal.hover).Cursor(ui.CursorPointer).Label(b[0]).Tooltip(b[1])
			el.Children(func() { ui.Text(c, b[0]).FontSize(11).TextColor(f.pal.accent) })
			if el.Clicked() {
				act(b[2])
			}
		}
		if len(more) == 0 {
			return
		}
		el := ui.Box(c.Key("more")).Padding(0, 6).Height(f.font.lineH - 2).Radius(4).AlignItems(ui.Center).
			Background(f.pal.hover).Cursor(ui.CursorPointer).Label(i18n.T("More Actions")).Tooltip(i18n.T("More Actions"))
		el.Children(func() { ui.Text(c, "⋯").FontSize(11).TextColor(f.pal.accent) })
		el.Menu(func(m *ui.Menu) {
			for _, b := range more {
				if m.Item(b[0]).Chosen() {
					act(b[2])
				}
			}
		})
	})
}

// diffBlockLeft is room a bar leaves at the left of its pane, for the line numbers.
const diffBlockLeft = 32

// diffBlockDrop ranks the actions a narrow bar gives up, the first first: what a key, a click
// on the row or another button also does goes before what only its button does.
var diffBlockDrop = []string{"openFile", "edit", "append", "ignore", "acceptRight", "discardFile", "revert", "stage", "unstage", "stageFile", "unstageFile"}

// diffBlockFit splits buttons into those that fit in room, in their order, and those for the
// "⋯" menu, in their order too; width measures a label.
func diffBlockFit(buttons [][3]string, room float32, width func(string) float32) (shown, more [][3]string) {
	const pad, gap = 12, 4
	w := func(b [3]string) float32 { return width(b[0]) + pad + gap }
	need := float32(0)
	for _, b := range buttons {
		need += w(b)
	}
	if need <= room {
		return buttons, nil
	}
	gone := map[string]bool{}
	need += width("⋯") + pad + gap
	for _, a := range diffBlockDrop {
		if need <= room {
			break
		}
		for _, b := range buttons {
			if b[2] == a && !gone[a] {
				gone[a], need = true, need-w(b)
			}
		}
	}
	for i := len(buttons) - 1; i >= 0 && need > room; i-- { // any not ranked go last, from the right
		if a := buttons[i][2]; !gone[a] {
			gone[a], need = true, need-w(buttons[i])
		}
	}
	for _, b := range buttons {
		if gone[b[2]] {
			more = append(more, b)
		} else {
			shown = append(shown, b)
		}
	}
	return shown, more
}

// onDiffBlock reports whether a press at x, y of column ci's pane is on a change's block buttons.
func (f *Frontend) onDiffBlock(ci int, x, y float32) bool {
	pf := &f.frame.Columns[ci].Pane
	if y < codePadY {
		return false
	}
	_, row := f.cellAt(x, y)
	if x < f.cols[ci].size.W-diffBlockW {
		return false
	}
	if int(row) < len(pf.Sticky) {
		return pf.Sticky[row].Block != view.DiffBlockNone
	}
	return int(row) < len(pf.Rows) && pf.Rows[row].Block != view.DiffBlockNone
}
