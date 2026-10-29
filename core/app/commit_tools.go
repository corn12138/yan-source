package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/text"
)

// Adapted from Mayoi (MIT), cmd/mayoi/commitactions.go, snapshot 8708da764770.
// Reads retain workspace and repository identity without invalidating status or history.
func (e *Editor) gitRead(label string, read func(*git.Repo) error, done func(*Editor) error) error {
	r := e.repository().git.repo
	if r == nil {
		return errors.New(i18n.T("Not in a Git repository"))
	}
	if e.repository().git.busy != "" {
		return errors.New(e.repository().git.busy)
	}
	e.repository().git.busy = label
	post := e.postGit()
	go func() {
		err := read(r)
		post(func(e *Editor) {
			if e.repository().git.repo != r {
				return
			}
			e.repository().git.busy = ""
			if err == nil && done != nil {
				err = done(e)
			}
			if err != nil {
				e.status = gitError(err)
			}
		})
	}()
	return nil
}

func (e *Editor) copyCommitHash(hash string) error {
	if e.host == nil {
		return errors.New(i18n.T("Clipboard unavailable"))
	}
	if err := e.host.SetClipboard(hash); err != nil {
		return err
	}
	e.status = i18n.F("Copied %s", git.ShortHash(hash))
	return nil
}

func (e *Editor) commitTarget(action func(*Editor, git.Commit) error) error {
	if t := e.cur(); e.area == areaEditor && t != nil && t.diff != nil {
		defer e.bindDiff(t.diff)()
	}
	if t := e.cur(); e.area == areaEditor && t != nil && t.diff != nil && t.diff.commit != nil {
		return action(e, *t.diff.commit)
	}
	if i, ok := e.scmSelectedCommit(); ok {
		return action(e, e.repository().scm.history.commits[i])
	}
	return errors.New(i18n.T("Select a commit in the history first, or open a commit tab"))
}

func (e *Editor) selectComparison(c git.Commit) error {
	e.repository().scm.compareRepo, e.repository().scm.compareSelected = e.repository().git.repo, c.Hash
	e.status = i18n.F("Selected comparison start %s", c.Short())
	return nil
}
func (e *Editor) compareSelectedCommit(c git.Commit) error {
	if e.repository().scm.compareRepo != e.repository().git.repo || e.repository().scm.compareSelected == "" {
		return errors.New(i18n.T("Choose \"Select as Comparison Start\" on another commit in the same repository first"))
	}
	return e.compareRevisions(e.repository().scm.compareSelected, c.Hash)
}

func (e *Editor) compareRevisions(from, to string) error {
	var a, b string
	return e.gitRead(i18n.T("Reading comparison…"), func(r *git.Repo) (err error) {
		a, err = r.ResolveCommit(from)
		if err == nil {
			b, err = r.ResolveCommit(to)
		}
		if err == nil && a == b {
			err = errors.New(i18n.T("Both ends are the same commit"))
		}
		return
	}, func(e *Editor) error { e.showHistoryTab(a, b, false); e.area = areaEditor; return nil })
}

func (e *Editor) commitToolItems(c git.Commit) []ovItem {
	items := []ovItem{
		{label: i18n.T("Export Patch…"), detail: i18n.F("%s · includes binary changes", c.Short()), run: func(e *Editor) error { return e.commitPatch(c, true) }},
		{label: i18n.T("Copy Patch"), detail: c.Short(), run: func(e *Editor) error { return e.commitPatch(c, false) }},
		{label: i18n.T("Compare HEAD → This Commit"), run: func(e *Editor) error { return e.compareRevisions("HEAD", c.Hash) }},
		{label: i18n.T("Compare This Commit → HEAD"), run: func(e *Editor) error { return e.compareRevisions(c.Hash, "HEAD") }},
		{label: i18n.T("Select as Comparison Start"), detail: c.Short(), run: func(e *Editor) error { return e.selectComparison(c) }},
		{label: i18n.T("Open Commit on Remote"), run: func(e *Editor) error { return e.commitRemote(c) }},
	}
	if e.repository().scm.compareRepo == e.repository().git.repo && e.repository().scm.compareSelected != "" && e.repository().scm.compareSelected != c.Hash {
		items = append(items, ovItem{label: i18n.T("Compare with Selected Start"), detail: git.ShortHash(e.repository().scm.compareSelected) + " → " + c.Short(), run: func(e *Editor) error { return e.compareSelectedCommit(c) }})
	}
	return items
}

