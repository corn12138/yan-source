package text

import "slices"

// Change 用 Text 替换旧快照的字节区间 [Start, End)。
type Change struct {
	Start, End int
	Text       string
}

// ChangeSet 有序、互不重叠，所有偏移都相对同一个旧快照。
type ChangeSet []Change

// Assoc 决定恰好落在插入点上的位置跟到插入文本的哪一侧。
type Assoc int8

const (
	AssocBefore Assoc = -1 // 留在插入文本之前
	AssocAfter  Assoc = 1  // 跟到插入文本之后（光标默认）
)

// Normalize 按起点排序并检查重叠；调用方构造多光标修改后调用一次。
func (cs ChangeSet) Normalize() ChangeSet {
	slices.SortStableFunc(cs, func(a, b Change) int { return a.Start - b.Start })
	return cs
}

// Apply 返回应用修改后的新快照；旧快照不受影响。
// 从后往前应用，前面修改的偏移就不会被后面的修改挪动。
func (cs ChangeSet) Apply(s Snapshot) Snapshot {
	if len(cs) == 0 {
		return s
	}
	t := s.t
	for i := len(cs) - 1; i >= 0; i-- {
		c := cs[i]
		t = t.replace(c.Start, c.End, []byte(c.Text))
	}
	return newSnapshot(t, s.meta)
}

// Invert 返回把 Apply 的结果还原成 old 的修改集（偏移相对新快照）。
func (cs ChangeSet) Invert(old Snapshot) ChangeSet {
	inv := make(ChangeSet, len(cs))
	delta := 0
	for i, c := range cs {
		start := c.Start + delta
		inv[i] = Change{Start: start, End: start + len(c.Text), Text: old.String(c.Start, c.End)}
		delta += len(c.Text) - (c.End - c.Start)
	}
	return inv
}

// MapPos 把旧快照中的位置映射到新快照。被删除区间内的位置落到替换文本的一端。
func (cs ChangeSet) MapPos(pos int, a Assoc) int {
	delta := 0
	for _, c := range cs {
		switch {
		case pos < c.Start:
			return pos + delta
		case pos > c.End:
			delta += len(c.Text) - (c.End - c.Start)
		case pos == c.Start && a == AssocBefore:
			return c.Start + delta
		default: // 落在被替换区间里（含两端）：跟到新文本之后
			return c.Start + delta + len(c.Text)
		}
	}
	return pos + delta
}

// Diff 用公共前后缀求出把 a 变成 b 的单个修改（撤销/重做时通知订阅者用）。
// 最坏 O(n)，但只在撤销时跑一次。
func Diff(a, b Snapshot) ChangeSet {
	al, bl := a.Len(), b.Len()
	p := 0
	for p < al && p < bl && a.ByteAt(p) == b.ByteAt(p) {
		p++
	}
	s := 0
	for s < al-p && s < bl-p && a.ByteAt(al-1-s) == b.ByteAt(bl-1-s) {
		s++
	}
	if p == al && p == bl {
		return nil
	}
	return ChangeSet{{Start: p, End: al - s, Text: b.String(p, bl-s)}}
}
