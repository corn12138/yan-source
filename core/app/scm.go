package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

// 源代码管理视图，照着 Mayoi（MIT，cmd/mayoi/scm.go）：改动按「合并冲突、已暂存、更改、未跟踪」分组；
// 暂存、取消暂存、放弃修改（未跟踪的文件移到废纸篓），写提交说明后提交、修改上次提交、撤销上次提交；
// 拉取、推送、抓取，切换和新建分支。git 命令都在后台跑，跑完重读状态。

// sideView 是侧边栏显示的视图。
type sideView uint8

const (
	sideExplorer sideView = iota
	sideSearch
	sideSCM
	sideGuide // 功能导览（guide.go）：没开文件夹也能看
)

type scmState struct {
	compareRepo     *git.Repo
	compareSelected string

	sel      int // 选中的行（所有分组的行连起来数）
	top      int
	message  string
	amend    bool
	focusMsg bool // 键盘在提交说明里
	branches []git.Branch
	history  historyState
	tree     bool            // 以树的形式查看：每组的文件按文件夹排
	closed   map[string]bool // 树里折起来的文件夹（分组 + 路径）
	picked   map[string]bool // 多选的文件（scmKey）；nil 表示只有选中的那一行（scmpick.go）
	anchor   int             // Shift 多选从这一行起
	panel    repoPanel       // 下面的仓库面板：分支、远程（repopanel.go）
}

// scmRows 把状态按分组排好，返回所有行（分组顺序）和每组的标题与行数。
func (e *Editor) scmGroups() []view.SCMGroup {
	gs := e.scmAllGroups()
	for i := range gs {
		if e.scmFolded[SCMSectionKey(gs[i].Kind)] {
			gs[i].Folded, gs[i].Count, gs[i].Rows = true, scmFileCount(gs[i].Rows), nil
		} else {
			gs[i].Count = scmFileCount(gs[i].Rows)
		}
	}
	return gs
}

// scmAllGroups are Source Control's groups with every row, folded or not: what acting on a
// whole group (stage all, the next changed file) goes through.
func (e *Editor) scmAllGroups() []view.SCMGroup {
	st := e.repository().git.status
	if st == nil || e.repository().git.repo == nil {
		return nil
	}
	var merge, staged, changes, untracked []view.SCMRow
	row := func(en git.Entry, staged bool) view.SCMRow {
		rel := e.repository().git.repo.Rel(en.Path)
		dir, name := filepath.Split(rel)
		l := en.Letter()
		if staged {
			l = en.X
		} else if en.Y != '.' && !en.Untracked() && !en.Conflicted() {
			l = en.Y
		}
		return view.SCMRow{Path: en.Path, Name: name, Dir: strings.TrimSuffix(dir, "/"), Letter: l, Staged: staged}
	}
	for _, en := range st.Entries {
		switch {
		case en.Conflicted():
			merge = append(merge, row(en, false))
		case en.Untracked():
			untracked = append(untracked, row(en, false))
		default:
			if en.Staged() {
				staged = append(staged, row(en, true))
			}
			if en.Unstaged() {
				changes = append(changes, row(en, false))
			}
		}
	}
	var out []view.SCMGroup
	defer func() {
		if e.repository().scm.tree {
			for i := range out {
				out[i].Rows = e.treeRows(out[i].Kind, out[i].Rows)
			}
		}
	}()
	for _, g := range []view.SCMGroup{
		{Kind: view.GroupMerge, Title: i18n.T("Merge Conflicts"), Rows: merge},
		{Kind: view.GroupStaged, Title: i18n.T("Staged Changes"), Rows: staged},
		{Kind: view.GroupChanges, Title: i18n.T("Changes"), Rows: changes},
		{Kind: view.GroupUntracked, Title: i18n.T("Untracked Files"), Rows: untracked},
	} {
		if len(g.Rows) > 0 {
			out = append(out, g)
		}
	}
	return out
}

// scmFlat 返回所有分组的行，按显示顺序。
func scmFlat(gs []view.SCMGroup) []view.SCMRow {
	var rows []view.SCMRow
	for _, g := range gs {
		rows = append(rows, g.Rows...)
	}
	return rows
}

// scmSelected 返回选中的文件（选中的是提交历史里的一行时返回 false）。
func (e *Editor) scmSelected() (view.SCMRow, bool) {
	rows := scmFlat(e.scmGroups())
	e.repository().scm.sel = max(min(e.repository().scm.sel, len(rows)+len(e.shownCommits())-1), 0)
	if e.repository().scm.sel >= len(rows) {
		return view.SCMRow{}, false
	}
	return rows[e.repository().scm.sel], true
}

