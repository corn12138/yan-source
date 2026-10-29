package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/view"
)

// gitRepo 在临时目录里建一个仓库，提交 files。
func gitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("没有 git")
	}
	// 编辑器自己提交时也要有身份：CI 这类没配过 git 的机器上没有全局的 user.name。
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(k, "t")
	}
	for _, k := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "t@t")
	}
	root := t.TempDir()
	for p, c := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(c), 0o644)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"}, {"add", "."}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "first"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return root
}

func TestGitStatusAndMarks(t *testing.T) {
	root := gitRepo(t, map[string]string{"src/a.go": "package a\n\nfunc A() {}\n", "b.txt": "b\n"})
	os.WriteFile(filepath.Join(root, "new.txt"), []byte("new\n"), 0o644)
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil })
	if got := e.branch(); got != "main" {
		t.Fatalf("branch = %q", got)
	}
	var f view.Frame
	e.Frame(&f)
	letters := map[string]byte{}
	for _, r := range f.Sidebar.Rows {
		letters[r.Name] = r.Git
	}
	if letters["new.txt"] != 'U' || letters["src"] != 0 {
		t.Fatalf("letters = %v", letters)
	}
	// 改一行、加一行：行号旁有标记
	e.OpenFile(filepath.Join(root, "src/a.go"))
	e.GotoLine(3)
	typeText(e, "// x\n")
	e.Run("workbench.action.files.save")
	pump(t, e, func() bool {
		e.Frame(&f)
		return f.Columns[0].Pane.Rows[2].Mark == view.MarkAdded
	})
	pump(t, e, func() bool {
		e.Frame(&f)
		for _, r := range f.Sidebar.Rows {
			if r.Name == "src" {
				return r.Git == 'M'
			}
		}
		return false
	})
	if f.Status.Branch != "main" {
		t.Fatalf("status branch = %q", f.Status.Branch)
	}
	// ⌥F5 跳到改动
	e.GotoLine(1)
	e.Run("workbench.action.editor.nextChange")
	s := e.cur().doc.buf.Snapshot()
	if line := s.LineAt(e.cur().doc.buf.Selections().Main().Head); line != 2 {
		t.Fatalf("next change at line %d", line)
	}
}

func TestBlameText(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	if got := relTime(now.Add(-3*24*time.Hour), now); got != "3 天前" {
		t.Fatalf("relTime = %q", got)
	}
	if !strings.Contains(relTime(now.Add(-30*time.Second), now), "刚才") {
		t.Fatal("seconds should read as just now")
	}
}

// newEditorIn 打开文件夹 root。
func newEditorIn(t *testing.T, root string) *Editor {
	t.Helper()
	e, _ := newFolderEditorAt(t, root)
	return e
}

func TestSCMStageCommitAndBranch(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("a2\n"), 0o644)
	os.WriteFile(filepath.Join(root, "c.txt"), []byte("c\n"), 0o644)
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 2 })
	e.Run("workbench.view.scm")
	var f view.Frame
	e.Frame(&f)
	scm := f.Sidebar.SCM
	if f.Sidebar.View != view.SideSCM || len(scm.Groups) != 2 || scm.Groups[0].Title != "更改" || scm.CommitLabel != "全部提交" {
		t.Fatalf("scm = %+v", scm)
	}
	// 空格暂存选中的 a.txt
	e.Handle(KeyInput{keymap.Chord{Key: keymap.KeyRune, Rune: ' '}})
	pump(t, e, func() bool { return e.stagedCount() == 1 })
	e.Frame(&f)
	if f.Sidebar.SCM.Groups[0].Kind != view.GroupStaged || f.Sidebar.SCM.CommitLabel != "提交" {
		t.Fatalf("after stage: %+v", f.Sidebar.SCM)
	}
	// Tab 到提交说明，写说明，⌘↵ 提交：只提交暂存的 a.txt，c.txt 仍是未跟踪
	e.Handle(key(keymap.KeyTab, 0))
	typeText(e, "改 a")
	e.Handle(KeyInput{keymap.Chord{Mods: keymap.ModCtrl, Key: keymap.KeyEnter}})
	pump(t, e, func() bool { return e.scm.message == "" && e.changeCount() == 1 })
	out, _ := exec.Command("git", "-C", root, "log", "--format=%s").Output()
	if !strings.HasPrefix(string(out), "改 a\n") {
		t.Fatalf("log = %q", out)
	}
	// 新建并切到分支
	e.Run("git.checkout")
	pump(t, e, func() bool { return len(e.scm.branches) > 0 })
	typeText(e, "feature")
	if e.ov.items[0].label != "新建分支 feature" {
		t.Fatalf("items = %+v", e.ov.items)
	}
	e.Handle(key(keymap.KeyEnter, 0))
	pump(t, e, func() bool { return e.branch() == "feature" })
}

