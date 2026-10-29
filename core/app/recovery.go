package app

import (
	"errors"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/recovery"
	"github.com/corn12138/yan-source/core/text"
)

// 未命名文档的文字跨退出保留（UP-38 第二批）。每个编辑器一个备份文件：开着文件夹的按文件夹认，
// 没开文件夹的有自己的编号。编辑停下一秒、关掉或另存为一个未命名文档后，在后台把这一刻所有未命名
// 文档的文字整个写一遍；关窗口、退出、重启更新、换文件夹前同步写一遍，写成了这些文档就不算
// 「未保存」，下次打开时回来。只管未命名的文档：有路径的文件照旧问保存还是放弃。

// recoveryDir 是备份文件所在的文件夹（测试里换掉）。
var recoveryDir = func() string { return filepath.Join(filepath.Dir(config.UserPath()), "recovery") }

const (
	recoveryDelay = time.Second
	keepTimeout   = 3 * time.Second
)

// recoveryJob 是要写的一次备份：文字是不可变的快照，在写的 goroutine 里才变成字符串。
type recoveryJob struct {
	dir     string
	cp      recovery.Checkpoint
	snaps   []text.Snapshot
	waiters []chan error
}

// recoveryWriter 一个一个地写备份。等着写的只留最新的一次（它包含了更早那次要写的一切）；
// 一次写的代号不比这个编号上次写成的新就不写，晚到的旧备份不会把已经放弃的文档写回来。
type recoveryWriter struct {
	mu      sync.Mutex
	pending *recoveryJob
	kick    chan struct{}
	stop    chan struct{}
	stopped chan struct{}
	written map[string]uint64
	failed  func(error)
}

func newRecoveryWriter(failed func(error)) *recoveryWriter {
	w := &recoveryWriter{kick: make(chan struct{}, 1), stop: make(chan struct{}), stopped: make(chan struct{}),
		written: map[string]uint64{}, failed: failed}
	go w.run()
	return w
}

func (w *recoveryWriter) submit(j *recoveryJob) {
	w.mu.Lock()
	if old := w.pending; old != nil {
		j.waiters = append(old.waiters, j.waiters...)
	}
	w.pending = j
	w.mu.Unlock()
	select {
	case w.kick <- struct{}{}:
	default:
	}
}

func (w *recoveryWriter) run() {
	defer close(w.stopped)
	for {
		select {
		case <-w.kick:
			w.writePending()
		case <-w.stop:
			w.writePending() // 关编辑器时还没写的那次也写完
			return
		}
	}
}

func (w *recoveryWriter) writePending() {
	w.mu.Lock()
	j := w.pending
	w.pending = nil
	w.mu.Unlock()
	if j == nil {
		return
	}
	var err error
	if j.cp.Gen > w.written[j.cp.ID] {
		for i, s := range j.snaps {
			j.cp.Drafts[i].Text = s.String(0, s.Len())
		}
		if err = recovery.Write(j.dir, j.cp); err == nil {
			w.written[j.cp.ID] = j.cp.Gen
		}
	}
	if err != nil && len(j.waiters) == 0 && w.failed != nil {
		w.failed(err)
	}
	for _, c := range j.waiters {
		c <- err
	}
}

// close 写完还没写的备份再停下（最多等 keepTimeout）。
func (w *recoveryWriter) close() {
	close(w.stop)
	select {
	case <-w.stopped:
	case <-time.After(keepTimeout):
	}
}

// recoveryOwner 是这个编辑器现在读写的备份编号，拿着它的锁：开着文件夹时是文件夹的编号。
// 那个编号被另一个 yan 进程占着时换成一个自己的编号，这里的未命名文件另外备份，不和那边的
// 互相覆盖（下次在没开文件夹的窗口里找回）。
func (e *Editor) recoveryOwner() (string, error) {
	r := &e.rec
	want := r.rootless
	if e.ws != nil {
		want = recovery.FolderID(e.ws.root)
		if r.owned != "" && r.ownedFor == e.ws.root {
			return r.owned, nil // 这个文件夹用的编号（可能是换过的）
		}
	} else if want != "" && want == r.owned {
		return want, nil
	}
	r.lock.Release()
	r.lock, r.owned = nil, ""
	if want == "" {
		want = recovery.NewRootlessID()
		r.rootless = want
	}
	l, err := recovery.Acquire(recoveryDir(), want)
	if errors.Is(err, recovery.ErrInUse) && e.ws != nil {
		want = recovery.NewRootlessID()
		l, err = recovery.Acquire(recoveryDir(), want)
		e.status = i18n.T("Another yan has this folder open: untitled files here are backed up separately; recover them next time in a window without a folder")
	}
	if err != nil {
		return "", err
	}
	r.lock, r.owned, r.ownedFor = l, want, ""
	if e.ws != nil {
		r.ownedFor = e.ws.root
	}
	return want, nil
}

