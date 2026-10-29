package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/view"
)

// 源代码管理里改动下面的提交历史，照着 Mayoi v0.5.1–v0.7.0（MIT，cmd/mayoi/scm.go、git.go）：
// 最近的提交画成图，每个分支一道、一种颜色，带着作者、分支和标签；HEAD 是空心的圈，合并是空心的点。
// 「显示所有分支」时读所有分支和标签，不只检出的分支。打开一个提交，浮层里列出它改的文件，
// 选一个看那个文件在这个提交里的改动；也能复制提交号、检出它、在它那里建分支或打标签。

// historyPage 是提交历史一次读多少个提交。
const historyPage = 200

// historyState 是读到的提交历史。
type historyState struct {
	files   map[string]*commitFilesState
	commits []git.Commit
	rows    []view.SCMCommit // 前端画的行，读一次算一次
	lanes   int
	of      string // 读的是哪个状态（historyKey）
	gen     int    // git 命令跑完后加一：分支、标签可能变了，状态却没变
	loading bool
	more    int       // 「显示更多提交」按了几次：每次多读 historyPage 个
	whenAt  time.Time // rows 里「多久以前」是按这个时刻写的
}

// refreshWhen 在「多久以前」过时了一分钟以上时按 now 重写（换一份新的切片，上一帧的不动）。
func (h *historyState) refreshWhen(now time.Time) {
	if len(h.rows) == 0 || now.Sub(h.whenAt) < time.Minute {
		return
	}
	h.rows = slices.Clone(h.rows)
	for k := range h.rows {
		h.rows[k].When = relTime(h.commits[k].Time, now)
	}
	h.whenAt = now
}

// historyKey 说明要读的历史：HEAD、分支、上游、读不读所有分支变了，就得重读。
func (e *Editor) historyKey() string {
	st := e.repository().git.status
	return fmt.Sprintf("%s|%s|%s|%d|%d|%v|%d|%d", st.Head, st.Branch, st.Upstream, st.Ahead, st.Behind, e.allBranches, e.repository().scm.history.gen, e.repository().scm.history.more)
}

// historyLimit 是这次读多少个提交。
func (e *Editor) historyLimit() int { return historyPage * (1 + e.repository().scm.history.more) }

// moreCommits 多读 historyPage 个提交（「显示更多提交」）。
func (e *Editor) moreCommits() error {
	e.repository().scm.history.more++
	e.refreshHistory()
	return nil
}

// refreshHistory 在后台重读提交历史（要读的和上次读的不一样时）。
func (e *Editor) refreshHistory() {
	g, h := e.repository().git, &e.repository().scm.history
	if g.repo == nil || g.status == nil || h.loading {
		return
	}
	key := e.historyKey()
	if key == h.of {
		return
	}
	h.loading = true
	r, all, head, limit := g.repo, e.allBranches, g.status.Head, e.historyLimit()
	post := e.postGit()
	go func() {
		cs, err := r.Log(limit, all)
		var unpushed map[string]bool
		if remotes, _ := r.Remotes(); len(remotes) > 0 { // 没有远程仓库就无所谓推没推
			unpushed = r.Unpushed(limit)
		}
		post(func(e *Editor) {
			h := &e.repository().scm.history
			h.loading = false
			if e.repository().git.repo != r {
				return
			}
			h.of = key
			if err != nil {
				e.status = gitError(err)
				return
			}
			h.commits, h.whenAt = cs, time.Now()
			h.rows, h.lanes = commitRows(cs, layoutGraph(cs), head, unpushed, h.whenAt)
			e.refreshHistory() // 读的时候状态又变了
		})
	}()
}

