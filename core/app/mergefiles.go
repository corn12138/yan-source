package app

// The files of a stopped merge, rebase, cherry-pick or revert, after the behaviour of Mayoi
// PR #50 (Garfielk, https://github.com/sorrycc/Mayoi/pull/50, unmerged): the files still in
// conflict and the ones a merge view marked resolved, worked file by file. The resolved ones
// are recorded in the repository's Git folder, so they stand in the list after a restart, and
// the record goes when the command is done. Rewritten for yan.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/corn12138/yan-source/core/git"
)

// mergeRecordName is the record of the resolved files, in the work tree's Git folder.
const mergeRecordName = "yan-merge-resolved.json"

// mergeRecord is what the record says: the command and the commit it brings in, and the files
// resolved (relative to the work tree).
type mergeRecord struct {
	Op    git.Op   `json:"op"`
	Head  string   `json:"head"`
	Files []string `json:"files"`
}

// mergeFile is a file of the stopped command: in conflict still, or resolved in a merge view.
type mergeFile struct {
	path     string
	resolved bool
}

// opHead is the commit the stopped command brings in, which names its record.
func opHead(r *git.Repo, op git.Op) string {
	name := map[git.Op]string{git.OpMerge: "MERGE_HEAD", git.OpCherryPick: "CHERRY_PICK_HEAD", git.OpRevert: "REVERT_HEAD", git.OpRebase: "REBASE_HEAD"}[op]
	b, err := os.ReadFile(filepath.Join(r.Dir, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// mergeRecordOf reads the record of the repository's stopped command; a record of another
// command (or none stopped) is removed and reads as empty.
func (e *Editor) mergeRecordOf() (*git.Repo, mergeRecord) {
	g := e.repository().git
	r := g.repo
	if r == nil || g.status == nil {
		return nil, mergeRecord{}
	}
	path := filepath.Join(r.Dir, mergeRecordName)
	op := g.status.Op
	var rec mergeRecord
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, &rec)
	}
	if op == "" || rec.Op != op || rec.Head != opHead(r, op) {
		if op == "" || rec.Op != "" {
			os.Remove(path)
		}
		return r, mergeRecord{Op: op, Head: opHead(r, op)}
	}
	return r, rec
}

func (e *Editor) writeMergeRecord(r *git.Repo, rec mergeRecord) {
	path := filepath.Join(r.Dir, mergeRecordName)
	if len(rec.Files) == 0 {
		os.Remove(path)
		return
	}
	if b, err := json.Marshal(rec); err == nil {
		tmp := path + ".tmp"
		if os.WriteFile(tmp, b, 0o644) == nil {
			os.Rename(tmp, path)
		}
	}
}

// mergeResolvedDone records path as resolved by a merge view.
func (e *Editor) mergeResolvedDone(path string) {
	defer e.bindRepository(e.repositoryForPath(path))()
	r, rec := e.mergeRecordOf()
	if r == nil || rec.Op == "" {
		return
	}
	if rel := r.Rel(path); !slices.Contains(rec.Files, rel) {
		rec.Files = append(rec.Files, rel)
		slices.Sort(rec.Files)
	}
	e.writeMergeRecord(r, rec)
}

// mergeResolvedUndone takes path out of the record, back in conflict.
func (e *Editor) mergeResolvedUndone(path string) {
	defer e.bindRepository(e.repositoryForPath(path))()
	r, rec := e.mergeRecordOf()
	if r == nil {
		return
	}
	rec.Files = slices.DeleteFunc(rec.Files, func(f string) bool { return f == r.Rel(path) })
	e.writeMergeRecord(r, rec)
}

// mergeFiles are the stopped command's files: those in conflict, then those resolved, each
// by path.
func (e *Editor) mergeFiles() []mergeFile {
	r, rec := e.mergeRecordOf()
	if r == nil || rec.Op == "" {
		return nil
	}
	var out []mergeFile
	seen := map[string]bool{}
	for _, en := range e.repository().git.status.Entries {
		if en.Conflicted() {
			out = append(out, mergeFile{path: en.Path})
			seen[en.Path] = true
		}
	}
	for _, f := range rec.Files {
		if p := filepath.Join(r.Root, filepath.FromSlash(f)); !seen[p] {
			out = append(out, mergeFile{path: p, resolved: true})
		}
	}
	return out
}

// showMergeFiles lists the stopped command's files: one in conflict opens in its merge view,
// a resolved one opens, or goes back in conflict.
func (e *Editor) showMergeFiles() error {
	files := e.mergeFiles()
	if len(files) == 0 {
		return errors.New("没有停下来的合并、变基、拣选或还原，或者它没有冲突文件")
	}
	r := e.repository().git.repo
	left := 0
	var items []ovItem
	for _, f := range files {
		rel := r.Rel(f.path)
		if !f.resolved {
			left++
			items = append(items, ovItem{label: "● " + rel, detail: "冲突", run: func(e *Editor) error { return e.openMerge(f.path) }})
			continue
		}
		items = append(items, ovItem{label: "✓ " + rel, detail: "已解决", run: func(e *Editor) error {
			e.choose(rel+" 已解决",
				ovItem{label: "打开文件", run: func(e *Editor) error { return e.openPath(f.path) }},
				ovItem{label: "恢复成冲突…", run: func(e *Editor) error { return e.restoreResolved(f.path) }})
			return nil
		}})
	}
	title := "冲突文件：还有 " + strconv.Itoa(left) + " 个没解决"
	if left == 0 {
		title = "冲突文件都解决了：可以「继续」"
	}
	e.choose(title, items...)
	return nil
}

// restoreResolved puts a file resolved before back in conflict, after asking.
func (e *Editor) restoreResolved(path string) error {
	e.choose("把 "+filepath.Base(path)+" 恢复成冲突？解决的结果会丢掉",
		ovItem{label: "恢复冲突", run: func(e *Editor) error {
			defer e.bindRepository(e.repositoryForPath(path))()
			return e.gitDo("恢复冲突中…", func(r *git.Repo) error { return r.RestoreConflict(path) }, func(e *Editor) {
				e.mergeResolvedUndone(path)
				e.reloadDocs(path)
				e.status = filepath.Base(path) + " 又是冲突了"
			})
		}},
		ovItem{label: "取消", run: func(*Editor) error { return nil }})
	return nil
}

// mergeApplyAndNext marks the merge view's file resolved and opens the next file in conflict.
func (e *Editor) mergeApplyAndNext() error {
	mv := mergeOf(e.cur())
	if mv == nil {
		return errors.New("先打开一个冲突文件的三方合并")
	}
	e.mergeSync(mv)
	if n := mv.unresolved(); n > 0 {
		return errors.New("还有冲突没处理：处理完再应用，或者用「标记为已解决」")
	}
	if hasConflictMarkers(mv.result.doc.buf.Snapshot()) {
		return errors.New("结果里还有冲突标记（<<<<<<<）：先处理它们")
	}
	defer e.bindRepository(e.repositoryForPath(mv.path))()
	if err := e.saveDoc(mv.result.doc); err != nil {
		return err
	}
	path := mv.path
	return e.gitDo("暂存中…", func(r *git.Repo) error { return r.Stage(path) }, func(e *Editor) {
		e.mergeResolvedDone(path)
		e.mergeDetach(mv)
		for _, f := range e.mergeFiles() {
			if !f.resolved && f.path != path {
				if err := e.openMerge(f.path); err != nil {
					e.status = err.Error()
				}
				return
			}
		}
		e.status = filepath.Base(path) + " 已解决：冲突文件都解决了，可以「继续」"
	})
}