func (e *Editor) scmOneFrame(dst *view.SCM) {
	*dst = view.SCM{Groups: dst.Groups[:0]}
	g := e.repository().git
	if g.repo == nil {
		dst.NoRepo = true
		e.nestedFrame(dst)
		return
	}
	// A shared history preference can change while this repository is inactive.
	e.refreshHistory()
	dst.Groups = e.scmGroups()
	if picked := e.repository().scm.picked; len(picked) > 0 {
		for gi := range dst.Groups {
			for ri, r := range dst.Groups[gi].Rows {
				dst.Groups[gi].Rows[ri].Picked = !r.Folder && picked[scmKey(r)]
			}
		}
	}
	h := &e.repository().scm.history
	h.refreshWhen(time.Now())
	dst.Commits, dst.Lanes, dst.AllBranches = e.shownCommits(), h.lanes, e.allBranches
	dst.CommitsFolded = e.scmFolded["commits"]
	dst.MoreCommits = len(h.commits) >= e.historyLimit()
	dst.Panel, dst.PanelRemotes = e.repoPanelOpen(), e.repoPanelRemotes()
	dst.PanelRows = e.repoPanelRows()
	dst.HistoryLoading = h.loading && len(h.rows) == 0
	n := len(scmFlat(dst.Groups)) + len(dst.Commits)
	e.repository().scm.sel = max(min(e.repository().scm.sel, n-1), 0)
	dst.Selected, dst.Message, dst.Amend = e.repository().scm.sel, e.repository().scm.message, e.repository().scm.amend
	dst.MessageFocused = e.repository().scm.focusMsg && e.area == areaTree && e.side == sideSCM
	dst.Branch, dst.Busy = e.branch(), g.busy
	if st := g.status; st != nil {
		dst.Ahead, dst.Behind, dst.Upstream = st.Ahead, st.Behind, st.Upstream != ""
	}
	dst.CommitLabel = e.commitLabel()
	e.opFrame(dst)
}

// commitLabel 是提交按钮上的字：修改上次提交、提交、全部提交（什么都没暂存时）。
func (e *Editor) commitLabel() string {
	switch {
	case e.repository().scm.amend:
		return i18n.T("Amend Last Commit")
	case e.stagedCount() == 0:
		return i18n.T("Commit All")
	}
	return i18n.T("Commit")
}

func (e *Editor) stagedCount() int {
	n := 0
	if st := e.repository().git.status; st != nil {
		for _, en := range st.Entries {
			if en.Staged() {
				n++
			}
		}
	}
	return n
}

// changeCount 是改动的文件数（侧边栏按钮上的数字）。
func (e *Editor) changeCount() int {
	n := 0
	for _, rc := range e.repositoryContexts() {
		if rc.git.status != nil {
			n += len(rc.git.status.Entries)
		}
	}
	return n
}

// gitDo 在后台跑一个 git 命令；busy 不为空时状态栏显示它（同一时间只跑一个这样的命令）。
// 跑完重读状态；出错时在状态栏说原因，成功时调用 then。
func (e *Editor) gitDo(busy string, fn func(r *git.Repo) error, then func(e *Editor)) error {
	g := e.repository().git
	r := g.repo
	if r == nil {
		return errors.New(i18n.T("Not in a Git repository"))
	}
	if busy != "" {
		if g.busy != "" {
			return fmt.Errorf(i18n.T("%s is not finished yet"), g.busy)
		}
		g.busy = busy
	}
	post := e.postGit()
	e.gitPending++
	go func() {
		err := fn(r)
		post(func(e *Editor) {
			if err != nil {
				e.fails++ // stops a runCommands waiting on this command
			}
			if busy != "" {
				e.repository().git.busy = ""
			}
			e.gitChanged()
			// 分支、标签可能变了，状态却没变：让重读的状态到了再重读历史，那时 HEAD 才是新的。
			e.repository().scm.history.gen++
			for _, d := range e.docs {
				d.marks.headMoved()
			}
			if err != nil {
				e.status = gitError(err)
				return
			}
			if then != nil {
				then(e)
			}
		})
		// Posted after the result, so it runs once that has (or was dropped with its folder).
		e.Post(func(e *Editor) {
			e.gitPending--
			e.pumpChain()
		})
	}()
	return nil
}

