package app

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/text"
)

// 未命名的文档：新建时没有路径，第一次保存时另存为。另存为先把文字写到磁盘上，写成了才把路径
// 给文档，所以写不成时文档还是原来那样、修改都在。不弹对话框的保存（自动保存、全部保存、提交前保存）
// 不碰它；git、监视、最近文件、会话也都等它有了路径再管。

// errUntitled 是不能问保存位置的地方遇到未命名文档时的错误。
var errUntitled = userError("还没存过的文件要先「另存为」")

// name 是文档在标签、状态栏里的名字：文件名，未命名时是「未命名-1」这样的。
func (d *document) name() string {
	if p := d.buf.Path(); p != "" {
		return filepath.Base(p)
	}
	return d.title
}

// untitled 报告文档是不是还没存过、没有路径。
func (d *document) untitled() bool { return d.buf.Path() == "" && d.title != "" }

// newUntitled 在当前列打开一个空的未命名文档（⌘N）。编号用开着的未命名文档里没用的最小的。
func (e *Editor) newUntitled() error {
	n := 1
	for slices.ContainsFunc(e.docs, func(d *document) bool { return d.untitled() && d.title == untitledTitle(n) }) {
		n++
	}
	d := &document{buf: edit.NewBuffer(text.FromString("", text.Meta{})), title: untitledTitle(n)}
	e.adopt(d)
	e.rec.touched = true
	c := e.column()
	e.addTab(c, c.active+1, d, nil)
	e.area = areaEditor
	return nil
}

// addTab 在列 c 的第 at 位多开一个显示文档 d 的标签（同一个文档：文字、撤销都共用），
// 光标和滚动位置从 like 开始。按路径打开的文件走 openIn；未命名的文档只能这样再开一个。
func (e *Editor) addTab(c *column, at int, d *document, like *tab) {
	t := &tab{doc: d, sels: edit.Single(edit.Cursor(0))}
	if like != nil {
		t.sels, t.scrollY = e.sels(like), like.scrollY
	}
	d.tabs = append(d.tabs, t)
	at = max(min(at, len(c.tabs)), 0)
	c.tabs = slices.Insert(c.tabs, at, t)
	if c.active >= at {
		c.active++
	}
	e.activate(c, at)
}

func untitledTitle(n int) string { return fmt.Sprintf("未命名-%d", n) }

// saveAs 问文档存到哪里（窗口版用系统的保存对话框，终端版在输入框里输路径），再写过去。
func (e *Editor) saveAs() error {
	t := e.cur()
	if t == nil || t.term != nil || t.diff != nil || t.mergeSide != nil || t.image {
		return errors.New("没有可以另存为的文件")
	}
	d := t.doc
	if d.buf.Snapshot().Meta().ReadOnly {
		return errors.New("只读文件，不能保存")
	}
	if d.askingPath {
		return nil // 已经在问了
	}
	dir, name := e.root, d.name()
	if p := d.buf.Path(); p != "" {
		dir = filepath.Dir(p)
	} else if filepath.Ext(name) == "" {
		name += ".txt"
	}
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	if h, ok := e.host.(interface {
		SaveAsDialog(string, func(string, error))
	}); ok {
		d.askingPath = true
		h.SaveAsDialog(filepath.Join(dir, name), func(path string, err error) {
			e.Post(func(e *Editor) {
				d.askingPath = false
				switch {
				case err != nil:
					e.status = err.Error()
				case path == "" || !slices.Contains(e.docs, d):
					// 取消了，或者对话框开着的时候文档关掉了：什么都不写
				default:
					// 系统的保存对话框已经问过要不要替换
					if err := e.writeAs(d, path, true); err != nil {
						e.status = err.Error()
					}
				}
			})
		})
		return nil
	}
	initial := filepath.Join(dir, name)
	if e.root != "" && strings.HasPrefix(initial, e.root+string(filepath.Separator)) {
		initial = e.rel(initial)
	}
	e.prompt("另存为", initial, func(s string) string { return "保存到 " + strings.TrimSpace(s) }, func(e *Editor, s string) error {
		path := strings.TrimSpace(s)
		if path == "" {
			return errors.New("路径不能为空")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(cmp.Or(e.root, dir), path)
		}
		if !slices.Contains(e.docs, d) {
			return nil
		}
		return e.writeAs(d, path, false)
	})
	return nil
}

// canonical 是 path 去掉符号链接后的样子（文件还不在时只解析它所在的文件夹），用来认出同一个文件。
func canonical(path string) string {
	if p, err := filepath.EvalSymlinks(path); err == nil {
		return p
	}
	if dir, err := filepath.EvalSymlinks(filepath.Dir(path)); err == nil {
		return filepath.Join(dir, filepath.Base(path))
	}
	return filepath.Clean(path)
}

// writeAs 把文档 d 写到 target，成功后 target 就是它的文件。replace 为真表示用户已经同意替换已有的
// 文件（系统对话框问过）；否则 target 已存在时先问。别的标签已经开着 target 时不写：两个可以各自
// 编辑的文档对着同一个文件，迟早有一个的修改被另一个覆盖。
func (e *Editor) writeAs(d *document, target string, replace bool) error {
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(abs); err == nil && fi.IsDir() {
		return userError(filepath.Base(abs) + " 是文件夹，请另选一个文件名")
	}
	same := canonical(abs)
	for _, o := range e.docs {
		if o != d && o.buf.Path() != "" && canonical(o.buf.Path()) == same {
			return userError(filepath.Base(abs) + " 已经在编辑器里打开了：先关掉它，或换一个名字")
		}
	}
	now, err := edit.StatStamp(abs)
	if err != nil {
		return err
	}
	if !now.ModTime.IsZero() && !replace {
		e.choose(filepath.Base(abs)+" 已经存在，要替换它吗？",
			ovItem{label: "替换", run: func(e *Editor) error { return e.writeOver(d, abs, now) }},
			ovItem{label: "取消", run: func(*Editor) error { return nil }})
		return nil
	}
	return e.writeOver(d, abs, now)
}

// writeOver 写过去；磁盘上的 abs 已经不是用户确认时（expect）的样子就不写。
func (e *Editor) writeOver(d *document, abs string, expect edit.Stamp) error {
	if !slices.Contains(e.docs, d) {
		return nil
	}
	if err := d.buf.SaveAs(abs, expect); errors.Is(err, edit.ErrConflict) {
		return userError(filepath.Base(abs) + " 刚被别的程序创建或改过，没有覆盖：请重新另存为")
	} else if err != nil {
		return fmt.Errorf("保存 %s 失败：%w", filepath.Base(abs), err)
	}
	e.bindPath(d)
	e.checkpointNow() // 有了文件：不再需要备份它
	return nil
}

// bindPath 在文档有了（新的）文件之后接上按路径管的东西：语法高亮、监视、最近文件、资源管理器、git。
func (e *Editor) bindPath(d *document) {
	path := d.buf.Path()
	if d.hl != nil {
		d.hl.Close()
		d.hl = nil
	}
	e.highlightFor(d)
	for _, t := range d.tabs {
		e.keep(t)
	}
	e.touchRecent(path)
	if e.ws != nil {
		e.ws.watcher.Add(filepath.Dir(path))
		e.ws.tree.Refresh()
		e.reindex()
	}
	e.gitChanged()
	e.status = "已保存 " + d.name()
	if e.isConfigFile(path) {
		e.reloadConfig()
	}
}
