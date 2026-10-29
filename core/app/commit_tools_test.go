package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

func commitToolRepo(t *testing.T) (*Editor, func(...string) string, git.Commit, git.Commit) {
	t.Helper()
	root := gitRepo(t, map[string]string{"a.txt": "old\nkeep\n", "rename.txt": "rename contents\n", "deleted.txt": "removed\n"})
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	r, err := git.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	first, err := r.CommitInfo("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("new\nkeep\n"), 0o644)
	os.WriteFile(filepath.Join(root, "binary"), []byte{0, 1, 2, 3}, 0o644)
	run("mv", "rename.txt", "renamed.txt")
	run("rm", "deleted.txt")
	run("add", ".")
	run("commit", "-qm", "second")
	second, err := r.CommitInfo("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil })
	return e, run, first, second
}
func loadedComparison(t *testing.T, e *Editor, from, to string) *tab {
	t.Helper()
	if err := e.compareRevisions(from, to); err != nil {
		t.Fatal(err)
	}
	var tb *tab
	pump(t, e, func() bool {
		tb = e.cur()
		return e.git.busy == "" && tb != nil && tb.diff != nil && tb.diff.compareFrom != nil && !tb.diff.loading
	})
	if tb.diff.loadError != "" {
		t.Fatal(tb.diff.loadError)
	}
	return tb
}
func tabText(t *tab) string { s := t.doc.buf.Snapshot(); return s.String(0, s.Len()) }

func TestCommitComparisonImmutableRestoreAndReview(t *testing.T) {
	e, run, first, second := commitToolRepo(t)
	statePath := filepath.Join(t.TempDir(), "state.json")
	e.SetState(config.LoadState(statePath))
	tb := loadedComparison(t, e, first.Hash, "HEAD")
	dt := tb.diff
	if dt.compareFrom.Hash != first.Hash || dt.commit.Hash != second.Hash {
		t.Fatal("comparison direction lost")
	}
	for _, want := range []string{"起点 " + first.Hash, "终点 " + second.Hash, "rename.txt → renamed.txt", "二进制文件", "deleted.txt", "\nold\n", "\nnew\n"} {
		if !strings.Contains(tabText(tb), want) {
			t.Fatalf("missing %q in %s", want, tabText(tb))
		}
	}
	if err := e.diffStage(tb, 4); err == nil {
		t.Fatal("comparison allowed staging")
	}
	if run("status", "--porcelain") != "" {
		t.Fatal("comparison changed worktree")
	}
	e.diffGo(tb, 1)
	if err := e.diffOpen(tb); err != nil {
		t.Fatal(err)
	}
	if clip, _ := e.host.Clipboard(); clip != first.Hash {
		t.Fatal("did not copy comparison origin")
	}
	e.diffGo(tb, 2)
	e.diffOpen(tb)
	if clip, _ := e.host.Clipboard(); clip != second.Hash {
		t.Fatal("did not copy comparison target")
	}
	// The review identity includes both revisions, so another base cannot reuse its notes.
	for i, l := range dt.lines {
		if l.kind == diffAdded && filepath.Base(dt.pathOf(l)) == "a.txt" {
			e.diffGo(tb, i)
			break
		}
	}
	if err := e.addReviewComment(); err != nil {
		t.Fatal(err)
	}
	e.SetReviewInput("Check the new value")
	if err := e.saveReview(); err != nil {
		t.Fatal(err)
	}
	if e.review.notes[0].Commit != first.Hash+".."+second.Hash {
		t.Fatal("review lost comparison identity")
	}
	run("commit", "--allow-empty", "-qm", "third")
	e.gitChanged()
	pump(t, e, func() bool { return e.git.status.Head != second.Hash })
	if dt.commit.Hash != second.Hash {
		t.Fatal("moving HEAD changed comparison")
	}
	e.toggleAllBranches()
	e.SaveSession()
	e2 := newEditorIn(t, e.root)
	e2.SetState(config.LoadState(statePath))
	if !e2.RestoreSession() {
		t.Fatal("no restored comparison")
	}
	pump(t, e2, func() bool { return e2.cur() != nil && e2.cur().diff != nil && !e2.cur().diff.loading })
	if e2.cur().diff.commit.Hash != second.Hash || e2.cur().diff.compareFrom.Hash != first.Hash || !e2.allBranches {
		t.Fatal("restored history changed")
	}
	if !strings.Contains(tabText(e2.cur()), "Check the new value") {
		t.Fatal("review did not return in comparison")
	}
	reverse := loadedComparison(t, e, second.Hash, first.Hash)
	for _, sec := range reverse.diff.secs {
		if filepath.Base(sec.path) == "a.txt" && (sec.base[0] != "new" || sec.cur[0] != "old") {
			t.Fatal("reverse comparison has forward contents")
		}
	}

	if strings.Contains(tabText(reverse), "Check the new value") {
		t.Fatal("review leaked into reverse comparison")
	}
	if e.reviewContext(reverse.diff, 0).Commit == e.review.notes[0].Commit {
		t.Fatal("comparison identities collide")
	}
	if err := e.compareRevisions(first.Hash, first.Hash); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.git.busy == "" })
	if !strings.Contains(e.status, "同一个提交") {
		t.Fatal(e.status)
	}
}

