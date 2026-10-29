package text

import (
	"iter"
	"runtime"
	"sort"
	"sync"
	"unicode/utf8"
)

// leafMax 是叶子的目标上限。2 KB 让一次按键只重建 1～2 片叶子。
const leafMax = 2048

// tree 隐藏存储：M1a 平铺叶子数组 + 前缀和，M1b 持久化 B+ 树。实现方绝不修改接收者。
type tree interface {
	summary() Summary
	replace(start, end int, text []byte) tree
	seekByte(off int) (leaf []byte, leafStart int, before Summary)
	seekLine(line int) (leaf []byte, leafStart int, before Summary)
	chunks(from, to int) iter.Seq[[]byte]
}

// flat 是 M1a 的 tree：leaves 与 pre 一经构造就不再修改，可被多个快照共享。
type flat struct {
	leaves [][]byte
	pre    []Summary // pre[i] = leaves[:i] 的摘要之和；len(pre) == len(leaves)+1
}

func newFlat(leaves [][]byte) *flat {
	f := &flat{leaves: leaves, pre: make([]Summary, len(leaves)+1)}
	sums := make([]Summary, len(leaves))
	summarizeAll(leaves, sums)
	for i, s := range sums {
		f.pre[i+1] = f.pre[i].Add(s)
	}
	return f
}

// summarizeAll 在大文件上按 CPU 数并行统计各叶子。
func summarizeAll(leaves [][]byte, out []Summary) {
	n := len(leaves)
	workers := runtime.GOMAXPROCS(0)
	if n < 256 || workers < 2 {
		for i, l := range leaves {
			out[i] = summarize(l)
		}
		return
	}
	var wg sync.WaitGroup
	per := (n + workers - 1) / workers
	for w := 0; w < n; w += per {
		lo, hi := w, min(w+per, n)
		wg.Go(func() {
			for i := lo; i < hi; i++ {
				out[i] = summarize(leaves[i])
			}
		})
	}
	wg.Wait()
}

// split 把 b 切成不超过 leafMax 的叶子，切点落在 UTF-8 字符边界上。叶子与 b 共享内存。
func split(b []byte) [][]byte {
	var out [][]byte
	for len(b) > 0 {
		k := min(len(b), leafMax)
		for k < len(b) && k > 0 && !utf8.RuneStart(b[k]) {
			k--
		}
		if k == 0 { // 不可能出现的超长非法序列：硬切
			k = min(len(b), leafMax)
		}
		out = append(out, b[:k:k]) // 三下标切片：防止 append 写穿到相邻叶子
		b = b[k:]
	}
	return out
}

func (f *flat) summary() Summary { return f.pre[len(f.pre)-1] }

// leafAt 返回包含字节 off 的叶子下标；off == 总长时返回最后一片。
func (f *flat) leafAt(off int) int {
	i := sort.Search(len(f.leaves), func(i int) bool { return f.pre[i+1].Bytes > off })
	return min(i, max(len(f.leaves)-1, 0))
}

func (f *flat) replace(start, end int, text []byte) tree {
	if len(f.leaves) == 0 {
		buf := append([]byte(nil), text...)
		return newFlat(split(buf))
	}
	i, j := f.leafAt(start), f.leafAt(max(end-1, start))
	li, lj := f.leaves[i], f.leaves[j]
	si, ej := start-f.pre[i].Bytes, end-f.pre[j].Bytes

	buf := make([]byte, 0, si+len(text)+len(lj)-ej)
	buf = append(buf, li[:si]...)
	buf = append(buf, text...)
	buf = append(buf, lj[ej:]...)
	mid := split(buf)

	leaves := make([][]byte, 0, len(f.leaves)-(j-i+1)+len(mid))
	leaves = append(leaves, f.leaves[:i]...)
	leaves = append(leaves, mid...)
	leaves = append(leaves, f.leaves[j+1:]...)

	nf := &flat{leaves: leaves, pre: make([]Summary, len(leaves)+1)}
	copy(nf.pre, f.pre[:i+1]) // 修改点之前的前缀和不变
	for k := i; k < len(leaves); k++ {
		nf.pre[k+1] = nf.pre[k].Add(summarize(leaves[k]))
	}
	return nf
}

func (f *flat) seekByte(off int) ([]byte, int, Summary) {
	if len(f.leaves) == 0 {
		return nil, 0, Summary{}
	}
	i := f.leafAt(off)
	return f.leaves[i], f.pre[i].Bytes, f.pre[i]
}

// seekLine 返回包含第 line 个换行符（从 1 数）的叶子；line == 0 返回第一片。
func (f *flat) seekLine(line int) ([]byte, int, Summary) {
	if len(f.leaves) == 0 || line == 0 {
		if len(f.leaves) == 0 {
			return nil, 0, Summary{}
		}
		return f.leaves[0], 0, Summary{}
	}
	i := sort.Search(len(f.leaves), func(i int) bool { return f.pre[i+1].Newlines >= line })
	i = min(i, len(f.leaves)-1)
	return f.leaves[i], f.pre[i].Bytes, f.pre[i]
}

func (f *flat) chunks(from, to int) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		if from >= to || len(f.leaves) == 0 {
			return
		}
		for i := f.leafAt(from); i < len(f.leaves); i++ {
			ls := f.pre[i].Bytes
			if ls >= to {
				return
			}
			l := f.leaves[i]
			lo, hi := max(from-ls, 0), min(to-ls, len(l))
			if lo < hi && !yield(l[lo:hi]) {
				return
			}
		}
	}
}
