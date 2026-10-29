// Package launch 解析命令行要打开的东西：终端版 yan 和窗口版 yan-app 共用。
package launch

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Target 是命令行要打开的东西。
type Target struct {
	Root   string // 项目根
	Folder bool   // Root 是打开的文件夹（有资源管理器、快速打开）；否则是单文件模式
	File   string // 要打开的文件；只给文件夹时是它的 README（有的话）
	Readme bool   // File 是自动挑的 README：有上次的会话时优先恢复会话
	Line   int    // 跳到第几行；0 表示不跳
}

// Resolve 解析一个参数（目录，或 文件[:行号]）。目录就作为项目根；文件就找它所在的 git 仓库
// 作为项目根，找不到仓库时是单文件模式：不建索引、不显示资源管理器（免得把整个主目录当项目遍历）。
func Resolve(arg string) Target {
	path, line := SplitLine(arg)
	t := ResolvePath(path)
	t.Line = line
	return t
}

func ResolvePath(path string) Target {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	t := Target{Root: path, Folder: true}
	if fi, err := os.Stat(path); err != nil || !fi.IsDir() {
		t.File = path
		t.Root, t.Folder = ProjectRoot(filepath.Dir(path))
	}
	if t.File == "" {
		t.File, t.Readme = Readme(t.Root), true
	}
	return t
}

// ProjectRoot 从 dir 往上找含 .git 的目录；找不到时返回 dir 本身和 false。
func ProjectRoot(dir string) (string, bool) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir, false
	}
	for d := abs; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d, true
		}
		if filepath.Dir(d) == d {
			return abs, false
		}
	}
}

// Readme 返回 root 下的 README 文件；没有时返回 ""。
func Readme(root string) string {
	for _, n := range []string{"README.md", "readme.md", "README.markdown", "README"} {
		p := filepath.Join(root, n)
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

// SplitLine 解析 "main.go:42"；文件名本身含冒号且没有合法行号时原样返回。
func SplitLine(arg string) (string, int) {
	i := strings.LastIndexByte(arg, ':')
	if i <= 0 {
		return arg, 0
	}
	n, err := strconv.Atoi(arg[i+1:])
	if err != nil || n <= 0 {
		return arg, 0
	}
	return arg[:i], n
}
