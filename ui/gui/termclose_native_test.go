//go:build selftest && (darwin || linux)

package gui

import (
	"regexp"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"

	"github.com/corn12138/yan-source/internal/termsession"
)

// 关终端时，shell 起的、忽略挂断的作业（同关掉后还在后台跑的开发服务器）要一起结束；
// 也守着 shellPid：换了 MyGo、读不到进程号时这里失败。
func TestNativeClosedTerminalEndsItsPrograms(t *testing.T) {
	term, err := terminal.New(terminal.Options{NoBlink: true, Command: []string{"/bin/sh", "-c",
		`set -m; sh -c 'trap "" HUP; echo "JOB=$$"; exec sleep 60' & wait`}})
	if err != nil {
		t.Fatal(err)
	}
	s := &termSession{t: term}
	t.Cleanup(func() { s.close(); termsession.Wait(5 * time.Second) })
	// The child reports readiness only after it can survive the hangup.
	job := 0
	for deadline := time.Now().Add(5 * time.Second); job == 0 && time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if m := regexp.MustCompile(`JOB=(\d+)`).FindStringSubmatch(term.Text()); m != nil {
			job, _ = strconv.Atoi(m[1])
		}
	}
	if job == 0 {
		t.Fatalf("the shell did not start its job: %q", term.Text())
	}
	pid := shellPid(term)
	if pid <= 0 {
		t.Fatal("shellPid cannot read the shell's process ID from this MyGo")
	}
	if s, err := syscall.Getsid(job); err != nil || s != pid {
		t.Fatalf("job %d is in session %d, not the shell's %d", job, s, pid)
	}
	s.close()
	termsession.Wait(5 * time.Second)
	for range 40 {
		if syscall.Kill(job, 0) != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("job %d still runs after its terminal closed", job)
}
