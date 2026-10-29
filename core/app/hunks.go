package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/git/linediff"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
)

// 行号旁的改动，照着 Mayoi（MIT，cmd/mayoi/hunks.go 的 peek、revert、stage）：点一个改动标记，在改动下面开一个框，
// 写着这几行在 HEAD 里是什么样；框里能还原这处改动（是撤销的一步），或者只把这几行暂存进索引。
// ⌥F3、⇧⌥F3 开下一处、上一处的框（同 VS Code 的 editor.action.dirtydiff.next）；⌃K R、⌃K S 还原、暂存光标所在的那处，
// 不开框也行（同 VS Code 的 git.revertSelectedRanges、git.stageSelectedRanges）。Esc 关框。

// hunkPeek 是一个标签里开着的框：开在哪一行（改动跟着编辑挪，每帧按它重新找是哪处改动）。
type hunkPeek struct {
	line int
}

// hunkAt 返回第 line 行所在的那处改动（删掉的行算在它下面那一行，文件末尾删掉的算在最后一行）。
func hunkAt(hs []linediff.Hunk, line int) (int, bool) {
	for i, h := range hs {
		first, last := h.B, h.B+h.BLen-1
		if h.BLen == 0 { // 只删了行：标在上面那一行
			first, last = max(h.B-1, 0), max(h.B-1, 0)
		}
		if line >= first && line <= last {
			return i, true
		}
	}
	return 0, false
}

// peekHunk 在第 line 行的改动下面开框（已经开着就关上）。
func (e *Editor) peekHunk(t *tab, line int) bool {
	g := &t.doc.marks
	i, ok := hunkAt(g.hunks, line)
	if !ok {
		return false
	}
	if t.peek != nil {
		if j, ok := hunkAt(g.hunks, t.peek.line); ok && j == i {
			t.peek = nil
			return true
		}
	}
	h := g.hunks[i]
	t.peek = &hunkPeek{line: max(h.B, 0)}
	if h.BLen == 0 {
		t.peek.line = max(h.B-1, 0)
	}
	return true
}

// stepPeek 在下一处（或上一处）改动下面开框，光标跟过去，到头绕回（⌥F3、⇧⌥F3）。
func (e *Editor) stepPeek(by int) error {
	t := e.cur()
	if t == nil {
		return nil
	}
	if err := e.stepChange(by); err != nil {
		return err
	}
	s := t.doc.buf.Snapshot()
	line := s.LineAt(t.doc.buf.Selections().Main().Head)
	t.peek = nil
	if !e.peekHunk(t, line) {
		e.peekHunk(t, line+1) // 只删了行的改动标在上面一行
	}
	return nil
}

// peekedHunk 返回框开着的那处改动；改动没了（还原了、改回去了）就关框。
func (e *Editor) peekedHunk(t *tab) (int, bool) {
	if t.peek == nil {
		return 0, false
	}
	i, ok := hunkAt(t.doc.marks.hunks, t.peek.line)
	if !ok && t.doc.marks.diffed {
		t.peek = nil
	}
	return i, ok
}

// targetHunk 是还原、暂存要做的那处：框开着就是框里那处，否则是光标所在的那处。
func (e *Editor) targetHunk() (*tab, linediff.Hunk, error) {
	t := e.cur()
	if t == nil || t.diff != nil {
		return nil, linediff.Hunk{}, errors.New("没有打开的文件")
	}
	g := &t.doc.marks
	if i, ok := e.peekedHunk(t); ok {
		return t, g.hunks[i], nil
	}
	line := t.doc.buf.Snapshot().LineAt(t.doc.buf.Selections().Main().Head)
	if i, ok := hunkAt(g.hunks, line); ok {
		return t, g.hunks[i], nil
	}
	return nil, linediff.Hunk{}, errors.New("光标所在的行没有改动")
}

