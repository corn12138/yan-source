// Package edit 在 text 之上维护选择区、事务与撤销历史。
package edit

import (
	"errors"
	"io/fs"
	"os"
	"slices"
	"time"

	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/text"
)

// Range 是一个选择区；Anchor == Head 时就是光标。GoalX 为上下移动的目标 x，-1 表示未设。
type Range struct {
	Anchor, Head int
	GoalX        int32
}

// Cursor 返回落在 pos 的光标。
func Cursor(pos int) Range { return Range{Anchor: pos, Head: pos, GoalX: -1} }

func (r Range) Empty() bool { return r.Anchor == r.Head }
func (r Range) From() int   { return min(r.Anchor, r.Head) }
func (r Range) To() int     { return max(r.Anchor, r.Head) }

// Selections 按起点排序且互不重叠；Primary 是真实光标所在的那一个。
type Selections struct {
	Ranges  []Range
	Primary int
}

// Single 返回只有一个选择区的 Selections。
func Single(r Range) Selections { return Selections{Ranges: []Range{r}} }

// Main 返回主选择区。
func (s Selections) Main() Range { return s.Ranges[s.Primary] }

// Map 把选择区随修改集一起挪动。
func (s Selections) Map(cs text.ChangeSet) Selections {
	out := Selections{Ranges: make([]Range, len(s.Ranges)), Primary: s.Primary}
	for i, r := range s.Ranges {
		out.Ranges[i] = Range{
			Anchor: cs.MapPos(r.Anchor, text.AssocAfter),
			Head:   cs.MapPos(r.Head, text.AssocAfter),
			GoalX:  -1,
		}
	}
	return out
}

// Origin 标记修改来源：决定撤销分组与订阅者行为。
type Origin uint8

const (
	OriginTyping Origin = iota
	OriginPaste
	OriginIME
	OriginCommand
	OriginUndo
)

// Transaction 是一次原子修改。
type Transaction struct {
	Changes text.ChangeSet
	After   Selections // 为空则由 Changes 映射
	Origin  Origin
}

// Edit 是发给订阅者（高亮、搜索标记、文档重解析、LSP）的通知。
type Edit struct {
	Before, After text.Snapshot
	Changes       text.ChangeSet
	Origin        Origin
}

// Revision 是撤销历史中的一组。快照式撤销：直接回到 Before，不需要逆向重放。
type Revision struct {
	Before, After       text.Snapshot
	SelBefore, SelAfter Selections
	Origin              Origin
	At                  time.Time
}

// groupWindow 内连续打字合并成一个撤销步，和 VS Code 的手感接近。
const groupWindow = time.Second

// History 是线性撤销栈。TODO(M1)：按估算的保留字节数封顶（默认 64 MB）。
type History struct {
	revs    []Revision
	cur     int // revs[:cur] 已应用
	saved   int // 与磁盘一致时的 cur；-1 表示永远不一致
	group   int // >0 时修改合成一步（BeginGroup）
	groupAt int // 这一组的那一步；-1 表示还没有
}

// Buffer 是一个文档当前快照的唯一所有者；所有修改都经过 Apply。
type Buffer struct {
	snap        text.Snapshot
	sels        Selections
	hist        History
	subs        []func(Edit)
	now         func() time.Time // 测试可替换
	diskContent text.Snapshot    // Immutable bytes last loaded/saved; edits must not advance the conflict baseline.
	disk        Stamp            // 上次读写磁盘时文件的样子：用来发现外部修改
	path        string           // 改过名之后的路径；"" 表示用快照里的
}

// Stamp 记下文件在磁盘上的一个版本（修改时间和大小）；零值表示文件不存在。
type Stamp struct {
	ModTime time.Time
	Size    int64
}

func (s Stamp) exists() bool { return !s.ModTime.IsZero() }

func stampOf(m text.Meta) Stamp { return Stamp{ModTime: m.ModTime, Size: m.Size} }

