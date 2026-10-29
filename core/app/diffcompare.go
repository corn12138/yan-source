package app

// Whitespace filtering and comparing any two files in the diffs, after the behaviour of
// Mayoi PR #50 (Garfielk, https://github.com/sorrycc/Mayoi/pull/50, unmerged): its
// compareDiffLines, withoutBlankHunks and Compare Two Files, rewritten for yan's diff tabs.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/git/linediff"
	"github.com/corn12138/yan-source/core/i18n"
)

// diffHunks are the changes from base to cur as the diffs show them: compared as the
// whitespace filter says, with the hunks still indexing the original lines, so that what is
// shown, counted, gone to and staged is one list.
func (e *Editor) diffHunks(base, cur []string) []linediff.Hunk {
	return whitespaceHunks(base, cur, e.cfg.DiffWhitespace)
}

func whitespaceHunks(base, cur []string, mode string) []linediff.Hunk {
	if mode == "" {
		return linediff.Hunks(base, cur)
	}
	hs := linediff.Hunks(whitespaceKeys(base, mode), whitespaceKeys(cur, mode))
	if mode == "blank" {
		hs = withoutBlankHunks(hs, base, cur)
	}
	return hs
}

// whitespaceKeys are the lines as the filter compares them.
func whitespaceKeys(lines []string, mode string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		switch mode {
		case "eol":
			l = strings.TrimRight(l, " \t")
		case "all", "blank":
			l = strings.Map(func(r rune) rune {
				if r == ' ' || r == '\t' || r == ' ' {
					return -1
				}
				return r
			}, l)
		}
		out[i] = l
	}
	return out
}

// withoutBlankHunks drops the changes made only of blank lines.
func withoutBlankHunks(hs []linediff.Hunk, base, cur []string) []linediff.Hunk {
	blank := func(lines []string) bool {
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				return false
			}
		}
		return true
	}
	out := hs[:0]
	for _, h := range hs {
		if !blank(base[h.A:min(h.A+h.ALen, len(base))]) || !blank(cur[h.B:min(h.B+h.BLen, len(cur))]) {
			out = append(out, h)
		}
	}
	return out
}

var diffWhitespaceLabels = []struct{ mode, label string }{
	{"", i18n.N("Compare All Characters")},
	{"eol", i18n.N("Ignore Trailing Whitespace")},
	{"all", i18n.N("Ignore All Whitespace")},
	{"blank", i18n.N("Ignore Whitespace and Blank Lines")},
}

func diffWhitespaceLabel(mode string) string {
	for _, w := range diffWhitespaceLabels {
		if w.mode == mode {
			return i18n.T(w.label)
		}
	}
	return mode
}

// chooseDiffWhitespace asks which whitespace the diffs compare.
func (e *Editor) chooseDiffWhitespace() error {
	var items []ovItem
	for _, w := range diffWhitespaceLabels {
		item := ovItem{label: i18n.T(w.label), run: func(e *Editor) error { return e.setDiffWhitespace(w.mode) }}
		if w.mode == e.cfg.DiffWhitespace {
			item.hint = i18n.T("Current")
		}
		items = append(items, item)
	}
	e.choose(i18n.T("Whitespace in Diff Views"), items...)
	return nil
}

// setDiffWhitespace sets the whitespace filter, keeps it for the next start and reads the open
// diffs again.
func (e *Editor) setDiffWhitespace(mode string) error {
	mode, ok := config.DiffWhitespaceMode(mode)
	if !ok {
		return errors.New(i18n.T("Unknown whitespace comparison"))
	}
	e.cfg.DiffWhitespace = mode
	if e.state != nil {
		v := mode
		e.state.DiffWhitespace = &v
		e.saveState()
	}
	e.refreshAllDiffs()
	e.status = i18n.F("Diff view: %s", diffWhitespaceLabel(mode))
	return nil
}

func (e *Editor) refreshAllDiffs() {
	for _, c := range e.cols {
		for _, t := range c.tabs {
			if t.diff != nil {
				e.refreshDiff(t.doc, t.diff)
			}
		}
	}
}

// CompareFiles opens a diff of two local files, whether or not either is in a repository:
// left is the old side, right the new. Open documents count with their unsaved text, and
// neither file is staged or changed by it.
func (e *Editor) CompareFiles(left, right string) error {
	left, right = filepath.Clean(left), filepath.Clean(right)
	if left == right {
		return errors.New(i18n.T("Choose two different files to compare"))
	}
	for _, p := range []string{left, right} {
		if e.docOf(p) != nil {
			continue
		}
		if fi, err := os.Stat(p); err != nil {
			return err
		} else if fi.IsDir() {
			return errors.New(i18n.F("%s is a folder: choose two files to compare", filepath.Base(p)))
		}
	}
	ln, rn := filepath.Base(left), filepath.Base(right)
	if ln == rn {
		ln, rn = filepath.Join(filepath.Base(filepath.Dir(left)), ln), filepath.Join(filepath.Base(filepath.Dir(right)), rn)
	}
	dt := &diffTab{path: right, peer: left, expanded: map[int]bool{}}
	e.showDiff(ln+" ↔ "+rn, dt, false)
	return nil
}