// recoveryState 是编辑器里备份的那些状态。
type recoveryState struct {
	w        *recoveryWriter
	gen      uint64
	soon     int
	touched  bool   // 有过未命名的文档：之后的备份要写（没有了就删掉文件）
	rootless string // 没开文件夹时的编号
	closed   bool   // 编辑器关了：不再写
	lock     *recovery.Lock
	owned    string // 拿着锁的编号
	ownedFor string // 它是给哪个文件夹的（没开文件夹时为空）
}

// recoveryJob 拍下这一刻所有有修改的未命名文档；拿不到备份编号时返回错误。
func (e *Editor) recoveryJob() (*recoveryJob, error) {
	id, err := e.recoveryOwner()
	if err != nil {
		return nil, err
	}
	e.sync()
	e.rec.gen++
	j := &recoveryJob{dir: recoveryDir(), cp: recovery.Checkpoint{ID: id, Gen: e.rec.gen}}
	if e.ws != nil {
		j.cp.Root = e.ws.root
	}
	seen := map[*document]bool{}
	for ci, c := range e.cols {
		for _, t := range c.tabs {
			d := t.doc
			if seen[d] || !d.untitled() || !d.buf.Dirty() {
				continue
			}
			seen[d] = true
			s := d.buf.Snapshot()
			m := e.sels(t).Main()
			j.cp.Drafts = append(j.cp.Drafts, recovery.Draft{Title: d.title, EOL: int(s.Meta().EOL), Column: ci, Anchor: m.Anchor, Head: m.Head})
			j.snaps = append(j.snaps, s)
		}
	}
	return j, nil
}

// submitCheckpoint 交给后台写；拿不到备份编号时在状态栏说。
func (e *Editor) submitCheckpoint() {
	j, err := e.recoveryJob()
	if err != nil {
		e.status = i18n.F("Couldn't write the backup of untitled files: %s", err.Error())
		return
	}
	e.recoveryWriter().submit(j)
}

func (e *Editor) recoveryWriter() *recoveryWriter {
	if e.rec.w == nil {
		e.rec.w = newRecoveryWriter(func(err error) {
			e.Post(func(e *Editor) { e.status = i18n.F("Couldn't write the backup of untitled files: %s", err.Error()) })
		})
	}
	return e.rec.w
}

// checkpointSoon 在编辑停下一会儿后备份未命名文档。
func (e *Editor) checkpointSoon() {
	if !e.rec.touched || e.rec.closed {
		return
	}
	e.rec.soon++
	soon := e.rec.soon
	time.AfterFunc(recoveryDelay, func() {
		e.Post(func(e *Editor) {
			if e.rec.soon == soon && !e.rec.closed {
				e.submitCheckpoint()
			}
		})
	})
}

// checkpointNow 马上在后台备份（关掉、另存为了一个未命名文档之后）。
func (e *Editor) checkpointNow() {
	if e.rec.touched && !e.rec.closed {
		e.rec.soon++
		e.submitCheckpoint()
	}
}

// KeepForRestart 把有修改的未命名文档写进备份并等它写完：成功后关窗口、退出不用再问它们，
// 下次打开时它们回来。没有这样的文档时什么都不做。
func (e *Editor) KeepForRestart() error {
	if !slices.ContainsFunc(e.docs, func(d *document) bool { return d.untitled() && d.buf.Dirty() }) {
		return nil
	}
	if e.rec.closed {
		return errors.New(i18n.T("The editor is already closed"))
	}
	e.rec.touched = true
	e.rec.soon++
	done := make(chan error, 1)
	j, err := e.recoveryJob()
	if err != nil {
		return err
	}
	j.waiters = []chan error{done}
	e.recoveryWriter().submit(j)
	select {
	case err := <-done:
		return err
	case <-time.After(keepTimeout):
		return errors.New(i18n.T("Writing the backup timed out"))
	}
}

// DirtyNamed 报告有没有未保存修改的有路径的文件（未命名的文档另算：见 KeepForRestart）。
func (e *Editor) DirtyNamed() bool {
	return slices.ContainsFunc(e.docs, func(d *document) bool { return d.buf.Dirty() && !d.untitled() })
}

