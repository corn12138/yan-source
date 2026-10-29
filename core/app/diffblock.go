package app

// Block actions of a diff, after the behaviour of Mayoi PR #50 (Garfielk,
// https://github.com/sorrycc/Mayoi/pull/50, unmerged): a change of the work tree is reverted to
// the side it is compared with, or that side's lines are appended after it, through the file's
// document as one step of undo; a comparison of two files takes either side's block into the
// other. The diff's own undo and redo follow the document it edited. Rewritten for yan's diffs.

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
)

// diffBlock is a block action's target: the file edited, the lines of its block (from at, n of
// them), and the lines the block takes from the other side.
type diffBlock struct {
	path  string
	at, n int
	lines []string
}

type blockAction uint8

const (
	blockRevert      blockAction = iota // the other side's lines in place of the block
	blockAppend                         // the other side's lines after the block
	blockAcceptRight                    // a comparison: the right side's block into the left file
)

// diffBlockAt is the target of action on the change at line i of the diff tab dt.
func (e *Editor) diffBlockAt(dt *diffTab, i int, act blockAction) (diffBlock, error) {
	if i < 0 || i >= len(dt.lines) || dt.lines[i].hunk < 0 {
		return diffBlock{}, errors.New(i18n.T("Put the cursor in a change"))
	}
	l := dt.lines[i]
	if dt.commit != nil || dt.fixed {
		return diffBlock{}, errors.New(i18n.T("Changes in a commit are read-only"))
	}
	if act == blockAcceptRight && dt.peer == "" {
		return diffBlock{}, errors.New(i18n.T("Only a comparison of two files can write the right side into the left"))
	}
	path, base, cur := dt.path, dt.base, []string(nil)
	if dt.changes {
		if l.sec < 0 || l.sec >= len(dt.secs) {
			return diffBlock{}, errors.New(i18n.T("Put the cursor in a change"))
		}
		s := dt.secs[l.sec]
		if s.group != changeUnstaged || !s.baseOK || len(s.cur) == 0 {
			return diffBlock{}, errors.New(i18n.T("Only unstaged changes can be reverted block by block: unstage staged ones first; new and deleted files are handled whole"))
		}
		path, base = s.path, s.base
		if now, _ := e.currentLines(path); !slices.Equal(now, s.cur) {
			return diffBlock{}, errors.New(i18n.T("The diff view hasn't caught up with the file yet: try again in a moment"))
		}
	} else if !dt.hasBase {
		base = nil
	}
	cur, ver := e.currentLines(path)
	if !dt.changes && (ver != dt.version || dt.peer != "" && e.diffStale(dt.peer, dt.peerVersion)) {
		return diffBlock{}, errors.New(i18n.T("The diff view hasn't caught up with the file yet: try again in a moment"))
	}
	hs := e.diffHunks(base, cur)
	if l.hunk >= len(hs) {
		return diffBlock{}, errors.New(i18n.T("The diff view hasn't caught up with the file yet: try again in a moment"))
	}
	h := hs[l.hunk]
	switch act {
	case blockAppend:
		if h.ALen == 0 {
			return diffBlock{}, errors.New(i18n.T("The other side has no lines to append"))
		}
		return diffBlock{path: path, at: h.B + h.BLen, lines: base[h.A : h.A+h.ALen]}, nil
	case blockAcceptRight:
		return diffBlock{path: dt.peer, at: h.A, n: h.ALen, lines: cur[h.B : h.B+h.BLen]}, nil
	}
	return diffBlock{path: path, at: h.B, n: h.BLen, lines: base[h.A : h.A+h.ALen]}, nil
}

// replaceLines is the change that puts lines in place of n lines of s from line at (n 0
// inserts them before it; no lines deletes the n lines with their line break).
func replaceLines(s text.Snapshot, at, n int, lines []string) text.Change {
	nl, count := eolOf(s), s.LineCount()
	switch {
	case n > 0 && len(lines) > 0:
		return text.Change{Start: s.LineStart(at), End: lineEnd(s, at+n-1), Text: strings.Join(lines, nl)}
	case n > 0:
		start, end := s.LineStart(at), lineEnd(s, at+n-1)
		if at+n < count {
			end = s.LineStart(at + n)
		} else if at > 0 {
			start = lineEnd(s, at-1)
		}
		return text.Change{Start: start, End: end}
	case at < count:
		p := s.LineStart(at)
		return text.Change{Start: p, End: p, Text: strings.Join(lines, nl) + nl}
	}
	return text.Change{Start: s.Len(), End: s.Len(), Text: nl + strings.Join(lines, nl)}
}

