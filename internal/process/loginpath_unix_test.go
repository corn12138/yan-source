//go:build darwin || linux

package process

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func probeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "shell with spaces")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func probeEnv(t *testing.T, shell string) []string {
	t.Helper()
	return []string{"SHELL=" + shell, "HOME=" + t.TempDir(), "PATH=/usr/bin:/bin"}
}

func quoteProbe(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

func TestDiscoverLoginPath(t *testing.T) {
	tools := filepath.Join(t.TempDir(), "tools with spaces")
	shell := probeScript(t, "printf 'startup banner\\n'\nPATH="+quoteProbe(tools+":/usr/bin")+"\nexport PATH\nshift\nexec /bin/sh -c \"$1\"\n")
	before := os.Getenv("PATH")
	if got := discoverLoginPath(context.Background(), probeEnv(t, shell), 30*time.Second); got != tools+":/usr/bin:/bin" {
		t.Fatalf("PATH = %q", got)
	}
	if os.Getenv("PATH") != before {
		t.Fatal("changed the process PATH")
	}
}

// macOS checks a newly written executable on its first run, which can take
// seconds under a loaded test run. Timing assertions therefore start once the
// fixture is running; a short deadline is only asserted to hold.

func TestDiscoverLoginPathFallsBack(t *testing.T) {
	for _, tc := range []struct {
		name, script string
	}{
		{"malformed", "printf 'not a path frame\\n'\n"},
		{"failed", "shift\n/bin/sh -c \"$1\"\nexit 7\n"},
		{"overflow", "while :; do printf '" + strings.Repeat("x", 1024) + "'; done\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			started := filepath.Join(t.TempDir(), "started")
			script := "printf 'started' > " + quoteProbe(started) + "\n" + tc.script
			const timeout = 30 * time.Second
			start := time.Now()
			if got := discoverLoginPath(context.Background(), probeEnv(t, probeScript(t, script)), timeout); got != "" {
				t.Fatalf("accepted failed probe: %q", got)
			}
			if _, err := os.Stat(started); err != nil {
				t.Fatalf("fixture never reached %s scenario: %v", tc.name, err)
			}
			if elapsed := time.Since(start); elapsed >= timeout {
				t.Fatalf("%s waited for the timeout: %s", tc.name, elapsed)
			}
		})
	}
	for _, shell := range []string{"relative-shell", filepath.Join(t.TempDir(), "missing")} {
		if got := discoverLoginPath(context.Background(), probeEnv(t, shell), time.Second); got != "" {
			t.Fatalf("accepted unavailable shell: %q", got)
		}
	}
}

func TestDiscoverLoginPathTimesOut(t *testing.T) {
	const timeout = 2 * time.Second
	start := time.Now()
	if got := discoverLoginPath(context.Background(), probeEnv(t, probeScript(t, "while :; do :; done\n")), timeout); got != "" {
		t.Fatalf("accepted a hanging probe: %q", got)
	}
	if elapsed := time.Since(start); elapsed > timeout+time.Second {
		t.Fatalf("probe took %s", elapsed)
	}
}