// compareFiles is the command: the window chooses the files with the system's dialog,
// elsewhere two prompts ask for their paths, starting from the file being edited.
func (e *Editor) compareFiles() error {
	dir := e.root
	cur := ""
	if t := e.cur(); t != nil && t.diff == nil && t.term == nil && t.doc.buf.Exists() {
		cur = t.doc.buf.Path()
		dir = filepath.Dir(cur)
	}
	if h, ok := e.host.(interface{ CompareFilesDialog(dir string) }); ok {
		h.CompareFilesDialog(dir)
		return nil
	}
	hint := func(s string) string {
		return i18n.F("Compare %s (relative paths start at the open folder)", strings.TrimSpace(s))
	}
	abs := func(p string) string {
		if p = strings.TrimSpace(p); p != "" && !filepath.IsAbs(p) {
			p = filepath.Join(e.root, p)
		}
		return p
	}
	e.prompt(i18n.T("Compare: left (old) file"), cur, hint, func(e *Editor, left string) error {
		left = abs(left)
		e.prompt(i18n.T("Compare: right (new) file"), "", hint, func(e *Editor, right string) error {
			return e.CompareFiles(left, abs(right))
		})
		return nil
	})
	return nil
}

// selectForCompare remembers the explorer's file as the left side of the next comparison.
func (e *Editor) selectForCompare() error {
	path, _, ok := e.treeTarget()
	if !ok || path == e.root {
		return errors.New(i18n.T("Select a file in the Explorer first"))
	}
	if fi, err := os.Stat(path); err != nil || fi.IsDir() {
		return errors.New(i18n.T("Choose a file, not a folder"))
	}
	e.compareLeft = path
	e.status = i18n.F("Selected %s as the left side: choose \"Compare with Selected\" on another file", filepath.Base(path))
	return nil
}

// compareWithSelected compares the file chosen with selectForCompare with the explorer's file.
func (e *Editor) compareWithSelected() error {
	if e.compareLeft == "" {
		return errors.New(i18n.T("Choose \"Select for Compare\" on a file first"))
	}
	path, _, ok := e.treeTarget()
	if !ok || path == e.root {
		return errors.New(i18n.T("Select a file in the Explorer first"))
	}
	return e.CompareFiles(e.compareLeft, path)
}

// CanCompareWithSelected reports whether a file is chosen as the left side of a comparison.
func (e *Editor) CanCompareWithSelected() bool { return e.compareLeft != "" }

var diffHighlightLabels = []struct{ mode, label string }{
	{"", i18n.N("Highlight Changed Words")},
	{"char", i18n.N("Highlight Changed Characters")},
	{"line", i18n.N("Highlight Changed Lines Only")},
}

// chooseDiffHighlight asks how changed lines show what changed within them.
func (e *Editor) chooseDiffHighlight() error {
	var items []ovItem
	for _, h := range diffHighlightLabels {
		item := ovItem{label: i18n.T(h.label), run: func(e *Editor) error {
			e.cfg.DiffHighlight = h.mode
			if e.state != nil {
				v := h.mode
				e.state.DiffHighlight = &v
				e.saveState()
			}
			e.refreshAllDiffs()
			e.status = i18n.F("Diff view: %s", i18n.T(h.label))
			return nil
		}}
		if h.mode == e.cfg.DiffHighlight {
			item.hint = i18n.T("Current")
		}
		items = append(items, item)
	}
	e.choose(i18n.T("How diff views highlight changes"), items...)
	return nil
}

// chooseDiffContext asks how many unchanged lines stay around each change.
func (e *Editor) chooseDiffContext() error {
	var items []ovItem
	for n := 0; n <= config.MaxDiffContext; n++ {
		item := ovItem{label: i18n.F("Keep %d lines around changes", n), run: func(e *Editor) error {
			e.cfg.DiffContext = n
			if e.state != nil {
				v := n
				e.state.DiffContext = &v
				e.saveState()
			}
			for _, c := range e.cols {
				for _, t := range c.tabs {
					if t.diff != nil {
						clear(t.diff.expanded) // 折叠的编号跟着变了
					}
				}
			}
			e.refreshAllDiffs()
			e.status = i18n.F("Diff view: keep %d unchanged lines around changes", n)
			return nil
		}}
		if n == e.cfg.DiffContext {
			item.hint = i18n.T("Current")
		}
		items = append(items, item)
	}
	e.choose(i18n.T("Context lines of diff views"), items...)
	return nil
}
