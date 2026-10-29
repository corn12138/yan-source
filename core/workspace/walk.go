package workspace

import (
	"context"
	"io/fs"
	"path/filepath"
	"sync"

	"github.com/charlievieth/fastwalk"

	"github.com/corn12138/yan-source/core/i18n"
)

// batchSize 是一批送回所有者的路径数：太小唤醒太频繁，太大首批出现得慢。
const batchSize = 512

// MaxFiles 是索引的上限：超过就停止遍历，避免误把主目录这类巨型目录当项目。
const MaxFiles = 200_000

// ErrTooMany 表示文件数超过 MaxFiles，索引只含前一部分。
var ErrTooMany = i18n.Error("Too many files: Quick Open indexed only the first 200,000")

// Walk 并行遍历 root，跳过永远隐藏的目录和被 gitignore 忽略的条目，
// 把相对路径分批交给 emit。emit 可能在任意 goroutine 上调用，调用方应该用 Post 转交所有者。
// ctx 取消后尽快返回。
func Walk(ctx context.Context, root string, ig *Ignore, emit func([]string)) error {
	var (
		mu    sync.Mutex
		batch = make([]string, 0, batchSize)
		total int
	)
	flush := func() {
		if len(batch) > 0 {
			emit(batch)
			batch = make([]string, 0, batchSize)
		}
	}
	conf := fastwalk.Config{Follow: false}
	err := fastwalk.Walk(&conf, root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil // 没权限的目录之类：跳过，不中断整个遍历
		}
		if path == root {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		if Hidden(d.Name()) || ig.Match(rel, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			ig.LoadDir(rel) // fastwalk 先回调目录本身，稍后才列它的子项，所以这里读到的规则来得及生效
			return nil
		}
		// fastwalk 的回调没有 worker 编号，所以用一把小锁保护共享批次。
		mu.Lock()
		defer mu.Unlock()
		if total >= MaxFiles {
			return ErrTooMany
		}
		total++
		batch = append(batch, filepath.ToSlash(rel))
		if len(batch) == batchSize {
			flush()
		}
		return nil
	})
	mu.Lock()
	flush()
	mu.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
