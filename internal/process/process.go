// Package process creates non-interactive helper commands without a Windows console window.
package process

import "os/exec"

func Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	hideWindow(cmd)
	return cmd
}