// StatStamp 读出 path 现在在磁盘上的版本；文件不在时返回零值。
func StatStamp(path string) (Stamp, error) {
	fi, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Stamp{}, nil
	}
	if err != nil {
		return Stamp{}, err
	}
	return Stamp{ModTime: fi.ModTime(), Size: fi.Size()}, nil
}

// DiskState 是磁盘上的文件相对编辑器上次读写时的样子。
type DiskState uint8

const (
	DiskSame    DiskState = iota // 没变（或者看不出来）
	DiskChanged                  // 被改过、被换掉，或者原来没有、现在有了
	DiskDeleted                  // 原来有，现在没了
)

// ErrConflict 表示磁盘上的文件在上次读写之后被别人改过：Save 不覆盖它。
var ErrConflict = i18n.Error("The file was changed on disk by another program")

// NewBuffer 用快照创建 buffer，光标在文首。
func NewBuffer(s text.Snapshot) *Buffer {
	return &Buffer{snap: s, sels: Single(Cursor(0)), now: time.Now, disk: stampOf(s.Meta()), diskContent: s}
}

// NewUnsaved 是一份从没和磁盘一致过的文字（找回来的未命名文档）：有未保存的修改，没有撤销记录。
func NewUnsaved(s text.Snapshot) *Buffer {
	b := NewBuffer(s)
	b.hist.saved = -1
	return b
}

// DiskStamp 返回上次加载或保存时文件在磁盘上的版本。
func (b *Buffer) DiskStamp() Stamp { return b.disk }

// DiskState 比较磁盘上的文件和上次加载或保存时的版本。读不了文件属性时当作没变，交给保存去报错。
func (b *Buffer) DiskState() DiskState { state, _ := b.diskState(); return state }

func (b *Buffer) diskState() (DiskState, error) { return diskStateOf(b.Path(), b.diskContent, b.disk) }

// DiskContent returns the immutable bytes last loaded or saved: the conflict baseline.
func (b *Buffer) DiskContent() text.Snapshot { return b.diskContent }

// DiskStateOf compares the file at path with a baseline taken from a buffer, so that the
// comparison can run off the goroutine that owns the buffer. Errors read as DiskSame.
func DiskStateOf(path string, base text.Snapshot, stamp Stamp) DiskState {
	state, _ := diskStateOf(path, base, stamp)
	return state
}

func diskStateOf(path string, base text.Snapshot, disk Stamp) (DiskState, error) {
	now, err := StatStamp(path)
	switch {
	case err != nil:
		return DiskSame, err
	case !now.exists() && disk.exists():
		return DiskDeleted, nil
	case now.exists() && !disk.exists():
		return DiskChanged, nil
	case now.exists() && (!now.ModTime.Equal(disk.ModTime) || now.Size != disk.Size):
		return DiskChanged, nil
	}
	if now.exists() {
		same, err := text.MatchesFile(base, path)
		if err != nil {
			return DiskSame, err
		}
		if !same {
			return DiskChanged, nil
		}
	}
	return DiskSame, nil
}

// Reload 用磁盘上的新内容替换当前内容。以差异形式应用，所以选区会跟着挪动、也可以撤销；
// 应用后视为「已保存」。
func (b *Buffer) Reload(s text.Snapshot) {
	if cs := text.Diff(b.snap, s); len(cs) > 0 {
		b.applySnapshot(Transaction{Changes: cs, Origin: OriginCommand}, s)
		b.BreakGroup()
	}
	b.hist.saved = b.hist.cur
	b.disk = stampOf(s.Meta())
	b.diskContent = s
}

func (b *Buffer) Snapshot() text.Snapshot { return b.snap }
func (b *Buffer) Selections() Selections  { return b.sels }
func (b *Buffer) Dirty() bool             { return b.hist.cur != b.hist.saved }
func (b *Buffer) OnEdit(fn func(Edit))    { b.subs = append(b.subs, fn) }

// SetSelections 只移动光标/选区，不产生修订。
func (b *Buffer) SetSelections(s Selections) {
	s.Ranges = slices.Clone(s.Ranges) // 历史记录里存着旧的 Selections，绝不原地修改
	n := b.snap.Len()
	for i := range s.Ranges {
		s.Ranges[i].Anchor = clamp(s.Ranges[i].Anchor, 0, n)
		s.Ranges[i].Head = clamp(s.Ranges[i].Head, 0, n)
	}
	b.sels = merge(s)
}

