package gui

import (
	"github.com/corn12138/yan-source/core/view"
	"github.com/egoist/mygo/ui"
	"time"
)

// Parsing is shared by file, but selection and scrolling belong to each visible pane.
type markdownViewport struct {
	list                ui.ListState
	scroll              ui.ScrollState
	rows                []ui.Handle
	touched             time.Time
	initialized, resync bool
	gen                 uint64 // the PreviewGen it scrolled to TopLine for
	source, sourceLine  int
	edit                *markdownEdit // the block edited in place, nil while none is
	click               *[2]float32   // a double click, for the next frame to place
	clickAt             time.Time
}

func (m *markdownState) viewport(ci int) *markdownViewport {
	if m.views == nil {
		m.views = map[int]*markdownViewport{}
	}
	if m.views[ci] == nil {
		m.views[ci] = &markdownViewport{source: -1, sourceLine: -1}
	}
	return m.views[ci]
}

func markdownBlockAt(blocks []markdownBlock, line int) int {
	at := 0
	for at+1 < len(blocks) && blocks[at+1].line <= line {
		at++
	}
	// An alert's title and paragraph share a source line; keep the title visible.
	for at > 0 && blocks[at-1].line == blocks[at].line {
		at--
	}
	return at
}

func (f *Frontend) syncMarkdown(c *ui.Context, m *markdownState, v *markdownViewport, ci int, pf *view.PaneFrame) {
	if len(m.blocks) == 0 {
		return
	}
	if len(v.rows) != len(m.blocks) {
		v.rows = make([]ui.Handle, len(m.blocks))
		v.resync = true
	}
	source := -1
	for i, col := range f.frame.Columns {
		if i != ci && col.Pane.Kind == view.PaneCode && len(col.Tabs) > 0 && col.Active >= 0 && col.Tabs[col.Active].Path == pf.Path {
			source = i
			if i == f.frame.Active {
				break
			}
		}
	}
	if v.gen != pf.PreviewGen { // opened again in place: start where the text was
		v.gen, v.initialized = pf.PreviewGen, false
	}
	if !v.initialized {
		v.list.ScrollTo(markdownBlockAt(m.blocks, pf.TopLine), ui.Start)
		v.initialized = true
	}
	// Pointer/keyboard scrolling leads briefly, including momentum. Programmatic scrolls
	// do not take leadership back, which prevents source/preview feedback loops.
	lead := time.Since(v.touched) < 400*time.Millisecond
	if !lead && source >= 0 {
		line := f.frame.Columns[source].Pane.TopLine
		if v.source != source || v.sourceLine != line || v.resync {
			v.source, v.sourceLine, v.resync = source, line, false
			v.list.ScrollTo(markdownBlockAt(m.blocks, line), ui.Start)
			f.ed.SetPreviewTop(ci, pf.Path, line)
			return
		}
	}
	first, last := v.list.Visible()
	top := f.cols[ci].pane.Bounds(c).Y
	for i := max(0, first); i <= last && i < len(v.rows); i++ {
		bounds := v.rows[i].Bounds(c)
		if bounds.H <= 0 || bounds.Y+bounds.H <= top {
			continue
		}
		line := m.blocks[i].line
		// Interpolate within tall paragraphs/code while preserving source logical lines.
		next := pf.LineCount
		for j := i + 1; j < len(m.blocks); j++ {
			if m.blocks[j].line > line {
				next = m.blocks[j].line
				break
			}
		}
		share := min(max((top-bounds.Y)/bounds.H, 0), 0.999)
		line += int(share * float32(max(1, next-line)))
		f.ed.SetPreviewTop(ci, pf.Path, line)
		if lead && source >= 0 {
			f.ed.ScrollSourceTo(source, pf.Path, line)
			v.source, v.sourceLine = source, line
		}
		break
	}
}
