// Package text 提供不可变分块 rope、快照与修改集。所有偏移都是 UTF-8 字节偏移。
//
// 这是 M1a 实现：叶子平铺在一个切片里，外加一份前缀和。修改时只重建受影响的
// 叶子并重算前缀和（O(叶子数)），对 ≤ 8 MB 的普通文件足够快。M1b 会把同样的
// tree 接口换成持久化 B+ 树，调用方无感知。
package text

import "unicode/utf8"

// Summary 按叶子缓存；只含 int，GC 不扫描。
type Summary struct{ Bytes, Newlines, Runes, UTF16 int }

// Add 返回两段摘要之和。
func (a Summary) Add(b Summary) Summary {
	return Summary{a.Bytes + b.Bytes, a.Newlines + b.Newlines, a.Runes + b.Runes, a.UTF16 + b.UTF16}
}

// summarize 统计一段字节。ASCII 走快路径：每字节 1 rune、1 个 UTF-16 码元。
func summarize(b []byte) Summary {
	s := Summary{Bytes: len(b)}
	for i := 0; i < len(b); {
		c := b[i]
		if c < utf8.RuneSelf {
			if c == '\n' {
				s.Newlines++
			}
			s.Runes++
			s.UTF16++
			i++
			continue
		}
		r, n := utf8.DecodeRune(b[i:])
		s.Runes++
		if r >= 0x10000 {
			s.UTF16 += 2
		} else {
			s.UTF16++
		}
		i += n
	}
	return s
}