// gitError 把 git 的报错写成一句话；要登录的命令提示去终端里跑一次。
func gitError(err error) string {
	if git.NeedsAuth(err) {
		return i18n.T("git needs you to sign in: run this command once in a terminal, then it works here")
	}
	var ge *git.Error
	if errors.As(err, &ge) && strings.TrimSpace(ge.Stderr) != "" {
		lines := strings.Split(strings.TrimSpace(ge.Stderr), "\n")
		return i18n.F("%s: %s", "git", lines[len(lines)-1])
	}
	return i18n.F("%s: %s", "git", err.Error())
}

// saveAllFor 先保存要提交、切换分支的文件，免得把旧内容提交上去。
func (e *Editor) saveDirtyInRepo() error {
	for _, d := range e.docs {
		if d.buf.Dirty() && e.repository().git.repo != nil && e.repositoryForPath(d.buf.Path()) == e.repository() {
			if err := e.saveDoc(d); err != nil {
				return err
			}
		}
	}
	return nil
}

// stage 暂存（或取消暂存）文件。
func (e *Editor) stage(paths []string, unstage bool) error {
	if len(paths) == 0 {
		return nil
	}
	if err := e.checkRepositoryPaths(paths); err != nil {
		return err
	}
	if !unstage {
		if err := e.saveDirtyInRepo(); err != nil {
			return err
		}
	}
	return e.gitDo("", func(r *git.Repo) error {
		if unstage {
			return r.Unstage(paths...)
		}
		return r.Stage(paths...)
	}, nil)
}

// discard 放弃文件的修改：跟踪的文件恢复成索引里的样子，未跟踪的移到废纸篓。开着的文档随后从磁盘重载。
func (e *Editor) discard(rows []view.SCMRow) error {
	paths := make([]string, 0, len(rows))
	for _, row := range rows {
		paths = append(paths, row.Path)
	}
	if err := e.checkRepositoryPaths(paths); err != nil {
		return err
	}
	var tracked []string
	for _, r := range rows {
		if r.Letter == 'U' && !r.Staged {
			if e.host == nil {
				return errors.New(i18n.F("This frontend can't move files to %s", e.trashName()))
			}
			if err := e.host.Trash(r.Path); err != nil {
				return fmt.Errorf(i18n.T("Couldn't move to %s: %w"), e.trashName(), err)
			}
			continue
		}
		tracked = append(tracked, r.Path)
	}
	if len(tracked) == 0 {
		e.gitChanged()
		e.checkDisk()
		return nil
	}
	return e.gitDo("", func(r *git.Repo) error { return r.Discard(tracked...) }, func(e *Editor) { e.checkDisk() })
}

// commit 提交暂存的改动；什么都没暂存时先暂存全部。amend 时修改上次提交。push 为真时提交后推送。
func (e *Editor) commit(push bool) error {
	if e.repository().git.repo == nil {
		return errors.New(i18n.T("Open a Git repository first"))
	}
	msg := strings.TrimSpace(e.repository().scm.message)
	if msg == "" && !e.repository().scm.amend {
		e.repository().scm.focusMsg = true
		return errors.New(i18n.T("Write a commit message first"))
	}
	if err := e.saveDirtyInRepo(); err != nil {
		return err
	}
	all := e.stagedCount() == 0 && !e.repository().scm.amend
	if all && e.changeCount() == 0 {
		return errors.New(i18n.T("No changes to commit"))
	}
	amend := e.repository().scm.amend
	branch, upstream := e.branch(), e.repository().git.status != nil && e.repository().git.status.Upstream != ""
	return e.gitDo(i18n.T("Committing…"), func(r *git.Repo) error {
		if all {
			if err := r.Stage(r.Root); err != nil {
				return err
			}
		}
		if err := r.Commit(msg, amend); err != nil {
			return err
		}
		if push {
			return r.Push(branch, upstream)
		}
		return nil
	}, func(e *Editor) {
		e.repository().scm.message, e.repository().scm.amend, e.repository().scm.focusMsg = "", false, false
		if push {
			e.status = i18n.T("Committed and pushed")
			return
		}
		e.tell(i18n.T("Committed"), i18n.T("Undo"), "git.undoCommit") // 撤销：提交拿回来，改动留在暂存区，说明放回提交说明里
	})
}

// startAmend 修改上次提交：把上次的说明填进提交说明里。
func (e *Editor) startAmend() error {
	r := e.repository().git.repo
	if r == nil {
		return errors.New(i18n.T("Not in a Git repository"))
	}
	e.showSide(sideSCM)
	e.repository().scm.amend, e.repository().scm.focusMsg = true, true
	post := e.postGit()
	go func() {
		msg := r.LastMessage()
		post(func(e *Editor) {
			if e.repository().scm.amend && strings.TrimSpace(e.repository().scm.message) == "" {
				e.repository().scm.message = msg
			}
		})
	}()
	return nil
}

