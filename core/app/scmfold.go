package app

// Folding Source Control's sections: a click on a group's title (merge conflicts, staged
// changes, changes, untracked files) or on the commits' title folds it, and the choice is kept
// for the next start. A folded group still counts its files, and acting on the whole group
// (stage all, discard all, the next changed file) still takes every file in it.

import (
	"slices"

	"github.com/corn12138/yan-source/core/view"
)

// SCMSectionKey names a group of Source Control for folding (SCMToggleSection).
func SCMSectionKey(k view.SCMGroupKind) string {
	switch k {
	case view.GroupMerge:
		return "merge"
	case view.GroupStaged:
		return "staged"
	case view.GroupUntracked:
		return "untracked"
	}
	return "changes"
}

// scmFileCount counts the files of a group's rows (a tree's folder rows are not files).
func scmFileCount(rows []view.SCMRow) int {
	n := 0
	for _, r := range rows {
		if !r.Folder {
			n++
		}
	}
	return n
}

// shownCommits are the history's rows Source Control shows: none while the commits are folded.
func (e *Editor) shownCommits() []view.SCMCommit {
	if e.scmFolded["commits"] {
		return nil
	}
	return e.expandedCommitRows()
}

// SCMToggleSection folds or unfolds a section of Source Control: a group's key
// (SCMSectionKey) or "commits". The selected row stays selected when it is still shown.
func (e *Editor) SCMToggleSection(key string) {
	e.sync()
	defer e.sync()
	sc := e.repository().scm
	var keep *view.SCMRow
	commit := -1
	if r, ok := e.scmSelected(); ok {
		keep = &r
	} else if i, ok := e.scmSelectedCommit(); ok {
		commit = i
	}
	if e.scmFolded == nil {
		e.scmFolded = map[string]bool{}
	}
	e.scmFolded[key] = !e.scmFolded[key]
	e.keepSCMFolded()
	rows := scmFlat(e.scmGroups())
	sc.sel = 0
	switch {
	case keep != nil:
		if i := slices.IndexFunc(rows, func(r view.SCMRow) bool { return scmKey(r) == scmKey(*keep) && r.Folder == keep.Folder }); i >= 0 {
			sc.sel = i
		}
	case commit >= 0 && commit < len(e.repository().scm.history.commits):
		e.selectHistoryRow(e.repository().scm.history.commits[commit].Hash, "")
	}
	sc.anchor = sc.sel
}

// unfoldSCM unfolds a section about to be selected in.
func (e *Editor) unfoldSCM(key string) {
	if e.scmFolded[key] {
		e.scmFolded[key] = false
		e.keepSCMFolded()
	}
}

// keepSCMFolded keeps the folded sections for the next start.
func (e *Editor) keepSCMFolded() {
	if e.state == nil {
		return
	}
	e.state.SCMFolded = e.state.SCMFolded[:0]
	for _, k := range []string{"merge", "staged", "changes", "untracked", "commits"} {
		if e.scmFolded[k] {
			e.state.SCMFolded = append(e.state.SCMFolded, k)
		}
	}
	e.saveState()
}

// SCMGroupFiles are every file of a group, folded or not, as a list: what discarding the whole
// group asks about.
func (e *Editor) SCMGroupFiles(kind view.SCMGroupKind) []view.SCMRow {
	tree := e.repository().scm.tree
	e.repository().scm.tree = false
	defer func() { e.repository().scm.tree = tree }()
	for _, g := range e.scmAllGroups() {
		if g.Kind == kind {
			return g.Rows
		}
	}
	return nil
}
