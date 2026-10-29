// Adapted from Mayoi ce553d24b746 (MIT); see THIRD_PARTY_NOTICES.md.
package process

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func hideWindow(cmd *exec.Cmd) {
	// HideWindow hides a program's own window; CREATE_NO_WINDOW suppresses the
	// console Windows allocates for helpers launched by a GUI process. Keep I/O and other flags.
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