// Apply 应用一个事务：生成新快照、记入历史、通知订阅者。
func (b *Buffer) Apply(tx Transaction) Edit {
	return b.applySnapshot(tx, tx.Changes.Apply(b.snap))
}

func (b *Buffer) applySnapshot(tx Transaction, after text.Snapshot) Edit {
	before, selBefore := b.snap, b.sels
	sels := tx.After
	if len(sels.Ranges) == 0 {
		sels = b.sels.Map(tx.Changes)
	}
	b.snap = after
	b.SetSelections(sels)
	b.record(before, after, selBefore, tx.Origin)
	return b.notify(Edit{Before: before, After: after, Changes: tx.Changes, Origin: tx.Origin})
}

// SetEOL stores both the converted bytes and newline preference in one revision,
// including empty documents whose future Enter presses need the new preference.
func (b *Buffer) SetEOL(eol text.EOL) {
	if b.snap.Meta().ReadOnly || eol > text.EOLCRLF {
		return
	}
	after, changes := b.snap.ConvertEOL(eol)
	if after.Version() == b.snap.Version() {
		return
	}
	b.BreakGroup()
	sels := b.sels.Map(changes)
	for i, r := range sels.Ranges {
		sels.Ranges[i].Anchor = after.CaretOffset(r.Anchor)
		sels.Ranges[i].Head = after.CaretOffset(r.Head)
	}
	b.applySnapshot(Transaction{Changes: changes, After: sels, Origin: OriginCommand}, after)
	b.BreakGroup()
}

func (b *Buffer) record(before, after text.Snapshot, selBefore Selections, o Origin) {
	now := b.now()
	h := &b.hist
	h.revs = h.revs[:h.cur] // 新修改丢弃重做分支
	if h.saved > h.cur {
		h.saved = -1
	}
	if n := len(h.revs); h.group > 0 && h.groupAt >= 0 && n-1 >= h.groupAt && h.saved != n {
		// 一组修改（vim 的改动和随后打的字）合成一步撤销。
		last := &h.revs[n-1]
		last.After, last.SelAfter, last.At = after, b.sels, now
		return
	}
	if n := len(h.revs); n > 0 && o == OriginTyping && h.saved != n {
		last := &h.revs[n-1]
		if last.Origin == OriginTyping && now.Sub(last.At) < groupWindow {
			last.After, last.SelAfter, last.At = after, b.sels, now
			return
		}
	}
	h.revs = append(h.revs, Revision{Before: before, After: after, SelBefore: selBefore, SelAfter: b.sels, Origin: o, At: now})
	h.cur = len(h.revs)
	if h.group > 0 && h.groupAt < 0 {
		h.groupAt = h.cur - 1 // 这一组从这一步开始
	}
}

// BeginGroup 开始一组修改：直到 EndGroup，所有修改合成一步撤销。join 为真时并进上一步（它是这一组的开头）。
func (b *Buffer) BeginGroup(join bool) {
	h := &b.hist
	h.group++
	h.groupAt = -1
	if join && h.cur > 0 && h.cur == len(h.revs) {
		h.groupAt = h.cur - 1
	}
}

// EndGroup 结束一组修改。
func (b *Buffer) EndGroup() {
	if b.hist.group > 0 {
		b.hist.group--
	}
	b.BreakGroup()
}

// BreakGroup 结束当前的打字合并（移动光标后再打字应是新的撤销步）。
func (b *Buffer) BreakGroup() {
	if n := len(b.hist.revs); n > 0 {
		b.hist.revs[n-1].At = time.Time{}
	}
}

// Undo 回到上一个修订之前的快照。
func (b *Buffer) Undo() (Edit, bool) {
	h := &b.hist
	if h.cur == 0 {
		return Edit{}, false
	}
	h.cur--
	r := h.revs[h.cur]
	return b.jump(r.Before, r.SelBefore), true
}

