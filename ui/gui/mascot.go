// Adapted from Mayoi cmd/mayoi/mascot.go (MIT); see THIRD_PARTY_NOTICES.md.
package gui

import (
	"github.com/egoist/mygo/ui"
	"time"
)

const (
	mascotPeriod = 4200 * time.Millisecond // between blinks
	mascotBlink  = 140 * time.Millisecond
	mascotBobAt  = 2100 * time.Millisecond // into every third period
	mascotBob    = 260 * time.Millisecond
	mascotHop    = 130 * time.Millisecond // a step of the hop
)

// mascotState is how the character is: moving whether the last paint
// asked for a next, and hopping whether it hopped.
type mascotState struct {
	moving, hopping bool
}

// mascotPose is a moment of the character's: the eyes shut, the body
// lifted by a part of its headroom, and which step of the hop.
type mascotPose struct {
	blink bool
	lift  float32
	step  int
}

// poseAt returns the character's pose at a moment, and how long it holds.
func poseAt(now time.Time, busy bool) (mascotPose, time.Duration) {
	ms := now.UnixMilli()
	if busy {
		step := mascotHop.Milliseconds()
		k := ms / step
		return mascotPose{lift: float32(k % 2), step: int(k % 2)}, time.Duration(step-ms%step) * time.Millisecond
	}
	period := mascotPeriod.Milliseconds()
	in, n := ms%period, ms/period
	// The moments the pose changes, in a period.
	marks := []int64{mascotBlink.Milliseconds(), period}
	var p mascotPose
	switch {
	case in < mascotBlink.Milliseconds():
		p.blink = true
	case n%3 == 0:
		at, end := mascotBobAt.Milliseconds(), (mascotBobAt + mascotBob).Milliseconds()
		marks = []int64{mascotBlink.Milliseconds(), at, end, period}
		if in >= at && in < end {
			p.lift = 0.5
		}
	}
	for _, m := range marks {
		if m > in {
			return p, time.Duration(m-in) * time.Millisecond
		}
	}
	return p, time.Duration(period-in) * time.Millisecond
}

func (f *Frontend) hasClawd() bool { return f.opts.Theme == "claude" && f.ed.ThemeEffects() }

func (f *Frontend) clawdView(c *ui.Context, label string, cell float32, bg ui.Color) {
	w, h := clawdSize(cell)
	width, height := w, h
	if cell == 1 {
		width, height = w+12, statusBarH
	}
	tip := "Claude · Clawd"
	if activity := f.ed.BackgroundActivity(); activity != "" {
		tip += " · " + activity
	}
	reduce := c.Preferences().ReduceMotion
	ui.Box(c.Key(label)).Size(width, height).Shrink(0).Label(label).Tooltip(tip).Draw(func(pt *ui.Painter, r ui.Rect) {
		still := reduce || f.blurred
		busy := f.ed.BackgroundActivity() != ""
		f.mascot.moving, f.mascot.hopping = !still, !still && busy
		var pose mascotPose
		if !still {
			var hold time.Duration
			pose, hold = poseAt(pt.Now(), busy)
			// Paint-only scheduling avoids rebuilding the editor between poses.
			pt.After(hold)
		}
		f.drawClawd(pt, r, cell, bg, pose)
	})
}
