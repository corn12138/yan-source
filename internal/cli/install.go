// Mayoi v0.8.x (MIT, internal/cli) inspired the explicit install action and collision protection.
// Package cli installs the GUI launcher without replacing a user's existing command.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const marker = "# Installed by yan GUI."

func Script(executable, goos string) (string, error) {
	switch goos {
	case "darwin", "linux":
		quoted := "'" + strings.ReplaceAll(executable, "'", "'\\''") + "'"
		return "#!/bin/sh\n" + marker + "\nif [ \"$#\" -eq 0 ]; then set -- .; fi\nnohup " + quoted + " \"$@\" </dev/null >/dev/null 2>&1 &\n", nil
	default:
		return "", fmt.Errorf("此平台的命令安装暂未提供")
	}
}

func Install(dir, executable, goos string) (string, error) {
	script, err := Script(executable, goos)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(executable) {
		return "", fmt.Errorf("应用路径必须是绝对路径")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "yan")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0755)
	if os.IsExist(err) {
		if fi, e := os.Lstat(path); e == nil && fi.Mode().IsRegular() {
			if b, e := os.ReadFile(path); e == nil && string(b) == script {
				return path, nil
			}
		}
		return "", fmt.Errorf("%s 已存在，未作修改；请先将它移到别处再安装", path)
	}
	if err != nil {
		return "", err
	}
	_, err = f.WriteString(script)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}