// Redo 重新应用被撤销的修订。
func (b *Buffer) Redo() (Edit, bool) {
	h := &b.hist
	if h.cur == len(h.revs) {
		return Edit{}, false
	}
	r := h.revs[h.cur]
	h.cur++
	return b.jump(r.After, r.SelAfter), true
}

func (b *Buffer) jump(to text.Snapshot, sels Selections) Edit {
	before := b.snap
	b.snap = to
	b.SetSelections(Selections{Ranges: slices.Clone(sels.Ranges), Primary: sels.Primary})
	return b.notify(Edit{Before: before, After: to, Changes: text.Diff(before, to), Origin: OriginUndo})
}

func (b *Buffer) notify(e Edit) Edit {
	for _, fn := range b.subs {
		fn(e)
	}
	return e
}

// Save 原子写回磁盘并记下「已保存」位置。磁盘上的文件在上次读写之后被改过（或者原来没有、
// 现在有了）时不写，返回 ErrConflict；文件被删掉了照常写回（磁盘上没有可丢的东西）。
func (b *Buffer) Save() error {
	return b.save(func() error {
		state, err := b.diskState()
		if err != nil {
			return err
		}
		if state == DiskChanged {
			return ErrConflict
		}
		return nil
	})
}

// Overwrite is the explicit conflict-resolution action; ordinary and automatic saves verify first.
func (b *Buffer) Overwrite() error { return b.save(nil) }

// SaveAs 把文字写到 path，写成了才让 path 成为文档的文件（新建的、另存为的）。expect 是用户选
// path 时它在磁盘上的样子（零值：那时还没有这个文件）；写的时候已经不是那样了就返回 ErrConflict，
// 不覆盖。写不成时文档的路径、未保存状态都不变。
func (b *Buffer) SaveAs(path string, expect Stamp) error {
	if err := text.SaveChecked(b.snap, path, func() error {
		now, err := StatStamp(path)
		if err != nil {
			return err
		}
		if now.exists() != expect.exists() || !now.ModTime.Equal(expect.ModTime) || now.Size != expect.Size {
			return ErrConflict
		}
		return nil
	}); err != nil {
		return err
	}
	b.path = path
	b.saved()
	return nil
}

func (b *Buffer) save(verify func() error) error {
	if err := text.SaveChecked(b.snap, b.Path(), verify); err != nil {
		return err
	}
	b.saved()
	return nil
}

// saved 记下刚写到磁盘上的版本：之后的修改、外部改动都和它比。
func (b *Buffer) saved() {
	b.hist.saved = b.hist.cur
	b.BreakGroup()
	b.diskContent = b.snap
	if st, err := StatStamp(b.Path()); err == nil {
		b.disk = st
	}
}

// Path 返回文件路径。
func (b *Buffer) Path() string {
	if b.path != "" {
		return b.path
	}
	return b.snap.Meta().Path
}

// Rename 让文档以后存到 path（资源管理器里改了名字、挪了位置）。撤销到旧快照也不会改回旧路径。
func (b *Buffer) Rename(path string) { b.path = path }

// Exists 报告文件是否已在磁盘上（新建文件第一次保存前为 false）。
func (b *Buffer) Exists() bool {
	_, err := os.Stat(b.Path())
	return err == nil
}

// merge 排序并合并重叠的选择区。
func merge(s Selections) Selections {
	if len(s.Ranges) <= 1 {
		return s
	}
	primary := s.Ranges[s.Primary]
	rs := slices.Clone(s.Ranges)
	slices.SortFunc(rs, func(a, b Range) int { return a.From() - b.From() })
	out := rs[:1]
	for _, r := range rs[1:] {
		last := &out[len(out)-1]
		if r.From() <= last.To() {
			if r.To() > last.To() {
				last.Head, last.Anchor = r.To(), last.From()
			}
			continue
		}
		out = append(out, r)
	}
	p := 0
	for i, r := range out {
		if r.From() <= primary.Head && primary.Head <= r.To() {
			p = i
		}
	}
	return Selections{Ranges: out, Primary: p}
}

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }
