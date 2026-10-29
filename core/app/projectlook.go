package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/clipperhouse/uax29/v2/graphemes"

	"github.com/corn12138/yan-source/core/config"
)

// A project's square on the rail can carry a mark and colour of the user's choosing, kept per
// folder in state.json on this machine, as Mayoi lets a project be told apart at a glance.

// projectColors are the rail's colour lanes, by number from 1, as the choice lists them.
var projectColors = []string{"蓝", "绿", "橙", "紫", "红", "青"}

// ProjectLook returns the mark and colour chosen for the folder root: "" and 0 when none is.
func (e *Editor) ProjectLook(root string) (mark string, color int) {
	if e.state == nil {
		return "", 0
	}
	l := e.state.ProjectLooks[root]
	return l.Mark, l.Color
}

// customizeProject asks for this project's mark, then its colour.
func (e *Editor) customizeProject() error {
	root := e.root
	if root == "" {
		return fmt.Errorf("没有打开文件夹")
	}
	mark, _ := e.ProjectLook(root)
	e.prompt("项目标识：最多两个字或一个表情；留空用名字的缩写", mark, func(s string) string {
		if n := graphemeCount(strings.TrimSpace(s)); n > 2 {
			return "最多两个字"
		}
		return "用「" + strings.TrimSpace(s) + "」作为标识"
	}, func(e *Editor, s string) error {
		s = strings.TrimSpace(s)
		if graphemeCount(s) > 2 {
			return fmt.Errorf("最多两个字或一个表情")
		}
		items := []ovItem{{label: "按文件夹自动", run: func(e *Editor) error { return e.setProjectLook(root, s, 0) }}}
		for i, name := range projectColors {
			items = append(items, ovItem{label: name, run: func(e *Editor) error { return e.setProjectLook(root, s, i+1) }})
		}
		e.choose("项目颜色", items...)
		return nil
	})
	return nil
}

func (e *Editor) setProjectLook(root, mark string, color int) error {
	if e.state == nil {
		return nil
	}
	if e.state.ProjectLooks == nil {
		e.state.ProjectLooks = map[string]config.ProjectLook{}
	}
	l := e.state.ProjectLooks[root]
	l.Mark, l.Color = mark, min(max(color, 0), len(projectColors))
	e.putLook(root, l)
	return nil
}

// putLook stores a project's look, forgetting one that sets nothing, and saves it.
func (e *Editor) putLook(root string, l config.ProjectLook) {
	if l == (config.ProjectLook{}) {
		delete(e.state.ProjectLooks, root)
	} else {
		e.state.ProjectLooks[root] = l
	}
	e.saveState()
	e.settingsGen++
}

// iconExts are the pictures a square may show: those the window decodes.
var iconExts = []string{".png", ".jpg", ".jpeg", ".webp", ".svg", ".ico"}

// maxIconBytes bounds a chosen picture: a square needs no more.
const maxIconBytes = 4 << 20

// IsIconFile reports whether path names a picture a project's square may show.
func IsIconFile(path string) bool {
	return slices.Contains(iconExts, strings.ToLower(filepath.Ext(path)))
}

// iconsDir is where chosen pictures are kept, beside the user's configuration.
func iconsDir() string { return filepath.Join(filepath.Dir(config.UserPath()), "icons") }

// ProjectIcon returns the picture chosen for the folder root, "" when none is.
func (e *Editor) ProjectIcon(root string) string {
	if e.state == nil || e.state.ProjectLooks[root].Icon == "" {
		return ""
	}
	return filepath.Join(iconsDir(), e.state.ProjectLooks[root].Icon)
}

// SetProjectIcon copies the picture at src beside the configuration, named by its bytes so the
// same picture is kept once, and shows it in the square of the folder root.
func (e *Editor) SetProjectIcon(root, src string) error {
	if e.state == nil || root == "" {
		return nil
	}
	if !IsIconFile(src) {
		return fmt.Errorf("%s 不是图片：可用 %s", filepath.Base(src), strings.Join(iconExts, " "))
	}
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() || fi.Size() > maxIconBytes {
		return fmt.Errorf("%s 不是 4 MB 以内的图片文件", filepath.Base(src))
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:8]) + strings.ToLower(filepath.Ext(src))
	if err := os.MkdirAll(iconsDir(), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(iconsDir(), name), data, 0o644); err != nil {
		return err
	}
	if e.state.ProjectLooks == nil {
		e.state.ProjectLooks = map[string]config.ProjectLook{}
	}
	l := e.state.ProjectLooks[root]
	old := l.Icon
	l.Icon = name
	e.putLook(root, l)
	e.forgetIcon(old)
	return nil
}

// ClearProjectIcon shows the folder's logo or initials again.
func (e *Editor) ClearProjectIcon(root string) {
	if e.state == nil {
		return
	}
	l := e.state.ProjectLooks[root]
	old := l.Icon
	l.Icon = ""
	e.putLook(root, l)
	e.forgetIcon(old)
}

// forgetIcon deletes a copied picture once no project shows it.
func (e *Editor) forgetIcon(name string) {
	if name == "" {
		return
	}
	for _, l := range e.state.ProjectLooks {
		if l.Icon == name {
			return
		}
	}
	os.Remove(filepath.Join(iconsDir(), name))
}

// graphemeCount counts what a reader sees as characters: an emoji with its modifiers is one.
func graphemeCount(s string) int {
	n := 0
	for it := graphemes.FromString(s); it.Next(); {
		n++
	}
	return n
}
