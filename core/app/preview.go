package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/text"
)

// isMarkdown reports whether path previews in a Markdown pane: Markdown, or a Mermaid file,
// which the pane shows as its one diagram.
func isMarkdown(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".mmd", ".mermaid":
		return true
	}
	return false
}

func (e *Editor) previewToSide() error {
	source := e.cur()
	if !e.profile.GUI || source == nil || source.diff != nil || (!isMarkdown(source.doc.buf.Path()) && !strings.EqualFold(filepath.Ext(source.doc.buf.Path()), ".svg")) {
		return errors.New("请在窗口版打开 Markdown、Mermaid 或 SVG 文件")
	}
	c, active, area := e.column(), e.column().active, e.area
	// Reuse an existing preview of this document; repeated commands must not grow columns.
	for _, other := range e.cols {
		if other == c {
			continue
		}
		for i, t := range other.tabs {
			if t.doc == source.doc && (t.markdown || t.svg) {
				other.active = i
				return nil
			}
		}
	}
	if err := e.splitRight(); err != nil {
		return err
	}
	dest := e.cur()
	dest.markdown, dest.svg = isMarkdown(dest.doc.buf.Path()), !isMarkdown(dest.doc.buf.Path())
	dest.previewLine = source.scrollY
	source.markdown, source.svg = false, false
	e.activate(c, active)
	e.area = area
	return nil
}

// SetPreviewTop records source lines rather than pixels so sessions survive font and window changes.
func (e *Editor) SetPreviewTop(col int, path string, line int) {
	if col < 0 || col >= len(e.cols) {
		return
	}
	t := e.cols[col].tab()
	if t != nil && t.markdown && t.doc.buf.Path() == path {
		t.previewLine = max(0, min(line, t.doc.buf.Snapshot().LineCount()-1))
	}
}

// ScrollSourceTo never changes focus, caret or selection; path guards a delayed preview frame.
func (e *Editor) ScrollSourceTo(col int, path string, line int) {
	if col < 0 || col >= len(e.cols) {
		return
	}
	t := e.cols[col].tab()
	if t != nil && !t.markdown && !t.svg && t.diff == nil && t.doc.buf.Path() == path {
		t.setTop(vrow{line: max(0, min(line, t.doc.buf.Snapshot().LineCount()-1))})
	}
}

// errStalePreview refuses an edit made in a preview built from older text.
var errStalePreview = errors.New("文档刚被别处改过：预览已更新，请在新的内容上再改一次")

// EditMarkdownLines replaces source lines [from, to) of the Markdown document that column col
// previews with src, as typing in its source would: grouped into the undo history with the
// typing around it, the document dirty and autosaved as usual. src has no final line break and
// uses "\n"; it is written with the document's own line endings. version is the text the
// preview showed: an edit of any other version is refused rather than applied to moved lines.
// It returns the document's version after the edit.
func (e *Editor) EditMarkdownLines(col int, path string, version uint64, from, to int, src string) (uint64, error) {
	if col < 0 || col >= len(e.cols) {
		return 0, errStalePreview
	}
	t := e.cols[col].tab()
	if t == nil || !t.markdown || t.doc.buf.Path() != path {
		return 0, errStalePreview
	}
	e.sync()
	defer e.sync()
	b := t.doc.buf
	s := b.Snapshot()
	if s.Version() != version {
		return s.Version(), errStalePreview
	}
	if s.Meta().ReadOnly {
		return version, errors.New("只读打开的文件不能编辑")
	}
	n := s.LineCount()
	from = max(0, min(from, n))
	to = max(from, min(to, n))
	start, end := s.LineStart(from), s.Len()
	if to < n {
		end = s.LineStart(to) - 1 // the line break before line to stays
		if end > start && s.String(end-1, end) == "\r" {
			end--
		}
	}
	if from == to { // a new block: it goes before line from, with its own line break
		end = start
		src += "\n"
	}
	src = strings.ReplaceAll(strings.ReplaceAll(src, "\r\n", "\n"), "\r", "\n")
	if s.Meta().EOL == text.EOLCRLF {
		src = strings.ReplaceAll(src, "\n", "\r\n")
	}
	if s.String(start, end) == src {
		return version, nil
	}
	b.Apply(edit.Transaction{Changes: text.ChangeSet{{Start: start, End: end, Text: src}}, Origin: edit.OriginTyping})
	e.guide.previewEdited = true
	e.guideCheck()
	return b.Snapshot().Version(), nil
}

// SaveMarkdownImage stores a pasted PNG beside the Markdown document at path, in its assets
// folder, under a name that does not replace another file, and returns the link to write in
// the document: relative, with forward slashes, as Markdown renderers and Git hosts read it.
func (e *Editor) SaveMarkdownImage(path string, png []byte) (string, error) {
	if len(png) == 0 {
		return "", errors.New("剪贴板里没有图片")
	}
	dir := filepath.Join(filepath.Dir(path), "assets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) + "-" + time.Now().Format("20060102-150405")
	for i := 0; ; i++ {
		name := base + ".png"
		if i > 0 {
			name = fmt.Sprintf("%s-%d.png", base, i)
		}
		f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, err = f.Write(png)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(f.Name())
			return "", err
		}
		return "assets/" + name, nil
	}
}
