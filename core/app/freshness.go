package app

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/search/glob"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/workspace"
)

// Disk freshness follows Mayoi (MIT, cmd/mayoi/app.go diskChanged and search.go searchAgain at
// 46990a1): a watcher reports changed files; clean documents reload, dirty ones keep their text
// and only warn, and search results for those files are found again. Every file read runs off
// the owner goroutine. A result is dropped when the folder changed (ws identity) or the document
// moved on (snapshot version or disk stamp) after the read began.

// watchFolder starts watching w's tree. Without file events the focus check still applies.
func (e *Editor) watchFolder(w *ws) {
	post := e.postHere()
	wt, err := workspace.NewWatcher(func(ch workspace.Changes) {
		post(func(e *Editor) {
			if e.ws == w {
				e.diskEvents(ch)
			}
		})
	})
	if err != nil {
		return
	}
	w.watcher = wt
	e.watchTree()
}

// watchTree watches the directories the explorer has read and those of open documents.
func (e *Editor) watchTree() {
	w := e.ws
	if w == nil || w.watcher == nil {
		return
	}
	for _, rel := range w.tree.LoadedDirs() {
		w.watcher.Add(filepath.Join(w.root, filepath.FromSlash(rel)))
	}
	for _, d := range e.docs {
		if !d.untitled() {
			w.watcher.Add(filepath.Dir(d.buf.Path()))
		}
	}
}

// close stops the background work tied to the open folder.
func (w *ws) close() {
	if w.cancel != nil {
		w.cancel()
	}
	if w.indexCancel != nil {
		w.indexCancel()
	}
	w.watcher.Close()
}

// diskEvents takes one batch from the watcher on the owner goroutine.
func (e *Editor) diskEvents(ch workspace.Changes) {
	w := e.ws
	treeChanged := false
	for _, dir := range ch.Dirs {
		if rel := e.rel(dir); !filepath.IsAbs(rel) || dir == w.root {
			if dir == w.root {
				rel = ""
			}
			treeChanged = w.tree.RefreshDir(rel) || treeChanged
		}
	}
	if treeChanged {
		e.refreshVisible()
		e.watchTree()
	}
	paths := append(slices.Clone(ch.Files), ch.Gone...)
	for _, p := range paths {
		if b := filepath.Base(p); b == ".git" || b == ".gitmodules" { // 仓库来了、走了：重新找仓库
			e.rediscoverRepositories()
			break
		}
	}
	ignoreChanged := false
	for _, p := range paths {
		if filepath.Base(p) != ".gitignore" {
			continue
		}
		if rel := e.rel(filepath.Dir(p)); filepath.Dir(p) == w.root || !filepath.IsAbs(rel) {
			if filepath.Dir(p) == w.root {
				rel = ""
			}
			// Commented out or added, a rule changes what the explorer dims, what quick open
			// lists and what Find in Files searches.
			w.ig.Reload(rel)
			w.tree.RefreshDir(rel)
			treeChanged, ignoreChanged = true, true
		}
	}
	if treeChanged && ignoreChanged {
		e.refreshVisible()
		e.watchTree()
	}
	if len(ch.Dirs) > 0 || ignoreChanged {
		w.searchAfterIndex = w.searchAfterIndex || ignoreChanged
		e.refreshIndex()
	}
	var docs []*document
	for _, p := range paths {
		if d := e.docOf(p); d != nil && !slices.Contains(docs, d) {
			docs = append(docs, d)
		}
	}
	e.checkDocs(docs)
	var rels []string
	for _, p := range paths {
		if rel := e.rel(p); !filepath.IsAbs(rel) {
			rels = append(rels, rel)
		}
	}
	e.searchAgain(rels)
	e.gitChanged()
	if project := filepath.Join(w.root, ".yan.toml"); slices.Contains(paths, project) {
		e.reloadConfig() // edited outside the editor; the user's own files come through watchConfig
	}
}

// diskRead is what a background read found of one open document's file.
type diskRead struct {
	doc     *document
	version uint64     // the buffer's snapshot when the read began
	stamp   edit.Stamp // its disk baseline then
	state   edit.DiskState
	snap    text.Snapshot // the file's content, when it changed
}

// checkDocs compares open documents with their files in the background, then reloads the
// clean ones and warns about the dirty ones.
func (e *Editor) checkDocs(docs []*document) {
	if len(docs) == 0 {
		return
	}
	type job struct {
		read diskRead
		base text.Snapshot
		path string
	}
	jobs := make([]job, 0, len(docs))
	for _, d := range docs {
		if d.untitled() {
			continue // 没有文件可比
		}
		jobs = append(jobs, job{
			read: diskRead{doc: d, version: d.buf.Snapshot().Version(), stamp: d.buf.DiskStamp()},
			base: d.buf.DiskContent(), path: d.buf.Path(),
		})
	}
	post := e.postHere()
	w := e.ws
	go func() {
		reads := make([]diskRead, 0, len(jobs))
		for _, j := range jobs {
			r := j.read
			r.state = edit.DiskStateOf(j.path, j.base, j.read.stamp)
			if r.state == edit.DiskChanged {
				s, err := text.Load(j.path)
				if err != nil {
					continue
				}
				r.snap = s
			}
			if r.state != edit.DiskSame {
				reads = append(reads, r)
			}
		}
		if len(reads) > 0 {
			post(func(e *Editor) {
				if e.ws == w {
					e.takeDiskReads(reads)
				}
			})
		}
	}()
}