// runProbeUntil starts discovery and returns once the fixture has created
// marker, with the time it did and a channel delivering discovery's result.
func runProbeUntil(t *testing.T, ctx context.Context, shell, marker string) (time.Time, <-chan string) {
	t.Helper()
	result := make(chan string, 1)
	go func() { result <- discoverLoginPath(ctx, probeEnv(t, shell), 30*time.Second) }()
	deadline := time.Now().Add(20 * time.Second)
	for {
		if b, err := os.ReadFile(marker); err == nil && len(b) > 0 {
			return time.Now(), result
		}
		if time.Now().After(deadline) {
			t.Fatal("fixture never started")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestDiscoverLoginPathCancelsHangingShell(t *testing.T) {
	started := filepath.Join(t.TempDir(), "started")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, result := runProbeUntil(t, ctx, probeScript(t, "printf 'started' > "+quoteProbe(started)+"\nwhile :; do :; done\n"), started)
	canceled := time.Now()
	cancel()
	select {
	case got := <-result:
		if got != "" {
			t.Fatalf("accepted a canceled probe: %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation did not end the hanging shell")
	}
	if elapsed := time.Since(canceled); elapsed > 2*time.Second {
		t.Fatalf("cancellation took %s", elapsed)
	}
}

func TestShellOutputCapsCopy(t *testing.T) {
	canceled := false
	output := &shellOutput{cancel: func() { canceled = true }}
	// An ordinary pipe has no WriterTo; io.Copy must not bypass Write through
	// bytes.Buffer's ReaderFrom optimization and evade the discovery limit.
	input := struct{ io.Reader }{strings.NewReader(strings.Repeat("x", maxShellOutput*2))}
	n, err := io.Copy(output, input)
	if err != nil || n != maxShellOutput*2 || len(output.Bytes()) != maxShellOutput || !output.exceeded || !canceled {
		t.Fatalf("copy size=%d stored=%d exceeded=%v canceled=%v err=%v", n, len(output.Bytes()), output.exceeded, canceled, err)
	}
}

func TestDiscoverLoginPathEndsDescendantPipe(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child")
	shell := probeScript(t, "/bin/sleep 30 &\nprintf '%s' \"$!\" > "+quoteProbe(pidFile)+"\nshift\n/bin/sh -c \"$1\"\nexit 0\n")
	running, result := runProbeUntil(t, context.Background(), shell, pidFile)
	b, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(b))
	if err != nil || pid <= 0 {
		t.Fatalf("child PID %q: %v", b, err)
	}
	defer syscall.Kill(pid, syscall.SIGKILL)
	select {
	case got := <-result:
		if got != "" {
			t.Fatalf("accepted a probe with unclosed pipes: %q", got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("waited for the descendant's pipe")
	}
	if elapsed := time.Since(running); elapsed > 5*time.Second {
		t.Fatalf("waited %s for the descendant's pipe", elapsed)
	}
	requireProbeExited(t, pid)
}

func requireProbeExited(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "stat=").Output()
		if err != nil || strings.HasPrefix(strings.TrimSpace(string(out)), "Z") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("probe left child running: %s", out)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestStopLoginShellPathCancelsAndJoinsChildren(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child")
	shell := probeScript(t, "/bin/sleep 30 &\nprintf '%s' \"$!\" > "+quoteProbe(pidFile)+"\nwait\n")
	result := startLoginPath(probeEnv(t, shell), 30*time.Second)
	t.Cleanup(func() {
		if err := result.stop(); err != nil {
			t.Error(err)
		}
	})
	deadline := time.Now().Add(20 * time.Second)
	var pid int
	for pid == 0 {
		if b, err := os.ReadFile(pidFile); err == nil {
			pid, _ = strconv.Atoi(string(b))
		}
		if time.Now().After(deadline) {
			t.Fatal("probe never started its child")
		}
		time.Sleep(5 * time.Millisecond)
	}
	defer syscall.Kill(pid, syscall.SIGKILL)
	select {
	case <-result.done:
		t.Fatal("fixture exited before owner shutdown")
	default:
	}
	const callers = 8
	errors := make(chan error, callers)
	for range callers {
		go func() { errors <- result.stop() }()
	}
	for range callers {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-result.done:
	default:
		t.Fatal("shutdown returned before probe cleanup")
	}
	if result.path != "" {
		t.Fatalf("accepted a canceled probe: %q", result.path)
	}
	requireProbeExited(t, pid)
}

func TestGitCommandWaitsForSharedPath(t *testing.T) {
	tools := t.TempDir()
	git := filepath.Join(tools, "git")
	if err := os.WriteFile(git, []byte("#!/bin/sh\nprintf '%s' \"$PATH\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	result := &pathResult{done: make(chan struct{})}
	const count = 12
	commands := make(chan *exec.Cmd, count)
	var started sync.WaitGroup
	started.Add(count)
	for range count {
		go func() {
			started.Done()
			commands <- gitCommand(result, []string{"PATH=/bin", "KEEP=value"}, "--version")
		}()
	}
	started.Wait()
	select {
	case <-commands:
		t.Fatal("Git escaped before discovery completed")
	case <-time.After(20 * time.Millisecond):
	}
	result.path = tools + ":/bin"
	close(result.done)
	for range count {
		cmd := <-commands
		if cmd.Path != git || envValue(cmd.Env, "KEEP") != "value" {
			t.Fatalf("wrong executable/environment: %s, %v", cmd.Path, cmd.Env)
		}
		out, err := cmd.Output()
		if err != nil || string(out) != result.path {
			t.Fatalf("Git PATH = %q, %v", out, err)
		}
	}
}

func TestGitCommandFallbackKeepsInheritedPath(t *testing.T) {
	result := &pathResult{done: make(chan struct{})}
	close(result.done)
	for _, r := range []*pathResult{nil, result} {
		cmd := gitCommand(r, []string{"PATH=ignored"})
		if cmd.Env != nil {
			t.Fatalf("fallback replaced inherited environment: %v", cmd.Env)
		}
	}
	result.path = t.TempDir()
	cmd := gitCommand(result, os.Environ())
	if cmd.Err == nil {
		t.Fatal("fell back to a Git outside the selected PATH")
	}
	if got := fmt.Sprint(cmd.Err); !strings.Contains(got, "executable file not found") {
		t.Fatal(got)
	}
}
