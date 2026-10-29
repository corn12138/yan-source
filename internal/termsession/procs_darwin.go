package termsession

import "golang.org/x/sys/unix"

func all() []int {
	ps, _ := unix.SysctlKinfoProcSlice("kern.proc.all")
	out := make([]int, 0, len(ps))
	for _, p := range ps {
		if pid := int(p.Proc.P_pid); pid > 0 {
			out = append(out, pid)
		}
	}
	return out
}
