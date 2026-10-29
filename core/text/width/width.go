// Package width 是唯一的宽度预言机：内核布局与 tcell 绘制用同一套规则，
// 光标位置才不会和终端实际画出来的字错位。
package width

import (
	"os"
	"strings"

	"github.com/clipperhouse/displaywidth"
)

// opts 与 tcell v3 internal/widthutil 的选项保持一致：只认 RUNEWIDTH_EASTASIAN。
var opts = func() displaywidth.Options {
	if rw := strings.ToLower(os.Getenv("RUNEWIDTH_EASTASIAN")); rw == "1" || rw == "true" || rw == "yes" {
		return displaywidth.Options{EastAsianWidth: true}
	}
	return displaywidth.Options{}
}()

// Graphemes 按字素簇遍历 b，回调给出 [start, end) 与显示宽度（单元格数）。
// 回调返回 false 时停止。
func Graphemes(b []byte, fn func(start, end, w int) bool) {
	g := opts.BytesGraphemes(b)
	pos := 0
	for g.Next() {
		v := g.Value()
		if !fn(pos, pos+len(v), g.Width()) {
			return
		}
		pos += len(v)
	}
}

// String 返回字符串的显示宽度。
func String(s string) int { return opts.String(s) }