func (e *Editor) takeDiskReads(reads []diskRead) {
	var again []*document
	for _, r := range reads {
		d := r.doc
		if !slices.Contains(e.docs, d) {
			continue // closed while the read ran
		}
		if d.buf.DiskStamp() != r.stamp || d.buf.Snapshot().Version() != r.version {
			if !d.buf.Dirty() {
				again = append(again, d) // edited and undone, or saved: look once more
			}
			continue
		}
		switch {
		case d.buf.Dirty() && r.state != edit.DiskSame:
			e.status = e.conflictError(d).Error() // the unsaved text stays; Save refuses too
		case r.state == edit.DiskDeleted:
			e.status = filepath.Base(d.buf.Path()) + " 已从磁盘上删除：标签里的内容还在，保存会重新创建它"
		case r.state == edit.DiskChanged:
			e.reloading = true
			d.buf.Reload(r.snap)
			e.reloading = false
		}
	}
	e.checkDocs(again)
}

// refreshIndex walks the folder again in the background and swaps the new index in once
// complete, so quick open never sees a half-built list. One walk runs at a time.
func (e *Editor) refreshIndex() {
	w := e.ws
	if w == nil {
		return
	}
	if w.indexing {
		w.indexAgain = true
		return
	}
	if w.indexCancel != nil {
		w.indexCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.indexCancel, w.indexing = cancel, true
	root, ig := w.root, w.ig
	post := e.postHere()
	go func() {
		var all []string
		err := workspace.Walk(ctx, root, ig, func(batch []string) { all = append(all, batch...) })
		post(func(e *Editor) {
			if e.ws != w || ctx.Err() != nil {
				return
			}
			w.indexing = false
			if err == nil {
				w.index = all
				e.overlayIndexGrew()
				if w.searchAfterIndex && e.fs.query != "" {
					e.fs.key = "" // the files searched changed with the ignore rules
					e.searchSoon(0)
				}
				w.searchAfterIndex = false
			}
			if w.indexAgain {
				w.indexAgain = false
				e.refreshIndex()
			}
		})
	}()
}

// searchAgain finds the query again in the files at rels, replacing or removing their results.
// Each file's latest request wins, so a slow read of older text never lands after a newer one.
func (e *Editor) searchAgain(rels []string) {
	s := &e.fs
	if s.re == nil || e.ws == nil || len(rels) == 0 {
		return
	}
	inc, exc := glob.ParseGlobs(s.include), glob.ParseGlobs(s.exclude)
	texts := map[string]searchText{}
	seqs := map[string]int{}
	if s.again == nil {
		s.again = map[string]int{}
	}
	var want []string
	for _, rel := range rels {
		if slices.Contains(want, rel) {
			continue
		}
		if (len(inc) > 0 && !inc.Match(rel)) || exc.Match(rel) || e.ws.ig.Match(rel, false) || hiddenRel(rel) {
			continue
		}
		want = append(want, rel)
		s.againSeq++
		s.again[rel], seqs[rel] = s.againSeq, s.againSeq
		if d := e.docOf(filepath.Join(e.root, filepath.FromSlash(rel))); d != nil && d.buf.Dirty() {
			snap := d.buf.Snapshot()
			texts[rel] = searchText{snap.String(0, snap.Len()), d, snap.Version()}
		}
	}
	if len(want) == 0 {
		return
	}
	root, re, gen := e.root, s.re, s.gen
	post := e.postHere()
	go func() {
		found := make(map[string]*searchFile, len(want))
		for _, rel := range want {
			found[rel] = searchFileText(root, rel, texts, re, maxSearchMatches)
		}
		post(func(e *Editor) { e.takeSearchAgain(gen, seqs, found) })
	}()
}

func (e *Editor) takeSearchAgain(gen int, seqs map[string]int, found map[string]*searchFile) {
	s := &e.fs
	if gen != s.gen {
		return
	}
	for rel, f := range found {
		if s.again[rel] != seqs[rel] {
			continue // a newer search of this file is on its way
		}
		delete(s.again, rel)
		i, ok := slices.BinarySearchFunc(s.files, rel, func(x *searchFile, rel string) int { return strings.Compare(x.rel, rel) })
		if ok {
			s.count -= len(s.files[i].matches)
			if f == nil {
				s.files = slices.Delete(s.files, i, i+1)
				continue
			}
			s.files[i] = f
		} else if f != nil {
			s.files = slices.Insert(s.files, i, f)
		} else {
			continue
		}
		if room := max(maxSearchMatches-s.count, 0); len(f.matches) > room {
			f.matches, s.stopped = f.matches[:room], true
		}
		s.count += len(f.matches)
	}
	if rows := len(e.searchRows()); s.sel >= rows {
		s.sel = max(rows-1, 0)
	}
}

// searchEditSoon finds the query again in an edited document once typing pauses.
func (e *Editor) searchEditSoon(d *document) {
	s := &e.fs
	if s.re == nil || e.ws == nil {
		return
	}
	rel := e.rel(d.buf.Path())
	if filepath.IsAbs(rel) {
		return
	}
	if s.edited == nil {
		s.edited = map[string]bool{}
	}
	s.edited[rel] = true
	if s.editTimer != nil {
		return
	}
	post := e.postHere()
	s.editTimer = time.AfterFunc(searchPause, func() {
		post(func(e *Editor) {
			s := &e.fs
			rels := make([]string, 0, len(s.edited))
			for rel := range s.edited {
				rels = append(rels, rel)
			}
			slices.Sort(rels)
			s.edited, s.editTimer = nil, nil
			e.searchAgain(rels)
		})
	})
}

func hiddenRel(rel string) bool {
	for _, p := range strings.Split(rel, "/") {
		if workspace.Hidden(p) {
			return true
		}
	}
	return false
}
