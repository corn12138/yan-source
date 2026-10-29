package app

import (
	"os/exec"
	"strings"
	"testing"
)

func gitOut(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// chooseItem runs the overlay item whose label starts with label.
func chooseItem(t *testing.T, e *Editor, label string) {
	t.Helper()
	for _, it := range e.ov.choices {
		if strings.HasPrefix(it.label, label) {
			e.closeOverlay()
			if err := it.run(e); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no %q among the choices", label)
}

func TestRenameAndSafelyDeleteBranches(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "a\n"})
	gitOut(t, root, "branch", "merged")
	gitOut(t, root, "switch", "-q", "-c", "work")
	exec.Command("sh", "-c", "cd "+root+" && echo w > w.txt && git add w.txt && git -c user.name=t -c user.email=t@t commit -qm w").Run()
	gitOut(t, root, "switch", "-q", "main")
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.branch() == "main" })

	e.Run("git.renameBranch")
	if err := e.ov.accept(e, "trunk"); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.branch() == "trunk" })

	// A merged branch goes at once; an unmerged one asks, and stays unless confirmed.
	e.Run("git.deleteBranch")
	pump(t, e, func() bool { return e.ov != nil && len(e.ov.choices) == 2 })
	chooseItem(t, e, "merged")
	pump(t, e, func() bool { return e.git.busy == "" && !strings.Contains(gitOut(t, root, "branch"), "merged") })
	e.Run("git.deleteBranch")
	pump(t, e, func() bool { return e.ov != nil && len(e.ov.choices) == 1 })
	chooseItem(t, e, "work")
	pump(t, e, func() bool { return e.ov != nil && strings.Contains(e.ov.title, "没合并") })
	chooseItem(t, e, "保留分支")
	if !strings.Contains(gitOut(t, root, "branch"), "work") {
		t.Fatal("an unmerged branch was deleted without confirmation")
	}
	e.Run("git.deleteBranch")
	pump(t, e, func() bool { return e.ov != nil && len(e.ov.choices) == 1 })
	chooseItem(t, e, "work")
	pump(t, e, func() bool { return e.ov != nil && strings.Contains(e.ov.title, "没合并") })
	chooseItem(t, e, "仍然删除")
	// A ref disappears before git finishes updating its logs; wait for the process
	// before TempDir cleanup can remove the repository beneath it.
	pump(t, e, func() bool { return e.git.busy == "" && !strings.Contains(gitOut(t, root, "branch"), "work") })
}

func TestManageRemotes(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "a\n"})
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.branch() == "main" })

	e.Run("git.addRemote")
	e.ov.accept(e, "upstream")
	e.ov.accept(e, "https://example.com/a.git")
	pump(t, e, func() bool { return strings.Contains(gitOut(t, root, "remote", "-v"), "example.com/a.git") })

	e.Run("yan.git.setRemoteURL")
	pump(t, e, func() bool { return e.ov != nil && len(e.ov.choices) == 1 })
	chooseItem(t, e, "upstream")
	if e.ov.input != "https://example.com/a.git" {
		t.Fatalf("the URL prompt must start from the current URL: %q", e.ov.input)
	}
	e.ov.accept(e, "https://example.com/b.git")
	pump(t, e, func() bool { return gitOut(t, root, "remote", "get-url", "upstream") == "https://example.com/b.git" })

	gitOut(t, root, "update-ref", "refs/remotes/upstream/gone", "HEAD")
	gitOut(t, root, "remote", "set-url", "upstream", root) // a reachable remote without that branch
	e.Run("yan.git.pruneRemote")
	pump(t, e, func() bool { return e.ov != nil && len(e.ov.choices) == 1 })
	chooseItem(t, e, "upstream")
	pump(t, e, func() bool { return !strings.Contains(gitOut(t, root, "branch", "-r"), "upstream/gone") })

	e.Run("git.removeRemote")
	pump(t, e, func() bool { return e.ov != nil && len(e.ov.choices) == 1 })
	chooseItem(t, e, "upstream")
	chooseItem(t, e, "删除 upstream")
	pump(t, e, func() bool { return gitOut(t, root, "remote") == "" })
}
