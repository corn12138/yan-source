package text

import (
	"bytes"
	"iter"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

type (
	// Encoding 是文件编码：保存时原样写回。
	Encoding uint8
	// EOL 是主流换行风格。
	EOL uint8
	// Tier 是文件分级：决定哪些功能开启。
	Tier uint8
)

const (
	EncUTF8 Encoding = iota
	EncUTF8BOM
)

const (
	EOLLF EOL = iota
	EOLCRLF
)

const (
	TierNormal Tier = iota // ≤ 8 MB：全部功能
	TierLarge              // 8–256 MB：只读、无高亮
)

func (e Encoding) String() string {
	if e == EncUTF8BOM {
		return "UTF-8 BOM"
	}
	return "UTF-8"
}

func (e EOL) String() string {
	if e == EOLCRLF {
		return "CRLF"
	}
	return "LF"
}

// Meta 描述快照对应的文件，各版本之间只读共享。
type Meta struct {
	Path     string
	Encoding Encoding
	EOL      EOL
	MixedEOL bool
	ModTime  time.Time
	Size     int64 // 读入时文件的字节数（和 ModTime 一起判断文件在磁盘上有没有被别人改过）
	Tier     Tier
	ReadOnly bool // 非 UTF-8 或超大文件：v0.0 只读打开
}

// Snapshot 是不可变的文档版本；复制为 O(1)，可无锁地交给任意 goroutine。
type Snapshot struct {
	t       tree
	version uint64 // 包级 atomic 计数器分配：全局唯一，永不复用
	meta    *Meta
}

var versions atomic.Uint64

func newSnapshot(t tree, m *Meta) Snapshot {
	return Snapshot{t: t, version: versions.Add(1), meta: m}
}

// FromString 用一段文本构造快照，主要给测试和新建文件用。
func FromString(s string, m Meta) Snapshot {
	return newSnapshot(newFlat(split([]byte(s))), &m)
}

func (s Snapshot) Version() uint64 { return s.version }
func (s Snapshot) Meta() Meta      { return *s.meta }
func (s Snapshot) Len() int        { return s.t.summary().Bytes }

// LineCount 返回逻辑行数；空文档也算 1 行，结尾换行后的空行也算 1 行。
func (s Snapshot) LineCount() int { return s.t.summary().Newlines + 1 }

// Chunks 依次产出 [from, to) 内的只读字节片段（零拷贝，调用方不得修改）。
func (s Snapshot) Chunks(from, to int) iter.Seq[[]byte] { return s.t.chunks(from, to) }

// LineStart 返回第 line 行（从 0 数）第一个字节的偏移。
func (s Snapshot) LineStart(line int) int {
	if line <= 0 {
		return 0
	}
	if line >= s.LineCount() {
		return s.Len()
	}
	leaf, start, before := s.t.seekLine(line)
	need := line - before.Newlines // 还要在本叶子里跨过几个换行
	for i, c := range leaf {
		if c == '\n' {
			need--
			if need == 0 {
				return start + i + 1
			}
		}
	}
	return s.Len() // 不可达：摘要保证换行在这片叶子里
}

// LineAt 返回偏移 off 所在的行号（从 0 数）。
func (s Snapshot) LineAt(off int) int {
	off = clamp(off, 0, s.Len())
	leaf, start, before := s.t.seekByte(off)
	return before.Newlines + bytes.Count(leaf[:off-start], []byte{'\n'})
}

// LineRange 返回第 line 行内容的 [start, end)，不含行尾的 "\n" 或 "\r\n"。
func (s Snapshot) LineRange(line int) (start, end int) {
	start = s.LineStart(line)
	if line+1 >= s.LineCount() {
		end = s.Len()
	} else {
		end = s.LineStart(line+1) - 1
		if end > start && s.ByteAt(end-1) == '\r' {
			end--
		}
	}
	return start, end
}

// ByteAt 返回偏移 off 处的字节。
func (s Snapshot) ByteAt(off int) byte {
	leaf, start, _ := s.t.seekByte(off)
	return leaf[off-start]
}

// AppendRange 把 [from, to) 追加到 dst 并返回。
func (s Snapshot) AppendRange(dst []byte, from, to int) []byte {
	for c := range s.Chunks(from, to) {
		dst = append(dst, c...)
	}
	return dst
}

// String 返回 [from, to) 的副本。
func (s Snapshot) String(from, to int) string { return string(s.AppendRange(nil, from, to)) }

// ByteToRune 把字节偏移换算成 rune 序号（Gio IME 用）。
func (s Snapshot) ByteToRune(off int) int {
	off = clamp(off, 0, s.Len())
	leaf, start, before := s.t.seekByte(off)
	return before.Runes + utf8.RuneCount(leaf[:off-start])
}

// UTF16Col 返回 off 在所在行内的 UTF-16 列（LSP 用）。
func (s Snapshot) UTF16Col(off int) int {
	ls := s.LineStart(s.LineAt(off))
	n := 0
	for c := range s.Chunks(ls, off) {
		n += summarize(c).UTF16
	}
	return n
}

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }
