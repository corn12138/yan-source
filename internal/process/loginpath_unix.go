//go:build darwin || linux

package process

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

const supportsLoginShell = true
const maxShellOutput = 64 << 10

func discoverLoginPath(parent context.Context, env []string, timeout time.Duration) string {
	shell := envValue(env, "SHELL")
	if shell == "" {
		shell = "/bin/sh"
		if runtime.GOOS == "darwin" {
			shell = "/bin/zsh"
		}
	}
	if !filepath.IsAbs(shell) {
		return ""
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, "-ilc", pathScript)
	cmd.Env = env
	if home := envValue(env, "HOME"); filepath.IsAbs(home) {
		cmd.Dir = home
	}
	// A startup script can leave children holding stdout after its shell exits.
	// The private process group and WaitDelay bound both cancellation and pipes.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killProbeGroup(cmd) }
	cmd.WaitDelay = 100 * time.Millisecond
	var output shellOutput
	output.cancel = cancel
	cmd.Stdout, cmd.Stderr = &output, io.Discard
	err := cmd.Run()
	_ = killProbeGroup(cmd)
	if err != nil || ctx.Err() != nil || output.exceeded {
		return ""
	}
	return parseLoginPath(output.Bytes(), envValue(env, "PATH"))
}

func killProbeGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

type shellOutput struct {
	// Embedding bytes.Buffer would expose ReaderFrom, letting io.Copy bypass
	// Write and its output limit when it drains the command's pipe.
	buffer   bytes.Buffer
	cancel   context.CancelFunc
	exceeded bool
}

func (b *shellOutput) Bytes() []byte { return b.buffer.Bytes() }

func (b *shellOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := maxShellOutput - b.buffer.Len()
	if n > remaining {
		p = p[:remaining]
		b.exceeded = true
		b.cancel()
	}
	_, _ = b.buffer.Write(p)
	return n, nil
}