// undoCommit 撤销上次提交：改动留在暂存区，说明放回提交说明里。
func (e *Editor) undoCommit() error {
	r := e.repository().git.repo
	if r == nil {
		return errors.New(i18n.T("Not in a Git repository"))
	}
	post := e.postGit()
	return e.gitDo(i18n.T("Undoing commit…"), func(r *git.Repo) error {
		msg := r.LastMessage()
		if err := r.UndoCommit(); err != nil {
			return err
		}
		post(func(e *Editor) { e.repository().scm.message = msg })
		return nil
	}, func(e *Editor) { e.status = i18n.T("Undid the last commit; its changes stay staged") })
}

func (e *Editor) push() error {
	branch := e.branch()
	upstream := e.repository().git.status != nil && e.repository().git.status.Upstream != ""
	return e.gitDo(i18n.T("Pushing…"), func(r *git.Repo) error { return r.Push(branch, upstream) },
		func(e *Editor) { e.status = i18n.T("Pushed") })
}

// gitSync pulls (fast-forward only) and then pushes the branch checked out, in one background
// operation; a branch without an upstream is published (after Mayoi #53).
func (e *Editor) gitSync() error {
	st := e.repository().git.status
	if e.repository().git.repo == nil || st == nil || st.Branch == "" || st.Head == "" {
		return errors.New(i18n.T("Check out a branch with commits before syncing"))
	}
	if st.Upstream == "" {
		return e.push()
	}
	if err := e.saveDirtyInRepo(); err != nil {
		return err
	}
	upstream, ahead, pulled := st.Upstream, st.Ahead, 0
	return e.gitDo(i18n.T("Syncing…"), func(r *git.Repo) error {
		before := r.Head()
		if err := r.Sync(); err != nil {
			return err
		}
		if before != "" {
			pulled = r.Count(before, "HEAD")
		}
		return nil
	}, func(e *Editor) {
		e.checkDisk()
		if pulled == 0 && ahead == 0 {
			e.status = i18n.T("Already up to date")
		} else {
			e.status = i18n.F("Synced with %s", upstream)
		}
	})
}

func (e *Editor) pull() error {
	if err := e.saveDirtyInRepo(); err != nil {
		return err
	}
	return e.gitDo(i18n.T("Pulling…"), (*git.Repo).Pull, func(e *Editor) {
		e.status = i18n.T("Pulled")
		e.checkDisk()
	})
}

func (e *Editor) fetch() error {
	return e.gitDo(i18n.T("Fetching…"), (*git.Repo).Fetch, func(e *Editor) { e.status = i18n.T("Fetched") })
}

// switchBranch 切换到分支（远程分支检出一个跟踪它的本地分支）；create 为真时新建。
// 分支已经检出在另一个 worktree 里时，问要不要打开那个 worktree。
func (e *Editor) switchBranch(name string, create bool) error {
	if err := e.saveDirtyInRepo(); err != nil {
		return err
	}
	var occupied *git.WorktreeInUse
	return e.gitDo(i18n.T("Switching branch…"), func(r *git.Repo) error {
		err := r.Switch(name, create)
		if errors.As(err, &occupied) {
			return nil
		}
		return err
	}, func(e *Editor) {
		if occupied != nil {
			e.askOpenWorktree(occupied)
			return
		}
		e.status = i18n.F("Switched to %s", name)
		e.checkDisk()
	})
}

// askOpenWorktree 问要不要打开检出着 w.Branch 的那个 worktree，照着 Mayoi v0.7.0（worktree.go）：
// 打开它不会改动任何一边检出的分支。它的文件夹已经被删掉时，问要不要清理掉再在这里切换。
func (e *Editor) askOpenWorktree(w *git.WorktreeInUse) {
	r := e.repository().git.repo
	cancel := ovItem{label: i18n.T("Cancel"), run: func(*Editor) error { return nil }}
	if _, err := os.Stat(w.Path); errors.Is(err, os.ErrNotExist) {
		e.gitChoose(i18n.F("The worktree folder of branch %s no longer exists", w.Branch),
			ovItem{label: i18n.F("Prune the stale worktree and switch to %s here", w.Branch), detail: shortHome(w.Path), run: func(e *Editor) error {
				if e.repository().git.repo != r {
					return nil
				}
				return e.removeAndSwitch(w.Path, w.Branch)
			}}, cancel)
		return
	}
	e.gitChoose(i18n.F("Branch %s is already checked out in another worktree", w.Branch),
		ovItem{label: i18n.T("Open That Worktree in This Window"), detail: shortHome(w.Path), run: func(e *Editor) error {
			if e.repository().git.repo != r {
				return nil
			}
			if fi, err := os.Stat(w.Path); err != nil || !fi.IsDir() {
				return fmt.Errorf(i18n.T("The worktree is not a folder: %s"), w.Path)
			}
			return e.SwitchFolder(w.Path)
		}}, cancel)
}

