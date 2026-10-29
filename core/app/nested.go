package app

import (
	"path/filepath"
	"slices"

	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/view"
)

// Repository discovery follows Mayoi #49 (MIT). An explicit separate-project
// action delegates to the host lifecycle and preserves this workspace.
func (e *Editor) nestedFrame(dst *view.SCM) {
	dst.RepositoryLoading = !e.git.ready
	dst.RepositoryError = e.git.nestedError
	for _, path := range e.git.nested {
		name, _ := filepath.Rel(e.root, path)
		dst.Repositories = append(dst.Repositories, view.SCMRepository{Path: path, Name: filepath.ToSlash(name)})
	}
	e.scm.sel = max(0, min(e.scm.sel, len(dst.Repositories)-1))
	dst.Selected = e.scm.sel
}

// SCMOpenRepository takes a path from the displayed list, not a row index that
// may now name a different repository after a background rescan.
func (e *Editor) SCMOpenRepository(path string) {
	e.sync()
	defer e.sync()
	if err := e.openNestedRepository(path); err != nil {
		e.status = err.Error()
	}
}

func (e *Editor) openNestedRepository(path string) error {
	if !slices.Contains(e.git.nested, path) && e.repositoryForRoot(path) == nil {
		return userError(i18n.T("The repository list changed; select again"))
	}
	post := e.postHere()
	go func() {
		r, err := git.Open(path)
		post(func(e *Editor) {
			// A removed repository may now resolve to an ancestor; never open that
			// ancestor as if it were the project the user chose.
			if err != nil || filepath.Clean(r.Root) != filepath.Clean(path) {
				e.status = i18n.T("This repository is no longer available; refresh the repository list")
				e.gitChanged()
				return
			}
			if h, ok := e.host.(interface{ OpenRepository(string) error }); ok {
				if err := h.OpenRepository(path); err != nil {
					e.status = err.Error()
				}
			} else if err := e.SwitchFolder(path); err != nil {
				e.status = err.Error()
			} else {
				e.showSide(sideSCM)
			}
		})
	}()
	return nil
}
