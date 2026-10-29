package app

// Whitespace filtering and comparing any two files in the diffs, after the behaviour of
// Mayoi PR #50 (Garfielk, https://github.com/sorrycc/Mayoi/pull/50, unmerged): its
// compareDiffLines, withoutBlankHunks and Compare Two Files, rewritten for yan's diff tabs.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/git/linediff"
)

// diffHunks are the changes from base to cur as the diffs show them: compared as the
// whitespace filter says, with the hunks still indexing the original lines, so that what is
// shown, counted, gone to and staged is one list.
func (e *Editor) diffHunks(base, cur []string) []linediff.Hunk {
	return whitespaceHunks(base, cur, e.cfg.DiffWhitespace)
}

func whitespaceHunks(base, cur []string, mode string) []linediff.Hunk {
	if mode == "" {
		return linediff.Hunks(base, cur)
	}
	hs := linediff.Hunks(whitespaceKeys(base, mode), whitespaceKeys(cur, mode))
	if mode == "blank" {
		hs = withoutBlankHunks(hs, base, cur)
	}
	return hs
}

// whitespaceKeys are the lines as the filter compares them.
func whitespaceKeys(lines []string, mode string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		switch mode {
		case "eol":
			l = strings.TrimRight(l, " \t")
		case "all", "blank":
			l = strings.Map(func(r rune) rune {
				if r == ' ' || r == '\t' || r == ' ' {
					return -1
				}
				return r
			}, l)
		}
		out[i] = l
	}
	return out
}

// withoutBlankHunks drops the changes made only of blank lines.
func withoutBlankHunks(hs []linediff.Hunk, base, cur []string) []linediff.Hunk {
	blank := func(lines []string) bool {
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				return false
			}
		}
		return true
	}
	out := hs[:0]
	for _, h := range hs {
		if !blank(base[h.A:min(h.A+h.ALen, len(base))]) || !blank(cur[h.B:min(h.B+h.BLen, len(cur))]) {
			out = append(out, h)
		}
	}
	return out
}

var diffWhitespaceLabels = []struct{ mode, label string }{
	{"", "比较全部字符"},
	{"eol", "忽略行尾空白"},
	{"all", "忽略所有空白"},
	{"blank", "忽略空白和空行"},
}

func diffWhitespaceLabel(mode string) string {
	for _, w := range diffWhitespaceLabels {
		if w.mode == mode {
			return w.label
		}
	}
	return mode
}

// chooseDiffWhitespace asks which whitespace the diffs compare.
func (e *Editor) chooseDiffWhitespace() error {
	var items []ovItem
	for _, w := range diffWhitespaceLabels {
		item := ovItem{label: w.label, run: func(e *Editor) error { return e.setDiffWhitespace(w.mode) }}
		if w.mode == e.cfg.DiffWhitespace {
			item.hint = "当前"
		}
		items = append(items, item)
	}
	e.choose("改动视图比较空白", items...)
	return nil
}

// setDiffWhitespace sets the whitespace filter, keeps it for the next start and reads the open
// diffs again.
func (e *Editor) setDiffWhitespace(mode string) error {
	mode, ok := config.DiffWhitespaceMode(mode)
	if !ok {
		return errors.New("未知的空白比较方式")
	}
	e.cfg.DiffWhitespace = mode
	if e.state != nil {
		v := mode
		e.state.DiffWhitespace = &v
		e.saveState()
	}
	e.refreshAllDiffs()
	e.status = "改动视图：" + diffWhitespaceLabel(mode)
	return nil
}

func (e *Editor) refreshAllDiffs() {
	for _, c := range e.cols {
		for _, t := range c.tabs {
			if t.diff != nil {
				e.refreshDiff(t.doc, t.diff)
			}
		}
	}
}

// CompareFiles opens a diff of two local files, whether or not either is in a repository:
// left is the old side, right the new. Open documents count with their unsaved text, and
// neither file is staged or changed by it.
func (e *Editor) CompareFiles(left, right string) error {
	left, right = filepath.Clean(left), filepath.Clean(right)
	if left == right {
		return errors.New("选两个不同的文件比较")
	}
	for _, p := range []string{left, right} {
		if e.docOf(p) != nil {
			continue
		}
		if fi, err := os.Stat(p); err != nil {
			return err
		} else if fi.IsDir() {
			return errors.New(filepath.Base(p) + " 是文件夹，选两个文件比较")
		}
	}
	ln, rn := filepath.Base(left), filepath.Base(right)
	if ln == rn {
		ln, rn = filepath.Join(filepath.Base(filepath.Dir(left)), ln), filepath.Join(filepath.Base(filepath.Dir(right)), rn)
	}
	dt := &diffTab{path: right, peer: left, expanded: map[int]bool{}}
	e.showDiff(ln+" ↔ "+rn, dt, false)
	return nil
}

