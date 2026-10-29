package edit

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/text/width"
)

// Insert 在每个选择区处插入 s（有选中内容时替换）。多光标天然成立。
func (b *Buffer) Insert(s string, o Origin) Edit {
	cs := make(text.ChangeSet, 0, len(b.sels.Ranges))
	for _, r := range b.sels.Ranges {
		cs = append(cs, text.Change{Start: r.From(), End: r.To(), Text: s})
	}
	return b.Apply(Transaction{Changes: cs, Origin: o})
}

// Newline 插入换行并沿用当前行的缩进；换行风格跟随文件（CRLF 文件插入 "\r\n"）。
func (b *Buffer) Newline() Edit {
	nl := "\n"
	if b.snap.Meta().EOL == text.EOLCRLF {
		nl = "\r\n"
	}
	cs := make(text.ChangeSet, 0, len(b.sels.Ranges))
	for _, r := range b.sels.Ranges {
		ls, le := b.snap.LineRange(b.snap.LineAt(r.From()))
		line := b.snap.String(ls, le)
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if r.From()-ls < len(indent) {
			indent = indent[:r.From()-ls]
		}
		cs = append(cs, text.Change{Start: r.From(), End: r.To(), Text: nl + indent})
	}
	return b.Apply(Transaction{Changes: cs, Origin: OriginTyping})
}

// DeleteBackward 删除选中内容；没有选中时删除光标前一个字素（"\r\n" 视为一个）。
func (b *Buffer) DeleteBackward() (Edit, bool) {
	return b.deleteBy(func(pos int) int { return PrevBoundary(b.snap, pos) })
}

// DeleteForward 删除选中内容；没有选中时删除光标后一个字素。
func (b *Buffer) DeleteForward() (Edit, bool) {
	return b.deleteBy(func(pos int) int { return NextBoundary(b.snap, pos) })
}

func (b *Buffer) deleteBy(target func(int) int) (Edit, bool) {
	var cs text.ChangeSet
	for _, r := range b.sels.Ranges {
		from, to := r.From(), r.To()
		if from == to {
			t := target(from)
			from, to = min(from, t), max(from, t)
		}
		if from < to {
			cs = append(cs, text.Change{Start: from, End: to})
		}
	}
	if len(cs) == 0 {
		return Edit{}, false
	}
	return b.Apply(Transaction{Changes: mergeAdjacent(cs), Origin: OriginTyping}), true
}

// mergeAdjacent 合并相邻或重叠的删除（多个光标删到同一处时）。
func mergeAdjacent(cs text.ChangeSet) text.ChangeSet {
	cs = cs.Normalize()
	out := cs[:1]
	for _, c := range cs[1:] {
		last := &out[len(out)-1]
		if c.Start <= last.End {
			last.End = max(last.End, c.End)
			continue
		}
		out = append(out, c)
	}
	return out
}

// PrevBoundary 返回 pos 之前最近的字素边界；行首则跳到上一行行尾（越过 "\n" 或 "\r\n"）。
func PrevBoundary(s text.Snapshot, pos int) int {
	if pos <= 0 {
		return 0
	}
	line := s.LineAt(pos)
	ls, le := s.LineRange(line)
	if pos <= ls {
		_, prevEnd := s.LineRange(line - 1)
		return prevEnd
	}
	pos = min(pos, le)
	buf := s.AppendRange(nil, ls, pos)
	prev := 0
	width.Graphemes(buf, func(start, end, _ int) bool {
		if end < len(buf) {
			prev = end
		}
		return true
	})
	return ls + prev
}

// NextBoundary 返回 pos 之后最近的字素边界；行尾则跳到下一行行首。
func NextBoundary(s text.Snapshot, pos int) int {
	if pos >= s.Len() {
		return s.Len()
	}
	line := s.LineAt(pos)
	_, le := s.LineRange(line)
	if pos >= le {
		if line+1 >= s.LineCount() {
			return s.Len()
		}
		return s.LineStart(line + 1)
	}
	buf := s.AppendRange(nil, pos, le)
	next := len(buf)
	width.Graphemes(buf, func(_, end, _ int) bool {
		next = end
		return false
	})
	return pos + next
}

type class int

const (
	clsSpace class = iota
	clsWord
	clsPunct
	clsCJK
)

func classOf(r rune) class {
	switch {
	case unicode.IsSpace(r):
		return clsSpace
	case unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r):
		return clsCJK // 中文没有空格分词：一段连续汉字算一个「词」
	case r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
		return clsWord
	}
	return clsPunct
}

// WordRight 跳到下一个词尾（Option+→ 的语义，与 VS Code 一致：先跳过空白，再跳过同类字符）。
func WordRight(s text.Snapshot, pos int) int {
	_, le := s.LineRange(s.LineAt(pos))
	if pos >= le {
		return NextBoundary(s, pos)
	}
	buf := s.AppendRange(nil, pos, le)
	i := 0
	for i < len(buf) {
		r, n := utf8.DecodeRune(buf[i:])
		if classOf(r) != clsSpace {
			break
		}
		i += n
	}
	if i < len(buf) {
		r, _ := utf8.DecodeRune(buf[i:])
		c := classOf(r)
		for i < len(buf) {
			r, n := utf8.DecodeRune(buf[i:])
			if classOf(r) != c {
				break
			}
			i += n
		}
	}
	return pos + i
}

// WordLeft 跳到上一个词首。
func WordLeft(s text.Snapshot, pos int) int {
	ls, _ := s.LineRange(s.LineAt(pos))
	if pos <= ls {
		return PrevBoundary(s, pos)
	}
	buf := s.AppendRange(nil, ls, pos)
	i := len(buf)
	for i > 0 {
		r, n := utf8.DecodeLastRune(buf[:i])
		if classOf(r) != clsSpace {
			break
		}
		i -= n
	}
	if i > 0 {
		r, _ := utf8.DecodeLastRune(buf[:i])
		c := classOf(r)
		for i > 0 {
			r, n := utf8.DecodeLastRune(buf[:i])
			if classOf(r) != c {
				break
			}
			i -= n
		}
	}
	return ls + i
}

// WordAt 返回 pos 所在词的范围（双击选词用）。
func WordAt(s text.Snapshot, pos int) (int, int) {
	ls, le := s.LineRange(s.LineAt(pos))
	if ls == le {
		return pos, pos
	}
	buf := s.AppendRange(nil, ls, le)
	i := min(pos-ls, len(buf))
	if i == len(buf) {
		_, n := utf8.DecodeLastRune(buf)
		i -= n
	}
	r, _ := utf8.DecodeRune(buf[i:])
	c := classOf(r)
	a, z := i, i
	for a > 0 {
		r, n := utf8.DecodeLastRune(buf[:a])
		if classOf(r) != c {
			break
		}
		a -= n
	}
	for z < len(buf) {
		r, n := utf8.DecodeRune(buf[z:])
		if classOf(r) != c {
			break
		}
		z += n
	}
	return ls + a, ls + z
}

// SmartHome 返回 Home 键的目标：先到首个非空白字符，已经在那里则到行首（VS Code 行为）。
func SmartHome(s text.Snapshot, pos int) int {
	ls, le := s.LineRange(s.LineAt(pos))
	line := s.String(ls, le)
	first := ls + len(line) - len(strings.TrimLeft(line, " \t"))
	if pos == first {
		return ls
	}
	return first
}