// commitRows 把提交和排好的图写成前端画的行，返回它们和图最多有几道。
func commitRows(cs []git.Commit, graph []graphRow, head string, unpushed map[string]bool, now time.Time) ([]view.SCMCommit, int) {
	out := make([]view.SCMCommit, len(cs))
	for k := range cs {
		c, gr := &cs[k], &graph[k]
		v := view.SCMCommit{
			Hash: c.Hash, Short: c.Short(), Subject: c.Subject, Author: c.Author, When: relTime(c.Time, now),
			Refs: refBadges(c.Refs), Head: c.Hash == head, Unpushed: unpushed[c.Hash], Merge: gr.merge,
			Lane: gr.lane, Color: gr.color,
			Above: make([]int, len(gr.above)), Joins: make([]bool, len(gr.above)),
			Below: make([]int, len(gr.below)), From: gr.from, Through: make([]bool, len(gr.below)),
		}
		for i, l := range gr.above {
			v.Above[i] = -1
			if l.hash != "" {
				v.Above[i], v.Joins[i] = l.color, l.hash == c.Hash
			}
		}
		for i, l := range gr.below {
			v.Below[i] = -1
			if l.hash != "" {
				v.Below[i] = l.color
				v.Through[i] = i != gr.lane && i < len(gr.above) && gr.above[i].hash == l.hash
			}
		}
		out[k] = v
	}
	return out, graphLanes(graph)
}

// refBadges 排好一个提交的引用：分离的 HEAD、检出的分支、别的分支、远程分支、标签；
// 和本地分支同名的远程分支并进本地分支里（照着 Mayoi 的 refBadges）。
func refBadges(refs []git.Ref) []view.SCMRef {
	rank := func(r git.Ref) int {
		switch {
		case r.Kind == git.RefHead:
			return 0
		case r.Head:
			return 1
		case r.Kind == git.RefBranch:
			return 2
		case r.Kind == git.RefRemote:
			return 3
		}
		return 4
	}
	sorted := slices.Clone(refs)
	slices.SortStableFunc(sorted, func(x, y git.Ref) int { return rank(x) - rank(y) })
	local := map[string]int{}
	var out []view.SCMRef
	for _, r := range sorted {
		if r.Kind == git.RefRemote {
			if remote, name, ok := strings.Cut(r.Name, "/"); ok {
				if i, ok := local[name]; ok {
					out[i].Remotes = append(out[i].Remotes, remote)
					continue
				}
			}
		}
		kind := map[git.RefKind]view.SCMRefKind{git.RefBranch: view.RefBranch, git.RefRemote: view.RefRemote, git.RefTag: view.RefTag, git.RefHead: view.RefHead}[r.Kind]
		if r.Kind == git.RefBranch {
			local[r.Name] = len(out)
		}
		out = append(out, view.SCMRef{Name: r.Name, Kind: kind, Current: r.Head})
	}
	return out
}

// toggleAllBranches 切换提交历史里显示所有分支和标签，还是只显示检出的分支。
func (e *Editor) toggleAllBranches() error {
	e.allBranches = !e.allBranches
	if e.state != nil {
		e.state.AllBranches = e.allBranches
		e.saveState()
	}
	e.refreshHistory()
	if e.allBranches {
		e.status = "提交历史：所有分支"
	} else {
		e.status = "提交历史：当前分支"
	}
	return nil
}

// scmSelectedCommit 返回选中的提交在历史里的下标（选中的是提交历史里的一行时）。
func (e *Editor) scmSelectedCommit() (int, bool) {
	row, ok := e.selectedHistoryRow()
	if !ok {
		return 0, false
	}
	i := slices.IndexFunc(e.repository().scm.history.commits, func(c git.Commit) bool { return c.Hash == row.Hash })
	return i, i >= 0
}

// openCommit 打开历史里的第 i 个提交：浮层里先列出它改的文件，再列出能对它做的事。
func (e *Editor) openCommit(i int) error {
	r := e.repository().git.repo
	if r == nil || i < 0 || i >= len(e.repository().scm.history.commits) {
		return nil
	}
	c := e.repository().scm.history.commits[i]
	post := e.postGit()
	go func() {
		fs, err := r.CommitFiles(c)
		post(func(e *Editor) {
			if e.repository().git.repo != r {
				return
			}
			if err != nil {
				e.status = gitError(err)
				return
			}
			e.commitChoices(c, fs)
		})
	}()
	return nil
}

