// Package fuzzy 封装 sahilm/fuzzy，给快速打开做文件路径的模糊匹配。
package fuzzy

import (
	"context"
	"slices"
	"strings"

	sf "github.com/sahilm/fuzzy"
)

// Result 是一条命中。Matched 是 Path 中命中字符的字节下标（用于加粗显示）。
type Result struct {
	Path    string
	Matched []int
	Score   int
}

// block 是两次检查取消之间处理的路径数：保证 Esc 或继续打字后很快停下。
const block = 4096

// Match 在 paths 中查找 query，返回得分最高的 limit 条。ctx 取消时返回 nil。
// paths 只读，可以是所有者正在追加的切片的一个前缀（调用方传 s[:n:n]）。
func Match(ctx context.Context, query string, paths []string, limit int) []Result {
	query = strings.ReplaceAll(query, " ", "")
	if query == "" {
		return nil
	}
	var all []Result
	for i := 0; i < len(paths); i += block {
		if ctx.Err() != nil {
			return nil
		}
		for _, m := range sf.FindNoSort(query, paths[i:min(i+block, len(paths))]) {
			all = append(all, Result{Path: m.Str, Matched: m.MatchedIndexes, Score: m.Score})
		}
	}
	// 得分高的在前；同分时路径短的在前（更可能是你要的那个）
	slices.SortStableFunc(all, func(a, b Result) int {
		if a.Score != b.Score {
			return b.Score - a.Score
		}
		return len(a.Path) - len(b.Path)
	})
	return all[:min(limit, len(all))]
}
