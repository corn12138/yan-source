package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/workspace"
)

// 外观、自动保存、会话，照着 Mayoi：菜单里换主题、外观、字号立即生效并记在 state.json 里；
// 自动保存在停止编辑 1 秒后、切换标签、窗口失去焦点时保存；关掉文件夹时记下标签，
// 下次打开同一个文件夹（没指定文件时）原样恢复；保存配置文件时配置立即生效。

const autoSaveDelay = time.Second

// SetState 交给编辑器状态文件（nil 表示不记：测试里）。
func (e *Editor) SetState(st *config.State) {
	e.state = st
	if st != nil {
		e.allBranches = st.AllBranches
		e.repoPanel, e.repoRemotes = st.RepositoryPanel, st.RepositoryRemotes
		e.mergeManual = st.MergeManual
		e.commitFilesOpen = st.CommitFilesOpen
		e.commitFilesWidth = max(140, min(st.CommitFilesWidth, 600))
		if st.CommitFilesWidth == 0 {
			e.commitFilesWidth = 220
		}
		e.scmFolded = map[string]bool{}
		for _, k := range st.SCMFolded {
			e.scmFolded[k] = true
		}
	}
}

// Settings 返回前端关心的外观设置；gen 在它们变化时递增。
func (e *Editor) Settings() (theme, appearance string, fontSize float64, gen uint64) {
	if e.ThemePreviewing() {
		return e.ov.preview.theme, e.ov.preview.appearance, e.cfg.FontSize, e.settingsGen
	}
	return e.cfg.Theme, e.cfg.Appearance, e.cfg.FontSize, e.settingsGen
}

// Font returns the code font family list and line height the configuration sets.
func (e *Editor) Font() (family string, lineHeight float64) {
	return e.cfg.FontFamily, e.cfg.LineHeight
}

// PreviewMermaid reports whether Markdown previews draw mermaid code blocks as diagrams.
func (e *Editor) PreviewMermaid() bool { return e.cfg.PreviewMermaid }

// ThemeEffects reports whether the GUI theme decorations are enabled.
func (e *Editor) ThemeEffects() bool { return e.cfg.ThemeEffects }

func (e *Editor) toggleThemeEffects() error {
	e.cfg.ThemeEffects = !e.cfg.ThemeEffects
	if e.state != nil {
		v := e.cfg.ThemeEffects
		e.state.ThemeEffects = &v
		e.saveState()
	}
	e.settingsGen++
	return nil
}

// BackgroundActivity includes searches even when their sidebar is hidden.
func (e *Editor) BackgroundActivity() string {
	if e.repository().git.busy != "" {
		return e.repository().git.busy
	}
	if e.fs.running {
		return "正在搜索…"
	}
	return ""
}

func (e *Editor) saveState() {
	if err := e.state.Save(); err != nil {
		e.status = "记不下设置：" + err.Error()
	}
}

func (e *Editor) setTheme(name string) error {
	if e.ThemePreviewing() {
		e.closeOverlay()
	}
	e.cfg.Theme = name
	if e.state != nil {
		e.state.Theme = &name
		e.saveState()
	}
	e.settingsGen++
	return nil
}

func (e *Editor) setAppearance(a string) error {
	if e.ThemePreviewing() {
		e.closeOverlay()
	}
	e.cfg.Appearance = a
	if e.state != nil {
		e.state.Appearance = &a
		e.saveState()
	}
	e.settingsGen++
	return nil
}

// zoom 放大、缩小代码的字号（by 为 0 时恢复默认的字号）；只对窗口版有意义。
func (e *Editor) zoom(by float64) error {
	if e.imageZoom(by) {
		return nil // 当前标签是图片：缩放图片
	}
	size := config.Default().FontSize
	if by != 0 {
		size = max(min(e.cfg.FontSize+by, 28), 9)
	}
	e.cfg.FontSize = size
	if e.state != nil {
		e.state.FontSize = &size
		e.saveState()
	}
	e.settingsGen++
	return nil
}

func (e *Editor) toggleAutoSave() error {
	e.autoSave = !e.autoSave
	if e.state != nil {
		v := e.autoSave
		e.state.AutoSave = &v
		e.saveState()
	}
	e.settingsGen++
	if e.autoSave {
		e.status = "自动保存：开"
		e.saveDirty()
	} else {
		e.status = "自动保存：关"
	}
	return nil
}

// AutoSave 报告自动保存开着没有（菜单的勾）。
func (e *Editor) AutoSave() bool { return e.autoSave }

// autoSaveSoon 在停止编辑 1 秒后保存（每次编辑重新计时）。
func (e *Editor) autoSaveSoon() {
	if !e.autoSave {
		return
	}
	e.autoSaveGen++
	gen := e.autoSaveGen
	time.AfterFunc(autoSaveDelay, func() {
		e.Post(func(e *Editor) {
			if e.autoSaveGen == gen {
				e.saveDirty()
			}
		})
	})
}