func TestDiffView(t *testing.T) {
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	root := gitRepo(t, map[string]string{"a.txt": strings.Join(lines, "\n") + "\n"})
	lines[9] = "line 10 changed"
	lines = append(lines[:15], append([]string{"inserted"}, lines[15:]...)...)
	os.WriteFile(filepath.Join(root, "a.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 1 })
	e.Run("workbench.view.scm")
	e.SCMClick(0, false)
	tabs := tabNames(e)
	if len(tabs) != 1 || tabs[0] != "~a.txt (改动)*" {
		t.Fatalf("tabs = %v", tabs)
	}
	txt := content(e)
	for _, want := range []string{"⋯ 3 行没改", "@@ -10,1 +10,1 @@", "line 10\nline 10 changed", "@@ -16,0 +16,1 @@\ninserted"} {
		if !strings.Contains(txt, want) {
			t.Fatalf("missing %q in:\n%s", want, txt)
		}
	}
	var f view.Frame
	e.FocusColumn(0)
	e.Frame(&f)
	kinds := map[string]bool{}
	for _, r := range f.Columns[0].Pane.Rows {
		switch r.Base {
		case style.DiffAdded:
			kinds["+"+string(r.Text)] = true
		case style.DiffRemoved:
			kinds["-"+string(r.Text)] = true
		}
	}
	if !kinds["-line 10"] || !kinds["+line 10 changed"] || !kinds["+inserted"] {
		t.Fatalf("kinds = %v", kinds)
	}
	// 在「line 10 changed」上回车：打开文件跳到第 10 行
	at := strings.Index(txt, "line 10 changed")
	e.cur().doc.buf.SetSelections(edit.Single(edit.Cursor(at)))
	e.Handle(key(keymap.KeyEnter, 0))
	if got := tabNames(e); len(got) != 2 || !strings.HasPrefix(got[1], "a.txt*") {
		t.Fatalf("tabs = %v", got)
	}
	if s := e.cur().doc.buf.Snapshot(); s.LineAt(e.cur().doc.buf.Selections().Main().Head) != 9 {
		t.Fatal("should jump to line 10")
	}
}

// 切到检出在另一个 worktree 里的分支：问一声，选了就在这个窗口打开那个 worktree，两边的分支都不动。
// 那个 worktree 的文件夹被删掉了：问要不要清理掉，再在这里切换。
func TestSwitchToBranchInAnotherWorktree(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "a\n"})
	gitIn := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	wt := filepath.Join(t.TempDir(), "topic")
	gitIn("worktree", "add", "-q", "-b", "topic", wt)
	wt, _ = filepath.EvalSymlinks(wt)
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil })
	if err := e.switchBranch("topic", false); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.ov != nil })
	if e.ov.kind != view.OverlayChoice || !strings.HasPrefix(e.ov.items[0].label, "在这个窗口打开") {
		t.Fatalf("overlay = %v %+v", e.ov.kind, e.ov.items)
	}
	e.Handle(key(keymap.KeyEnter, 0))
	if got, _ := filepath.EvalSymlinks(e.root); got != wt {
		t.Fatalf("root = %q, want %q", got, wt)
	}
	pump(t, e, func() bool { return e.git.status != nil && e.branch() == "topic" })

	// 回到主仓库，删掉 worktree 的文件夹
	if err := e.SwitchFolder(root); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.git.status != nil && e.branch() == "main" })
	// 另一个文件夹不在的 worktree（像没插上的移动硬盘）：清理 topic 的时候不能连它一起忘掉
	other := filepath.Join(t.TempDir(), "unmounted")
	gitIn("worktree", "add", "-q", "-b", "other", other)
	os.RemoveAll(other)
	os.RemoveAll(wt)
	if err := e.switchBranch("topic", false); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.ov != nil })
	if e.ov.kind != view.OverlayChoice || !strings.HasPrefix(e.ov.items[0].label, "清理失效的 worktree") {
		t.Fatalf("overlay = %v %+v", e.ov.kind, e.ov.items)
	}
	e.Handle(key(keymap.KeyEnter, 0))
	pump(t, e, func() bool { return e.git.busy == "" && e.git.status != nil && e.branch() == "topic" })
	if got, err := e.git.repo.WorktreeForBranch("other"); err != nil || got == "" {
		t.Fatalf("the other missing worktree was forgotten too: %q, %v", got, err)
	}
}

