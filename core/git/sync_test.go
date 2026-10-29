package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Sync fast-forwards from the upstream, then pushes the branch's own commits to it.
func TestSync(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	bare := filepath.Join(t.TempDir(), "o.git")
	a, b := t.TempDir(), t.TempDir()
	run := func(dir string, args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run(a, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(a, "f"), []byte("1\n"), 0o644)
	run(a, "add", ".")
	run(a, "commit", "-qm", "one")
	run(a, "init", "-q", "--bare", "-b", "main", bare)
	run(a, "remote", "add", "origin", bare)
	run(a, "push", "-q", "-u", "origin", "main")
	run(b, "clone", "-q", "-b", "main", bare, ".")
	os.WriteFile(filepath.Join(b, "g"), []byte("2\n"), 0o644)
	run(b, "add", ".")
	run(b, "commit", "-qm", "two")
	run(b, "push", "-q")
	r, err := Open(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Sync(); err != nil { // behind: pulls
		t.Fatal(err)
	}
	if got := run(a, "log", "-1", "--format=%s"); got != "two" {
		t.Fatalf("after pull: %q", got)
	}
	os.WriteFile(filepath.Join(a, "h"), []byte("3\n"), 0o644)
	run(a, "add", ".")
	run(a, "commit", "-qm", "three")
	if err := r.Sync(); err != nil { // ahead: pushes
		t.Fatal(err)
	}
	if got := run(a, "log", "-1", "--format=%s", "origin/main"); got != "three" {
		t.Fatalf("after push: %q", got)
	}
	run(a, "switch", "-q", "-c", "lonely")
	if err := r.Sync(); err == nil {
		t.Fatal("synced a branch without an upstream")
	}
}
