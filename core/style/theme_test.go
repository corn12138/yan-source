package style

import (
	"math"
	"testing"
)

func luminance(c Color) float64 {
	r, g, b, _ := c.Parts()
	lin := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

func contrast(a, b Color) float64 {
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// The default theme's text stays readable on every surface it is drawn on: body and secondary
// text at 4.5:1, line numbers, comments and the accent as text at 3:1.
func TestDefaultThemeContrast(t *testing.T) {
	for _, dark := range []bool{false, true} {
		p := ThemeFor("", dark)
		for _, c := range []struct {
			name   string
			fg, bg Color
			min    float64
		}{
			{"text on editor", p.Text, p.Editor, 4.5},
			{"text on chrome", p.Text, p.Chrome, 4.5},
			{"muted on editor", p.Muted, p.Editor, 4.5},
			{"muted on chrome", p.Muted, p.Chrome, 4.5},
			{"muted on selected row", p.Muted, p.Active, 3},
			{"status text", p.StatusFg, p.Status, 4.5},
			{"button text", p.FillText, p.Fill, 4.5},
			{"tooltip text", p.InverseText, p.Inverse, 4.5},
			{"line numbers", p.Gutter, p.Editor, 3},
			{"comments", p.Syntax[Comment-Keyword], p.Editor, 3},
			{"accent as text", p.Accent, p.Editor, 3},
			{"added", p.Added, p.Panel, 3},
			{"removed", p.Removed, p.Panel, 3},
			{"modified", p.Modified, p.Panel, 3},
			{"danger", p.Danger, p.Editor, 3},
		} {
			if got := contrast(c.fg, c.bg); got < c.min {
				t.Errorf("dark=%v %s: %.2f:1, want %.1f:1", dark, c.name, got, c.min)
			}
		}
	}
}
