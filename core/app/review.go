package app

// Review interaction is adapted from Mayoi (MIT), cmd/mayoi/review.go.

import (
	"errors"
	"os"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/review"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
)

type reviewNote struct {
	review.Note
	id      int
	version uint64
}

type reviewState struct {
	notes   []*reviewNote
	draft   *reviewNote
	input   string
	undo    []*reviewNote
	nextID  int
	pending int
	gen     uint64
}

func selectedLines(t *tab) (int, int) {
	s := t.doc.buf.Snapshot()
	r := t.doc.buf.Selections().Main()
	lo, hi := s.LineAt(r.From()), s.LineAt(r.To())
	if hi > lo && r.To() == s.LineStart(hi) {
		hi--
	}
	return lo, hi
}

func snapshotLine(s text.Snapshot, i int) string { a, z := s.LineRange(i); return s.String(a, z) }

func (e *Editor) reviewExcerpts(t *tab) []review.Note {
	if t == nil || t.term != nil || t.image || t.svg || t.markdown {
		return nil
	}
	lo, hi := selectedLines(t)
	if t.diff == nil {
		if !t.doc.buf.Exists() {
			return nil
		}
		s := t.doc.buf.Snapshot()
		n := review.Note{Path: t.doc.buf.Path(), From: lo, To: hi + 1}
		for i := lo; i <= hi; i++ {
			n.Lines = append(n.Lines, snapshotLine(s, i))
		}
		n.Snippet = slices.Clone(n.Lines)
		return []review.Note{n}
	}
	dt := t.diff
	var out []review.Note
	// Select coordinates, not rendered text: split columns may clip lines or expand tabs.
	for at := lo; at <= hi && at < len(dt.lines); {
		sec := dt.lines[at].sec
		n, base, cur := e.reviewSource(dt, sec)
		old, newLines := map[int]bool{}, map[int]bool{}
		add := func(l diffLine) {
			if l.kind != diffSame && l.kind != diffAdded && l.kind != diffRemoved {
				return
			}
			if l.split {
				if l.left && l.leftGone {
					old[l.oldLine] = true
				}
				if l.right {
					newLines[l.line] = true
				}
			} else if l.kind == diffRemoved {
				old[l.line] = true
			} else {
				newLines[l.line] = true
			}
		}
		for ; at <= hi && at < len(dt.lines) && dt.lines[at].sec == sec; at++ {
			l := dt.lines[at]
			if l.kind == diffHeader && l.hunk >= 0 && l.line >= 0 {
				for j := at + 1; j < len(dt.lines) && dt.lines[j].sec == sec; j++ {
					if dt.lines[j].hunk != l.hunk {
						break
					}
					add(dt.lines[j])
				}
			}
			add(l)
		}
		if len(old)+len(newLines) == 0 {
			continue
		}
		// Recreate unified order, so inline and split layouts copy the same excerpt.
		var canonical []diffLine
		var words []string
		fold := 0
		expanded := map[int]bool{}
		hunks := e.diffHunks(base, cur)
		for i := 0; i <= len(hunks); i++ {
			expanded[i] = true
		}
		writeHunks(func(s string, l diffLine) { canonical = append(canonical, l); words = append(words, s) }, base, cur, hunks, sec, &fold, expanded, e.diffOpts(0))
		last := -1
		for i, l := range canonical {
			take, prefix := false, " "
			switch l.kind {
			case diffRemoved:
				take, prefix = old[l.line], "-"
			case diffAdded:
				take, prefix = newLines[l.line], "+"
			case diffSame:
				take = newLines[l.line]
			}
			if !take {
				continue
			}
			if last >= 0 && i > last+1 {
				n.Snippet = append(n.Snippet, "…")
			}
			n.Snippet = append(n.Snippet, prefix+words[i])
			last = i
		}
		lines, set := cur, newLines
		if len(newLines) == 0 {
			lines, set, n.OldSide = base, old, true
		}
		n.From, n.To = len(lines), 0
		for i := range set {
			if i >= 0 && i < len(lines) {
				n.From = min(n.From, i)
				n.To = max(n.To, i+1)
			}
		}
		if n.From >= n.To {
			continue
		}
		if !n.OldSide {
			n.Base = ""
		}
		n.Lines = slices.Clone(lines[n.From:n.To])
		n.Diff = true
		out = append(out, n)
	}
	return out
}

