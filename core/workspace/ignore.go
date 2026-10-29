// Package workspace 管理项目根目录：gitignore 规则、后台文件索引、按需展开的目录树。
package workspace

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// alwaysHidden 是无论 gitignore 怎么写都不显示、不索引的目录（与 VS Code 的 files.exclude 默认值相近）。
var alwaysHidden = map[string]bool{".git": true, "node_modules": true, ".DS_Store": true}

// Hidden 报告一个文件名是否永远隐藏。
func Hidden(name string) bool { return alwaysHidden[name] }

// Ignore 是整个项目的 gitignore 规则集。遍历是并发的，所以用读写锁保护。
//
// 只用 go-git 的 ParsePattern；.gitignore 文件由我们自己读，不需要 billy 文件系统抽象。
// 规则按所在目录分桶：匹配一个路径时只看它沿途各级目录的规则，
// 而不是把全项目的规则挨个试一遍（依赖目录里成百上千个 .gitignore 时差别很大）。
type Ignore struct {
	root   string
	mu     sync.RWMutex
	byDir  map[string][]gitignore.Pattern // 目录相对路径 → 该目录 .gitignore 的规则（按文件顺序）
	loaded map[string]bool                // 已读过 .gitignore 的目录
}

// NewIgnore 读取根目录的 .gitignore 和 .git/info/exclude。
func NewIgnore(root string) *Ignore {
	ig := &Ignore{root: root, loaded: map[string]bool{}, byDir: map[string][]gitignore.Pattern{}}
	ig.addFile(filepath.Join(root, ".git", "info", "exclude"), "") // 优先级最低，所以最先加入根目录的桶
	ig.LoadDir("")
	return ig
}

// LoadDir 读取相对目录 rel 下的 .gitignore（每个目录只读一次）。
// 必须在列出该目录的子项之前调用，规则才会对子项生效。
func (ig *Ignore) LoadDir(rel string) {
	ig.mu.Lock()
	done := ig.loaded[rel]
	ig.loaded[rel] = true
	ig.mu.Unlock()
	if done {
		return
	}
	ig.addFile(filepath.Join(ig.root, rel, ".gitignore"), filepath.ToSlash(rel))
}

// Reload reads the .gitignore of directory rel again, and for the root .git/info/exclude too,
// as after one of them changed: a rule commented out or added applies from then on.
func (ig *Ignore) Reload(rel string) {
	rel = filepath.ToSlash(rel)
	var ps []gitignore.Pattern
	if rel == "" {
		ps = parseIgnore(filepath.Join(ig.root, ".git", "info", "exclude"), "")
	}
	ps = append(ps, parseIgnore(filepath.Join(ig.root, filepath.FromSlash(rel), ".gitignore"), rel)...)
	ig.mu.Lock()
	ig.loaded[rel] = true
	if len(ps) == 0 {
		delete(ig.byDir, rel)
	} else {
		ig.byDir[rel] = ps
	}
	ig.mu.Unlock()
}

// ReloadAll reads every .gitignore read so far again (when the editor regains focus without
// file events, say).
func (ig *Ignore) ReloadAll() {
	ig.mu.RLock()
	dirs := make([]string, 0, len(ig.loaded))
	for d := range ig.loaded {
		dirs = append(dirs, d)
	}
	ig.mu.RUnlock()
	for _, d := range dirs {
		ig.Reload(d)
	}
}

func (ig *Ignore) addFile(path, rel string) {
	ps := parseIgnore(path, rel)
	if len(ps) == 0 {
		return
	}
	ig.mu.Lock()
	ig.byDir[rel] = append(ig.byDir[rel], ps...)
	ig.mu.Unlock()
}

// parseIgnore reads the patterns of an ignore file in directory rel; none when it cannot be read.
func parseIgnore(path, rel string) []gitignore.Pattern {
	var domain []string
	if rel != "" {
		domain = strings.Split(rel, "/")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var ps []gitignore.Pattern
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ps = append(ps, gitignore.ParsePattern(line, domain))
	}
	return ps
}

// Match 报告相对路径 rel 是否被忽略。git 的优先级：越深的目录越优先，同一文件里越靠后越优先；
// 第一个给出明确结论（排除或 ! 重新包含）的规则说了算。
func (ig *Ignore) Match(rel string, isDir bool) bool {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	ig.mu.RLock()
	defer ig.mu.RUnlock()
	for d := len(parts) - 1; d >= 0; d-- {
		ps := ig.byDir[strings.Join(parts[:d], "/")]
		for i := len(ps) - 1; i >= 0; i-- {
			switch ps[i].Match(parts, isDir) {
			case gitignore.Exclude:
				return true
			case gitignore.Include:
				return false
			}
		}
	}
	return false
}