func TestCommitToolsPatchExportAndScope(t *testing.T) {
	e, run, first, second := commitToolRepo(t)
	if err := e.commitPatch(second, false); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.git.busy == "" })
	patch, _ := e.host.Clipboard()
	if !strings.Contains(patch, "GIT binary patch") || !strings.Contains(patch, "+new") {
		t.Fatalf("patch %q", patch)
	}
	path := filepath.Join(t.TempDir(), "result.patch")
	if err := e.writePatch(path, patch, false); err != nil {
		t.Fatal(err)
	}
	if err := e.writePatch(path, "overwrite", false); err == nil {
		t.Fatal("overwrote existing file without confirmation")
	}
	run("apply", "--reverse", "--check", path)
	if err := e.commitPatch(second, true); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.ov != nil })
	e.ov.input = filepath.Join(t.TempDir(), "export.patch")
	e.setOverlayInput(e.ov.input)
	e.Handle(key(keymap.KeyEnter, 0))
	if !strings.Contains(e.status, "补丁已保存") {
		t.Fatal(e.status)
	}
	if err := e.selectComparison(first); err != nil {
		t.Fatal(err)
	}
	if err := e.SwitchFolder(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := e.compareSelectedCommit(second); err == nil {
		t.Fatal("comparison selection leaked across projects")
	}
}

type patchDialogHost struct {
	fakeHost
	done func(string, error)
}

func (h *patchDialogHost) SavePatchDialog(_ string, done func(string, error)) { h.done = done }
func TestCommitPatchDialogCancelAndWorkspaceFence(t *testing.T) {
	e, _, _, second := commitToolRepo(t)
	h := &patchDialogHost{}
	e.host = h
	e.commitPatch(second, true)
	pump(t, e, func() bool { return h.done != nil })
	h.done("", nil)
	e.RunPending()
	if strings.Contains(e.status, "补丁已保存") {
		t.Fatal("cancel reported saved")
	}
	h.done = nil
	e.commitPatch(second, true)
	pump(t, e, func() bool { return h.done != nil })
	if err := e.SwitchFolder(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "late.patch")
	h.done(path, nil)
	e.RunPending()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("late dialog wrote after switching workspace")
	}
}

func TestCommitComparisonMissingObject(t *testing.T) {
	e, _, first, _ := commitToolRepo(t)
	tb := e.showHistoryTab(first.Hash, strings.Repeat("a", 40), false)
	pump(t, e, func() bool { return !tb.diff.loading })
	if tb.diff.loadError == "" || !strings.Contains(tabText(tb), "读取失败") || strings.Contains(tabText(tb), "没有改动") {
		t.Fatalf("missing commit misleading: %s", tabText(tb))
	}
}

func TestCommitSessionColumnsAndNonRepository(t *testing.T) {
	e, _, first, second := commitToolRepo(t)
	path := filepath.Join(t.TempDir(), "state.json")
	e.SetState(config.LoadState(path))
	e.OpenFile(filepath.Join(e.root, "a.txt"))
	e.cols = append(e.cols, newColumn())
	e.col = 1
	loadedComparison(t, e, first.Hash, second.Hash)
	e.showHistoryTab("", first.Hash, true)
	pump(t, e, func() bool { return !e.cur().diff.loading })
	e.FocusColumn(0)
	e.SaveSession()
	e2 := newEditorIn(t, e.root)
	e2.SetState(config.LoadState(path))
	if !e2.RestoreSession() {
		t.Fatal("no session restored")
	}
	pump(t, e2, func() bool {
		return len(e2.cols) == 2 && len(e2.cols[1].tabs) == 2 && !e2.cols[1].tabs[0].diff.loading && !e2.cols[1].tabs[1].diff.loading
	})
	if e2.col != 0 || e2.cols[1].active != 1 || !e2.cols[1].tabs[1].preview {
		t.Fatal("restored history stole focus or lost active preview")
	}
	if e2.cols[1].tabs[0].diff.compareFrom.Hash != first.Hash || e2.cols[1].tabs[1].diff.commit.Hash != first.Hash {
		t.Fatal("wrong restored revisions")
	}
	// A removed .git directory leaves an honest error tab, rather than dropping it or spinning forever.
	empty := t.TempDir()
	state := config.LoadState(filepath.Join(t.TempDir(), "empty.json"))
	state.Sessions = map[string]config.Session{empty: {Columns: []config.SessionColumn{{Tabs: []config.SessionTab{{Revision: second.Hash, CompareFrom: first.Hash}}}}}}
	e3 := newEditorIn(t, empty)
	e3.SetState(state)
	if !e3.RestoreSession() {
		t.Fatal("error tab dropped")
	}
	pump(t, e3, func() bool { return !e3.cur().diff.loading })
	if !strings.Contains(tabText(e3.cur()), "已不在 Git 仓库") {
		t.Fatal(tabText(e3.cur()))
	}
}

func TestCommitMenuUsesHashAfterHistoryRefresh(t *testing.T) {
	e, _, first, second := commitToolRepo(t)
	e.scm.history.commits = []git.Commit{first, second}
	e.scm.history.rows = []view.SCMCommit{{Hash: first.Hash}, {Hash: second.Hash}}
	e.SCMCommitCommand(second.Hash, "yan.git.selectCompare")
	if e.repository().scm.compareSelected != second.Hash {
		t.Fatal("menu selected a row instead of its commit")
	}
	e.SCMCommitCommand(strings.Repeat("f", 40), "yan.git.selectCompare")
	if e.repository().scm.compareSelected != second.Hash || !strings.Contains(e.status, "重新选择") {
		t.Fatal("stale menu silently targeted another commit")
	}
}