// removeAndSwitch 让 git 忘掉文件夹已经没了的那个 worktree（它还占着 branch），再切换到 branch。
// 只清理这一个：Mayoi 用 git worktree prune，会连没插上的移动硬盘里的 worktree 一起忘掉。
func (e *Editor) removeAndSwitch(path, branch string) error {
	if err := e.saveDirtyInRepo(); err != nil {
		return err
	}
	return e.gitDo(i18n.T("Switching branch…"), func(r *git.Repo) error {
		removeErr := r.RemoveWorktree(path)
		err := r.Switch(branch, false)
		var occupied *git.WorktreeInUse
		if errors.As(err, &occupied) { // 锁住的 worktree 清理不掉
			return fmt.Errorf(i18n.T("Couldn't prune the worktree of %s (if it is locked, unlock it with git worktree unlock first): %w"), occupied.Path, removeErr)
		}
		return err
	}, func(e *Editor) {
		e.status = i18n.F("Switched to %s", branch)
		e.checkDisk()
	})
}

// openBranchPicker 打开切换分支的浮层：列出本地和远程分支，打一个新名字可以新建。
func (e *Editor) openBranchPicker() error {
	r := e.repository().git.repo
	if r == nil {
		return errors.New(i18n.T("Not in a Git repository"))
	}
	e.closeOverlay()
	e.ov = &overlay{kind: view.OverlayBranch, repository: e.repository()}
	ov := e.ov
	e.setOverlayInput("")
	post := e.postGit()
	go func() {
		bs, err := r.BranchList()
		post(func(e *Editor) {
			if err != nil {
				e.status = gitError(err)
				return
			}
			e.repository().scm.branches = bs
			if e.ov == ov {
				e.setOverlayInput(ov.input)
			}
		})
	}()
	return nil
}

// branchItems 列出符合输入的分支；输入不是已有分支名时，第一项是「新建分支」。
func (e *Editor) branchItems(q string) {
	ov := e.ov
	ov.items, ov.footer = ov.items[:0], ""
	q = strings.TrimSpace(q)
	cur := e.branch()
	exists := false
	for _, b := range e.repository().scm.branches {
		if q != "" && !strings.Contains(strings.ToLower(b.Name), strings.ToLower(q)) {
			continue
		}
		if b.Name == q {
			exists = true
		}
		name := b.Name
		detail := b.Subject
		if b.Name == cur {
			detail = i18n.F("Current branch · %s", detail)
		}
		remote := b.Remote
		ov.items = append(ov.items, ovItem{label: name, detail: detail, run: func(e *Editor) error {
			if name == cur {
				return nil
			}
			if remote { // 远程分支：检出同名的本地分支并跟踪它
				local := name[strings.IndexByte(name, '/')+1:]
				if slices.ContainsFunc(e.repository().scm.branches, func(b git.Branch) bool { return !b.Remote && b.Name == local }) {
					return e.switchBranch(local, false)
				}
				return e.switchBranch(local, false) // git switch 会自动从同名远程分支建跟踪分支
			}
			return e.switchBranch(name, false)
		}})
	}
	if q != "" && !exists && branchNameProblem(q, nil) == "" {
		name := q
		ov.items = slices.Insert(ov.items, 0, ovItem{label: i18n.F("Create branch %s", name), detail: i18n.F("From %s", cur), run: func(e *Editor) error {
			return e.switchBranch(name, true)
		}})
	}
	if q == "" { // 常驻的一项：问名字再新建（照着 Mayoi #60）
		ov.items = slices.Insert(ov.items, 0, ovItem{label: i18n.T("New Branch…"), detail: i18n.F("Create from %s and switch to it", cur), run: (*Editor).createBranchPrompt})
	}
	if len(e.repository().scm.branches) == 0 {
		ov.footer = i18n.T("Reading branches…")
	}
}

// ---- 键盘与前端的语义动作 ----

