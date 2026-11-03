package gui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/keymap"
)

type fontZoomGesture struct {
	delta float32
	last  time.Time
}

func (f *Frontend) zoomFontOnScroll(ci int, ev ui.InputEvent) bool {
	st := &f.cols[ci]
	by, handled := st.fontZoom.step(ev, f.Profile(), time.Now())
	if handled {
		st.scroll, st.scrollX = 0, 0
		st.split.wheel = [2]float32{}
		if by != 0 {
			f.ed.ZoomFont(float64(by))
		}
	}
	return handled
}

func (z *fontZoomGesture) step(ev ui.InputEvent, profile keymap.Profile, now time.Time) (int, bool) {
	modifier := ui.Ctrl
	if profile.MacKeys() {
		modifier = ui.Super
	}
	if ev.Mods != modifier {
		*z = fontZoomGesture{}
		return 0, false
	}
	if ev.DY == 0 || max(ev.DX, -ev.DX) > max(ev.DY, -ev.DY) {
		*z = fontZoomGesture{}
		return 0, true
	}
	if !ev.Precise {
		*z = fontZoomGesture{}
		if ev.DY < 0 {
			return 1, true
		}
		return -1, true
	}
	// 触控板每 40 DIP 调一级，灵敏度不随字号改变；反向或停顿后不沿用上次的余量。
	// 修饰键滚动即使还不足一级也要消费掉，避免字号和文件位置一起动。
	if now.Sub(z.last) > 250*time.Millisecond || z.delta*ev.DY < 0 {
		z.delta = 0
	}
	z.last = now
	z.delta += ev.DY
	steps := int(z.delta / 40)
	z.delta -= float32(steps) * 40
	return -steps, true
}
