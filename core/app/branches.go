package app

import (
	"errors"
	"strings"

	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/i18n"
)

// Branch and remote management (GI-27). Every command acts on the repository the Source Control
// view has chosen, lists come from a background read, and git runs through gitDo, so a
// runCommands chain waits for it and a failure stops the chain.

// loadThen reads what a command chooses from in the background, then runs show on the owner.
func loadThen[T any](e *Editor, read func(r *git.Repo) (T, error), show func(e *Editor, v T)) error {
	r := e.repository().git.repo
	if r == nil {
		return errors.New(i18n.T("Not in a Git repository"))
	}
	post := e.postGit()
	go func() {
		v, err := read(r)
		post(func(e *Editor) {
			if err != nil {
				e.status = gitError(err)
				return
			}
			show(e, v)
		})
	}()
	return nil
}

// renameBranch renames the current branch.
func (e *Editor) renameBranch() error {
	cur := e.branch()
	if cur == "" {
		return errors.New(i18n.T("HEAD is not on a branch: switch to the branch you want to rename first"))
	}
	e.prompt(i18n.F("Rename branch %s to", cur), cur, func(s string) string { return i18n.F("Rename to %s", strings.TrimSpace(s)) }, func(e *Editor, name string) error {
		name = strings.TrimSpace(name)
		if name == "" || name == cur {
			return nil
		}
		return e.gitDo(i18n.T("Renaming branch…"), func(r *git.Repo) error { return r.RenameBranch(cur, name) }, func(e *Editor) {
			e.status = i18n.F("Branch %s renamed to %s", cur, name)
		})
	})
	return nil
}

// deleteBranch deletes a local branch other than the current one. Git refuses a branch with
// commits merged nowhere else; deleting it anyway is a second, explicit choice.
func (e *Editor) deleteBranch() error {
	return loadThen(e, (*git.Repo).BranchList, func(e *Editor, bs []git.Branch) {
		cur := e.branch()
		var items []ovItem
		for _, b := range bs {
			if b.Remote || b.Name == cur {
				continue
			}
			name := b.Name
			items = append(items, ovItem{label: name, detail: b.Subject, run: func(e *Editor) error { return e.deleteBranchNamed(name, false) }})
		}
		if len(items) == 0 {
			e.status = i18n.T("There are no local branches besides the current one")
			return
		}
		e.gitChoose(i18n.T("Delete which local branch (unmerged branches ask again)"), items...)
	})
}

func (e *Editor) deleteBranchNamed(name string, force bool) error {
	notMerged := false
	return e.gitDo(i18n.T("Deleting branch…"), func(r *git.Repo) error {
		err := r.DeleteBranch(name, force)
		if errors.Is(err, git.ErrNotMerged) {
			notMerged = true
			return nil
		}
		return err
	}, func(e *Editor) {
		if notMerged {
			e.gitChoose(i18n.F("Branch %s has commits not merged anywhere else; deleting it loses them", name),
				ovItem{label: i18n.T("Keep Branch"), run: func(*Editor) error { return nil }},
				ovItem{label: i18n.F("Delete %s Anyway", name), run: func(e *Editor) error { return e.deleteBranchNamed(name, true) }})
			return
		}
		e.status = i18n.F("Deleted branch %s", name)
	})
}

// addRemote asks for a remote's name, then its URL.
func (e *Editor) addRemote() error {
	if e.repository().git.repo == nil {
		return errors.New(i18n.T("Not in a Git repository"))
	}
	e.prompt(i18n.T("Name of the new remote"), "origin", func(s string) string { return i18n.F("Name: %s", strings.TrimSpace(s)) }, func(e *Editor, name string) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil
		}
		e.prompt(i18n.F("URL of remote %s", name), "", func(s string) string { return i18n.F("Add %s: %s", name, strings.TrimSpace(s)) }, func(e *Editor, url string) error {
			url = strings.TrimSpace(url)
			if url == "" {
				return nil
			}
			return e.gitDo(i18n.T("Adding remote…"), func(r *git.Repo) error { return r.AddRemote(name, url) }, func(e *Editor) {
				e.status = i18n.F("Added remote %s", name)
			})
		})
		return nil
	})
	return nil
}

// chooseRemote lists the remotes, then runs act on the one chosen.
func (e *Editor) chooseRemote(title string, act func(e *Editor, rm git.Remote) error) error {
	return loadThen(e, (*git.Repo).RemoteList, func(e *Editor, rs []git.Remote) {
		if len(rs) == 0 {
			e.status = i18n.T("This repository has no remotes")
			return
		}
		items := make([]ovItem, 0, len(rs))
		for _, rm := range rs {
			items = append(items, ovItem{label: rm.Name, detail: rm.URL, run: func(e *Editor) error { return act(e, rm) }})
		}
		e.gitChoose(title, items...)
	})
}

