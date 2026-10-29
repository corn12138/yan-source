package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/view"
)

// The status bar offers sync for a branch with commits: it pulls and pushes, or publishes a
// branch without an upstream.
func TestGitSyncFromTheStatusBar(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "a\n"})
	bare := filepath.Join(t.TempDir(), "o.git")
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "--bare", bare)
	run("remote", "add", "origin", bare)
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil })
	var f view.Frame
	e.Frame(&f)
	if !f.Status.Sync || !f.Status.Publish {
		t.Fatalf("status %+v", f.Status)
	}
	e.Run("git.sync") // publish
	// The watcher can observe the upstream ref before the publish command finishes.
	pump(t, e, func() bool { return e.git.busy == "" && e.git.status != nil && e.git.status.Upstream != "" })
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("b\n"), 0o644)
	run("commit", "-qam", "second")
	e.gitChanged()
	pump(t, e, func() bool { return e.git.status.Ahead == 1 })
	e.Frame(&f)
	if !f.Status.Sync || f.Status.Publish {
		t.Fatalf("status with upstream %+v", f.Status)
	}
	e.Run("git.sync")
	pump(t, e, func() bool { return e.git.busy == "" && e.status != "" && e.git.status.Ahead == 0 })
	if got := run("log", "-1", "--format=%s", "origin/main"); got != "second" {
		t.Fatalf("remote has %q (status %q)", got, e.status)
	}
}
