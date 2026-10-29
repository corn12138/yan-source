package gui

import (
	"github.com/corn12138/yan-source/core/view"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// The terminal owns file drops in its body; edge targets sit above it only during a drag.
func (f *Frontend) paneDrops(c *ui.Context, pane ui.Element, ci int) {
	opts := transfer.DropOptions{Formats: []transfer.Format{transfer.FileList}}
	d, tabOver := ui.DragOver[tabDrag](pane)
	tabOver = tabOver && d.f == f
	_, fileOver := ui.DataDragOver(pane, opts)

	if d, ok := ui.Drop[tabDrag](pane); ok && d.f == f {
		f.do(func() {
			if m := d.match; m != nil {
				f.ed.DropSearchMatch(ci, m.row, m.line, 0)
			} else if d.s != nil {
				f.panelToColumn(d.s, ci, len(f.frame.Columns[ci].Tabs))
			} else {
				f.ed.MoveTab(d.col, d.at, ci, len(f.frame.Columns[ci].Tabs))
			}
		})
	}
	if data, ok := ui.DropData(pane, opts); ok {
		if paths, err := data.Data.Files(); err == nil {
			f.do(func() { f.ed.DropFiles(ci, paths) })
		}
	}
	// Keep edge controls present through the release frame even after DragOver clears.
	cs := &f.cols[ci]
	if tabOver || fileOver {
		cs.dropFrames = 2
	}
	if cs.dropFrames == 0 {
		return
	}
	cs.dropFrames--
	pane.Children(func() {
		for _, before := range []bool{true, false} {
			edge := ui.Box(c.Key(before)).Absolute().Top(0).Bottom(0).Width(32)
			if before {
				edge.Left(0)
			} else {
				edge.Right(0)
			}
			d, over := ui.DragOver[tabDrag](edge)
			over = over && d.f == f
			_, filesOver := ui.DataDragOver(edge, opts)

			if over || filesOver {
				cs.dropFrames = 2
				edge.Background(f.pal.accent.Alpha(0.24))
			}
			if d, ok := ui.Drop[tabDrag](edge); ok && d.f == f {
				f.do(func() {
					if m := d.match; m != nil {
						split := 1
						if before {
							split = -1
						}
						f.ed.DropSearchMatch(ci, m.row, m.line, split)
					} else if d.s != nil {
						f.panelToSplit(d.s, ci, before)
					} else {
						f.ed.MoveTabToSplit(d.col, d.at, ci, before)
					}
				})
			}
			if data, ok := ui.DropData(edge, opts); ok {
				if paths, err := data.Data.Files(); err == nil {
					f.do(func() { f.ed.DropFilesInSplit(ci, before, paths) })
				}
			}
		}
	})
	if fileOver && f.frame.Columns[ci].Pane.Kind != view.PaneTerminal {
		pane.Border(2, f.pal.accent)
	}
}

func (f *Frontend) panelToSplit(s *termSession, ci int, before bool) {
	for i, o := range f.terms {
		if o != s {
			continue
		}
		id := f.ed.OpenTerminalInSplit(ci, before, s.dir, s.title)
		if id == 0 {
			return
		}
		f.terms = append(f.terms[:i:i], f.terms[i+1:]...)
		f.termActive = max(min(f.termActive, len(f.terms)-1), 0)
		if len(f.terms) == 0 {
			f.hidePanel()
		}
		s.id = id
		if f.tabTerms == nil {
			f.tabTerms = map[int]*termSession{}
		}
		f.tabTerms[id] = s
		f.shownTerm, f.focusTerm, f.focusRoot = 0, false, false
		return
	}
}
