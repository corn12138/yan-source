package gui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/workspace"
)

// The breadcrumbs' folder list follows Mayoi (MIT, cmd/mayoi/crumbs.go at 46990a1): a folder of
// the path opens a box with that folder's files as a tree of its own, whose expansion leaves
// the explorer as it is. It reads folders itself, off the UI thread, so it also works for files
// outside the open folder; a read for a box since closed is dropped.

// crumbsPicker is the open box, if any.
type crumbsPicker struct {
	open bool
	// col and file are the column and file whose breadcrumbs opened the box, dir the folder.
	col       int
	file, dir string
	gen       int // counts the boxes opened: an earlier box's reads are dropped
	tree      ui.OutlineState[string]
	sel       int
	view      ui.Handle
	focus     bool   // give the tree the keyboard as it shows
	reveal    string // the entry on the way to the file, chosen once it shows

	kids    map[string][]string
	dirs    map[string]bool
	loading map[string]bool
	errs    map[string]string
}

// crumbDirs returns, for each part of a breadcrumb path, the folder it names ("" for the file).
func crumbDirs(root, path string, parts []string) []string {
	dirs := make([]string, len(parts))
	base := ""
	if rel, err := filepath.Rel(root, path); root != "" && err == nil && !strings.HasPrefix(rel, "..") {
		base = root
	} else if filepath.IsAbs(path) {
		base = string(filepath.Separator)
		if v := filepath.VolumeName(path); v != "" {
			base = v + string(filepath.Separator)
		}
	}
	at := base
	for i, p := range parts[:max(len(parts)-1, 0)] {
		if p == "" {
			continue
		}
		at = filepath.Join(at, p)
		dirs[i] = at
	}
	return dirs
}

// openCrumbs opens the box of folder dir for the file shown in column ci, or closes it when it
// is the one open.
func (f *Frontend) openCrumbs(ci int, file, dir string) {
	k := &f.crumbs
	if k.open && k.col == ci && k.dir == dir {
		f.closeCrumbs()
		return
	}
	reveal := file
	for reveal != "" && filepath.Dir(reveal) != dir && filepath.Dir(reveal) != reveal {
		reveal = filepath.Dir(reveal)
	}
	// The tree's state stays where it is: list input already sent to it can still arrive.
	k.open, k.col, k.file, k.dir, k.gen = true, ci, file, dir, k.gen+1
	k.sel, k.focus, k.reveal = -1, true, reveal
	k.tree.Open.Clear()
	k.tree.List.Selected = &k.sel
	k.kids, k.dirs = map[string][]string{}, map[string]bool{dir: true}
	k.loading, k.errs = map[string]bool{}, map[string]string{}
	f.crumbsLoad(dir)
}

// closeCrumbs closes the box, dropping what it read; reads still running are dropped by gen.
// The tree's state is kept, as in openCrumbs.
func (f *Frontend) closeCrumbs() {
	k := &f.crumbs
	k.open, k.file, k.dir, k.gen, k.sel, k.reveal = false, "", "", k.gen+1, -1, ""
	k.tree.List.Selected = &k.sel
	k.kids, k.dirs, k.loading, k.errs = nil, nil, nil, nil
}

// crumbsLoad reads a folder of the box in the background, unless it has been.
func (f *Frontend) crumbsLoad(dir string) {
	k := &f.crumbs
	if _, ok := k.kids[dir]; ok || k.loading[dir] {
		return
	}
	k.loading[dir] = true
	gen := k.gen
	go func() {
		kids, dirs, err := readCrumbDir(dir)
		f.do(func() {
			k := &f.crumbs
			if k.gen != gen {
				return
			}
			delete(k.loading, dir)
			if err != nil {
				k.errs[dir] = err.Error()
				return
			}
			for _, d := range dirs {
				k.dirs[d] = true
			}
			k.kids[dir] = kids
		})
	}()
}

// readCrumbDir lists a folder as the explorer does: folders first, then by name, without the
// entries the explorer hides.
func readCrumbDir(dir string) (kids, dirs []string, err error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	type ent struct {
		path string
		dir  bool
	}
	var es []ent
	for _, e := range ents {
		if workspace.Hidden(e.Name()) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			if fi, err := os.Stat(p); err == nil {
				isDir = fi.IsDir()
			}
		}
		es = append(es, ent{p, isDir})
	}
	slices.SortFunc(es, func(a, b ent) int {
		if a.dir != b.dir {
			if a.dir {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(filepath.Base(a.path)), strings.ToLower(filepath.Base(b.path)))
	})
	for _, e := range es {
		kids = append(kids, e.path)
		if e.dir {
			dirs = append(dirs, e.path)
		}
	}
	return kids, dirs, nil
}

