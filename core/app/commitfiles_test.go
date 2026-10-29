package app

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/git"
)

func TestCommitFileTreeNavigationAndIdentity(t *testing.T) {
	e, run, first, second := commitToolRepo(t)
	e.Run("workbench.view.scm")
	pump(t, e, func() bool { return len(e.scm.history.commits) == 2 })
	e.SCMToggleCommitFiles(second.Hash)
	pump(t, e, func() bool { return e.scm.history.files[second.Hash].loaded })
	rows := e.shownCommits()
	if len(rows) != 6 {
		t.Fatalf("expanded rows=%d", len(rows))
	}
	e.SCMCommitFile(second.Hash, filepath.Join(e.root, "deleted.txt"), false)
	tb := e.cur()
	pump(t, e, func() bool { return !tb.diff.loading })
	cf := e.commitFilesFrame(tb)
	if cf.Visible || cf.Count != 4 || cf.Selected != filepath.Join(e.root, "deleted.txt") || !tb.preview {
		t.Fatalf("preview=%v files=%+v", tb.preview, cf)
	}
	state := filepath.Join(t.TempDir(), "state.json")
	e.SetState(config.LoadState(state))
	e.Run("git.toggleCommitFiles")
	e.SetCommitFilesWidth(280)
	for _, name := range []string{"binary", "renamed.txt", "a.txt"} {
		path := filepath.Join(e.root, name)
		e.CommitFileAction(e.col, tb.diff.path, path, true)
		if e.commitFilesFrame(tb).Selected != path || tb.preview {
			t.Fatalf("did not pin/jump to %s", name)
		}
		if l := tb.diff.lines[tb.scrollY]; l.kind != diffFile || tb.diff.pathOf(l) != path {
			t.Fatalf("wrong header %s", name)
		}
	}
	stored := config.LoadState(state)
	if !stored.CommitFilesOpen || stored.CommitFilesWidth != 280 {
		t.Fatalf("state=%+v", stored)
	}
	e.SCMCommitCommand(first.Hash, "yan.git.selectCompare")
	if e.ComparisonStart() != first.Hash {
		t.Fatal("expanded children retargeted commit menu")
	}
	e.selectHistoryRow(second.Hash, filepath.Join(e.root, "a.txt"))
	e.side = sideSCM
	e.Run("list.collapse")
	if r, _ := e.selectedHistoryRow(); r.File != nil || r.Hash != second.Hash {
		t.Fatal("left did not select parent commit")
	}
	e.Run("list.collapse")
	if len(e.shownCommits()) != 2 {
		t.Fatal("left did not collapse commit")
	}
	e.Run("list.expand")
	if len(e.shownCommits()) != 6 {
		t.Fatal("right did not expand cached files")
	}
	identity := tb.diff.path
	e.openCommitTab(1, false)
	other := e.cur()
	before := other.scrollY
	e.CommitFileAction(e.col, identity, filepath.Join(e.root, "binary"), true)
	if other.scrollY != before || other.diff.commit.Hash != first.Hash {
		t.Fatal("stale tab action changed replacement tab")
	}
	if run("status", "--porcelain") != "" {
		t.Fatal("history navigation changed worktree")
	}
}

func TestCommitFileFoldersAndScroll(t *testing.T) {
	root := gitRepo(t, map[string]string{"src/a.txt": "a\n", "src/nested/b.txt": "b\n", "z.txt": "z\n"})
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return len(e.scm.history.commits) > 0 })
	e.Run("yan.scm.toggleTree")
	e.openCommitTab(0, false)
	tb := e.cur()
	pump(t, e, func() bool { return !tb.diff.loading })
	cf := e.commitFilesFrame(tb)
	if len(cf.Rows) != 5 || !cf.Rows[0].Folder || cf.Rows[0].Name != "src" {
		t.Fatalf("rows=%+v", cf.Rows)
	}
	folder := filepath.Join(root, "src")
	e.CommitFileAction(0, cf.Identity, folder, false)
	if rows := e.commitFilesFrame(tb).Rows; len(rows) != 2 || !rows[0].Collapsed {
		t.Fatalf("fold=%+v", rows)
	}
	e.CommitFileAction(0, cf.Identity, folder, false)
	for i, l := range tb.diff.lines {
		if l.kind == diffFile && tb.diff.pathOf(l) == filepath.Join(root, "z.txt") {
			tb.scrollY = i
			tb.diff.rightTop = i
			break
		}
	}
	if e.commitFilesFrame(tb).Selected != filepath.Join(root, "z.txt") {
		t.Fatal("tree did not follow scroll")
	}
	e.SCMCommitCommand(e.scm.history.commits[0].Hash, "noop")
	e.SCMToggleCommitFiles(e.scm.history.commits[0].Hash)
	pump(t, e, func() bool { return e.scm.history.files[tb.diff.commit.Hash].loaded })
	e.SCMCommitFile(tb.diff.commit.Hash, folder, false)
	rows := e.shownCommits()
	if len(rows) != 3 || rows[1].File == nil || !rows[1].File.Collapsed {
		t.Fatalf("history fold=%+v", rows)
	}
}

