package app

import (
	"cmp"
	"errors"
	"strings"

	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/view"
)

// 在提交上合并、变基、重置、还原、拣选，照着 Mayoi v0.8.0（MIT，cmd/mayoi/git.go 的 runOp 等）：
// 顺序同 Sourcetree。合并、变基、拣选、还原遇到冲突停下来时，源代码管理在提交说明上面说一声，
// 给「继续」「中止」（命令面板里也有），提交说明里放好 git 拟的说明。Mayoi 弹确认框的地方，砚用选一项的浮层。

// opTitle 是 git.Op 的中文叫法。
func opTitle(op git.Op) string {
	switch op {
	case git.OpMerge:
		return i18n.T("Merge")
	case git.OpRebase:
		return i18n.T("Rebase")
	case git.OpCherryPick:
		return i18n.T("Cherry-Pick")
	case git.OpRevert:
		return i18n.T("Revert")
	}
	return ""
}

// localBranch 是提交上唯一的本地分支；没有或者不止一个时为 ""。
func localBranch(c git.Commit) string {
	name := ""
	for _, r := range c.Refs {
		if r.Kind == git.RefBranch {
			if name != "" {
				return ""
			}
			name = r.Name
		}
	}
	return name
}

// revOf 是给命令用的提交的名字：有唯一的本地分支就用它（合并的说明里会写分支名），否则用提交号。
func revOf(c git.Commit) string { return cmp.Or(localBranch(c), c.Hash) }

// inHead 报告 HEAD 有没有提交 hash（按读到的历史算：比它更早的算没有）。
func (e *Editor) inHead(hash string) bool {
	st := e.repository().git.status
	if st == nil || st.Head == "" {
		return false
	}
	parents := map[string][]string{}
	for _, c := range e.repository().scm.history.commits {
		parents[c.Hash] = c.Parents
	}
	seen := map[string]bool{}
	todo := []string{st.Head}
	for len(todo) > 0 {
		h := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if h == hash {
			return true
		}
		if seen[h] {
			continue
		}
		seen[h] = true
		todo = append(todo, parents[h]...)
	}
	return false
}

// commitOpItems 是提交浮层里合并、变基、重置、还原、拣选这几项：HEAD 已经有的提交不能合并、变基、拣选，
// 没有的不能还原；分离的 HEAD 不能合并、变基、重置。
func (e *Editor) commitOpItems(c git.Commit) []ovItem {
	branch, at := e.branch(), c.Short()
	name := cmp.Or(localBranch(c), at)
	in := e.inHead(c.Hash)
	merge := len(c.Parents) > 1
	var items []ovItem
	if branch != "" && !in {
		rev := revOf(c)
		items = append(items,
			ovItem{label: i18n.F("Merge into %s", branch), detail: name + " · " + c.Subject, run: func(e *Editor) error {
				return e.runOp(git.OpMerge, i18n.T("Merging…"), i18n.F("Merged %s into %s", name, branch), func(r *git.Repo) error { return r.Merge(rev) })
			}},
			ovItem{label: i18n.F("Rebase %s onto Here", branch), detail: i18n.F("Replays the commits only %[2]s has on top of %[1]s; pushed commits are rewritten", name, branch), run: func(e *Editor) error {
				return e.runOp(git.OpRebase, i18n.T("Rebasing…"), i18n.F("Rebased %s onto %s", branch, name), func(r *git.Repo) error { return r.Rebase(rev) })
			}},
		)
	}
	if branch != "" {
		items = append(items, ovItem{label: i18n.F("Reset %s to Here…", branch), detail: at, run: func(e *Editor) error {
			e.resetTo(c, branch)
			return nil
		}})
	}
	if in {
		items = append(items, ovItem{label: i18n.T("Revert This Commit"), detail: i18n.T("Create a new commit that undoes its changes"), run: func(e *Editor) error {
			return e.runOp(git.OpRevert, i18n.T("Reverting…"), i18n.F("Reverted %s", at), func(r *git.Repo) error { return r.Revert(c.Hash, merge) })
		}})
	} else if e.repository().git.status != nil && e.repository().git.status.Head != "" {
		items = append(items, ovItem{label: i18n.F("Cherry-Pick onto %s", cmp.Or(branch, "HEAD")), detail: i18n.T("Make its changes into a new commit"), run: func(e *Editor) error {
			return e.runOp(git.OpCherryPick, i18n.T("Cherry-picking…"), i18n.F("Cherry-picked %s", at), func(r *git.Repo) error { return r.CherryPick(c.Hash, merge) })
		}})
	}
	return items
}

// resetModes 是重置的三种方式，按浮层里的顺序。
var resetModes = []struct {
	mode          git.ResetMode
	label, detail string
}{
	{git.ResetSoft, i18n.N("Soft Reset (soft)"), i18n.N("Changes of the commits after it stay staged; the working tree is untouched")},
	{git.ResetMixed, i18n.N("Mixed Reset (mixed)"), i18n.N("Changes of the commits after it and of the working tree are kept, unstaged")},
	{git.ResetHard, i18n.N("Hard Reset (hard)"), i18n.N("Discards the commits after it and every change in the working tree; can't be undone")},
}