// showSide 显示侧边栏的某个视图并把焦点交给它。
func (e *Editor) showSide(v sideView) {
	if e.ws == nil && v != sideSCM && v != sideGuide {
		return
	}
	e.sidebar, e.side, e.area = true, v, areaTree
	e.closeOverlay()
}

// ShowSidebarView 显示侧边栏的视图：0 资源管理器、1 搜索、2 源代码管理（侧边栏顶上的按钮）。
func (e *Editor) ShowSidebarView(v int) {
	e.sync()
	defer e.sync()
	e.showSide(sideView(v))
}

// SCMClick 选中源代码管理的第 i 行并打开它的文件；open 为真（双击）时把焦点交给编辑器。
func (e *Editor) SCMClick(i int, open bool) {
	e.sync()
	defer e.sync()
	e.side, e.area, e.repository().scm.sel, e.repository().scm.focusMsg = sideSCM, areaTree, i, false
	e.scmClearPicked()
	if err := e.scmOpen(open); err != nil {
		e.status = err.Error()
	}
}

func (e *Editor) scmOpen(keep bool) error {
	if row, ok := e.selectedHistoryRow(); ok && row.File != nil {
		e.SCMCommitFile(row.Hash, row.File.Path, keep)
		return nil
	}
	if e.repository().git.repo == nil {
		if e.repository().scm.sel >= 0 && e.repository().scm.sel < len(e.repository().git.nested) {
			return e.openNestedRepository(e.repository().git.nested[e.repository().scm.sel])
		}
		return nil
	}
	if kind, row, ok := e.scmGroupOf(e.repository().scm.sel); ok && row.Folder { // 文件夹：折起或展开
		e.toggleSCMFolder(kind, row.Path)
		return nil
	}
	r, ok := e.scmSelected()
	if !ok {
		if i, ok := e.scmSelectedCommit(); ok {
			area := e.area
			if err := e.openCommitTab(i, !keep); err != nil {
				return err
			}
			if !keep {
				e.area = area // 点着一个个看：键盘留在列表里
			}
			return nil
		}
		return nil
	}
	// A gitlink belongs to the parent index; opening the child panel exposes its files.
	if en, ok := e.repository().git.entries[r.Path]; ok && en.Submodule() {
		return e.openAllChanges()
	}
	if en, ok := e.repository().git.entries[r.Path]; ok && en.Conflicted() {
		if err := e.openMerge(r.Path); err == nil {
			return nil
		}
	}
	if !keep && e.revealInChanges(r) { // 所有改动开着：滚到这个文件，不另开标签
		return nil
	}
	// 照着 Mayoi：点源代码管理里的文件，看它的改动。
	if err := e.openChange(r.Path, !keep); err != nil {
		return err
	}
	if keep {
		e.area = areaEditor
	}
	return nil
}

// SCMStage 暂存（unstage 为真时取消暂存）一个文件。
func (e *Editor) SCMStage(path string, unstage bool) {
	if err := e.stage([]string{path}, unstage); err != nil {
		e.status = err.Error()
	}
}

// SCMStageGroup 暂存或取消暂存一组的全部文件。
func (e *Editor) SCMStageGroup(kind view.SCMGroupKind) {
	for _, g := range e.scmAllGroups() {
		if g.Kind != kind {
			continue
		}
		var paths []string
		for _, r := range g.Rows {
			paths = append(paths, r.Path)
		}
		if err := e.stage(paths, kind == view.GroupStaged); err != nil {
			e.status = err.Error()
		}
	}
}

// SCMDiscard 放弃这些文件的修改（前端先问过用户）。
func (e *Editor) SCMDiscard(rows []view.SCMRow) {
	e.sync()
	defer e.sync()
	if err := e.discard(rows); err != nil {
		e.status = err.Error()
	}
}

// SetCommitMessage 用前端输入框里的文字替换提交说明。
func (e *Editor) SetCommitMessage(s string) { e.repository().scm.message = s }

// FocusCommitMessage 记下键盘在不在提交说明里。
func (e *Editor) FocusCommitMessage(on bool) {
	e.repository().scm.focusMsg = on
	if on {
		e.side, e.area = sideSCM, areaTree
	}
}

// scmToggleStage 暂存或取消暂存选中的文件（列表里按空格）；选中的是提交时，列出能对它做的事（同窗口版的右键）。
func (e *Editor) scmToggleStage() error {
	if rows := e.scmPicked(); len(rows) > 0 {
		return e.scmStagePicked(rows)
	}
	r, ok := e.scmSelected()
	if !ok {
		if i, ok := e.scmSelectedCommit(); ok {
			return e.openCommit(i)
		}
		return nil
	}
	if r.Folder {
		e.SCMStageFolder(e.repository().scm.sel, r.Staged)
		return nil
	}
	return e.stage([]string{r.Path}, r.Staged)
}

