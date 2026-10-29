package app

import (
	"github.com/corn12138/yan-source/core/git/linediff"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
)

// 窗格右边的一条（照着 Mayoi 的 overview）：整个文件里相对 HEAD 改了的行、查找的匹配、光标所在的行、眼前这一屏，
// 都按占文件的比例画。点它、拖它跳到文件的那个地方。

// overviewCache 是一个文档改了的行连成的段，按 git 标记（或改动视图的行）的版本记着，不必每帧重算。
type overviewCache struct {
	version         uint64
	marksGeneration uint64
	lines           int
	runs            []view.OverviewRun
}

// overviewBuckets 是查找的匹配最多分成几格：一万个匹配也只画这么多道。
const overviewBuckets = 1000

// overviewFrame 填窗格右边那一条。
func (e *Editor) overviewFrame(t *tab, s text.Snapshot, top int, pane *view.PaneFrame) {
	pane.Overview = nil
	n := s.LineCount()
	if n <= 1 || !isFileDoc(t) && t.diff == nil {
		return
	}
	ov := &t.overview
	*ov = view.Overview{Lines: n, Top: top, Caret: int(pane.Current), Changes: e.overviewRuns(t), Matches: ov.Matches[:0], Reviews: ov.Reviews[:0]}
	if k := len(pane.Rows); k > 0 {
		ov.Visible = int(pane.Rows[k-1].Line) - top + 1
	}
	if f := &e.find; f.open && f.doc == t.doc && len(f.matches) > 0 {
		last := -1
		for _, m := range f.matches {
			if m[0] > s.Len() {
				break
			}
			line := s.LineAt(m[0])
			if b := line * overviewBuckets / n; b != last {
				ov.Matches = append(ov.Matches, line)
				last = b
			}
		}
	}
	if t.diff != nil {
		lastID := 0
		for i, row := range t.diff.lines {
			if row.noteID != 0 && row.noteID != lastID {
				ov.Reviews = append(ov.Reviews, i)
			}
			lastID = row.noteID
		}
	} else {
		for _, note := range e.review.notes {
			if note.Path == t.doc.buf.Path() && note.Commit == "" && !note.Staged && !note.OldSide && !note.Outdated {
				ov.Reviews = append(ov.Reviews, note.From)
			}
		}
	}
	pane.Overview = ov
}

// overviewRuns 返回改了的行连成的段：文件是相对 HEAD 的标记，改动视图是加上、删掉的行。
func (e *Editor) overviewRuns(t *tab) []view.OverviewRun {
	d := t.doc
	c := &d.overview
	if dt := t.diff; dt != nil {
		v := d.buf.Snapshot().Version()
		if c.runs != nil && c.version == v && c.lines == len(dt.lines) {
			return c.runs
		}
		c.version, c.lines, c.runs = v, len(dt.lines), c.runs[:0]
		for i := 0; i < len(dt.lines); {
			k := dt.lines[i].kind
			j := i + 1
			for j < len(dt.lines) && dt.lines[j].kind == k {
				j++
			}
			switch k {
			case diffAdded:
				c.runs = append(c.runs, view.OverviewRun{From: i, To: j, Mark: view.MarkAdded})
			case diffRemoved:
				c.runs = append(c.runs, view.OverviewRun{From: i, To: j, Mark: view.MarkDeleted})
			}
			i = j
		}
		if c.runs == nil {
			c.runs = []view.OverviewRun{}
		}
		return c.runs
	}
	g := &d.marks
	if c.runs != nil && c.version == g.version && c.marksGeneration == g.generation && c.lines == len(g.lines) {
		return c.runs
	}
	c.version, c.lines, c.runs = g.version, len(g.lines), c.runs[:0]
	c.marksGeneration = g.generation
	for i := 0; i < len(g.lines); {
		k := g.lines[i]
		j := i + 1
		for j < len(g.lines) && g.lines[j] == k && k != linediff.Deleted && k != linediff.DeletedBelow {
			j++
		}
		if m := g.mark(i); m != view.MarkNone {
			c.runs = append(c.runs, view.OverviewRun{From: i, To: j, Mark: m})
		}
		i = j
	}
	if c.runs == nil {
		c.runs = []view.OverviewRun{}
	}
	return c.runs
}

// OverviewJump 把第 col 列滚到文件的 frac 处（0 是开头，1 是末尾），那一行在这一屏的中间：点、拖窗格右边那一条。
func (e *Editor) OverviewJump(col int, frac float64) {
	e.sync()
	defer e.sync()
	if col < 0 || col >= len(e.cols) {
		return
	}
	c := e.cols[col]
	t := c.tab()
	if t == nil {
		return
	}
	s := t.doc.buf.Snapshot()
	n := s.LineCount()
	line := int(min(max(frac, 0), 1) * float64(n))
	line = min(max(line-int(c.size.H)/2, 0), max(n-1, 0))
	t.setTop(vrow{line, 0})
}