// 新分支、新标签的名字：空的和以 - 开头的（git 会当成选项）不交给 git。
func TestRefName(t *testing.T) {
	for in, want := range map[string]string{" v1.2 ": "v1.2", "topic/x": "topic/x", "": "", "  ": "", "-f": "", "--force": ""} {
		got, err := refName(in)
		if got != want || (err == nil) != (want != "") {
			t.Errorf("refName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

// 提交历史里的「多久以前」过了一分钟就按现在重写，上一帧拿到的切片不动。
func TestHistoryWhenRefreshes(t *testing.T) {
	now := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	h := historyState{
		commits: []git.Commit{{Time: now.Add(-3 * time.Hour)}},
		rows:    []view.SCMCommit{{When: relTime(now.Add(-3*time.Hour), now.Add(-3*time.Hour))}},
		whenAt:  now.Add(-3 * time.Hour),
	}
	old := h.rows
	h.refreshWhen(now.Add(-3*time.Hour + 30*time.Second))
	if &h.rows[0] != &old[0] {
		t.Fatal("rewrote the times within a minute")
	}
	h.refreshWhen(now)
	if want := relTime(now.Add(-3*time.Hour), now); h.rows[0].When != want || old[0].When == want {
		t.Fatalf("when = %q (old %q), want %q", h.rows[0].When, old[0].When, want)
	}
}

// 源代码管理里的提交历史：最近的提交带着提交图和分支；打开一个提交列出它改的文件，选一个看改动。
// 显示所有分支时，别的分支上的提交也在，各在一道。
func TestCommitHistory(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "one\n"})
	day := 0
	gitIn := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		cmd.Dir = root
		// 每个提交晚一天：按日期排的历史才有确定的顺序
		day++
		date := fmt.Sprintf("2030-01-%02dT12:00:00Z", day)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitIn("switch", "-q", "-c", "side")
	os.WriteFile(filepath.Join(root, "b.txt"), []byte("b\n"), 0o644)
	gitIn("add", "b.txt")
	gitIn("commit", "-q", "-m", "on side")
	gitIn("switch", "-q", "main")
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\ntwo\n"), 0o644)
	gitIn("commit", "-q", "-am", "second")

	e := newEditorIn(t, root)
	e.Run("workbench.view.scm")
	var f view.Frame
	pump(t, e, func() bool { e.Frame(&f); return len(f.Sidebar.SCM.Commits) == 2 })
	cs := f.Sidebar.SCM.Commits
	if cs[0].Subject != "second" || !cs[0].Head || cs[1].Subject != "first" || cs[0].Lane != 0 {
		t.Fatalf("commits %+v", cs)
	}
	if len(cs[0].Refs) != 1 || cs[0].Refs[0].Name != "main" || !cs[0].Refs[0].Current {
		t.Fatalf("refs %+v", cs[0].Refs)
	}
	// 没有改动：第一行就是第一个提交。点它在预览标签里打开；右键的「检出、合并…」里能选它改的文件
	e.SCMClick(0, false)
	if tb := e.cur(); tb == nil || tb.diff == nil || tb.diff.commit == nil || e.area != areaTree {
		t.Fatalf("a click should preview the commit and keep the list focused (area %v)", e.area)
	}
	e.SCMCommitActions(0)
	pump(t, e, func() bool { return e.ov != nil })
	if e.ov.kind != view.OverlayChoice || e.ov.items[0].label != "a.txt" {
		t.Fatalf("overlay %+v", e.ov.items)
	}
	e.Handle(key(keymap.KeyEnter, 0))
	pump(t, e, func() bool { e.Frame(&f); return len(f.Columns[0].Tabs) == 2 }) // 提交的预览标签旁边
	if name := f.Columns[0].Tabs[1].Name; name != "a.txt ("+cs[0].Short+")" {
		t.Fatalf("tab %q", name)
	}
	var added []string
	for _, r := range f.Columns[0].Pane.Rows {
		if r.Base == style.DiffAdded {
			added = append(added, string(r.Text))
		}
	}
	if strings.Join(added, ",") != "two" {
		t.Fatalf("added %q", added)
	}
	// 提交里的改动：行号是那个提交里的，回车不跳到现在的文件
	if err := e.diffOpen(e.cur()); err == nil {
		t.Fatal("Enter in a commit's diff should not open the current file")
	}
	if e.Frame(&f); len(f.Columns[0].Tabs) != 2 {
		t.Fatalf("tabs %d", len(f.Columns[0].Tabs))
	}
	// 所有分支：side 上的提交也在，在第二道
	e.Run("yan.scm.toggleAllBranches")
	pump(t, e, func() bool { e.Frame(&f); return len(f.Sidebar.SCM.Commits) == 3 })
	var side view.SCMCommit
	for _, c := range f.Sidebar.SCM.Commits {
		if c.Subject == "on side" {
			side = c
		}
	}
	if side.Lane != 1 || f.Sidebar.SCM.Lanes != 2 || !f.Sidebar.SCM.AllBranches {
		t.Fatalf("side %+v, lanes %d", side, f.Sidebar.SCM.Lanes)
	}
}

// 不在仓库里的文件夹：初始化仓库后就在 git 里了。提交之后状态栏说一声，旁边是「撤销」；按它，提交拿回来。
// 「显示更多提交」每次多读一页。
func TestInitCommitUndoAndMoreCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("没有 git")
	}
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(k, "t")
	}
	for _, k := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "t@t")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o644)
	e := newEditorIn(t, root)
	var f view.Frame
	e.Run("workbench.view.scm")
	if e.Frame(&f); !f.Sidebar.SCM.NoRepo {
		t.Fatal("a plain folder is in a repository")
	}
	e.Run("git.init")
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 1 })

	e.scm.message = "第一个提交"
	if err := e.commit(false); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.git.busy == "" && e.status == "已提交" })
	if e.Frame(&f); f.Status.Action.Command != "git.undoCommit" || f.Status.Action.Label != "撤销" {
		t.Fatalf("status %q, action %+v", f.Status.Message, f.Status.Action)
	}
	e.Run(f.Status.Action.Command)
	pump(t, e, func() bool { return e.git.busy == "" && e.scm.message != "" })
	if out, err := exec.Command("git", "-C", root, "rev-parse", "--verify", "-q", "HEAD").CombinedOutput(); err == nil {
		t.Fatalf("HEAD still there after undo: %s", out)
	}

	if e.historyLimit() != historyPage {
		t.Fatal("first page")
	}
	e.Run("yan.scm.moreCommits")
	if e.historyLimit() != 2*historyPage {
		t.Fatalf("after more: %d", e.historyLimit())
	}
}

