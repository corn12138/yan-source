//go:build darwin || linux

package termsession

import (
	"bufio"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// 一个自成会话的 shell 开着作业控制（同交互式 shell，作业另成进程组）起了一个忽略挂断的程序，
// 就像关终端后留在后台的开发服务器：挂断 shell 的进程组之后它还在，End 要把它结束掉。
func TestEndKillsWhatIgnoresTheHangup(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", `set -m; sh -c 'trap "" HUP; echo $$; exec sleep 60' & wait`)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	leader := cmd.Process.Pid
	t.Cleanup(func() {
		end(leader, 0)
		_ = cmd.Wait()
	})
	// The child announces itself after installing SIG_IGN. The parent's $! only proves
	// it forked: a group hangup can otherwise arrive before the child's trap is ready.
	ready := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(out).ReadString('\n')
		ready <- line
	}()
	var line string
	select {
	case line = <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("the job did not announce readiness")
	}
	job, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("job pid: %q", line)
	}
	if !alive(job) || sid(job) != leader {
		t.Fatal("the job is not running")
	}

	syscall.Kill(-leader, syscall.SIGHUP) // what closing the terminal does to the shell's group
	time.Sleep(100 * time.Millisecond)
	if !alive(job) {
		t.Fatal("the job did not survive the hangup; the test proves nothing")
	}
	End(leader, 300*time.Millisecond)
	Wait(5 * time.Second)
	cmd.Wait()
	for range 40 {
		if !alive(job) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("job %d still runs after End", job)
}

func alive(pid int) bool { return syscall.Kill(pid, 0) == nil && sid(pid) != -1 }
