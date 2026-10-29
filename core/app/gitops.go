package app

import (
	"cmp"
	"errors"
	"fmt"
	"strings"

	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/view"
)

// 在提交上合并、变基、重置、还原、拣选，照着 Mayoi v0.8.0（MIT，cmd/mayoi/git.go 的 runOp 等）：
// 顺序同 Sourcetree。合并、变基、拣选、还原遇到冲突停下来时，源代码管理在提交说明上面说一声，
// 给「继续」「中止」（命令面板里也有），提交说明里放好 git 拟的说明。Mayoi 弹确认框的地方，砚用选一项的浮层。

// opTitle 是 git.Op 的中文叫法。
func opTitle(op git.Op) string {
	switch op {
	case git.OpMerge:
		return "合并"
	case git.OpRebase:
		return "变基"
	case git.OpCherryPick:
		return "拣选"
	case git.OpRevert:
		return "还原"
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
			ovItem{label: "合并到 " + branch, detail: name + " · " + c.Subject, run: func(e *Editor) error {
				return e.runOp(git.OpMerge, "合并中…", "已把 "+name+" 合并到 "+branch, func(r *git.Repo) error { return r.Merge(rev) })
			}},
			ovItem{label: "把 " + branch + " 变基到这里", detail: "在 " + name + " 上重做 " + branch + " 独有的提交；推送过的提交会被改写", run: func(e *Editor) error {
				return e.runOp(git.OpRebase, "变基中…", "已把 "+branch+" 变基到 "+name, func(r *git.Repo) error { return r.Rebase(rev) })
			}},
		)
	}
	if branch != "" {
		items = append(items, ovItem{label: "把 " + branch + " 重置到这里…", detail: at, run: func(e *Editor) error {
			e.resetTo(c, branch)
			return nil
		}})
	}
	if in {
		items = append(items, ovItem{label: "还原这个提交", detail: "新建一个提交，撤掉它的改动", run: func(e *Editor) error {
			return e.runOp(git.OpRevert, "还原中…", "已还原 "+at, func(r *git.Repo) error { return r.Revert(c.Hash, merge) })
		}})
	} else if e.repository().git.status != nil && e.repository().git.status.Head != "" {
		items = append(items, ovItem{label: "拣选到 " + cmp.Or(branch, "HEAD"), detail: "把它的改动做成一个新提交", run: func(e *Editor) error {
			return e.runOp(git.OpCherryPick, "拣选中…", "已拣选 "+at, func(r *git.Repo) error { return r.CherryPick(c.Hash, merge) })
		}})
	}
	return items
}

// resetModes 是重置的三种方式，按浮层里的顺序。
var resetModes = []struct {
	mode          git.ResetMode
	label, detail string
}{
	{git.ResetSoft, "软重置（soft）", "它之后的提交的改动留在暂存区，工作区不动"},
	{git.ResetMixed, "混合重置（mixed）", "它之后的提交的改动和工作区的改动都留着，不暂存"},
	{git.ResetHard, "硬重置（hard）", "丢掉它之后的提交和工作区的所有改动，不能撤销"},
}

// resetTo 问怎么重置，再把 branch 重置到提交 c。
func (e *Editor) resetTo(c git.Commit, branch string) {
	var items []ovItem
	for _, m := range resetModes {
		items = append(items, ovItem{label: m.label, detail: m.detail, run: func(e *Editor) error {
			if err := e.saveDirtyInRepo(); err != nil {
				return err
			}
			return e.gitDo("重置中…", func(r *git.Repo) error { return r.Reset(c.Hash, m.mode) }, func(e *Editor) {
				e.status = fmt.Sprintf("已把 %s 重置到 %s（%s）", branch, c.Short(), m.mode)
				e.checkDisk()
			})
		}})
	}
	items = append(items, ovItem{label: "取消", run: func(*Editor) error { return nil }})
	e.gitChoose("把 "+branch+" 重置到 "+c.Short()+"？", items...)
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
		e.status = opTitle(op) + "遇到冲突：解决并暂存后点「继续」，或者「中止」"
	})
}

// continueOp 冲突解决、暂存之后接着做停下来的命令：合并、拣选、还原有提交说明时就用它提交。
func (e *Editor) continueOp() error {
	st := e.repository().git.status
	if st == nil || st.Op == "" {
		return errors.New("没有停下来的合并、变基、拣选或还原")
	}
	op := st.Op
	for _, en := range st.Entries {
		if en.Conflicted() {
			return errors.New("先解决冲突，再暂存这些文件")
		}
	}
	if op != git.OpRebase && strings.TrimSpace(e.repository().scm.message) != "" {
		return e.commit(false)
	}
	if err := e.saveDirtyInRepo(); err != nil {
		return err
	}
	stopped := false
	return e.gitDo("继续"+opTitle(op)+"中…", func(r *git.Repo) error {
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
			e.status = opTitle(op) + "又遇到冲突：解决并暂存后再点「继续」"
			return
		}
		e.repository().scm.message = ""
		e.status = opTitle(op) + "完成"
	})
}

// abortOp 问过之后中止停下来的命令：HEAD、暂存区、工作区回到开始之前。
func (e *Editor) abortOp() error {
	st := e.repository().git.status
	if st == nil || st.Op == "" {
		return errors.New("没有停下来的合并、变基、拣选或还原")
	}
	op := st.Op
	e.gitChoose("中止"+opTitle(op)+"？",
		ovItem{label: "中止" + opTitle(op), detail: "HEAD、暂存区和工作区回到" + opTitle(op) + "之前", run: func(e *Editor) error {
			return e.gitDo("中止"+opTitle(op)+"中…", func(r *git.Repo) error { return r.Abort(op) }, func(e *Editor) {
				e.repository().scm.message = ""
				e.status = "已中止" + opTitle(op)
				e.checkDisk()
			})
		}},
		ovItem{label: "取消", run: func(*Editor) error { return nil }})
	return nil
}

// opFrame 把停下来的命令写进源代码管理视图。
func (e *Editor) opFrame(dst *view.SCM) {
	if st := e.repository().git.status; st != nil {
		dst.Op = opTitle(st.Op)
	}
}
