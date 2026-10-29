//go:build darwin || linux

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/internal/process"
)

func TestCommitHookUsesLoginPath(t *testing.T) {
	if os.Getenv("YAN_GIT_PATH_HELPER") == "1" {
		before := os.Getenv("PATH")
		process.EnableLoginShellPath()
		r, err := Open(os.Getenv("YAN_GIT_PATH_ROOT"))
		if err != nil {
			t.Fatal(err)
		}
		if err := r.Commit("hook from GUI environment", false); err != nil {
			t.Fatal(err)
		}
		if got := os.Getenv("PATH"); got != before {
			t.Fatalf("process PATH changed: %q", got)
		}
		return
	}
	r, run := repo(t)
	write(t, r.Root, "c.txt", "hook test\n")
	run("add", "c.txt")
	tools := t.TempDir()
	tool := filepath.Join(tools, "yan-shell-hook-tool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nprintf '%s' \"$PATH\" > \"$YAN_GIT_PATH_MARKER\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(r.Dir, "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nyan-shell-hook-tool\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shell := filepath.Join(t.TempDir(), "login-shell")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	script := "#!/bin/sh\nprintf 'shell startup banner\\n'\nPATH=" + quote(tools+":"+filepath.Dir(git)+":/usr/bin:/bin") + "\nexport PATH\nshift\nexec /bin/sh -c \"$1\"\n"
	if err := os.WriteFile(shell, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// macOS checks a newly written executable on its first run, which under a
	// loaded test run can outlast the GUI's three-second discovery deadline.
	if err := exec.Command(shell, "-ilc", "true").Run(); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "hook-path")
	// A fresh process keeps the GUI's one-time opt-in isolated from other Git
	// tests, and proves a narrow desktop PATH is not silently fixed globally.
	cmd := exec.Command(os.Args[0], "-test.run=^TestCommitHookUsesLoginPath$")
	cmd.Env = append(os.Environ(), "YAN_GIT_PATH_HELPER=1", "YAN_GIT_PATH_ROOT="+r.Root,
		"YAN_GIT_PATH_MARKER="+marker, "SHELL="+shell, "HOME="+t.TempDir(), "PATH=/usr/bin:/bin")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit hook: %v\n%s", err, out)
	}
	out, err := os.ReadFile(marker)
	if err != nil || !strings.Contains(":"+string(out)+":", ":"+tools+":") {
		t.Fatalf("hook PATH = %q: %v", out, err)
	}
}
