package gui

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/workspace"
)

// A project's square shows, in order: the mark chosen for it, the picture chosen for it, the
// logo found in the folder, its initials. The logo is looked for once per folder, off the UI
// thread; decoding reads only regular files of bounded size.

// projectLogo is what the rail found for one folder.
type projectLogo struct {
	root    string
	src     ui.ImageSource // nil when the folder has none that decodes
	looking bool
}

// maxLogoBytes bounds a logo read for the rail.
const maxLogoBytes = 4 << 20

// logoFor returns the logo of the project shown by pf, starting the search the first time.
func (g *group) logoFor(pf *Frontend, root string) ui.ImageSource {
	if root == "" {
		return nil
	}
	l := &pf.logo
	if l.root != root {
		*l = projectLogo{root: root, looking: true}
		go func() {
			var src ui.ImageSource
			for _, p := range workspace.LogoFiles(root) {
				if src = decodeProjectImage(p); src != nil {
					break
				}
			}
			pf.do(func() {
				if pf.logo.root == root {
					pf.logo.src, pf.logo.looking = src, false
				}
			})
		}()
	}
	return l.src
}

// iconImage returns the decoded picture chosen for a project, cached by file.
func (g *group) iconImage(path string) ui.ImageSource {
	if path == "" {
		return nil
	}
	if g.icons == nil {
		g.icons = map[string]ui.ImageSource{}
	}
	src, ok := g.icons[path]
	if !ok {
		src = decodeProjectImage(path)
		g.icons[path] = src
	}
	return src
}

// decodeProjectImage decodes a picture for a square, or returns nil.
func decodeProjectImage(path string) ui.ImageSource {
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxLogoBytes {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".svg":
		s, err := ui.ParseSVG(data)
		if err != nil {
			return nil
		}
		if w, h := s.Size(); w <= 0 || h <= 0 {
			return nil
		}
		return s
	case ".ico":
		data = icoPNG(data)
		if data == nil {
			return nil
		}
	}
	bm, err := ui.DecodeBitmap(data)
	if err != nil {
		return nil
	}
	if w, h := bm.Size(); w <= 0 || h <= 0 {
		return nil
	}
	return bm
}

// icoPNG returns the largest PNG image an .ico file holds, or nil: modern favicons store PNG,
// and older bitmap-only icons fall back to the next logo or the initials.
func icoPNG(data []byte) []byte {
	if len(data) < 6 || binary.LittleEndian.Uint16(data[0:]) != 0 || binary.LittleEndian.Uint16(data[2:]) != 1 {
		return nil
	}
	n := int(binary.LittleEndian.Uint16(data[4:]))
	var best []byte
	bestW := -1
	for i := range n {
		e := 6 + 16*i
		if e+16 > len(data) {
			break
		}
		w := int(data[e])
		if w == 0 {
			w = 256
		}
		size := int(binary.LittleEndian.Uint32(data[e+8:]))
		off := int(binary.LittleEndian.Uint32(data[e+12:]))
		if off < 0 || size <= 0 || off+size > len(data) {
			continue
		}
		img := data[off : off+size]
		if bytes.HasPrefix(img, []byte("\x89PNG\r\n\x1a\n")) && w > bestW {
			best, bestW = img, w
		}
	}
	return best
}

// chooseProjectIcon asks for a picture to show in the square of the folder root.
func (g *group) chooseProjectIcon(pf *Frontend, root string) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: g.win, Title: i18n.T("Choose Project Icon"), ButtonLabel: i18n.T("Selection"),
			Filters: []mygo.FileFilter{{Name: i18n.T("Images"), Extensions: []string{"png", "jpg", "jpeg", "webp", "svg", "ico"}}},
		})
		if err == nil && len(paths) > 0 {
			pf.do(func() { g.setProjectIcon(pf, root, paths[0]) })
		}
	}()
}

// setProjectIcon shows the picture at path in the square of the folder root, if it decodes.
func (g *group) setProjectIcon(pf *Frontend, root, path string) {
	if decodeProjectImage(path) == nil {
		pf.ed.Notify(i18n.F("%s is not an image that can be shown", filepath.Base(path)))
		return
	}
	if err := pf.ed.SetProjectIcon(root, path); err != nil {
		pf.ed.Notify(err.Error())
		return
	}
	g.icons = nil // the new copy decodes next frame
}