// crumbsBox shows the box below the crumb anchor while it is open for folder dir of column ci.
func (f *Frontend) crumbsBox(c *ui.Context, anchor ui.Element) {
	k := &f.crumbs
	if !k.open {
		return
	}
	ui.PopoverBase(c, anchor, &k.open, func(panel ui.Element) {
		p := f.pal
		f.floating(c, panel.Width(300).Radius(8).Padding(4).Gap(0)).Role(ui.RoleDialog).Label(i18n.F("Folder %s", filepath.Base(k.dir)))
		panel.Children(func() {
			kids, read := k.kids[k.dir]
			note := func(s string, col ui.Color) {
				ui.Text(c, s).FontSize(12.5).TextColor(col).Padding(6, 10)
			}
			switch {
			case k.errs[k.dir] != "":
				note(i18n.F("Couldn't read %s: %s", filepath.Base(k.dir), k.errs[k.dir]), p.danger)
			case !read:
				note(i18n.T("Reading…"), p.muted)
			case len(kids) == 0:
				note(i18n.T("Empty folder"), p.muted)
			default:
				f.crumbsTree(c, kids)
			}
		})
	})
	if !k.open {
		f.closeCrumbs()
	}
}

// crumbsMaxH is the tallest the box's tree gets before it scrolls.
const crumbsMaxH = 360

// crumbsTree is the box's tree: a click on a folder opens or closes it, on a file previews it in
// the column; the arrow keys move among the rows and Enter does what a click does.
func (f *Frontend) crumbsTree(c *ui.Context, kids []string) {
	p := f.pal
	k := &f.crumbs
	rows := k.tree.Rows()
	if rows == 0 {
		rows = len(kids)
	}
	// The chosen row is marked as the explorer marks it, not in the accent of a button.
	base := c.Theme()
	quiet := *base
	quiet.Accent, quiet.AccentText, quiet.SurfaceHover = p.active, p.text, p.hover
	c.SetTheme(&quiet)
	var clicked string
	out := ui.Outline(c.Key("crumbs-tree"), &k.tree, kids, func(path string) []string {
		if !k.dirs[path] {
			return nil
		}
		if k.tree.Open.Has(path) {
			f.crumbsLoad(path)
		}
		if kk, ok := k.kids[path]; ok {
			return kk
		}
		return []string{}
	}, func(path string) {
		isDir := k.dirs[path]
		name := filepath.Base(path)
		row := ui.Row(c).Grow(1).Gap(6).AlignItems(ui.Center).Height(treeRowH).Label(name)
		if row.Clicked() {
			clicked = path
		}
		row.Children(func() {
			icon, col := iconFolder, p.accent
			switch {
			case isDir && k.tree.Open.Has(path):
				icon = iconFolderOpen
			case !isDir:
				icon, col = fileIcon(p, name)
			}
			ui.Icon(c, icon).FontSize(14).TextColor(col)
			label := ui.Text(c, name).FontSize(13).SingleLine().Shrink(1).TextColor(p.text)
			if path == k.file {
				label.FontWeight(600)
			}
			if isDir && k.tree.Open.Has(path) && k.errs[path] != "" {
				ui.Spacer(c)
				ui.Icon(c, iconX).FontSize(11).TextColor(p.danger).Tooltip(k.errs[path])
			}
		})
	}).Height(min(float32(rows)*(treeRowH+4)+4, crumbsMaxH)).Gap(0)
	out.Bind(&k.view)
	c.SetTheme(base)
	if k.focus {
		k.focus = false
		k.view.Focus(c)
	}
	if k.reveal != "" {
		for i := range k.tree.Rows() {
			if k.tree.Item(i) == k.reveal {
				k.sel = i
				k.tree.List.ScrollIntoView(i)
				k.reveal = ""
			}
		}
	}
	if clicked == "" && out.Submitted() && k.sel >= 0 && k.sel < k.tree.Rows() {
		clicked = k.tree.Item(k.sel)
	}
	if clicked == "" {
		return
	}
	if !k.dirs[clicked] {
		ci := k.col
		f.closeCrumbs()
		if err := f.ed.PreviewFileIn(ci, clicked); err != nil {
			f.ed.Notify(err.Error())
		}
		f.focusRoot = true
		return
	}
	for i := range k.tree.Rows() {
		if k.tree.Item(i) == clicked {
			k.sel = i
		}
	}
	if k.tree.Open.Has(clicked) {
		k.tree.Open.Remove(clicked)
	} else {
		k.tree.Open.Add(clicked)
	}
}