// revertHunk 把一处改动还原成 HEAD 里的样子：编辑器里的一步，能撤销。
func (e *Editor) revertHunk() error {
	t, h, err := e.targetHunk()
	if err != nil {
		return err
	}
	base := t.doc.marks.base
	if h.A+h.ALen > len(base) {
		return errors.New("改动和 HEAD 对不上了：等一下再试")
	}
	old := base[h.A : h.A+h.ALen]
	return e.editDoc(func(b *edit.Buffer) {
		s := b.Snapshot()
		nl := eolOf(s)
		var ch text.Change
		switch {
		case h.BLen > 0: // 改了、加了行：换成 HEAD 里的（可能是没有）
			start := s.LineStart(h.B)
			end := lineEnd(s, h.B+h.BLen-1)
			repl := strings.Join(old, nl)
			if h.ALen == 0 { // 只是加的行：连同一个换行符删掉
				if h.B+h.BLen < s.LineCount() {
					end = s.LineStart(h.B + h.BLen)
				} else if h.B > 0 {
					start = lineEnd(s, h.B-1)
				}
			}
			ch = text.Change{Start: start, End: end, Text: repl}
		case h.B < s.LineCount(): // 只删了行：插回到第 B 行前面
			at := s.LineStart(h.B)
			ch = text.Change{Start: at, End: at, Text: strings.Join(old, nl) + nl}
		default: // 文件末尾删掉的
			at := s.Len()
			ch = text.Change{Start: at, End: at, Text: nl + strings.Join(old, nl)}
		}
		b.Apply(edit.Transaction{Changes: text.ChangeSet{ch}, Origin: edit.OriginCommand})
		b.BreakGroup()
		t.peek = nil
		e.status = "已还原这处改动（可以撤销）"
	})
}

// stageHunk 只把一处改动暂存进索引：在后台读索引里的文字，按索引和编辑器里的文字找出这处改动，换进去。
func (e *Editor) stageHunk() error {
	if t := e.cur(); t != nil {
		defer e.bindRepository(e.repositoryForPath(t.doc.buf.Path()))()
	}
	t, h, err := e.targetHunk()
	if err != nil {
		return err
	}
	r := e.repository().git.repo
	if r == nil {
		return errors.New("不在 git 仓库里")
	}
	path := t.doc.buf.Path()
	s := t.doc.buf.Snapshot()
	cur := strings.Split(strings.ReplaceAll(s.String(0, s.Len()), "\r\n", "\n"), "\n")
	first, last := h.B, h.B+h.BLen-1
	if h.BLen == 0 {
		first, last = h.B, h.B
	}
	t.peek = nil
	return e.gitDo("暂存中…", func(r *git.Repo) error {
		idx, ok, err := r.Show("", path)
		if err != nil {
			return err
		}
		var lines []string
		if ok {
			lines = strings.Split(idx, "\n")
		}
		// 索引里可能已经暂存过别的改动：按索引和编辑器里的文字重新比较，找和这处改动重叠的那一处。
		for _, ih := range linediff.Hunks(lines, cur) {
			a, z := ih.B, ih.B+max(ih.BLen, 1)-1
			if z < first || a > last {
				continue
			}
			out := append(append(append([]string{}, lines[:ih.A]...), cur[ih.B:ih.B+ih.BLen]...), lines[ih.A+ih.ALen:]...)
			return r.StageText(path, strings.Join(out, "\n"))
		}
		return errors.New("这处改动已经暂存了")
	}, func(e *Editor) {
		e.status = "已暂存这处改动"
	})
}

// closePeek 关掉框（Esc）；没开着时返回 false，Esc 照常。
func (e *Editor) closePeek() bool {
	if t := e.cur(); t != nil && t.peek != nil {
		t.peek = nil
		return true
	}
	return false
}

// peekFrame 把开着的框写进窗格：画在哪一行下面、HEAD 里是什么、第几处。
func (e *Editor) peekFrame(t *tab, pane *view.PaneFrame) {
	pane.Peek = nil
	i, ok := e.peekedHunk(t)
	if !ok {
		return
	}
	g := &t.doc.marks
	h := g.hunks[i]
	end := h.B + h.BLen - 1
	if h.BLen == 0 {
		end = max(h.B-1, 0)
	}
	after := -1
	for k, row := range pane.Rows {
		if int(row.Line) == end {
			after = k // 改动最后一行的最后一个视觉行
		}
	}
	p := &view.Peek{After: after, Index: i, Count: len(g.hunks), Added: h.BLen}
	if h.A+h.ALen <= len(g.base) {
		p.Removed = g.base[h.A : h.A+h.ALen]
	}
	p.Title = fmt.Sprintf("第 %d 处改动，共 %d 处", i+1, len(g.hunks))
	pane.Peek = p
}

// GutterClick 报告点在第 col 列窗格的 (x, y) 是不是点在了改动标记上，是的话开、关它的框。
func (e *Editor) gutterClick(c *column, t *tab, x, y view.Unit) bool {
	s := t.doc.buf.Snapshot()
	if x >= e.gutter(s) || !isFileDoc(t) {
		return false
	}
	pos := e.posAt(c, t, e.gutter(s), y)
	return e.peekHunk(t, s.LineAt(pos))
}

// isFileDoc 报告标签是不是一个能有 git 改动的文件（不是改动视图、图片、预览）。
func isFileDoc(t *tab) bool {
	return t.diff == nil && t.term == nil && !t.image && !t.svg && !t.markdown
}
