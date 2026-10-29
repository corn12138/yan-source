// Package trash 把文件移到废纸篓（终端版用；窗口版用 MyGo 的 Shell.TrashItem）。
package trash

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Move 把 path 移到当前用户的废纸篓：macOS 是 ~/.Trash，其他系统是 XDG 的 ~/.local/share/Trash/files。
// 废纸篓里已有同名文件时，名字后面加上时间。
func Move(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".Trash")
	if runtime.GOOS != "darwin" {
		dir = filepath.Join(home, ".local", "share", "Trash", "files")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	dst := filepath.Join(dir, filepath.Base(path))
	if _, err := os.Lstat(dst); err == nil {
		ext := filepath.Ext(dst)
		dst = fmt.Sprintf("%s %s%s", dst[:len(dst)-len(ext)], time.Now().Format("15.04.05"), ext)
	}
	return os.Rename(path, dst)
}