// saveDirty 保存所有有未保存修改、磁盘上已有的文件（新建还没存过的文件等用户自己存）。
func (e *Editor) saveDirty() {
	if !e.autoSave {
		return
	}
	saved := false
	for _, d := range e.docs {
		if d.buf.Dirty() && d.buf.Exists() && !d.buf.Snapshot().Meta().ReadOnly {
			if err := e.saveDoc(d); err != nil {
				e.status = err.Error()
				continue
			}
			saved = true
			if e.isConfigFile(d.buf.Path()) {
				e.reloadConfig()
			}
		}
	}
	if saved {
		e.gitChanged()
	}
}

// reloadConfig 在配置文件保存后重新读配置：键位、主题、字体、git blame 立即生效。
func (e *Editor) reloadConfig() {
	cfg, warns := config.Load(e.root)
	e.state.Apply(cfg)
	warns = append(warns, e.compileKeys(cfg)...)
	if cfg.AutoSave != nil {
		e.autoSave = *cfg.AutoSave
	}
	e.cfg = cfg
	e.settingsGen++
	e.status = "配置已生效"
	e.configProblem = ""
	if len(warns) > 0 {
		e.status = "配置：" + warns[0]
		e.configProblem = warns[0] // stays in the status bar until a reload reads cleanly
	}
}

// watchConfig reloads the configuration when the user's files change on disk: saved in another
// window, or by another program. The project's .yan.toml comes through the folder's watcher.
func (e *Editor) watchConfig() {
	user, keys := config.UserPath(), config.KeybindingsPath()
	w, err := workspace.NewWatcher(func(ch workspace.Changes) {
		for _, p := range append(ch.Files, ch.Gone...) {
			if p == user || p == keys {
				e.Post(func(e *Editor) { e.reloadConfig() })
				return
			}
		}
	})
	if err != nil {
		return
	}
	if !w.Add(filepath.Dir(user)) {
		w.Close() // no configuration folder yet: the focus check and saves still reload
		return
	}
	e.cfgWatcher = w
}

// isConfigFile 报告 path 是不是用户或项目的配置文件。
func (e *Editor) isConfigFile(path string) bool {
	return path == config.UserPath() || path == config.KeybindingsPath() || e.root != "" && path == filepath.Join(e.root, ".yan.toml")
}

// ---- 会话 ----

// SaveSession 记下当前文件夹开着的标签（关窗口、退出、换文件夹前调用）。
func (e *Editor) SaveSession() {
	if e.state == nil || e.ws == nil {
		return
	}
	e.sync()
	s := config.Session{Review: e.reviewNotes()}
	for _, c := range e.cols {
		sc := config.SessionColumn{Locked: c.locked}
		for _, t := range c.tabs {
			if t.term == nil && (t.diff == nil || t.diff.commit == nil) && !t.doc.buf.Exists() {
				continue
			}
			m := e.sels(t).Main()
			st := config.SessionTab{Path: sessionRel(e.ws.root, t.doc.buf.Path()), Preview: t.preview, Anchor: m.Anchor, Head: m.Head, ScrollY: t.scrollY, Wrap: t.wrap, RenderPreview: t.markdown || t.svg, PreviewLine: t.previewLine}
			if t.diff != nil && t.diff.commit != nil {
				st.Path, st.Revision = "", t.diff.commit.Hash
				st.Repository = sessionRel(e.ws.root, t.diff.repoRoot)
				if t.diff.compareFrom != nil {
					st.CompareFrom = t.diff.compareFrom.Hash
				}
			}
			if t.term != nil {
				st.Path, st.Terminal = sessionRel(e.ws.root, t.term.dir), true
			}
			if t == c.tab() {
				sc.Active = len(sc.Tabs)
			}
			sc.Tabs = append(sc.Tabs, st)
		}
		sc.Weight = c.weight
		if len(sc.Tabs) > 0 || c.locked {
			if c == e.column() {
				s.Active = len(s.Columns)
			}
			s.Columns = append(s.Columns, sc)
		}
	}
	s.Sidebar = e.sidebar
	if e.state.Sessions == nil {
		e.state.Sessions = map[string]config.Session{}
	}
	e.state.Sessions[e.ws.root] = s
	if e.movedFrom != "" && e.movedFrom != e.ws.root {
		delete(e.state.Sessions, e.movedFrom) // 文件夹搬过家：旧位置的记录换成新位置的
		e.movedFrom = ""
	}
	e.state.AddRecent(e.ws.root)
	e.saveState()
}