// diffBlockAction runs act on the change under the caret of the diff tab being edited.
func (e *Editor) diffBlockAction(act blockAction) error {
	t := e.cur()
	if t == nil || t.diff == nil {
		return errors.New(i18n.T("Open a diff view first"))
	}
	dt := t.diff
	line := t.doc.buf.Snapshot().LineAt(t.doc.buf.Selections().Main().Head)
	b, err := e.diffBlockAt(dt, line, act)
	if err != nil {
		return err
	}
	d, err := e.blockDoc(b.path)
	if err != nil {
		return err
	}
	if d.buf.Snapshot().Meta().ReadOnly {
		return errors.New(i18n.T("Read-only file: can't change it"))
	}
	d.buf.BreakGroup()
	d.buf.Apply(edit.Transaction{Changes: text.ChangeSet{replaceLines(d.buf.Snapshot(), b.at, b.n, b.lines)}, Origin: edit.OriginCommand})
	d.buf.BreakGroup()
	dt.editTarget = d
	if act != blockAcceptRight {
		dt.glowLines(b.path, b.at, len(b.lines)) // 写进去的行亮一下
	}
	e.rediff(t)
	e.diffGo(t, line)
	e.status = map[blockAction]string{
		blockRevert:      i18n.T("Reverted this change (can be undone)"),
		blockAppend:      i18n.T("Appended the other side's lines after this change (can be undone)"),
		blockAcceptRight: i18n.T("Wrote this right-side change into the left file (can be undone)"),
	}[act]
	if dt.peer != "" && act == blockRevert {
		e.status = i18n.T("Wrote this left-side change into the right file (can be undone)")
	}
	return nil
}

// blockDoc is the document of path a block action edits: open already, or opened in a tab
// behind the diff, so that the change shows as unsaved and saves like any edit.
func (e *Editor) blockDoc(path string) (*document, error) {
	if d := e.docOf(path); d != nil && len(d.tabs) > 0 {
		return d, nil
	}
	c, at := e.column(), e.column().active
	fc := e.fileColumn()
	if err := e.openIn(fc, fc.active+1, path, false); err != nil {
		return nil, err
	}
	e.activate(c, at)
	if d := e.docOf(path); d != nil {
		return d, nil
	}
	return nil, errors.New(i18n.F("Couldn't open %s", path))
}

// diffHistory is undo (or redo) in a diff tab: of the document its block actions edited.
func (e *Editor) diffHistory(t *tab, undo bool) {
	d := t.diff.editTarget
	if d == nil || !slices.Contains(e.docs, d) {
		e.status = i18n.T("Nothing to undo in the diff view")
		return
	}
	var ok bool
	if undo {
		_, ok = d.buf.Undo()
	} else {
		_, ok = d.buf.Redo()
	}
	if !ok {
		e.status = map[bool]string{true: i18n.T("Nothing to undo"), false: i18n.T("Nothing to redo")}[undo]
		return
	}
	line := t.doc.buf.Snapshot().LineAt(t.doc.buf.Selections().Main().Head)
	e.rediff(t)
	e.diffGo(t, line)
}

// rediff reads a diff tab again after its file changed under it.
func (e *Editor) rediff(t *tab) {
	if t.diff.changes {
		e.refreshChanges(t)
		return
	}
	e.refreshDiff(t.doc, t.diff)
}

// DiffBlockAt runs a block action, "revert", "append" or "acceptRight", on the change whose
// header is at line of column col's diff (the buttons on the header rows).
func (e *Editor) DiffBlockAt(col, line int, action string) {
	e.FocusColumn(col)
	defer e.sync()
	t := e.cur()
	if t == nil || t.diff == nil {
		return
	}
	s := t.doc.buf.Snapshot()
	if line < 0 || line >= s.LineCount() {
		return
	}
	t.doc.buf.SetSelections(edit.Single(edit.Cursor(s.LineStart(line))))
	var err error
	switch action {
	case "stage", "unstage", "stageFile", "unstageFile":
		err = e.diffStage(t, line)
	case "openFile":
		err = e.diffOpenFile(t, line)
	case "discardFile":
		err = e.diffDiscardFile(t, line)
	case "edit":
		err = e.diffEditBeside(t)
	case "merge":
		err = e.diffOpen(t)
	default:
		act := map[string]blockAction{"revert": blockRevert, "append": blockAppend, "acceptRight": blockAcceptRight}[action]
		err = e.diffBlockAction(act)
	}
	if err != nil {
		e.status = err.Error()
	}
}