// Unsaved 是关窗口、退出前要问的：先试着把未命名文档留到下次，留住了就只问有路径的文件。
// kept 为 nil 时未命名文档已经留住；否则它说明为什么没留住，未命名文档也算未保存。
func (e *Editor) Unsaved() (dirty bool, kept error) {
	kept = e.KeepForRestart()
	if kept != nil {
		return e.Dirty(), kept
	}
	return e.DirtyNamed(), nil
}

// restoreFolderDrafts 找回这个文件夹上次留下的未命名文档（打开文件夹之后）。另一个 yan 开着
// 这个文件夹时留给它。
func (e *Editor) restoreFolderDrafts() {
	if e.ws == nil || !recovery.Exists(recoveryDir(), recovery.FolderID(e.ws.root)) {
		return
	}
	id, err := e.recoveryOwner()
	if err != nil {
		e.status = i18n.F("Couldn't open the backup of untitled files: %s", err.Error())
		return
	}
	if id == recovery.FolderID(e.ws.root) {
		e.restoreDrafts(id)
	}
}

// restoreDrafts 打开备份 id 里的未命名文档：按原来的名字（和开着的重名时换一个）、列和光标。
// 它们还在备份里，直到保存或放弃：再崩溃一次也丢不了。
func (e *Editor) restoreDrafts(id string) {
	cp, err := recovery.Read(recoveryDir(), id)
	if err != nil {
		e.status = i18n.F("Couldn't read the backup of untitled files; moved it aside: %s", err.Error())
		return
	}
	if len(cp.Drafts) == 0 {
		return
	}
	e.rec.touched = true
	e.rec.gen = max(e.rec.gen, cp.Gen)
	var extra *column // 原来的列没回来（只放着未命名文档的列不记进会话）：它们一起放进新开的一列
	for _, dr := range cp.Drafts {
		title := dr.Title
		if title == "" || slices.ContainsFunc(e.docs, func(d *document) bool { return d.untitled() && d.title == title }) {
			n := 1
			for slices.ContainsFunc(e.docs, func(d *document) bool { return d.untitled() && d.title == untitledTitle(n) }) {
				n++
			}
			title = untitledTitle(n)
		}
		d := &document{buf: edit.NewUnsaved(text.FromString(dr.Text, text.Meta{EOL: text.EOL(dr.EOL)})), title: title}
		e.adopt(d)
		var c *column
		switch {
		case dr.Column > 0 && dr.Column >= len(e.cols) && extra == nil:
			extra = newColumn()
			extra.size = e.cols[0].size
			e.cols = append(e.cols, extra)
			c = extra
		case dr.Column > 0 && dr.Column >= len(e.cols)-1 && extra != nil:
			c = extra
		default:
			c = e.cols[min(max(dr.Column, 0), len(e.cols)-1)]
		}
		e.addTab(c, len(c.tabs), d, nil)
		n := d.buf.Snapshot().Len()
		t := c.tabs[c.active]
		t.sels = edit.Single(edit.Range{Anchor: min(max(dr.Anchor, 0), n), Head: min(max(dr.Head, 0), n), GoalX: -1})
		if e.loaded == t {
			d.buf.SetSelections(t.sels)
		}
	}
	e.status = i18n.F("Recovered %d untitled files", len(cp.Drafts))
}

// RootlessDrafts 是没开文件夹的编辑器留下的备份编号（窗口版启动时各开一个项目找回）。
// problems 是读不出来、已经移到一边的备份。
func RootlessDrafts() (ids []string, problems []error) {
	cps, problems := recovery.Load(recoveryDir())
	for _, cp := range cps {
		if recovery.Rootless(cp.ID) && len(cp.Drafts) > 0 {
			ids = append(ids, cp.ID)
		}
	}
	slices.Sort(ids)
	return ids, problems
}

// AdoptDrafts 让这个没开文件夹的编辑器接着用备份 id，并打开里面的未命名文档。另一个 yan
// 进程拿着它时不动。
func (e *Editor) AdoptDrafts(id string) {
	if e.ws != nil || !recovery.Rootless(id) {
		return
	}
	l, err := recovery.Acquire(recoveryDir(), id)
	if err != nil {
		return
	}
	e.rec.lock.Release()
	e.rec.lock, e.rec.owned, e.rec.ownedFor, e.rec.rootless = l, id, "", id
	e.restoreDrafts(id)
}