func (e *Editor) commitPatch(c git.Commit, save bool) error {
	var patch string
	return e.gitRead(i18n.T("Reading patch…"), func(r *git.Repo) (err error) { patch, err = r.CommitPatch(c.Hash); return }, func(e *Editor) error {
		if !save {
			if e.host == nil {
				return errors.New(i18n.T("Clipboard unavailable"))
			}
			if err := e.host.SetClipboard(patch); err != nil {
				return err
			}
			e.status = i18n.F("Copied the patch of %s", c.Short())
			return nil
		}
		name := filepath.Join(e.repository().git.repo.Root, c.Short()+".patch")
		if h, ok := e.host.(interface {
			SavePatchDialog(string, func(string, error))
		}); ok {
			post, r := e.postGit(), e.repository().git.repo
			h.SavePatchDialog(name, func(path string, err error) {
				post(func(e *Editor) {
					if e.repository().git.repo != r {
						return
					}
					if err == nil && path != "" {
						err = e.writePatch(path, patch, true)
					}
					if err != nil {
						e.status = err.Error()
					}
				})
			})
			return nil
		}
		e.gitPrompt(i18n.T("Where to save the patch"), name, func(s string) string { return i18n.F("Save to %s", s) }, func(e *Editor, path string) error {
			path = strings.TrimSpace(path)
			if path == "" {
				return errors.New(i18n.T("The path can't be empty"))
			}
			if !filepath.IsAbs(path) {
				path = filepath.Join(e.repository().git.repo.Root, path)
			}
			return e.writePatch(path, patch, false)
		})
		return nil
	})
}

func (e *Editor) writePatch(path, patch string, replace bool) error {
	// Native save dialogs confirm replacement; the TUI's path prompt does not.
	if replace {
		if err := text.Save(text.FromString(patch, text.Meta{}), path); err != nil {
			return err
		}
	} else {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if os.IsExist(err) {
			return fmt.Errorf(i18n.T("The file already exists; choose another patch name: %w"), err)
		}
		if err != nil {
			return err
		}
		_, err = f.WriteString(patch)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			os.Remove(path)
			return err
		}
	}

	e.status = i18n.F("Patch saved to %s", path)
	return nil
}

func (e *Editor) commitRemote(c git.Commit) error {
	var link string
	return e.gitRead(i18n.T("Reading remote URL…"), func(r *git.Repo) (err error) { link, err = r.CommitRemoteURL(c); return }, func(e *Editor) error {
		if h, ok := e.host.(interface{ OpenURL(string) }); ok {
			h.OpenURL(link)
			return nil
		}
		if e.host == nil {
			return errors.New(i18n.T("Couldn't open the browser or copy the link"))
		}
		if err := e.host.SetClipboard(link); err != nil {
			return err
		}
		e.status = i18n.T("Commit link copied")
		return nil
	})
}

func (e *Editor) ComparisonStart() string {
	if e.repository().scm.compareRepo == e.repository().git.repo {
		return e.repository().scm.compareSelected
	}
	return ""
}

func (e *Editor) SCMCommitCommand(hash, command string) {
	e.sync()
	defer e.sync()
	// Status refreshes can move row indices while a context menu is open.
	for _, c := range e.repository().scm.history.commits {
		if c.Hash != hash {
			continue
		}
		e.unfoldSCM("commits")
		e.side, e.area, e.repository().scm.sel, e.repository().scm.focusMsg = sideSCM, areaTree, 0, false
		e.selectHistoryRow(hash, "")
		e.Run(command)
		return
	}
	e.status = i18n.T("Commit history refreshed; select the commit again")
}
