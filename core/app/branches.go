package app

import (
	"errors"
	"strings"

	"github.com/corn12138/yan-source/core/git"
)

// Branch and remote management (GI-27). Every command acts on the repository the Source Control
// view has chosen, lists come from a background read, and git runs through gitDo, so a
// runCommands chain waits for it and a failure stops the chain.

// loadThen reads what a command chooses from in the background, then runs show on the owner.
func loadThen[T any](e *Editor, read func(r *git.Repo) (T, error), show func(e *Editor, v T)) error {
	r := e.repository().git.repo
	if r == nil {
		return errors.New("不在 git 仓库里")
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
		return errors.New("HEAD 不在分支上：先切换到要改名的分支")
	}
	e.prompt("把分支 "+cur+" 改名为", cur, func(s string) string { return "改名为 " + strings.TrimSpace(s) }, func(e *Editor, name string) error {
		name = strings.TrimSpace(name)
		if name == "" || name == cur {
			return nil
		}
		return e.gitDo("改分支名中…", func(r *git.Repo) error { return r.RenameBranch(cur, name) }, func(e *Editor) {
			e.status = "分支 " + cur + " 已改名为 " + name
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
			e.status = "除了当前分支没有别的本地分支"
			return
		}
		e.gitChoose("删除哪个本地分支（未合并的会再问）", items...)
	})
}

func (e *Editor) deleteBranchNamed(name string, force bool) error {
	notMerged := false
	return e.gitDo("删除分支中…", func(r *git.Repo) error {
		err := r.DeleteBranch(name, force)
		if errors.Is(err, git.ErrNotMerged) {
			notMerged = true
			return nil
		}
		return err
	}, func(e *Editor) {
		if notMerged {
			e.gitChoose("分支 "+name+" 还有没合并到别处的提交，删除会丢掉它们",
				ovItem{label: "保留分支", run: func(*Editor) error { return nil }},
				ovItem{label: "仍然删除 " + name, run: func(e *Editor) error { return e.deleteBranchNamed(name, true) }})
			return
		}
		e.status = "已删除分支 " + name
	})
}

// addRemote asks for a remote's name, then its URL.
func (e *Editor) addRemote() error {
	if e.repository().git.repo == nil {
		return errors.New("不在 git 仓库里")
	}
	e.prompt("新远程仓库的名字", "origin", func(s string) string { return "名字：" + strings.TrimSpace(s) }, func(e *Editor, name string) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil
		}
		e.prompt("远程仓库 "+name+" 的地址", "", func(s string) string { return "添加 " + name + "：" + strings.TrimSpace(s) }, func(e *Editor, url string) error {
			url = strings.TrimSpace(url)
			if url == "" {
				return nil
			}
			return e.gitDo("添加远程仓库中…", func(r *git.Repo) error { return r.AddRemote(name, url) }, func(e *Editor) {
				e.status = "已添加远程仓库 " + name
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
			e.status = "这个仓库没有远程仓库"
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
	return e.chooseRemote("删除哪个远程仓库", (*Editor).removeRemoteNamed)
}

func (e *Editor) removeRemoteNamed(rm git.Remote) error {
	{
		e.gitChoose("删除远程仓库 "+rm.Name+"？它的远程跟踪分支也会删掉，本地分支不受影响",
			ovItem{label: "取消", run: func(*Editor) error { return nil }},
			ovItem{label: "删除 " + rm.Name, run: func(e *Editor) error {
				return e.gitDo("删除远程仓库中…", func(r *git.Repo) error { return r.RemoveRemote(rm.Name) }, func(e *Editor) {
					e.status = "已删除远程仓库 " + rm.Name
				})
			}})
		return nil
	}
}

func (e *Editor) setRemoteURL() error {
	return e.chooseRemote("修改哪个远程仓库的地址", (*Editor).setRemoteURLOf)
}

func (e *Editor) setRemoteURLOf(rm git.Remote) error {
	{
		e.prompt("远程仓库 "+rm.Name+" 的新地址", rm.URL, func(s string) string { return "改为 " + strings.TrimSpace(s) }, func(e *Editor, url string) error {
			url = strings.TrimSpace(url)
			if url == "" || url == rm.URL {
				return nil
			}
			return e.gitDo("修改远程地址中…", func(r *git.Repo) error { return r.SetRemoteURL(rm.Name, url) }, func(e *Editor) {
				e.status = "远程仓库 " + rm.Name + " 改为 " + url
			})
		})
		return nil
	}
}

func (e *Editor) pruneRemote() error {
	return e.chooseRemote("清理哪个远程仓库里已删除的分支", func(e *Editor, rm git.Remote) error { return e.pruneRemoteNamed(rm.Name) })
}

func (e *Editor) pruneRemoteNamed(name string) error {
	return e.gitDo("清理远程分支中…", func(r *git.Repo) error { return r.PruneRemote(name) }, func(e *Editor) {
		e.status = "已清理 " + name + " 上不存在的远程跟踪分支"
	})
}

// branchNameProblem says what is wrong with name as a new local branch's, "" for nothing:
// Git's rules for ref names, and a branch of that name already there.
func branchNameProblem(name string, branches []git.Branch) string {
	switch {
	case name == "":
		return "写一个分支名"
	case strings.HasPrefix(name, "-") || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") || strings.HasSuffix(name, "."):
		return "分支名不能以 - 或 / 开头，不能以 / 或 . 结尾"
	case strings.ContainsAny(name, " ~^:?*[\\\x7f") || strings.Contains(name, "..") || strings.Contains(name, "@{") || strings.Contains(name, "//") || name == "@":
		return "分支名里不能有空格、~ ^ : ? * [ \\、..、@{ 或 //"
	case strings.HasSuffix(name, ".lock") || strings.Contains(name, "/.") || strings.HasPrefix(name, "."):
		return "分支名的每一段不能以 . 开头或以 .lock 结尾"
	}
	for _, r := range name {
		if r < ' ' {
			return "分支名里不能有控制字符"
		}
	}
	for _, b := range branches {
		if !b.Remote && b.Name == name {
			return "已经有分支 " + name
		}
	}
	return ""
}

// createBranchPrompt asks for a new branch's name, saying what is wrong with it as it is typed,
// then creates the branch and switches to it, in the repository it was asked in.
func (e *Editor) createBranchPrompt() error {
	if e.repository().git.repo == nil {
		return errors.New("不在 git 仓库里")
	}
	rc := e.repository()
	cur := e.branch()
	e.prompt("新分支的名字（从 "+cur+"）", "", func(s string) string {
		if p := branchNameProblem(strings.TrimSpace(s), rc.scm.branches); p != "" {
			return p
		}
		return "新建并切换到 " + strings.TrimSpace(s)
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
