//go:build darwin || linux

package termsession

import "golang.org/x/sys/unix"

func sid(pid int) int {
	s, err := unix.Getsid(pid)
	if err != nil {
		return -1
	}
	return s
}

func kill(pid int) { _ = unix.Kill(pid, unix.SIGKILL) }
