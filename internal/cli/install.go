// Mayoi v0.8.x (MIT, internal/cli) inspired the explicit install action and collision protection.
// Package cli installs the GUI launcher without replacing a user's existing command.
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/corn12138/yan-source/core/i18n"
)

const marker = "# Installed by yan GUI."

func Script(executable, goos string) (string, error) {
	switch goos {
	case "darwin", "linux":
		quoted := "'" + strings.ReplaceAll(executable, "'", "'\\''") + "'"
		return "#!/bin/sh\n" + marker + "\nif [ \"$#\" -eq 0 ]; then set -- .; fi\nnohup " + quoted + " \"$@\" </dev/null >/dev/null 2>&1 &\n", nil
	default:
		return "", errors.New(i18n.T("Installing the command is not available on this platform yet"))
	}
}

func Install(dir, executable, goos string) (string, error) {
	script, err := Script(executable, goos)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(executable) {
		return "", errors.New(i18n.T("The app path must be absolute"))
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
		return "", fmt.Errorf(i18n.T("%s already exists and was left unchanged; move it elsewhere before installing"), path)
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
