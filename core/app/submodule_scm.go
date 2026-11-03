package app

import (
	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/i18n"
)

func submoduleState(en git.Entry, staged bool) (detail, tip string) {
	if staged || len(en.Sub) == 4 && en.Sub[1] == 'C' {
		detail = i18n.T("Commit reference")
		tip = i18n.T("The submodule commit reference changed; stage and commit it in this repository separately.")
	}
	// The staged row describes the parent's index, never the child's dirty files.
	if !staged && en.Dirty() {
		if detail != "" {
			detail += " · "
			tip += " "
		}
		detail += i18n.T("Content changes")
		tip += i18n.T("Commit files in the submodule first, then stage its updated reference here.")
	}
	return
}

// SCMOpenSubmodule keeps the workspace and parent diff in place. The path must
// still be a gitlink in the invoking repository, not an arbitrary nested folder.
func (e *Editor) SCMOpenSubmodule(path string) {
	e.sync()
	defer e.sync()
	parent := e.repository()
	child := e.repositoryForRoot(path)
	if !parent.git.entries[path].Submodule() || child == nil || child == parent {
		e.status = i18n.T("This submodule is not available; initialize it outside Yan, then refresh the repository list.")
		return
	}
	child.collapsed, child.expansionSet = false, true
	child.scm.focusMsg = false
	e.repositories.selected = child.git.repo.Root
	e.showSide(sideSCM)
}