// scmDiscardSelected 放弃选中文件的修改：要按两次（会丢掉改动）。
func (e *Editor) scmDiscardSelected() error {
	if rows := e.scmPicked(); len(rows) > 0 {
		var changed []view.SCMRow
		for _, r := range rows {
			if !r.Staged && r.Letter != 'C' {
				changed = append(changed, r)
			}
		}
		if len(changed) == 0 {
			return errors.New(i18n.T("The selected files have no changes to discard (unstage staged ones first)"))
		}
		id := e.running
		key := fmt.Sprint(len(changed), changed[0].Path)
		if e.armed != id || e.armedRepository != e.repository().id || e.armedPath != key {
			e.armedRepository, e.armedPath, e.armed = e.repository().id, key, id
			return fmt.Errorf(i18n.T("Press %s again to discard changes to the %d selected files (can't be undone)"), e.hintOr(id, "Delete"), len(changed))
		}
		e.armed = ""
		return e.discard(changed)
	}
	r, ok := e.scmSelected()
	if !ok {
		return nil
	}
	id := e.running
	if e.armed != id || e.armedRepository != e.repository().id || e.armedPath != r.Path {
		e.armedRepository, e.armedPath = e.repository().id, r.Path
		e.armed = id
		return fmt.Errorf(i18n.T("Press %s again to discard changes to %s (can't be undone)"), e.hintOr(id, "Delete"), e.repositoryName(e.repository().git.repo.Root)+"/"+r.Name)
	}
	e.armed = ""
	if r.Folder {
		return e.discard(e.SCMFolderFiles(e.repository().scm.sel))
	}
	return e.discard([]view.SCMRow{r})
}

// handleMessageKey 处理键盘在提交说明里时的按键；返回 false 表示交给 keymap。
func (e *Editor) handleMessageKey(c keymap.Chord) bool {
	if c.Mods&^keymap.ModShift != 0 {
		return false // ⌘↵ 提交等交给 keymap
	}
	switch c.Key {
	case keymap.KeyEsc, keymap.KeyTab:
		e.repository().scm.focusMsg = false
	case keymap.KeyEnter:
		e.repository().scm.message += "\n"
	case keymap.KeyBackspace:
		if m := e.repository().scm.message; m != "" {
			_, n := utf8.DecodeLastRuneInString(m)
			e.repository().scm.message = m[:len(m)-n]
		}
	default:
		return c.Key == keymap.KeyRune
	}
	return true
}

// initRepo 在打开的文件夹里初始化 git 仓库（不在仓库里的文件夹，源代码管理里的按钮），然后盯着它。
func (e *Editor) initRepo() error {
	defer e.bindRepository(e.rootRepository())()
	if e.repository().git.repo != nil {
		return errors.New(i18n.T("Already in a Git repository"))
	}
	if e.root == "" {
		return errors.New(i18n.T("Open a folder first"))
	}
	root := e.root
	post := e.postGit()
	go func() {
		err := git.Init(root)
		post(func(e *Editor) {
			if err != nil {
				e.status = gitError(err)
				return
			}
			e.startGit(root)
			e.status = i18n.T("Initialized Git repository")
		})
	}()
	return nil
}

// ---- 以树的形式查看（照着 Mayoi 的 View as Tree） ----