// diffOpenFile opens the file of line i of the diff tab t, at its first change.
func (e *Editor) diffOpenFile(t *tab, i int) error {
	dt := t.diff
	if i < 0 || i >= len(dt.lines) {
		return nil
	}
	path, at := dt.pathOf(dt.lines[i]), 0
	for j := i + 1; j < len(dt.lines) && dt.lines[j].sec == dt.lines[i].sec && dt.lines[j].kind != diffFile; j++ {
		if dt.lines[j].kind == diffHeader && dt.lines[j].line >= 0 {
			at = dt.lines[j].line
			break
		}
	}
	c := e.fileColumn()
	if err := e.openIn(c, c.active+1, path, false); err != nil {
		return err
	}
	e.GotoLine(at + 1)
	return nil
}

// diffDiscardFile discards the changes of the file of line i of the tab of all the changes,
// after asking: a deleted file comes back from the index, a file Git does not track goes to the
// trash, a changed one goes back to the index's text; an open document reads the file again.
func (e *Editor) diffDiscardFile(t *tab, i int) error {
	dt := t.diff
	if !dt.changes || i < 0 || i >= len(dt.lines) || dt.lines[i].sec < 0 || dt.lines[i].sec >= len(dt.secs) {
		return nil
	}
	defer e.bindDiff(dt)()
	s := dt.secs[dt.lines[i].sec]
	if s.group == changeStaged || s.group == changeConflict {
		return errors.New(i18n.T("Unstage staged changes first; resolve conflicts in the three-way merge"))
	}
	row := view.SCMRow{Path: s.path, Name: filepath.Base(s.path), Letter: byte(e.secLetter(&s))}
	title, label := i18n.F("Discard changes to %s? This can't be undone", row.Name), i18n.T("Discard Changes")
	switch {
	case s.group == changeUntracked:
		row.Letter = 'U'
		title, label = i18n.F("Move %s to %s?", row.Name, e.trashName()), i18n.T("Delete File")
	case s.baseOK && len(s.cur) == 0:
		title, label = i18n.F("Restore %s from the index?", row.Name), i18n.T("Restore File")
	}
	e.choose(title,
		ovItem{label: label, run: func(e *Editor) error { return e.discard([]view.SCMRow{row}) }},
		ovItem{label: i18n.T("Cancel"), run: func(*Editor) error { return nil }})
	return nil
}

// diffEditBeside opens the working tree's file of the diff tab t beside it, at the line of
// the caret's change, to edit there: the diff stays and reads the file again as it changes,
// and undo, redo and saving are the file's own (after Mayoi's editable diff side).
func (e *Editor) diffEditBeside(t *tab) error {
	dt := t.diff
	if dt.commit != nil || dt.fixed {
		return errors.New(i18n.T("The version in a commit is read-only"))
	}
	i := t.doc.buf.Snapshot().LineAt(t.doc.buf.Selections().Main().Head)
	if i < 0 || i >= len(dt.lines) {
		return nil
	}
	l := dt.lines[i]
	if dt.changes && (l.sec < 0 || l.sec >= len(dt.secs) || dt.secs[l.sec].group == changeStaged || dt.secs[l.sec].lazy) {
		return errors.New(i18n.T("Only the working tree side can be edited: unstage staged changes first"))
	}
	path := dt.pathOf(l)
	line := l.line
	if l.kind == diffRemoved || l.kind == diffFold || l.kind == diffFile || line < 0 {
		line = 0
		for j := i; j >= 0; j-- { // 这处改动在新文件里的那一行
			if h := dt.lines[j]; h.sec == l.sec && h.kind == diffHeader && h.line >= 0 {
				line = h.line
				break
			}
			if dt.lines[j].sec == l.sec && dt.lines[j].kind != diffRemoved && dt.lines[j].line >= 0 && dt.lines[j].kind != diffFile {
				line = dt.lines[j].line
				break
			}
		}
	}
	ci := slices.Index(e.cols, e.column())
	var c *column
	if ci+1 < len(e.cols) && !e.cols[ci+1].locked {
		c = e.cols[ci+1]
	} else {
		c = e.splitColumn(ci, false)
	}
	if err := e.openIn(c, c.active+1, path, false); err != nil {
		return err
	}
	e.GotoLine(line + 1)
	e.status = i18n.F("Editing %s alongside: the diff view follows it; saving and undo belong to that file", filepath.Base(path))
	return nil
}
