// Package clipboard 通过 macOS 自带的 pbcopy / pbpaste 读写系统剪贴板（无 cgo）。
package clipboard

import (
	"os/exec"
	"strings"
)

// Set 把 text 写入系统剪贴板。
func Set(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// Get 读取系统剪贴板中的文本。
func Get() (string, error) {
	out, err := exec.Command("pbpaste").Output()
	return string(out), err
}
