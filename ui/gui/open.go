package gui

// Folder reuse and Dock recents follow Mayoi v0.8.x (MIT, cmd/mayoi/main.go).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/corn12138/yan-source/internal/launch"
	"github.com/egoist/mygo"
)

func sameFolder(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	a, _ = filepath.Abs(a)
	b, _ = filepath.Abs(b)
	if a == b {
		return true
	}
	ai, ae := os.Stat(a)
	bi, be := os.Stat(b)
	return ae == nil && be == nil && os.SameFile(ai, bi)
}

func (f *Frontend) folderProject(path string) *Frontend {
	if f.g != nil {
		for _, p := range f.g.projects {
			if sameFolder(p.ed.Root(), path) {
				return p
			}
		}
	} else if sameFolder(f.ed.Root(), path) {
		return f
	}
	for _, g := range groups {
		for _, p := range g.projects {
			if sameFolder(p.ed.Root(), path) {
				return p
			}
		}
	}
	return nil
}

func (f *Frontend) bringForward() {
	if f.g != nil {
		for i, p := range f.g.projects {
			if p == f {
				f.g.show(i)
				break
			}
		}
	}
	if f.win != nil {
		if f.win.IsMinimized() {
			f.win.Restore()
		}
		f.win.Show()
		f.win.Focus()
		mygo.App.Focus()
	}
}

func (f *Frontend) openFolder(path string, separate bool) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("不是文件夹：%s", path)
	}
	if found := f.folderProject(path); found != nil {
		found.bringForward()
		return nil
	}
	if separate {
		if makeWindow == nil {
			return fmt.Errorf("无法创建窗口")
		}
		ed, next := makeWindow()
		if err := ed.SwitchFolder(path); err != nil {
			ed.Close()
			return err
		}
		x, y := 0, 0
		if f.win != nil {
			x, y = f.win.Position()
			x += 28
			y += 28
		}
		next.open(ed, x, y)
		next.bringForward()
	} else if f.g != nil && f.ed.ProjectTabs() && f.ed.Root() != "" {
		f.g.add(path)
	} else {
		if err := f.ed.SwitchFolder(path); err != nil {
			return err
		}
	}
	f.refreshDock()
	return nil
}

func (f *Frontend) fileProject(path string) *Frontend {
	var best *Frontend
	visit := func(p *Frontend) {
		root := p.ed.Root()
		if root == "" {
			return
		}
		if mapped, ok := projectFile(root, path); ok {
			if best == nil {
				best = p
				return
			}
			old, _ := projectFile(best.ed.Root(), path)
			rel, _ := filepath.Rel(root, mapped)
			oldRel, _ := filepath.Rel(best.ed.Root(), old)
			if strings.Count(rel, string(filepath.Separator)) < strings.Count(oldRel, string(filepath.Separator)) {
				best = p
			}
		}
	}
	visit(f)
	if f.g != nil {
		for _, p := range f.g.projects {
			visit(p)
		}
	}
	for _, g := range groups {
		for _, p := range g.projects {
			visit(p)
		}
	}
	return best
}

// Preserve the project's spelling of paths so aliases do not create a second buffer.
func projectFile(root, path string) (string, bool) {
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		if sameFolder(root, dir) {
			rel, err := filepath.Rel(dir, path)
			if err == nil {
				return filepath.Join(root, rel), true
			}
			return "", false
		}
		if filepath.Dir(dir) == dir {
			return "", false
		}
	}
}

func (f *Frontend) openTarget(t launch.Target, separate bool) error {
	if t.Readme {
		return f.openFolder(t.Root, separate)
	}
	dest := f.fileProject(t.File)
	if dest == nil {
		if makeWindow == nil {
			return fmt.Errorf("无法创建窗口")
		}
		ed, next := makeWindow()
		if t.Folder {
			if err := ed.SwitchFolder(t.Root); err != nil {
				ed.Close()
				return err
			}
		} else {
			ed.SetRoot(t.Root)
		}
		if err := ed.OpenFile(t.File); err != nil {
			ed.Close()
			return err
		}
		if t.Line > 0 {
			ed.GotoLine(t.Line)
		}
		next.open(ed, 0, 0)
		next.bringForward()
		return nil
	}
	if mapped, ok := projectFile(dest.ed.Root(), t.File); ok {
		t.File = mapped
	}
	if err := dest.ed.OpenFile(t.File); err != nil {
		return err
	}
	if t.Line > 0 {
		dest.ed.GotoLine(t.Line)
	}
	dest.bringForward()
	return nil
}

func (f *Frontend) openRequests(args []string, wd string) {
	request := launch.Parse(args, wd)
	if len(request.Targets) == 0 {
		if request.NewWindow {
			newWindow()
		} else {
			f.bringForward()
		}
		return
	}
	for _, t := range request.Targets {
		if err := f.openTarget(t, request.NewWindow); err != nil {
			f.ed.Notify(err.Error())
		}
	}
}

func (f *Frontend) refreshDock() {
	if f.win == nil {
		return
	}
	items := []*mygo.MenuItem{{Label: "新建窗口", Click: func(*mygo.MenuItem, *mygo.Window) { f.spawn() }}}
	for _, dir := range f.ed.AllRecentFolders() {
		label := dir
		if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(dir, home+string(filepath.Separator)) {
			label = "~" + dir[len(home):]
		}
		items = append(items, &mygo.MenuItem{Label: label, Click: func(*mygo.MenuItem, *mygo.Window) {
			current := f
			if g := focusedGroup(); g != nil {
				current = g.cur()
			}
			current.do(func() {
				if err := current.openFolder(dir, true); err != nil {
					current.ed.Notify(err.Error())
				}
			})
		}})
	}
	mygo.App.Dock.SetMenu(mygo.NewMenu(items))
}