func (e *Editor) reviewContext(dt *diffTab, sec int) review.Note {
	n := review.Note{Repository: dt.repoRoot, Path: dt.path, Base: "HEAD", Commit: dt.commitHash}
	if dt.changes && sec >= 0 && sec < len(dt.secs) {
		s := dt.secs[sec]
		n.Path = s.path
		n.Staged = s.group == changeStaged
		if !n.Staged {
			n.Base = ""
		}
	}
	if dt.commit != nil {
		n.Commit = dt.commit.Hash
		if dt.compareFrom != nil {
			n.Commit = dt.compareFrom.Hash + ".." + dt.commit.Hash
		}
	}
	return n
}
func (e *Editor) reviewSource(dt *diffTab, sec int) (review.Note, []string, []string) {
	n := e.reviewContext(dt, sec)
	if dt.changes && sec >= 0 && sec < len(dt.secs) {
		s := dt.secs[sec]
		return n, s.base, s.cur
	}
	cur := dt.after
	if !dt.fixed {
		cur, _ = e.currentLines(dt.path)
	}
	return n, dt.base, cur
}

func (e *Editor) addReviewComment() error {
	t := e.cur()
	if t == nil {
		return errors.New(i18n.T("Open a file or changes first"))
	}
	if t.diff != nil {
		lo, _ := selectedLines(t)
		if lo < len(t.diff.lines) && t.diff.lines[lo].noteID != 0 {
			return e.editReview(t.diff.lines[lo].noteID)
		}
	}
	ns := e.reviewExcerpts(t)
	if len(ns) == 0 {
		return errors.New(i18n.T("Select lines in a saved file or a diff"))
	}
	n := ns[len(ns)-1]
	e.followReview(true)
	for _, old := range e.review.notes {
		if old.SameLocation(n) || !n.Diff && old.Commit == "" && !old.Staged && !old.OldSide && !old.Outdated && old.Path == n.Path && old.From <= n.From && old.To >= n.To {
			return e.editReview(old.id)
		}
	}
	e.review.nextID++
	e.review.draft = &reviewNote{Note: n, id: e.review.nextID}
	e.review.input = ""
	e.closeOverlay()
	return nil
}

func (e *Editor) editReview(id int) error {
	for _, n := range e.review.notes {
		if n.id == id {
			e.review.draft = n
			e.review.input = n.Text
			e.closeOverlay()
			return nil
		}
	}
	return errors.New(i18n.T("This comment no longer exists"))
}

func (e *Editor) EditReview(id int) {
	e.sync()
	defer e.sync()
	if err := e.editReview(id); err != nil {
		e.status = err.Error()
	}
}
func (e *Editor) SetReviewInput(s string) {
	if e.review.draft != nil {
		e.review.input = s
	}
}
func (e *Editor) cancelReview() error {
	e.review.draft = nil
	e.review.input = ""
	return nil
}
func (e *Editor) saveReview() error {
	n := e.review.draft
	if n == nil {
		return nil
	}
	value := strings.TrimSpace(e.review.input)
	if value == "" {
		return errors.New(i18n.T("The comment can't be empty"))
	}
	n.Text = value
	if !slices.Contains(e.review.notes, n) {
		e.review.notes = append(e.review.notes, n)
	}
	e.cancelReview()
	e.reviewChanged()
	return nil
}
func (e *Editor) reviewChanged() { e.review.gen++; e.SaveSession() }

