package gui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const paletteFadeDuration = 160 * time.Millisecond

type paletteFade struct {
	from, to *palette
	at       time.Time
}

// Interrupted transitions start from the displayed colors, not the old target.
// Reduced motion also ends an in-flight fade immediately without scheduling more frames.
func (f *paletteFade) step(now time.Time, to *palette, reduce bool) (*palette, bool) {
	if reduce || f.to == nil {
		f.from, f.to = nil, to
		return to, false
	}
	if f.to != to {
		f.from, f.at = f.atTime(now), now
		f.to = to
	}
	p := f.atTime(now)
	if p == to {
		f.from = nil
	}
	return p, p != to
}

func (f *paletteFade) atTime(now time.Time) *palette {
	k := float32(now.Sub(f.at)) / float32(paletteFadeDuration)
	if f.from == nil || k >= 1 || k < 0 {
		return f.to
	}
	return mixPalette(f.from, f.to, ui.EaseInOut(k))
}

func mixPalette(from, to *palette, k float32) *palette {
	p := *to
	p.editor = from.editor.Mix(to.editor, k)
	p.chrome = from.chrome.Mix(to.chrome, k)
	p.panel = from.panel.Mix(to.panel, k)
	p.border = from.border.Mix(to.border, k)
	p.hover = from.hover.Mix(to.hover, k)
	p.active = from.active.Mix(to.active, k)
	p.text = from.text.Mix(to.text, k)
	p.muted = from.muted.Mix(to.muted, k)
	p.faint = from.faint.Mix(to.faint, k)
	p.accent = from.accent.Mix(to.accent, k)
	p.accentDim = from.accentDim.Mix(to.accentDim, k)
	p.fill = from.fill.Mix(to.fill, k)
	p.fillHover = from.fillHover.Mix(to.fillHover, k)
	p.fillPressed = from.fillPressed.Mix(to.fillPressed, k)
	p.fillText = from.fillText.Mix(to.fillText, k)
	p.surface = from.surface.Mix(to.surface, k)
	p.surfaceHover = from.surfaceHover.Mix(to.surfaceHover, k)
	p.surfacePressed = from.surfacePressed.Mix(to.surfacePressed, k)
	p.scrollbar = from.scrollbar.Mix(to.scrollbar, k)
	p.inverse = from.inverse.Mix(to.inverse, k)
	p.inverseText = from.inverseText.Mix(to.inverseText, k)
	p.line = from.line.Mix(to.line, k)
	p.selection = from.selection.Mix(to.selection, k)
	p.selected = from.selected.Mix(to.selected, k)
	p.selectedDim = from.selectedDim.Mix(to.selectedDim, k)
	p.gutter = from.gutter.Mix(to.gutter, k)
	p.gutterNow = from.gutterNow.Mix(to.gutterNow, k)
	p.status = from.status.Mix(to.status, k)
	p.statusFg = from.statusFg.Mix(to.statusFg, k)
	p.modified = from.modified.Mix(to.modified, k)
	p.danger = from.danger.Mix(to.danger, k)
	p.match = from.match.Mix(to.match, k)
	p.matchNow = from.matchNow.Mix(to.matchNow, k)
	p.matchRing = from.matchRing.Mix(to.matchRing, k)
	p.guide = from.guide.Mix(to.guide, k)
	p.guideNow = from.guideNow.Mix(to.guideNow, k)
	p.bracket = from.bracket.Mix(to.bracket, k)
	p.word = from.word.Mix(to.word, k)
	p.shadow = from.shadow.Mix(to.shadow, k)
	p.added = from.added.Mix(to.added, k)
	p.changed = from.changed.Mix(to.changed, k)
	p.removed = from.removed.Mix(to.removed, k)
	p.caret = from.caret.Mix(to.caret, k)
	for i := range p.fg {
		p.fg[i] = from.fg[i].Mix(to.fg[i], k)
		if from.hasBg[i] && to.hasBg[i] {
			p.bg[i] = from.bg[i].Mix(to.bg[i], k)
		}
	}
	return &p
}
