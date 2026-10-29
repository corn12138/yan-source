package app

import (
	"context"
	"fmt"
	"github.com/corn12138/yan-source/internal/process"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/git/linediff"
	"github.com/corn12138/yan-source/core/view"
)

// Git 的做法照着 Mayoi（MIT，cmd/mayoi/git.go、internal/gitmarks）：一个后台 goroutine 每 1.5 秒看一眼
// 索引和 HEAD 的修改时间，变了（或者满 5 秒）就重读 git status；文件树标出改动的字母，状态栏显示分支，
// 行号旁标出相对 HEAD 新增、修改、删除的行；光标在一行停 350 毫秒后显示这一行最后是谁、什么时候改的。

const (
	gitCheckEvery  = 1500 * time.Millisecond
	gitStatusEvery = 5 * time.Second
	// repoDiscoverEvery 是不看文件变化时多久重新找一次仓库（嵌套仓库、子模块）：只是兜底。
	repoDiscoverEvery = 2 * time.Minute
	blameDelay        = 350 * time.Millisecond
	diffPause         = 150 * time.Millisecond
)

// gitState 是窗口知道的 git 工作区状态。
type gitState struct {
	nested      []string
	nestedError string
	ready       bool      // Repository discovery finished, including a non-repository folder.
	repo        *git.Repo // 不在工作区里时为 nil
	status      *git.Status
	entries     map[string]git.Entry // 有改动的文件（绝对路径）
	dirs        map[string]byte      // 下面有改动的目录 → 最要紧的那个字母
	kick        chan struct{}        // 请后台马上重读状态（保存之后、git 命令之后、窗口拿回焦点时）
	cancel      context.CancelFunc
	blame       blameState
	busy        string // 正在后台跑的 git 命令，如「推送中…」
	// showMergeFiles 是一个命令遇到冲突停下来了：读到它的状态时列出冲突文件（mergefiles.go）。
	showMergeFiles bool
}

type blameState struct {
	path    string
	line    int
	version uint64
	text    string // 显示在行尾的说明；"" 表示没有
	timer   *time.Timer
}

// Close 停下编辑器的后台工作（关窗口时：应用里还有别的窗口，进程不退出）。之后不要再用它。
func (e *Editor) Close() {
	e.rec.closed = true
	if e.rec.w != nil {
		e.rec.w.close()
		e.rec.w = nil
	}
	e.rec.lock.Release()
	e.rec.lock, e.rec.owned = nil, ""
	if e.ws != nil {
		e.ws.close()
	}
	e.cfgWatcher.Close()
	e.closeRepositories()
}

func (e *Editor) startGit(root string) { e.startRepositories(root) }

// watchGit 在后台运行：HEAD、索引一变，或每隔 5 秒，读一次状态；和上次一样就不打扰前端。
func watchGit(ctx context.Context, e *Editor, rc *repositoryContext, r *git.Repo, kick chan struct{}) {
	t := time.NewTicker(gitCheckEvery)
	defer t.Stop()
	var meta, read time.Time
	var last *git.Status
	for first := true; ; first = false {
		moved := false
		if !first {
			select {
			case <-ctx.Done():
				return
			case <-kick:
				moved = true
			case <-t.C:
			}
		}
		if m := gitMeta(r); !m.Equal(meta) {
			moved = moved || !meta.IsZero()
			meta = m
		}
		if !first && !moved && time.Since(read) < gitStatusEvery {
			continue
		}
		st, err := r.Status()
		read = time.Now()
		if err != nil || st.Equal(last) && !moved {
			continue
		}
		last = st
		e.Post(func(e *Editor) {
			if rc.dead || ctx.Err() != nil {
				return // 已经换了文件夹
			}
			e.inRepository(rc, func() { e.setStatus(st) })
			if moved {
				for _, d := range e.docs {
					if e.repositoryForPath(d.buf.Path()) == rc {
						d.marks.headMoved()
					}
				}
			}
		})
	}
}

// gitMeta 返回 HEAD 移动时 git 会改的那些文件（索引、HEAD、它的日志、fetch 写的）最后的修改时间。
func gitMeta(r *git.Repo) time.Time {
	var t time.Time
	for _, p := range []string{
		filepath.Join(r.Dir, "index"), filepath.Join(r.Dir, "HEAD"), filepath.Join(r.Dir, "logs", "HEAD"),
		filepath.Join(r.Common, "FETCH_HEAD"),
	} {
		if info, err := os.Stat(p); err == nil && info.ModTime().After(t) {
			t = info.ModTime()
		}
	}
	return t
}

// gitChanged 请后台马上重读状态。
func (e *Editor) gitChanged() {
	for _, rc := range e.repositoryContexts() {
		select {
		case rc.git.kick <- struct{}{}:
		default:
		}
	}
	select {
	case e.repositories.kick <- struct{}{}:
	default:
	}
}