// treeRows 把一组的文件按文件夹排成树：文件夹在前、文件在后，各按名字；只有一个子文件夹、没有文件的文件夹
// 和子文件夹连成一行；折起来的文件夹不列下面的东西。
func (e *Editor) treeRows(kind view.SCMGroupKind, files []view.SCMRow) []view.SCMRow {
	type node struct {
		name  string
		path  string
		dirs  map[string]*node
		files []view.SCMRow
	}
	root := &node{dirs: map[string]*node{}}
	r := e.repository().git.repo
	for _, f := range files {
		n := root
		if f.Dir != "" {
			for _, part := range strings.Split(f.Dir, "/") {
				c := n.dirs[part]
				if c == nil {
					c = &node{name: part, path: filepath.Join(n.path, part), dirs: map[string]*node{}}
					if n == root {
						c.path = part
					}
					n.dirs[part] = c
				}
				n = c
			}
		}
		n.files = append(n.files, f)
	}
	var out []view.SCMRow
	var walk func(n *node, depth int)
	walk = func(n *node, depth int) {
		names := make([]string, 0, len(n.dirs))
		for k := range n.dirs {
			names = append(names, k)
		}
		slices.Sort(names)
		for _, k := range names {
			c := n.dirs[k]
			label := c.name
			for len(c.files) == 0 && len(c.dirs) == 1 { // 只有一个子文件夹：连起来写
				for _, only := range c.dirs {
					label += "/" + only.name
					c = only
				}
			}
			abs := c.path
			if r != nil {
				abs = r.Abs(c.path)
			}
			closed := e.repository().scm.closed[fmt.Sprint(kind)+":"+abs]
			out = append(out, view.SCMRow{Path: abs, Name: label, Folder: true, Depth: depth, Collapsed: closed, Staged: kind == view.GroupStaged})
			if !closed {
				walk(c, depth+1)
			}
		}
		slices.SortFunc(n.files, func(a, b view.SCMRow) int { return strings.Compare(a.Name, b.Name) })
		for _, f := range n.files {
			f.Depth, f.Dir = depth, "" // 树里文件夹已经写在上面
			out = append(out, f)
		}
	}
	walk(root, 0)
	return out
}

// toggleSCMTree 换成以树的形式查看，或换回列表。
func (e *Editor) toggleSCMTree() error {
	selected, ok := e.selectedHistoryRow()
	defer func() {
		if ok {
			path := ""
			if selected.File != nil {
				path = selected.File.Path
			}
			e.selectHistoryRow(selected.Hash, "")
			e.selectHistoryRow(selected.Hash, path)
		}
	}()
	e.repository().scm.tree = !e.repository().scm.tree
	if e.repository().scm.tree {
		e.status = i18n.T("Source Control: View as Tree")
	} else {
		e.status = i18n.T("Source Control: View as List")
	}
	return nil
}

// scmGroupOf 是选中的第 i 行所在的分组和那一行。
func (e *Editor) scmGroupOf(i int) (view.SCMGroupKind, view.SCMRow, bool) {
	for _, g := range e.scmGroups() {
		if i < len(g.Rows) {
			return g.Kind, g.Rows[i], true
		}
		i -= len(g.Rows)
	}
	return 0, view.SCMRow{}, false
}

// folderFiles 是第 kind 组里文件夹 dir 下面的文件（不管折没折起来）。
func (e *Editor) folderFiles(kind view.SCMGroupKind, dir string) []view.SCMRow {
	tree := e.repository().scm.tree
	e.repository().scm.tree = false
	defer func() { e.repository().scm.tree = tree }()
	var out []view.SCMRow
	for _, g := range e.scmAllGroups() {
		if g.Kind != kind {
			continue
		}
		for _, r := range g.Rows {
			if strings.HasPrefix(r.Path, dir+string(filepath.Separator)) {
				out = append(out, r)
			}
		}
	}
	return out
}

// toggleSCMFolder 折起或展开树里的一个文件夹。
func (e *Editor) toggleSCMFolder(kind view.SCMGroupKind, dir string) {
	if e.repository().scm.closed == nil {
		e.repository().scm.closed = map[string]bool{}
	}
	k := fmt.Sprint(kind) + ":" + dir
	e.repository().scm.closed[k] = !e.repository().scm.closed[k]
}

// SCMFolderFiles 是第 i 行（一个文件夹）下面这一组的文件：前端放弃这些文件的修改前用它问一声。
func (e *Editor) SCMFolderFiles(i int) []view.SCMRow {
	kind, r, ok := e.scmGroupOf(i)
	if !ok || !r.Folder {
		return nil
	}
	return e.folderFiles(kind, r.Path)
}

// SCMStageFolder 暂存（unstage 为真时取消暂存）第 i 行这个文件夹下面这一组的文件。
func (e *Editor) SCMStageFolder(i int, unstage bool) {
	var paths []string
	for _, r := range e.SCMFolderFiles(i) {
		paths = append(paths, r.Path)
	}
	if len(paths) > 0 {
		if err := e.stage(paths, unstage); err != nil {
			e.status = err.Error()
		}
	}
}

func (e *Editor) checkRepositoryPaths(paths []string) error {
	r := e.repository().git.repo
	if r == nil || e.repository().dead {
		return errors.New(i18n.T("This repository is no longer available"))
	}
	for _, path := range paths {
		if !r.Contains(path) {
			return errors.New(i18n.T("The file doesn't belong to the selected repository; select again"))
		}
	}
	return nil
}