func (e *Editor) DeleteReview(id int) {
	for i, n := range e.review.notes {
		if n.id == id {
			e.review.undo = slices.Clone(e.review.notes)
			e.review.notes = slices.Delete(e.review.notes, i, i+1)
			if e.review.draft == n {
				e.cancelReview()
			}
			e.reviewChanged()
			e.tell(i18n.T("Deleted review comment"), i18n.T("Undo"), "review.undo")
			return
		}
	}
}
func (e *Editor) deleteReview() error {
	if e.review.draft != nil {
		e.DeleteReview(e.review.draft.id)
		e.cancelReview()
		return nil
	}
	if t := e.cur(); t != nil && t.diff != nil {
		i, _ := selectedLines(t)
		if i < len(t.diff.lines) {
			e.DeleteReview(t.diff.lines[i].noteID)
		}
	}
	return nil
}
func (e *Editor) clearReview() error {
	if len(e.review.notes) == 0 {
		return nil
	}
	e.review.undo = slices.Clone(e.review.notes)
	e.review.notes = nil
	e.cancelReview()
	e.reviewChanged()
	e.tell(i18n.T("Cleared review comments"), i18n.T("Undo"), "review.undo")
	return nil
}
func (e *Editor) undoReview() error {
	if e.review.undo != nil {
		restored := slices.Clone(e.review.undo)
		for _, n := range e.review.notes {
			if !slices.Contains(restored, n) {
				restored = append(restored, n)
			}
		}
		e.review.notes, e.review.undo = restored, nil
		e.reviewChanged()
	}
	return nil
}

func (e *Editor) followReview(disk bool) {
	changed := false
	for _, n := range e.review.notes {
		if n.Commit != "" {
			continue
		}
		from, outdated := n.From, n.Outdated
		if !n.Staged && !n.OldSide {
			if d := e.docOf(n.Path); d != nil {
				s := d.buf.Snapshot()
				if n.version == s.Version() {
					continue
				}
				n.version = s.Version()
				n.Follow(s.LineCount(), func(i int) string { return snapshotLine(s, i) })
			} else if disk {
				b, err := os.ReadFile(n.Path)
				if err != nil {
					n.Outdated = true
				} else {
					ls := splitText(string(b))
					n.Follow(len(ls), func(i int) string { return ls[i] })
				}
			}
		} else if rc := e.reviewRepository(n.Note); disk && rc != nil {
			rev := ""
			if n.OldSide {
				rev = n.Base
			}
			s, ok, err := rc.git.repo.Show(rev, n.Path)
			if err != nil || !ok {
				n.Outdated = true
			} else {
				ls := splitText(s)
				n.Follow(len(ls), func(i int) string { return ls[i] })
			}
		}
		changed = changed || from != n.From || outdated != n.Outdated
	}
	if changed {
		e.review.gen++
	}
}

func (e *Editor) reviewNotes() []review.Note {
	out := make([]review.Note, 0, len(e.review.notes))
	for _, n := range e.review.notes {
		out = append(out, n.Note)
	}
	return out
}
func (e *Editor) restoreReview(ns []review.Note) {
	e.review = reviewState{}
	for _, n := range ns {
		if n.Valid() {
			e.review.nextID++
			e.review.notes = append(e.review.notes, &reviewNote{Note: n, id: e.review.nextID})
		}
	}
	e.review.gen++
}
func (e *Editor) reviewText() string {
	e.followReview(true)
	return review.Format(e.reviewNotes(), e.root)
}
func (e *Editor) copyReview() error {
	if len(e.review.notes) == 0 {
		return errors.New(i18n.T("No review comments yet"))
	}
	if err := e.host.SetClipboard(e.reviewText()); err != nil {
		return err
	}
	e.status = i18n.T("Copied review comments")
	return nil
}
func (e *Editor) copyReviewExcerpt() error {
	ns := e.reviewExcerpts(e.cur())
	if len(ns) == 0 {
		return errors.New(i18n.T("Select lines first"))
	}
	return e.host.SetClipboard(review.Format(ns, e.root))
}

// PasteReview is optional: TUI hosts use the clipboard. A frontend returns false
// if no live terminal exists, and an error without claiming success if delivery fails.
func (e *Editor) sendReview() error {
	if len(e.review.notes) == 0 {
		return errors.New(i18n.T("No review comments yet"))
	}
	value := e.reviewText()
	if h, ok := e.host.(interface{ PasteReview(string) (bool, error) }); ok {
		sent, err := h.PasteReview(value)
		if err != nil {
			return err
		}
		if sent {
			e.clearReview()
			e.tell(i18n.T("Review comments pasted into the terminal; check them before submitting"), i18n.T("Undo Clear"), "review.undo")
			return nil
		}
	}
	if err := e.host.SetClipboard(value); err != nil {
		return err
	}
	e.status = i18n.T("Not sent to the terminal: the comments were copied and are still kept in the project")
	return nil
}