// compareFiles is the command: the window chooses the files with the system's dialog,
// elsewhere two prompts ask for their paths, starting from the file being edited.
func (e *Editor) compareFiles() error {
	dir := e.root
	cur := ""
	if t := e.cur(); t != nil && t.diff == nil && t.term == nil && t.doc.buf.Exists() {
		cur = t.doc.buf.Path()
		dir = filepath.Dir(cur)
	}
	if h, ok := e.host.(interface{ CompareFilesDialog(dir string) }); ok {
		h.CompareFilesDialog(dir)
		return nil
	}
	hint := func(s string) string {
		return "比较 " + strings.TrimSpace(s) + "（相对路径从打开的文件夹算起）"
	}
	abs := func(p string) string {
		if p = strings.TrimSpace(p); p != "" && !filepath.IsAbs(p) {
			p = filepath.Join(e.root, p)
		}
		return p
	}
	e.prompt("比较：左边（旧）的文件", cur, hint, func(e *Editor, left string) error {
		left = abs(left)
		e.prompt("比较：右边（新）的文件", "", hint, func(e *Editor, right string) error {
			return e.CompareFiles(left, abs(right))
		})
		return nil
	})
	return nil
}

// selectForCompare remembers the explorer's file as the left side of the next comparison.
func (e *Editor) selectForCompare() error {
	path, _, ok := e.treeTarget()
	if !ok || path == e.root {
		return errors.New("先在资源管理器里选一个文件")
	}
	if fi, err := os.Stat(path); err != nil || fi.IsDir() {
		return errors.New("选一个文件，不是文件夹")
	}
	e.compareLeft = path
	e.status = "已选 " + filepath.Base(path) + " 作为比较的左边：在另一个文件上选「与已选文件比较」"
	return nil
}

// compareWithSelected compares the file chosen with selectForCompare with the explorer's file.
func (e *Editor) compareWithSelected() error {
	if e.compareLeft == "" {
		return errors.New("先在一个文件上选「选为比较对象」")
	}
	path, _, ok := e.treeTarget()
	if !ok || path == e.root {
		return errors.New("先在资源管理器里选一个文件")
	}
	return e.CompareFiles(e.compareLeft, path)
}

// CanCompareWithSelected reports whether a file is chosen as the left side of a comparison.
func (e *Editor) CanCompareWithSelected() bool { return e.compareLeft != "" }

var diffHighlightLabels = []struct{ mode, label string }{
	{"", "按词标出改了的部分"},
	{"char", "按字标出改了的部分"},
	{"line", "只标出改了的行"},
}

// chooseDiffHighlight asks how changed lines show what changed within them.
func (e *Editor) chooseDiffHighlight() error {
	var items []ovItem
	for _, h := range diffHighlightLabels {
		item := ovItem{label: h.label, run: func(e *Editor) error {
			e.cfg.DiffHighlight = h.mode
			if e.state != nil {
				v := h.mode
				e.state.DiffHighlight = &v
				e.saveState()
			}
			e.refreshAllDiffs()
			e.status = "改动视图：" + h.label
			return nil
		}}
		if h.mode == e.cfg.DiffHighlight {
			item.hint = "当前"
		}
		items = append(items, item)
	}
	e.choose("改动视图怎么标出改动", items...)
	return nil
}

// chooseDiffContext asks how many unchanged lines stay around each change.
func (e *Editor) chooseDiffContext() error {
	var items []ovItem
	for n := 0; n <= config.MaxDiffContext; n++ {
		item := ovItem{label: fmt.Sprintf("改动前后留 %d 行", n), run: func(e *Editor) error {
			e.cfg.DiffContext = n
			if e.state != nil {
				v := n
				e.state.DiffContext = &v
				e.saveState()
			}
			for _, c := range e.cols {
				for _, t := range c.tabs {
					if t.diff != nil {
						clear(t.diff.expanded) // 折叠的编号跟着变了
					}
				}
			}
			e.refreshAllDiffs()
			e.status = fmt.Sprintf("改动视图：改动前后留 %d 行没改的", n)
			return nil
		}}
		if n == e.cfg.DiffContext {
			item.hint = "当前"
		}
		items = append(items, item)
	}
	e.choose("改动视图的上下文行数", items...)
	return nil
}