// RestoreSession 重新打开这个文件夹上次开着的标签；没有记录（或文件都没了）时返回 false。
func (e *Editor) RestoreSession() bool {
	if e.state == nil || e.ws == nil {
		return false
	}
	s, ok := e.state.Sessions[e.ws.root]
	if !ok {
		s, ok = e.movedSession()
	}
	for ci := range s.Columns { // 文件夹里的路径记的是相对的：换成现在的位置
		for si := range s.Columns[ci].Tabs {
			st := &s.Columns[ci].Tabs[si]
			st.Path, st.Repository = sessionAbs(e.ws.root, st.Path), sessionAbs(e.ws.root, st.Repository)
		}
	}
	if ok {
		e.restoreReview(s.Review)
	}
	if !ok || len(s.Columns) == 0 {
		return false
	}
	e.sync()
	defer e.sync()
	opened := false
	for ci, sc := range s.Columns {
		if ci > 0 {
			e.cols = append(e.cols, newColumn())
			e.cols[ci].size = e.cols[0].size
		}
		c := e.cols[min(ci, len(e.cols)-1)]
		if sc.Weight > 0 {
			c.weight = sc.Weight
		}
		active := 0
		for si, st := range sc.Tabs {
			if st.Revision != "" {
				e.col = ci
				root := st.Repository
				if root == "" {
					root = e.root
				}
				t := e.showHistoryTabAt(root, st.CompareFrom, st.Revision, st.Preview)
				t.diff.restore = &st
				opened = true
				if si == sc.Active {
					active = c.active
				}
				continue
			}
			if st.Terminal {
				h, ok := e.host.(interface{ EmbeddedTerminal() bool })
				if !ok || !h.EmbeddedTerminal() {
					continue
				}
				if st.Path != "" {
					if fi, err := os.Stat(st.Path); err != nil || !fi.IsDir() {
						continue
					}
				}
				e.newTerminalTab(c, len(c.tabs), st.Path, "")
				opened = true
				if si == sc.Active {
					active = c.active
				}
				continue
			}
			if _, err := os.Stat(st.Path); err != nil {
				continue
			}
			if err := e.openIn(c, len(c.tabs), st.Path, st.Preview); err != nil {
				continue
			}
			opened = true
			t := c.tab()
			if si == sc.Active {
				active = c.active
			}
			t.wrap = min(max(st.Wrap, -1), 1)
			if st.RenderPreview && e.profile.GUI {
				t.markdown = isMarkdown(t.doc.buf.Path())
				t.svg = strings.EqualFold(filepath.Ext(t.doc.buf.Path()), ".svg")
			}
			t.previewLine = max(0, min(st.PreviewLine, t.doc.buf.Snapshot().LineCount()-1))
			n := t.doc.buf.Snapshot().Len()
			t.doc.buf.SetSelections(edit.Single(edit.Range{Anchor: min(st.Anchor, n), Head: min(st.Head, n), GoalX: -1}))
			t.sels = t.doc.buf.Selections()
			t.scrollY = min(st.ScrollY, t.doc.buf.Snapshot().LineCount()-1)
		}
		if len(c.tabs) > 0 {
			c.active = min(active, len(c.tabs)-1)
		}
		c.locked = sc.Locked // after its tabs: restoration targets a locked column on purpose
	}
	// 文件都没了的列不要；锁定的列空着也留下，并且总留一列不锁的给文件
	for i := len(e.cols) - 1; i >= 0; i-- {
		if c := e.cols[i]; len(c.tabs) == 0 && e.removable(c) {
			e.cols = slices.Delete(e.cols, i, i+1)
		}
	}
	e.col = max(min(s.Active, len(e.cols)-1), 0)
	if !slices.ContainsFunc(e.cols, func(c *column) bool { return !c.locked }) {
		e.fileColumn()
		e.col = 0
	}
	e.sidebar = s.Sidebar
	e.area = areaEditor
	return opened
}

// sessionRel is how a session keeps path: relative to the folder when it is inside it, so the
// session comes back after the folder moves; absolute otherwise.
func sessionRel(root, path string) string {
	if path == "" || root == "" {
		return path
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return path
	}
	return filepath.ToSlash(rel)
}

// sessionAbs is the path a session kept, in the folder where it is now.
func sessionAbs(root, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, filepath.FromSlash(path))
}

// movedSession finds the session of this folder from before it moved: one kept for a folder
// of the same name that is gone, whose files are all here.
func (e *Editor) movedSession() (config.Session, bool) {
	name := filepath.Base(e.ws.root)
	for old, s := range e.state.Sessions {
		if old == e.ws.root || filepath.Base(old) != name {
			continue
		}
		if _, err := os.Stat(old); err == nil {
			continue // the old folder is still there: another folder of the same name
		}
		fits, files := true, 0
		for _, c := range s.Columns {
			for _, st := range c.Tabs {
				if st.Revision != "" || st.Terminal || st.Path == "" {
					continue
				}
				if filepath.IsAbs(st.Path) {
					fits = false // an absolute path: kept before sessions were relative, or outside the folder
					continue
				}
				files++
				if _, err := os.Stat(sessionAbs(e.ws.root, st.Path)); err != nil {
					fits = false
				}
			}
		}
		if fits && files > 0 {
			e.movedFrom = old
			return s, true
		}
	}
	return config.Session{}, false
}

// UpdateChecks reports whether the window checks for a new version every hour (Help menu).
func (e *Editor) UpdateChecks() bool { return e.state == nil || !e.state.UpdateChecksOff }

// SetUpdateChecks turns the hourly checks for a new version on or off, for the next start too.
func (e *Editor) SetUpdateChecks(on bool) {
	if e.state != nil {
		e.state.UpdateChecksOff = !on
		e.saveState()
	}
}
