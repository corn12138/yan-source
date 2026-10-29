package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/view"
)

func TestShortRemoteURL(t *testing.T) {
	for in, want := range map[string]string{
		"git@github.com:me/app.git":       "github.com/me/app",
		"https://github.com/me/app":       "github.com/me/app",
		"ssh://git@host.example/x/y.git/": "host.example/x/y",
		"/srv/repos/app.git":              "/srv/repos/app",
	} {
		if got := shortRemoteURL(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

// panelRepo is a repository whose main tracks origin/main and is a commit ahead of it, with a
// branch feature beside.
func panelRepo(t *testing.T) (*Editor, string) {
	t.Helper()
	root := gitRepo(t, map[string]string{"a.txt": "a\n"})
	bare := filepath.Join(t.TempDir(), "origin.git")
	run := func(dir string, args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(root, "init", "-q", "--bare", bare)
	run(root, "remote", "add", "origin", bare)
	run(root, "push", "-q", "-u", "origin", "main")
	run(root, "branch", "feature")
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("b\n"), 0o644)
	run(root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qam", "ahead of origin")
	e := newEditorIn(t, root)
	e.SetState(config.LoadState(filepath.Join(t.TempDir(), "state.json")))
	pump(t, e, func() bool { return e.git.status != nil })
	return e, root
}

func panelRows(e *Editor) []view.SCMPanelRow {
	var f view.Frame
	e.Frame(&f)
	return f.Sidebar.SCM.PanelRows
}

func rowNamed(rows []view.SCMPanelRow, kind, label string) (view.SCMPanelRow, bool) {
	for _, r := range rows {
		if r.Kind == kind && strings.HasPrefix(r.Label, label) {
			return r, true
		}
	}
	return view.SCMPanelRow{}, false
}

// The panel reads nothing while closed; opened, it lists the branches, each closed; a branch
// opened reads its history and offers its outgoing commits; it is remembered as open.
func TestRepositoryPanelBranches(t *testing.T) {
	e, _ := panelRepo(t)
	e.Run("workbench.view.scm")
	if rows := panelRows(e); rows != nil || e.scm.panel.key != "" {
		t.Fatalf("closed panel read: %+v", rows)
	}
	e.Run("yan.scm.toggleRepositoryPanel")
	if !e.state.RepositoryPanel {
		t.Fatal("not remembered")
	}
	var rows []view.SCMPanelRow
	pump(t, e, func() bool { rows = panelRows(e); _, ok := rowNamed(rows, "branch", "feature"); return ok })
	main, ok := rowNamed(rows, "branch", "main")
	if !ok || !main.Current || main.Open || !strings.Contains(main.Detail, "origin/main ↑1") {
		t.Fatalf("main row %+v", main)
	}
	if len(e.scm.panel.logs) != 0 {
		t.Fatal("a closed branch read its history")
	}
	e.SCMPanelActivate(main)
	pump(t, e, func() bool { rows = panelRows(e); _, ok := rowNamed(rows, "commit", "ahead of origin"); return ok })
	out, ok := rowNamed(rows, "range", "待推送（1 个提交）")
	if !ok {
		t.Fatalf("no outgoing row: %+v", rows)
	}
	e.SCMPanelActivate(out)
	pump(t, e, func() bool {
		rows = panelRows(e)
		n := 0
		for _, r := range rows {
			if r.Kind == "commit" && r.Label == "ahead of origin" {
				n++
			}
		}
		return n == 2 // in the outgoing commits and in the history
	})
	c, _ := rowNamed(rows, "commit", "ahead of origin")
	e.SCMPanelActivate(c)
	if t2 := e.cur(); t2 == nil || t2.diff == nil || t2.diff.commit == nil || t2.diff.commit.Hash != c.Hash {
		t.Fatalf("commit did not open: %v", tabNames(e))
	}
}

// The Remotes tab lists the remotes with where they point; one opened shows its URLs and its
// branches.
func TestRepositoryPanelRemotes(t *testing.T) {
	e, _ := panelRepo(t)
	e.Run("workbench.view.scm")
	e.SCMPanelTab(true)
	var rows []view.SCMPanelRow
	pump(t, e, func() bool { rows = panelRows(e); _, ok := rowNamed(rows, "remote", "origin"); return ok })
	origin, _ := rowNamed(rows, "remote", "origin")
	if !strings.HasSuffix(origin.Detail, "origin") {
		t.Fatalf("origin %+v", origin)
	}
	e.SCMPanelActivate(origin)
	pump(t, e, func() bool {
		rows = panelRows(e)
		_, url := rowNamed(rows, "url", "/")
		_, br := rowNamed(rows, "branch", "origin/main")
		return url && br
	})
	if u, _ := rowNamed(rows, "url", "/"); u.Detail != "拉取、推送" {
		t.Fatalf("url %+v", u)
	}
}
