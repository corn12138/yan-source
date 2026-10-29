package gui

import (
	"reflect"

	"github.com/egoist/mygo/plugins/terminal"
)

// shellPid 是终端里 shell 的进程号（它也是 shell 那个会话的号）；shell 还没起来、终端连的是 Conn 时是 0。
// MyGo 不公开它，只能照着现在这个版本的字段去读（Terminal.proc → pty.PTY.cmd.Process.Pid；Windows 是 PTY.pid）。
// 换了 MyGo 版本、字段对不上时返回 0，关终端退回 MyGo 自己的挂断；`-tags selftest` 的
// TestNativeClosedTerminalEndsItsPrograms 会在那时失败。MyGo 公开进程号之后改用它。
func shellPid(t *terminal.Terminal) int {
	if t == nil {
		return 0
	}
	proc := reflect.ValueOf(t).Elem().FieldByName("proc")
	if proc.Kind() != reflect.Pointer || proc.IsNil() {
		return 0
	}
	p := proc.Elem()
	if pid := p.FieldByName("pid"); pid.IsValid() && pid.CanInt() {
		return int(pid.Int())
	}
	v := p.FieldByName("cmd")
	for _, name := range []string{"Process", "Pid"} {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return 0
			}
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return 0
		}
		v = v.FieldByName(name)
	}
	if !v.IsValid() || !v.CanInt() {
		return 0
	}
	return int(v.Int())
}