// 以树的形式查看：文件按文件夹排，只有一个子文件夹的连起来写；点文件夹折起；空格暂存文件夹下面这一组的文件。
func TestSCMTree(t *testing.T) {
	root := gitRepo(t, map[string]string{"internal/git/a.go": "package git\n", "internal/git/b.go": "package git\n", "top.txt": "t\n"})
	for _, p := range []string{"internal/git/a.go", "internal/git/b.go", "top.txt"} {
		os.WriteFile(filepath.Join(root, p), []byte("changed\n"), 0o644)
	}
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 3 })
	e.Run("workbench.view.scm")
	e.Run("yan.scm.toggleTree")
	names := func() string {
		var ns []string
		for _, g := range e.scmGroups() {
			for _, r := range g.Rows {
				ns = append(ns, strings.Repeat(" ", r.Depth)+r.Name)
			}
		}
		return strings.Join(ns, "|")
	}
	if got := names(); got != "internal/git| a.go| b.go|top.txt" {
		t.Fatalf("tree %q", got)
	}
	e.SCMClick(0, false) // 折起 internal/git
	if got := names(); got != "internal/git|top.txt" {
		t.Fatalf("folded %q", got)
	}
	e.scm.sel = 0
	e.Handle(KeyInput{keymap.Chord{Key: keymap.KeyRune, Rune: ' '}}) // 暂存这个文件夹
	pump(t, e, func() bool { return e.stagedCount() == 2 })
	out, _ := exec.Command("git", "-C", root, "diff", "--cached", "--name-only").Output()
	if strings.TrimSpace(string(out)) != "internal/git/a.go\ninternal/git/b.go" {
		t.Fatalf("staged %q", out)
	}
}
