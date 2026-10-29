package process

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const shellPathTimeout = 3 * time.Second

var loginPathOnce sync.Once
var loginPath atomic.Pointer[pathResult]

type pathResult struct {
	done   chan struct{}
	cancel context.CancelFunc
	path   string
}

// EnableLoginShellPath is a GUI startup opt-in. Every Git command waits for the
// same bounded discovery, so hooks cannot inherit a different PATH just because
// a window or repository opened before the shell finished. The process environment
// and terminal launches are left alone.
func EnableLoginShellPath() {
	loginPathOnce.Do(func() {
		if !supportsLoginShell {
			return
		}
		loginPath.Store(startLoginPath(os.Environ(), shellPathTimeout))
	})
}

func startLoginPath(env []string, timeout time.Duration) *pathResult {
	ctx, cancel := context.WithCancel(context.Background())
	result := &pathResult{done: make(chan struct{}), cancel: cancel}
	go func() {
		defer close(result.done)
		defer cancel()
		result.path = discoverLoginPath(ctx, env, timeout)
	}()
	return result
}

// StopLoginShellPath joins cleanup before the GUI exits: once the Go process
// stops, a background context timer can no longer terminate the probe's children.
// It is safe to call repeatedly, concurrently, or without enabling discovery.
func StopLoginShellPath() error { return loginPath.Load().stop() }

func (r *pathResult) stop() error {
	if r == nil {
		return nil
	}
	if r.cancel != nil {
		r.cancel()
	}
	timer := time.NewTimer(shellPathTimeout + 100*time.Millisecond)
	defer timer.Stop()
	select {
	case <-r.done:
		return nil
	case <-timer.C:
		return errors.New("login shell PATH probe cleanup did not finish before shutdown")
	}
}

func GitCommand(args ...string) *exec.Cmd {
	return gitCommand(loginPath.Load(), os.Environ(), args...)
}

func gitCommand(result *pathResult, env []string, args ...string) *exec.Cmd {
	if result == nil {
		return Command("git", args...)
	}
	<-result.done
	if result.path == "" {
		return Command("git", args...)
	}
	cmd := Command("git", args...)
	cmd.Env = replacePath(env, result.path)
	// exec.Command resolves names before Cmd.Env is assigned. Resolve against the
	// very same PATH the hook receives, including a Git installed by the shell.
	cmd.Err = &exec.Error{Name: "git", Err: exec.ErrNotFound}
	for _, dir := range filepath.SplitList(result.path) {
		if path, err := exec.LookPath(filepath.Join(dir, "git")); err == nil {
			cmd.Path, cmd.Err = path, nil
			break
		}
	}
	return cmd
}

func replacePath(env []string, path string) []string {
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, "PATH=") {
			out = append(out, entry)
		}
	}
	return append(out, "PATH="+path)
}

func envValue(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if value, ok := strings.CutPrefix(env[i], key+"="); ok {
			return value
		}
	}
	return ""
}

const pathFrame = "\x00YAN_LOGIN_PATH\x00"
const pathScript = `printf '\000YAN_LOGIN_PATH\000%s\000' "$PATH"`

func parseLoginPath(output []byte, inherited string) string {
	if bytes.Count(output, []byte(pathFrame)) != 1 {
		return ""
	}
	start := bytes.Index(output, []byte(pathFrame)) + len(pathFrame)
	end := bytes.IndexByte(output[start:], 0)
	if end <= 0 {
		return ""
	}
	var paths []string
	seen := map[string]bool{}
	for _, source := range []string{string(output[start : start+end]), inherited} {
		for _, dir := range filepath.SplitList(source) {
			// Git runs in the repository, not the shell's home. Relative entries
			// would unexpectedly resolve to executables from that repository.
			if !filepath.IsAbs(dir) {
				continue
			}
			dir = filepath.Clean(dir)
			if !seen[dir] {
				seen[dir] = true
				paths = append(paths, dir)
			}
		}
		if len(paths) == 0 {
			return ""
		}
	}
	return strings.Join(paths, string(os.PathListSeparator))
}