func (e *Editor) removeRemote() error {
	return e.chooseRemote(i18n.T("Delete which remote"), (*Editor).removeRemoteNamed)
}

func (e *Editor) removeRemoteNamed(rm git.Remote) error {
	{
		e.gitChoose(i18n.F("Delete remote %s? Its remote-tracking branches are deleted too; local branches are not affected", rm.Name),
			ovItem{label: i18n.T("Cancel"), run: func(*Editor) error { return nil }},
			ovItem{label: i18n.F("Delete %s", rm.Name), run: func(e *Editor) error {
				return e.gitDo(i18n.T("Deleting remote…"), func(r *git.Repo) error { return r.RemoveRemote(rm.Name) }, func(e *Editor) {
					e.status = i18n.F("Deleted remote %s", rm.Name)
				})
			}})
		return nil
	}
}

func (e *Editor) setRemoteURL() error {
	return e.chooseRemote(i18n.T("Change the URL of which remote"), (*Editor).setRemoteURLOf)
}

func (e *Editor) setRemoteURLOf(rm git.Remote) error {
	{
		e.prompt(i18n.F("New URL of remote %s", rm.Name), rm.URL, func(s string) string { return i18n.F("Change to %s", strings.TrimSpace(s)) }, func(e *Editor, url string) error {
			url = strings.TrimSpace(url)
			if url == "" || url == rm.URL {
				return nil
			}
			return e.gitDo(i18n.T("Changing remote URL…"), func(r *git.Repo) error { return r.SetRemoteURL(rm.Name, url) }, func(e *Editor) {
				e.status = i18n.F("Remote %s changed to %s", rm.Name, url)
			})
		})
		return nil
	}
}

func (e *Editor) pruneRemote() error {
	return e.chooseRemote(i18n.T("Prune deleted branches of which remote"), func(e *Editor, rm git.Remote) error { return e.pruneRemoteNamed(rm.Name) })
}

func (e *Editor) pruneRemoteNamed(name string) error {
	return e.gitDo(i18n.T("Pruning remote branches…"), func(r *git.Repo) error { return r.PruneRemote(name) }, func(e *Editor) {
		e.status = i18n.F("Pruned remote-tracking branches that no longer exist on %s", name)
	})
}

// branchNameProblem says what is wrong with name as a new local branch's, "" for nothing:
// Git's rules for ref names, and a branch of that name already there.
func branchNameProblem(name string, branches []git.Branch) string {
	switch {
	case name == "":
		return i18n.T("Enter a branch name")
	case strings.HasPrefix(name, "-") || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") || strings.HasSuffix(name, "."):
		return i18n.T("A branch name cannot start with - or / or end with / or .")
	case strings.ContainsAny(name, " ~^:?*[\\\x7f") || strings.Contains(name, "..") || strings.Contains(name, "@{") || strings.Contains(name, "//") || name == "@":
		return i18n.T("A branch name cannot contain spaces, ~ ^ : ? * [ \\, .., @{ or //")
	case strings.HasSuffix(name, ".lock") || strings.Contains(name, "/.") || strings.HasPrefix(name, "."):
		return i18n.T("No part of a branch name can start with . or end with .lock")
	}
	for _, r := range name {
		if r < ' ' {
			return i18n.T("A branch name cannot contain control characters")
		}
	}
	for _, b := range branches {
		if !b.Remote && b.Name == name {
			return i18n.F("Branch %s already exists", name)
		}
	}
	return ""
}

// createBranchPrompt asks for a new branch's name, saying what is wrong with it as it is typed,
// then creates the branch and switches to it, in the repository it was asked in.
func (e *Editor) createBranchPrompt() error {
	if e.repository().git.repo == nil {
		return errors.New(i18n.T("Not in a Git repository"))
	}
	rc := e.repository()
	cur := e.branch()
	e.prompt(i18n.F("Name of the new branch (from %s)", cur), "", func(s string) string {
		if p := branchNameProblem(strings.TrimSpace(s), rc.scm.branches); p != "" {
			return p
		}
		return i18n.F("Create and switch to %s", strings.TrimSpace(s))
	}, func(e *Editor, name string) error {
		name = strings.TrimSpace(name)
		if p := branchNameProblem(name, rc.scm.branches); p != "" {
			return errors.New(p)
		}
		defer e.bindRepository(rc)()
		return e.switchBranch(name, true)
	})
	return nil
}
