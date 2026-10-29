package process

import (
	"os/exec"
	"syscall"
	"testing"
)

func TestHideWindowPreservesFlags(t *testing.T) {
	cmd := Command("cmd", "/c", "echo", "hi")
	if a := cmd.SysProcAttr; a == nil || !a.HideWindow || a.CreationFlags&createNoWindow == 0 {
		t.Fatalf("console flags: %+v", a)
	}
	cmd = exec.Command("cmd")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x200, CmdLine: "cmd /c echo hi"}
	hideWindow(cmd)
	if cmd.SysProcAttr.CreationFlags != createNoWindow|0x200 || cmd.SysProcAttr.CmdLine != "cmd /c echo hi" {
		t.Fatal("lost caller flags")
	}
}