func (e *Editor) reviewList() error {
	e.followReview(true)
	var items []ovItem
	for _, n := range e.review.notes {
		items = append(items, ovItem{label: n.Ref(e.root), detail: strings.ReplaceAll(n.Text, "\n", " "), run: func(e *Editor) error { return e.goToReview(n) }})
	}
	e.choose(i18n.T("Review Comments"), items...)
	return nil
}
func (e *Editor) stepReview(by int) error {
	if len(e.review.notes) == 0 {
		return errors.New(i18n.T("No review comments yet"))
	}
	e.followReview(true)
	ns := slices.Clone(e.review.notes)
	slices.SortStableFunc(ns, func(a, b *reviewNote) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return a.From - b.From
	})
	at := -1
	if t := e.cur(); t != nil {
		lo, _ := selectedLines(t)
		for i, n := range ns {
			if t.diff != nil {
				if lo < len(t.diff.lines) && t.diff.lines[lo].noteID == n.id {
					at = i
					break
				}
			} else if n.Path == t.doc.buf.Path() && n.From <= lo {
				at = i
			}
		}
	}
	if at < 0 && by < 0 {
		at = 0
	}
	return e.goToReview(ns[(at+by+len(ns))%len(ns)])
}
func (e *Editor) goToReview(n *reviewNote) error {
	defer e.bindRepository(e.reviewRepository(n.Note))()
	if !n.Staged && !n.OldSide && n.Commit == "" {
		if err := e.OpenFile(n.Path); err != nil {
			return e.editReview(n.id) // A removed source file must not make its saved quote inaccessible.
		}
		e.GotoLine(n.From + 1)
		return e.editReview(n.id)
	}
	for _, c := range e.cols {
		for i, t := range c.tabs {
			if t.diff != nil {
				for line, l := range t.diff.lines {
					if l.noteID == n.id {
						e.activate(c, i)
						e.diffGo(t, line)
						return e.editReview(n.id)
					}
				}
			}
		}
	}
	if e.repository().git.repo == nil {
		return e.editReview(n.id)
	}
	if n.Commit != "" {
		if from, to, ok := strings.Cut(n.Commit, ".."); ok {
			e.showHistoryTab(from, to, false)
			e.review.pending = n.id
			return nil
		}
		e.showHistoryTab("", n.Commit, false)
	} else {
		if err := e.openAllChanges(); err != nil {
			return err
		}
	}
	e.review.pending = n.id
	return nil
}

func (e *Editor) revealReview() {
	id := e.review.pending
	if id == 0 {
		return
	}
	t := e.cur()
	if t == nil || t.diff == nil {
		e.review.pending = 0
		return
	}
	if t.diff.loading {
		return
	}
	for i, l := range t.diff.lines {
		if l.noteID == id {
			e.diffGo(t, i)
			break
		}
	}
	e.review.pending = 0
	e.editReview(id)
}

func (e *Editor) reviewKey(c keymap.Chord) bool {
	if e.review.draft == nil {
		return false
	}
	switch {
	case c.Key == keymap.KeyEsc:
		e.cancelReview()
	case (c.Key == keymap.KeyEnter || c.Key == keymap.KeyRune && c.Rune == 's') && c.Mods&(keymap.ModCtrl|keymap.ModSuper) != 0:
		if err := e.saveReview(); err != nil {
			e.status = err.Error()
		}
	case c.Key == keymap.KeyEnter:
		e.review.input += "\n"
	case c.Key == keymap.KeyBackspace:
		if s := e.review.input; s != "" {
			_, n := utf8.DecodeLastRuneInString(s)
			e.review.input = s[:len(s)-n]
		}
	case c.Key == keymap.KeyRune && c.Mods&(keymap.ModCtrl|keymap.ModSuper) != 0 && c.Rune == 'v':
		if s, err := e.host.Clipboard(); err == nil {
			e.review.input += s
		}
	}
	return true
}