// setStatus 收下读到的状态，并算出每个目录下面最要紧的改动。
func (e *Editor) setStatus(st *git.Status) {
	g := e.repository().git
	g.status = st
	g.entries = make(map[string]git.Entry, len(st.Entries))
	g.dirs = map[string]byte{}
	if g.repo == nil {
		return
	}
	if st.Op == "" {
		g.showMergeFiles = false
		os.Remove(filepath.Join(g.repo.Dir, mergeRecordName)) // 命令做完了：解决过的文件不用再记
	} else if g.showMergeFiles {
		g.showMergeFiles = false
		defer func() {
			if e.ov == nil && slices.ContainsFunc(st.Entries, func(en git.Entry) bool { return en.Conflicted() }) {
				e.showMergeFiles()
			}
		}()
	}
	e.refreshHistory()
	for _, en := range st.Entries {
		g.entries[en.Path] = en
		l := en.Letter()
		for d := filepath.Dir(en.Path); len(d) >= len(g.repo.Root); d = filepath.Dir(d) {
			if old, ok := g.dirs[d]; ok && letterRank(old) >= letterRank(l) {
				break
			}
			g.dirs[d] = l
			if d == filepath.Dir(d) {
				break
			}
		}
	}
}

// letterRank 排目录要显示哪个字母：冲突比修改要紧，修改比新增要紧。
func letterRank(l byte) int {
	switch l {
	case 'C':
		return 4
	case 'M', 'R', 'T', 'D':
		return 3
	case 'A':
		return 2
	case 'U':
		return 1
	}
	return 0
}

// branch 是检出的分支（HEAD 游离时是提交号），不在工作区里时是 ""。
func (e *Editor) branch() string {
	st := e.repository().git.status
	switch {
	case st == nil:
		return ""
	case st.Branch != "":
		return st.Branch
	case st.Head != "":
		return git.ShortHash(st.Head)
	}
	return ""
}

// gitLetter 返回文件（或目录）在 git 里的字母：M 修改、A 新增、D 删除、U 未跟踪、C 冲突；目录是下面最要紧的那个。
func (e *Editor) gitLetter(abs string, dir bool) byte {
	if !dir {
		if rc := e.repositoryForPath(abs); rc != nil {
			if en, ok := rc.git.entries[abs]; ok {
				return en.Letter()
			}
		}
		return 0
	}
	var best byte
	for _, rc := range e.repositoryContexts() {
		if l := rc.git.dirs[abs]; letterRank(l) > letterRank(best) {
			best = l
		}
		if rc.git.repo != nil {
			rel, err := filepath.Rel(abs, rc.git.repo.Root)
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
				if l := rc.git.dirs[rc.git.repo.Root]; letterRank(l) > letterRank(best) {
					best = l
				}
			}
		}
	}
	return best
}

// ---- 行号旁的改动标记 ----

// gitMarks 是文档相对 HEAD 改了哪些行：HEAD 里的文本在后台读，比较也在后台做。
type gitMarks struct {
	lines      []linediff.Kind // 每行一个
	hunks      []linediff.Hunk
	version    uint64 // lines 是哪个版本的
	generation uint64 // HEAD 变更也会重算同一文本版本的标记
	base       []string
	loaded     bool
	loading    bool
	stale      bool // HEAD 可能动了：重读
	diffed     bool
	running    bool
	started    time.Time
	waiting    bool
}

func (g *gitMarks) headMoved() { g.stale = true }

// refreshMarks 需要时重读 HEAD 里的文本，文字变了就重新比较（两次比较之间至少隔 150 毫秒）。
func (e *Editor) refreshMarks(d *document) {
	if d.untitled() {
		return
	}
	defer e.bindRepository(e.repositoryForPath(d.buf.Path()))()
	g := &d.marks
	path := d.buf.Path()
	if e.repository().git.repo == nil || !e.repository().git.repo.Contains(path) {
		return
	}
	if (!g.loaded || g.stale) && !g.loading {
		g.loading, g.stale = true, false
		dir, name := filepath.Dir(path), filepath.Base(path)
		post := e.postGit()
		go func() {
			out, err := process.GitCommand("-C", dir, "show", "HEAD:./"+name).Output()
			post(func(*Editor) {
				g.loading, g.loaded, g.diffed = false, true, false
				g.base = nil
				if err == nil {
					g.base = strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
				} else {
					g.lines, g.hunks = nil, nil
					g.generation++
				}
			})
		}()
	}
	snap := d.buf.Snapshot()
	if !g.loaded || g.base == nil || g.running || g.waiting || g.diffed && g.version == snap.Version() {
		return
	}
	if wait := diffPause - time.Since(g.started); g.diffed && wait > 0 {
		g.waiting = true
		time.AfterFunc(wait, func() { e.Post(func(*Editor) { g.waiting = false }) })
		return
	}
	g.running, g.started = true, time.Now()
	base := g.base
	go func() {
		lines := strings.Split(strings.ReplaceAll(snap.String(0, snap.Len()), "\r\n", "\n"), "\n")
		hunks := linediff.Hunks(base, lines)
		marks := linediff.Marks(hunks, len(lines))
		e.Post(func(*Editor) {
			g.running = false
			if g.loaded && len(g.base) == len(base) && (len(base) == 0 || &g.base[0] == &base[0]) {
				g.lines, g.hunks, g.version, g.diffed = marks, hunks, snap.Version(), true
				g.generation++
			}
		})
	}()
}