// commitChoices 是打开一个提交的浮层。
func (e *Editor) commitChoices(c git.Commit, fs []git.CommitFile) {
	r := e.repository().git.repo
	var items []ovItem
	for _, f := range fs {
		rel := r.Rel(f.Path)
		dir, name := filepath.Split(rel)
		detail := string(f.Status)
		if dir != "" {
			detail += " · " + strings.TrimSuffix(dir, "/")
		}
		if f.Orig != "" {
			detail += " · 从 " + r.Rel(f.Orig) + " 改名"
		}
		items = append(items, ovItem{label: name, detail: detail, run: func(e *Editor) error { return e.openCommitChange(c, f) }})
	}
	at := c.Short()
	items = append(items,
		ovItem{label: "复制提交号", detail: c.Hash, run: func(e *Editor) error {
			if e.host == nil {
				return errors.New("剪贴板不可用")
			}
			if err := e.host.SetClipboard(c.Hash); err != nil {
				return err
			}
			e.status = "已复制 " + at
			return nil
		}},
		ovItem{label: "检出这个提交", detail: "HEAD 会分离到 " + at, run: func(e *Editor) error {
			if err := e.saveDirtyInRepo(); err != nil {
				return err
			}
			return e.gitDo("检出中…", func(r *git.Repo) error { return r.Checkout(c.Hash) }, func(e *Editor) {
				e.status = "已检出 " + at
				e.checkDisk()
			})
		}},
	)
	items = append(items, e.commitToolItems(c)...)
	items = append(items, e.commitOpItems(c)...)
	items = append(items,
		ovItem{label: "在这里新建分支…", detail: at, run: func(e *Editor) error {
			e.gitPrompt("在 "+at+" 新建分支", "", func(s string) string { return "新建分支 " + strings.TrimSpace(s) }, func(e *Editor, name string) error {
				name, err := refName(name)
				if err != nil {
					return err
				}
				return e.gitDo("新建分支中…", func(r *git.Repo) error { return r.CreateBranch(name, c.Hash) }, func(e *Editor) {
					e.status = "已在 " + at + " 新建分支 " + name
				})
			})
			return nil
		}},
		ovItem{label: "在这里打标签…", detail: at, run: func(e *Editor) error {
			e.gitPrompt("给 "+at+" 打标签", "", func(s string) string { return "打标签 " + strings.TrimSpace(s) }, func(e *Editor, name string) error {
				name, err := refName(name)
				if err != nil {
					return err
				}
				return e.gitDo("打标签中…", func(r *git.Repo) error { return r.CreateTag(name, c.Hash, "") }, func(e *Editor) {
					e.status = "已给 " + at + " 打标签 " + name
				})
			})
			return nil
		}},
	)
	title := at + " " + c.Subject
	if len(fs) == 0 {
		title += "（没有改文件）"
	}
	e.gitChoose(title, items...)
}

// refName 检查新分支、新标签的名字：不能空着，也不能以 - 开头（git 会把它当成选项，
// 比如 git tag -f <提交号> 会在 HEAD 上打一个叫提交号的标签）。别的规矩由 git 自己查。
func refName(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", errors.New("名字不能空着")
	case strings.HasPrefix(s, "-"):
		return "", fmt.Errorf("名字不能以 - 开头：%s", s)
	}
	return s, nil
}

// openCommitChange 在标签里显示文件 f 在提交 c 里的改动（相对它的第一个父提交）。
func (e *Editor) openCommitChange(c git.Commit, f git.CommitFile) error {
	r := e.repository().git.repo
	if r == nil {
		return errors.New("不在 git 仓库里")
	}
	post := e.postGit()
	go func() {
		var before, after string
		var hasBase bool
		var err error
		if len(c.Parents) > 0 && f.Status != 'A' {
			orig := f.Path
			if f.Orig != "" {
				orig = f.Orig
			}
			before, hasBase, err = r.Show(c.Parents[0], orig)
		}
		if err == nil && f.Status != 'D' {
			after, _, err = r.Show(c.Hash, f.Path)
		}
		post(func(e *Editor) {
			if err != nil {
				e.status = gitError(err)
				return
			}
			dt := &diffTab{path: f.Path, expanded: map[int]bool{}, fixed: true, commitHash: c.Hash, after: diffLines(after), hasBase: hasBase}
			if hasBase {
				dt.base = diffLines(before)
			}
			e.showDiff(filepath.Base(f.Path)+" ("+c.Short()+")", dt, false)
		})
	}()
	return nil
}

// diffLines 把文字拆成行：空文字（删掉的、新加的文件的另一边）没有行。
func diffLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}