func (e *Editor) reviewFrame(dst *view.Frame) {
	dst.Review = nil
	dst.ReviewCount = len(e.review.notes)
	if n := e.review.draft; n != nil {
		dst.Review = &view.ReviewDraft{ID: n.id, Ref: n.Ref(e.root), Text: e.review.input, Snippet: strings.Join(n.Snippet, "\n"), Saved: slices.Contains(e.review.notes, n)}
	}
}

func (e *Editor) markReview(t *tab, row *view.RowFrame, line int) {
	if t.diff != nil {
		if line < len(t.diff.lines) {
			row.ReviewID = t.diff.lines[line].noteID
		}
		return
	}
	for _, n := range e.review.notes {
		if n.Path == t.doc.buf.Path() && n.Commit == "" && !n.Staged && !n.OldSide && !n.Outdated && line >= n.From && line < n.To {
			row.ReviewID = n.id
			if line == n.To-1 {
				row.Ghost = i18n.F("Comment: %s", strings.Split(n.Text, "\n")[0])
			}
		}
	}
}

func (e *Editor) reviewDiffText(dt *diffTab, body string) string {
	dt.reviewGen = e.review.gen
	if len(e.review.notes) == 0 {
		return body
	}
	source := strings.Split(body, "\n")
	original := slices.Clone(dt.lines)
	after := map[int][]*reviewNote{}
	for _, n := range e.review.notes {
		last, header := -1, -1
		for i, l := range original {
			context := e.reviewContext(dt, l.sec)
			if n.Repository != "" && context.Repository != n.Repository || context.Path != n.Path || context.Commit != n.Commit || context.Staged != n.Staged || n.OldSide && context.Base != n.Base {
				continue
			}
			if l.kind == diffFile {
				header = i
			}
			ln, valid := l.line, l.kind == diffSame || l.kind == diffAdded
			if n.OldSide {
				if l.split {
					ln, valid = l.oldLine, l.left && l.leftGone
				} else {
					valid = l.kind == diffRemoved
				}
			}
			if valid && ln >= n.From && ln < n.To {
				last = i
			}
		}
		if last < 0 {
			last = header
		}
		if last >= 0 {
			after[last] = append(after[last], n)
		}
	}
	var out strings.Builder
	dt.lines = dt.lines[:0]
	put := func(s string, l diffLine) {
		if out.Len() > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(s)
		dt.lines = append(dt.lines, l)
	}
	for i, l := range original {
		put(source[i], l)
		for _, n := range after[i] {
			card := diffLine{kind: diffReview, line: -1, sec: l.sec, hunk: -1, noteID: n.id}
			put(i18n.F("┌ Comment · %s", n.Ref(e.root)), card)
			if n.Outdated {
				for _, q := range n.Snippet {
					put("│ "+q, card)
				}
			}
			for _, s := range strings.Split(n.Text, "\n") {
				put("│ "+s, card)
			}
			put(i18n.T("└ c Edit · Delete Remove"), card)
		}
	}
	return out.String()
}

// ReviewLine is the gutter entry point. A click inside a selection keeps its range.
func (e *Editor) ReviewLine(col, line int) {
	e.FocusColumn(col)
	defer e.sync()
	t := e.cur()
	if t == nil {
		return
	}
	lo, hi := selectedLines(t)
	if line < lo || line > hi {
		s := t.doc.buf.Snapshot()
		t.doc.buf.SetSelections(edit.Single(edit.Cursor(s.LineStart(line))))
	}
	if err := e.addReviewComment(); err != nil {
		e.status = err.Error()
	}
}

// Stored ownership disambiguates a parent gitlink from the child worktree at the
// same path. Older notes can still recover ownership from their absolute file path.
func (e *Editor) reviewRepository(n review.Note) *repositoryContext {
	if n.Repository != "" {
		return e.repositoryForRoot(n.Repository)
	}
	return e.repositoryForPath(n.Path)
}