// mark 返回第 line 行的改动标记。
func (g *gitMarks) mark(line int) view.GitMark {
	if line < 0 || line >= len(g.lines) {
		return view.MarkNone
	}
	switch g.lines[line] {
	case linediff.Added:
		return view.MarkAdded
	case linediff.Modified:
		return view.MarkModified
	case linediff.Deleted:
		return view.MarkDeleted
	case linediff.DeletedBelow:
		return view.MarkDeletedBelow
	}
	return view.MarkNone
}

// stepChange 走到下一个（或上一个）改动的开头，到头绕回（⌥F5、⇧⌥F5）。
func (e *Editor) stepChange(by int) error {
	t := e.cur()
	if t == nil {
		return nil
	}
	if t.diff != nil {
		e.diffStepChange(t, by)
		return nil
	}
	g := &t.doc.marks
	if len(g.hunks) == 0 {
		e.status = "没有改动"
		return nil
	}
	s := t.doc.buf.Snapshot()
	line := s.LineAt(t.doc.buf.Selections().Main().Head)
	at := -1
	if by > 0 {
		for i, h := range g.hunks {
			if h.B > line {
				at = i
				break
			}
		}
		if at < 0 {
			at = 0
		}
	} else {
		for i := len(g.hunks) - 1; i >= 0; i-- {
			if g.hunks[i].B+max(g.hunks[i].BLen, 1)-1 < line {
				at = i
				break
			}
		}
		if at < 0 {
			at = len(g.hunks) - 1
		}
	}
	target := min(g.hunks[at].B, s.LineCount()-1)
	e.GotoLine(target + 1)
	return nil
}

// ---- 行尾的 blame ----

// scheduleBlame 在光标停在一行 350 毫秒后，在后台问 git 这一行最后是谁改的。
func (e *Editor) scheduleBlame() {
	if t := e.cur(); t != nil {
		defer e.bindRepository(e.repositoryForPath(t.doc.buf.Path()))()
	}
	t := e.cur()
	b := &e.repository().git.blame
	if t == nil || e.repository().git.repo == nil || !e.cfg.Blame || !e.repository().git.repo.Contains(t.doc.buf.Path()) {
		b.text = ""
		return
	}
	s := t.doc.buf.Snapshot()
	line := s.LineAt(t.doc.buf.Selections().Main().Head)
	path := t.doc.buf.Path()
	if b.path == path && b.line == line && b.version == s.Version() {
		return
	}
	b.path, b.line, b.version, b.text = path, line, s.Version(), ""
	if b.timer != nil {
		b.timer.Stop()
	}
	r := e.repository().git.repo
	post := e.postGit()
	b.timer = time.AfterFunc(blameDelay, func() {
		post(func(e *Editor) {
			if b.path != path || b.line != line || b.version != s.Version() {
				return // 光标已经走了
			}
			go func() {
				bl, err := r.BlameLine(path, line, s.String(0, s.Len()))
				post(func(e *Editor) {
					if err != nil || b.path != path || b.line != line || b.version != s.Version() {
						return
					}
					b.text = blameText(bl, time.Now())
				})
			}()
		})
	})
}

// blameText 写出行尾的说明：「你，3 天前 • 提交摘要」；还没提交的行写「还没提交的修改」。
func blameText(b git.Blame, now time.Time) string {
	if b.Commit == "" {
		return "还没提交的修改"
	}
	return fmt.Sprintf("%s，%s • %s", b.Author, relTime(b.Time, now), b.Summary)
}

// relTime 把时间写成「刚才」「5 分钟前」「3 天前」这样。
func relTime(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "刚才"
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟前", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时前", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d 天前", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%d 个月前", int(d.Hours()/24/30))
	}
	return fmt.Sprintf("%d 年前", int(d.Hours()/24/365))
}

// statusBranch 写出状态栏上的分支：领先上游几个提交（↑），落后几个（↓）。
func (e *Editor) statusBranch() string {
	b := e.branch()
	if b == "" {
		return ""
	}
	st := e.repository().git.status
	if st.Ahead > 0 {
		b += fmt.Sprintf(" ↑%d", st.Ahead)
	}
	if st.Behind > 0 {
		b += fmt.Sprintf(" ↓%d", st.Behind)
	}
	return b
}

func (e *Editor) blameFor(path string) blameState {
	if rc := e.repositoryForPath(path); rc != nil {
		return rc.git.blame
	}
	return blameState{}
}

// rediscoverRepositories asks the background to look for the folder's repositories again.
func (e *Editor) rediscoverRepositories() {
	select {
	case e.repositories.kick <- struct{}{}:
	default:
	}
}
