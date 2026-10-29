package gui

import (
	"fmt"
	"os"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/view"
)

// 看图，照着 Mayoi（cmd/mayoi/viewer.go）：PNG、JPEG、GIF、WebP、BMP 在棋盘格上显示，默认适应窗格；
// ⌘= ⌘- 缩放（⌘ 加滚轮也行），⌘0 回到适应窗格，双击在适应和实际大小之间切换；拖动或滚动平移；
// 文件变了就重新读。SVG 按 ⇧⌘V 显示成图，边打字边更新。

type imageEntry struct {
	bmp     *ui.Bitmap
	svg     *ui.SVG
	w, h    float32
	mod     time.Time
	checked time.Time
	err     string
	loading bool
	src     string // SVG 的文字（上次解析成功的）
}

func (f *Frontend) imageOf(pf *view.PaneFrame) *imageEntry {
	if f.images == nil {
		f.images = map[string]*imageEntry{}
	}
	im := f.images[pf.Path]
	if im == nil {
		im = &imageEntry{}
		f.images[pf.Path] = im
	}
	if pf.SVG != nil {
		if src := string(pf.SVG); src != im.src {
			if s, err := ui.ParseSVG(pf.SVG); err == nil {
				im.svg, im.src, im.err = s, src, ""
				im.w, im.h = 0, 0
			} else if im.svg == nil {
				im.err = i18n.F("Couldn't parse the SVG: %s", err.Error())
			}
		}
		return im
	}
	// 位图：每半秒看一次文件有没有变，变了就在后台重新解码。
	if now := time.Now(); !im.loading && now.Sub(im.checked) > 500*time.Millisecond {
		im.checked = now
		if fi, err := os.Stat(pf.Path); err == nil && !fi.ModTime().Equal(im.mod) {
			im.loading = true
			path, mod := pf.Path, fi.ModTime()
			go func() {
				data, err := os.ReadFile(path)
				var bmp *ui.Bitmap
				if err == nil {
					bmp, err = ui.DecodeBitmap(data)
				}
				f.do(func() {
					im.loading, im.mod = false, mod
					if err != nil {
						im.err = i18n.F("Couldn't read the image: %s", err.Error())
						return
					}
					w, h := bmp.Size()
					im.bmp, im.err, im.w, im.h = bmp, "", float32(w), float32(h)
				})
			}()
		}
	}
	return im
}

// imagePane 是图片标签的窗格。
func (f *Frontend) imagePane(c *ui.Context, pane ui.Element, ci int) {
	pf := &f.frame.Columns[ci].Pane
	p := f.pal
	im := f.imageOf(pf)
	if im.loading && im.bmp == nil {
		c.After(100 * time.Millisecond)
	}
	if f.pans == nil {
		f.pans = map[string][2]float32{}
	}
	path := pf.Path
	pane.Background(p.editor).HandleInput(func(ev ui.InputEvent) bool {
		switch ev.Kind {
		case ui.InputPointerDown:
			f.ed.FocusColumn(ci)
			if ev.Clicks == 2 {
				f.ed.ToggleImageFit(ci)
				f.pans[path] = [2]float32{}
			}
			f.panFrom = [2]float32{ev.X, ev.Y}
			return true
		case ui.InputPointerMove:
			if ev.Button == 0 {
				d := f.pans[path]
				f.pans[path] = [2]float32{d[0] + ev.X - f.panFrom[0], d[1] + ev.Y - f.panFrom[1]}
				f.panFrom = [2]float32{ev.X, ev.Y}
				return true
			}
		case ui.InputPointerUp:
			return true
		case ui.InputScroll:
			if ev.Mods&ui.Super != 0 { // ⌘ 加滚轮：缩放
				z := pf.Zoom
				if z == 0 {
					z = float64(f.fitScale(im, f.cols[ci].pane.Bounds(c)))
				}
				f.ed.SetImageZoom(ci, float64(z)*(1-float64(ev.DY)/200))
				return true
			}
			d := f.pans[path]
			f.pans[path] = [2]float32{d[0] - ev.DX, d[1] - ev.DY}
			return true
		}
		return false
	}).Draw(func(pt *ui.Painter, r ui.Rect) {
		if im.err != "" {
			pt.Text(r.X+24, r.Y+24, im.err, 13, p.danger)
			return
		}
		w, h := im.w, im.h
		if im.svg != nil && w == 0 {
			w, h = 256, 256
			if sw, sh := im.svg.Size(); sw > 0 && sh > 0 {
				w, h = sw, sh
			}
			im.w, im.h = w, h
		}
		if w == 0 || h == 0 {
			return
		}
		s := float32(pf.Zoom)
		if s == 0 {
			s = f.fitScale(im, r)
		}
		pan := f.pans[path]
		box := ui.Rect{X: r.X + (r.W-w*s)/2 + pan[0], Y: r.Y + (r.H-h*s)/2 + pan[1], W: w * s, H: h * s}
		pt.Clip(r, 0, func() {
			f.checkerboard(pt, box)
			if im.bmp != nil {
				pt.Image(im.bmp, box, ui.FillBox)
			} else if im.svg != nil {
				pt.Image(im.svg, box, ui.FillBox)
			}
		})
		label := fmt.Sprintf("%.0f × %.0f · %.0f%%", w, h, s*100)
		pt.Text(r.X+r.W-float32(len(label))*6.5-14, r.Y+r.H-22, label, 11, p.muted)
	})
}

// fitScale 是适应窗格的缩放：放得下就是实际大小，放不下就缩小。
func (f *Frontend) fitScale(im *imageEntry, r ui.Rect) float32 {
	if im.w == 0 || im.h == 0 || r.W <= 0 || r.H <= 0 {
		return 1
	}
	return min(1, (r.W-32)/im.w, (r.H-32)/im.h)
}

// checkerboard 在图片底下画棋盘格，透明的地方看得出来。
func (f *Frontend) checkerboard(pt *ui.Painter, box ui.Rect) {
	p := f.pal
	a, b := p.editor.Mix(p.text, 0.06), p.editor.Mix(p.text, 0.12)
	pt.Fill(box, a, 0)
	const tile = 10
	for y := float32(0); y < box.H; y += tile {
		for x := float32(0); x < box.W; x += tile {
			if (int(x/tile)+int(y/tile))%2 == 0 {
				pt.Fill(ui.Rect{X: box.X + x, Y: box.Y + y, W: min(tile, box.W-x), H: min(tile, box.H-y)}, b, 0)
			}
		}
	}
}