func TestCommitFilesStayWithRepository(t *testing.T) {
	e, a, b := multiEditor(t)
	ac, bc := e.repositoryForRoot(a), e.repositoryForRoot(b)
	hash := ac.scm.history.commits[0].Hash
	e.WithRepository(ac.id, func(e *Editor) { e.SCMToggleCommitFiles(hash) })
	e.repositories.selected = b
	pump(t, e, func() bool { return ac.scm.history.files[hash].loaded })
	if len(bc.scm.history.files) != 0 {
		t.Fatal("load applied to another repo")
	}
	e.WithRepository(ac.id, func(e *Editor) { e.SCMCommitFile(hash, filepath.Join(a, "file.txt"), false) })
	tb := e.cur()
	if tb == nil || tb.diff == nil {
		t.Fatal("source commit did not open")
	}
	pump(t, e, func() bool { return !tb.diff.loading })
	if tb.diff.repoRoot != a || e.commitFilesFrame(tb).Selected != filepath.Join(a, "file.txt") {
		t.Fatal("jump escaped original repository")
	}
	e.CommitFilesToggleTree(e.col, tb.diff.path)
	if !ac.scm.tree || bc.scm.tree {
		t.Fatal("tree preference changed wrong repository")
	}
}

func TestCommitFileRowOrder(t *testing.T) {
	root := t.TempDir()
	files := []git.CommitFile{{Path: filepath.Join(root, "z.txt"), Status: 'D'}, {Path: filepath.Join(root, "src", "b.txt"), Status: 'R'}, {Path: filepath.Join(root, "src", "a.txt"), Status: 'A'}}
	rows := commitFileRows(root, files, true, nil)
	names := []string{}
	for _, r := range rows {
		names = append(names, r.Name)
	}
	if !slices.Equal(names, []string{"src", "a.txt", "b.txt", "z.txt"}) {
		t.Fatal(names)
	}
	if rows[2].Status != 'R' || rows[3].Status != 'D' {
		t.Fatal("status lost")
	}
}

func TestCommitFileSelectionActivatesItsColumn(t *testing.T) {
	e, _, first, second := commitToolRepo(t)
	e.showHistoryTab("", second.Hash, false)
	tb := e.cur()
	pump(t, e, func() bool { return !tb.diff.loading })
	e.Run("workbench.action.splitEditor")
	e.showHistoryTab("", first.Hash, false)
	other := e.cur()
	pump(t, e, func() bool { return !other.diff.loading })
	if e.col != 1 {
		t.Fatal("fixture did not create another column")
	}
	e.CommitFileAction(0, tb.diff.path, filepath.Join(e.root, "deleted.txt"), false)
	if e.col != 0 || e.cur() != tb || e.commitFilesFrame(tb).Selected != filepath.Join(e.root, "deleted.txt") {
		t.Fatal("file list targeted the wrong column")
	}
}

func TestEmptyCommitExpandDoesNotSelectAnotherCommit(t *testing.T) {
	e, run, _, _ := commitToolRepo(t)
	run("commit", "--allow-empty", "-qm", "empty")
	hash := run("rev-parse", "HEAD")
	e.gitChanged()
	pump(t, e, func() bool { return len(e.scm.history.commits) > 0 && e.scm.history.commits[0].Hash == hash })
	e.SCMToggleCommitFiles(hash)
	pump(t, e, func() bool { return e.scm.history.files[hash].loaded })
	e.Run("list.expand")
	if row, ok := e.selectedHistoryRow(); !ok || row.Hash != hash || row.File != nil {
		t.Fatal("right on an empty commit left its row")
	}
}