// resetTo 问怎么重置，再把 branch 重置到提交 c。
func (e *Editor) resetTo(c git.Commit, branch string) {
	var items []ovItem
	for _, m := range resetModes {
		items = append(items, ovItem{label: i18n.T(m.label), detail: i18n.T(m.detail), run: func(e *Editor) error {
			if err := e.saveDirtyInRepo(); err != nil {
				return err
			}
			return e.gitDo(i18n.T("Resetting…"), func(r *git.Repo) error { return r.Reset(c.Hash, m.mode) }, func(e *Editor) {
				e.status = i18n.F("Reset %s to %s (%s)", branch, c.Short(), m.mode)
				e.checkDisk()
			})
		}})
	}
	items = append(items, ovItem{label: i18n.T("Cancel"), run: func(*Editor) error { return nil }})
	e.gitChoose(i18n.F("Reset %s to %s?", branch, c.Short()), items...)
}

// runOp 先保存仓库里的文件，再跑一个会动 HEAD 的命令；它遇到冲突停下来时说怎么接着做，
// 把 git 拟的说明放进提交说明里（变基没有这一步）。
func (e *Editor) runOp(op git.Op, busy, done string, fn func(r *git.Repo) error) error {
	if err := e.saveDirtyInRepo(); err != nil {
		return err
	}
	stopped, msg := false, ""
	return e.gitDo(busy, func(r *git.Repo) error {
		err := fn(r)
		if err != nil && r.InProgress() == op {
			stopped, msg = true, r.MergeMessage()
			return nil
		}
		return err
	}, func(e *Editor) {
		e.checkDisk()
		if !stopped {
			e.status = done
			return
		}
		if op != git.OpRebase && strings.TrimSpace(e.repository().scm.message) == "" {
			e.repository().scm.message, e.repository().scm.amend = msg, false
		}
		e.showSide(sideSCM)
		e.repository().git.showMergeFiles = true
		e.status = i18n.F("%s hit conflicts: resolve and stage them, then click \"Continue\", or \"Abort\"", opTitle(op))
	})
}

// continueOp 冲突解决、暂存之后接着做停下来的命令：合并、拣选、还原有提交说明时就用它提交。
func (e *Editor) continueOp() error {
	st := e.repository().git.status
	if st == nil || st.Op == "" {
		return errors.New(i18n.T("No merge, rebase, cherry-pick or revert is in progress"))
	}
	op := st.Op
	for _, en := range st.Entries {
		if en.Conflicted() {
			return errors.New(i18n.T("Resolve the conflicts first, then stage these files"))
		}
	}
	if op != git.OpRebase && strings.TrimSpace(e.repository().scm.message) != "" {
		return e.commit(false)
	}
	if err := e.saveDirtyInRepo(); err != nil {
		return err
	}
	stopped := false
	return e.gitDo(i18n.F("Continuing %s…", opTitle(op)), func(r *git.Repo) error {
		err := r.Continue(op)
		if err != nil && r.InProgress() == op {
			// 变基到下一个有冲突的提交又会停下来。
			if s, serr := r.Status(); serr == nil {
				for _, en := range s.Entries {
					if en.Conflicted() {
						stopped = true
						return nil
					}
				}
			}
		}
		return err
	}, func(e *Editor) {
		e.checkDisk()
		if stopped {
			e.repository().git.showMergeFiles = true
			e.status = i18n.F("%s hit conflicts again: resolve and stage them, then click \"Continue\" again", opTitle(op))
			return
		}
		e.repository().scm.message = ""
		e.status = i18n.F("%s complete", opTitle(op))
	})
}

// abortOp 问过之后中止停下来的命令：HEAD、暂存区、工作区回到开始之前。
func (e *Editor) abortOp() error {
	st := e.repository().git.status
	if st == nil || st.Op == "" {
		return errors.New(i18n.T("No merge, rebase, cherry-pick or revert is in progress"))
	}
	op := st.Op
	e.gitChoose(i18n.F("Abort %s?", opTitle(op)),
		ovItem{label: i18n.F("Abort %s", opTitle(op)), detail: i18n.F("HEAD, the index and the working tree return to before the %s", opTitle(op)), run: func(e *Editor) error {
			return e.gitDo(i18n.F("Aborting %s…", opTitle(op)), func(r *git.Repo) error { return r.Abort(op) }, func(e *Editor) {
				e.repository().scm.message = ""
				e.status = i18n.F("Aborted %s", opTitle(op))
				e.checkDisk()
			})
		}},
		ovItem{label: i18n.T("Cancel"), run: func(*Editor) error { return nil }})
	return nil
}

// opFrame 把停下来的命令写进源代码管理视图。
func (e *Editor) opFrame(dst *view.SCM) {
	if st := e.repository().git.status; st != nil {
		dst.Op = opTitle(st.Op)
	}
}
