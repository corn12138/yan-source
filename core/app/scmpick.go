package app

// Choosing several files in Source Control, after Mayoi's SCM list: Shift-click (or Shift+↑↓)
// takes the files from the last one chosen to the one clicked, the platform's modifier-click
// adds or removes one, and Select All takes every file. Stage, unstage and discard then act on
// the files chosen, and only on them.

import (
	"slices"
	"strconv"

	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/view"
)

// scmKey names a row of Source Control: a file is in the staged group, or another one.
func scmKey(r view.SCMRow) string {
	if r.Staged {
		return "S" + r.Path
	}
	return "W" + r.Path
}

// scmPicked are the files chosen, in the list's order; nil when no more than the selected row is.
func (e *Editor) scmPicked() []view.SCMRow {
	sc := e.repository().scm
	if len(sc.picked) == 0 {
		return nil
	}
	var out []view.SCMRow
	for _, r := range scmFlat(e.scmGroups()) {
		if !r.Folder && sc.picked[scmKey(r)] {
			out = append(out, r)
		}
	}
	return out
}

// scmClearPicked goes back to one selected row.
func (e *Editor) scmClearPicked() {
	sc := e.repository().scm
	sc.picked, sc.anchor = nil, sc.sel
}

// SCMPick chooses row i with a modifier held: extend takes every file from the anchor to it,
// toggle adds or removes it alone.
func (e *Editor) SCMPick(i int, extend, toggle bool) {
	e.sync()
	defer e.sync()
	sc := e.repository().scm
	rows := scmFlat(e.scmGroups())
	if i < 0 || i >= len(rows) {
		return
	}
	e.side, e.area, sc.focusMsg = sideSCM, areaTree, false
	switch {
	case extend:
		a := max(min(sc.anchor, len(rows)-1), 0)
		lo, hi := min(a, i), max(a, i)
		sc.picked = map[string]bool{}
		for _, r := range rows[lo : hi+1] {
			if !r.Folder {
				sc.picked[scmKey(r)] = true
			}
		}
	case toggle:
		if sc.picked == nil {
			sc.picked = map[string]bool{}
			if sc.sel >= 0 && sc.sel < len(rows) && !rows[sc.sel].Folder {
				sc.picked[scmKey(rows[sc.sel])] = true
			}
		}
		if k := scmKey(rows[i]); !rows[i].Folder {
			if sc.picked[k] {
				delete(sc.picked, k)
			} else {
				sc.picked[k] = true
			}
		}
		sc.anchor = i
	}
	sc.sel = i
	e.status = scmPickedStatus(len(e.scmPicked()))
}

func scmPickedStatus(n int) string {
	if n <= 1 {
		return ""
	}
	return "选了 " + strconv.Itoa(n) + " 个文件：空格暂存或取消暂存，Delete 放弃修改"
}

// scmSelectAll chooses every file of Source Control.
func (e *Editor) scmSelectAll() error {
	if e.side != sideSCM {
		return nil
	}
	sc := e.repository().scm
	sc.picked = map[string]bool{}
	for _, r := range scmFlat(e.scmGroups()) {
		if !r.Folder {
			sc.picked[scmKey(r)] = true
		}
	}
	e.status = scmPickedStatus(len(sc.picked))
	return nil
}

// scmExtend moves the selection by one row with Shift held, choosing the files passed over.
func (e *Editor) scmExtend(by int) error {
	if e.side != sideSCM {
		return e.sideMove(by)
	}
	sc := e.repository().scm
	if sc.picked == nil {
		sc.anchor = sc.sel
	}
	e.SCMPick(sc.sel+by, true, false)
	return nil
}

// scmStagePicked stages the chosen files that are not staged and unstages the staged ones, in
// one Git operation so the two never contend for the index.
func (e *Editor) scmStagePicked(rows []view.SCMRow) error {
	var stage, unstage []string
	for _, r := range rows {
		if r.Staged {
			unstage = append(unstage, r.Path)
		} else {
			stage = append(stage, r.Path)
		}
	}
	if err := e.checkRepositoryPaths(append(slices.Clone(stage), unstage...)); err != nil {
		return err
	}
	if len(stage) > 0 {
		if err := e.saveDirtyInRepo(); err != nil {
			return err
		}
	}
	return e.gitDo("", func(r *git.Repo) error {
		if len(stage) > 0 {
			if err := r.Stage(stage...); err != nil {
				return err
			}
		}
		if len(unstage) > 0 {
			return r.Unstage(unstage...)
		}
		return nil
	}, nil)
}

// SCMPicked are the files chosen in Source Control (the frontends' menus act on them).
func (e *Editor) SCMPicked() []view.SCMRow { return e.scmPicked() }

// SCMStageRows stages the rows not staged and unstages the staged ones (a row's buttons, for
// the files chosen).
func (e *Editor) SCMStageRows(rows []view.SCMRow) {
	if err := e.scmStagePicked(rows); err != nil {
		e.status = err.Error()
	}
}
