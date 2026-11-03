package app

import (
	"slices"

	"github.com/corn12138/yan-source/core/view"
)

// Background status updates may insert rows before the selection. Preserve the
// file and index side so the next Space/Delete cannot silently target its neighbor.
func (e *Editor) keepSCMSelection() func() {
	sc := e.repository().scm
	before := scmFlat(e.scmGroups())
	selected, anchor := sc.sel, sc.anchor
	commit, inHistory := e.selectedHistoryRow()
	return func() {
		after := scmFlat(e.scmGroups())
		find := func(at int) int {
			if at >= 0 && at < len(before) {
				old := before[at]
				if i := slices.IndexFunc(after, func(r view.SCMRow) bool {
					return scmKey(r) == scmKey(old) && r.Folder == old.Folder
				}); i >= 0 {
					return i
				}
			}
			return max(0, min(at, len(after)-1))
		}
		sc.sel, sc.anchor = find(selected), find(anchor)
		if inHistory {
			path := ""
			if commit.File != nil {
				path = commit.File.Path
			}
			e.selectHistoryRow(commit.Hash, path)
		}
		if len(sc.picked) > 0 {
			present := make(map[string]bool, len(after))
			for _, row := range after {
				if !row.Folder {
					present[scmKey(row)] = true
				}
			}
			for key := range sc.picked {
				if !present[key] {
					delete(sc.picked, key)
				}
			}
		}
	}
}
