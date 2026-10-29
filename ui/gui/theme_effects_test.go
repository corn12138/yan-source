package gui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/corn12138/yan-source/core/config"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestClawdLifecycle(t *testing.T) {
	for _, appearance := range []string{"light", "dark"} {
		t.Run(appearance, func(t *testing.T) {
			h := newHarness(t, map[string]string{"a.go": "package a\n"}, Options{Theme: "claude", Appearance: appearance})
			defer h.ed.Close()
			for _, label := range []string{"welcome-clawd", "status-clawd"} {
				if _, ok := h.tt.Find(label); !ok {
					t.Fatalf("missing %s", label)
				}
			}
			if !h.f.mascot.moving || h.f.mascot.hopping {
				t.Fatalf("idle: %+v", h.f.mascot)
			}
			h.tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			if h.f.mascot.moving {
				t.Fatal("reduced motion still schedules frames")
			}
			h.shot("claude-" + appearance)
			h.tt.SetPreferences(ui.Preferences{})
			h.f.focusChanged(false)
			h.settle()
			if h.f.mascot.moving {
				t.Fatal("blurred mascot moves")
			}
			h.f.focusChanged(true)
			h.settle()
			if !h.f.mascot.moving {
				t.Fatal("focus did not resume")
			}
			h.open("a.go")
			if _, ok := h.tt.Find("welcome-clawd"); ok {
				t.Fatal("welcome mascot over file")
			}
			if _, ok := h.tt.Find("status-clawd"); !ok {
				t.Fatal("status mascot missing with file open")
			}
			h.tt.SetSize(480, 600)
			h.settle()
			if got := strings.Join(h.tt.Texts(), "|"); !strings.Contains(got, "行 1") {
				t.Fatalf("narrow status: %s", got)
			}
			h.ed.Run("yan.theme.mayoi")
			h.settle()
			if h.f.mascot.moving {
				t.Fatal("mascot keeps animating after theme change")
			}
			if _, ok := h.tt.Find("status-clawd"); ok {
				t.Fatal("mascot remained in Mayoi")
			}
		})
	}
}

func TestMatrixRainLifecycle(t *testing.T) {
	h := newHarness(t, map[string]string{"a.go": "package a\n"}, Options{Theme: "matrix"})
	defer h.ed.Close()
	if !h.f.rain.moving {
		t.Fatal("no rain on welcome")
	}
	h.f.focusChanged(false)
	h.settle()
	step := h.f.rain.step
	h.tt.Advance(rainStep)
	h.tt.Frame()
	if h.f.rain.moving || step != h.f.rain.step {
		t.Fatal("blurred rain advances")
	}
	h.f.focusChanged(true)
	h.tt.Advance(rainStep)
	h.settle()
	if !h.f.rain.moving || step == h.f.rain.step {
		t.Fatal("rain did not resume")
	}
	h.tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	step = h.f.rain.step
	h.tt.Advance(rainStep)
	h.tt.Frame()
	if h.f.rain.moving || step != h.f.rain.step {
		t.Fatal("reduced-motion rain advances")
	}
	h.shot("matrix-welcome")
	h.tt.SetPreferences(ui.Preferences{})
	h.open("a.go")
	if h.f.rain.moving {
		t.Fatal("rain over a file")
	}
	h.ed.Run("workbench.action.closeActiveEditor")
	h.settle()
	if !h.f.rain.moving {
		t.Fatal("rain did not return to welcome")
	}
	h.ed.Run("yan.theme.claude")
	h.settle()
	if h.f.rain.moving {
		t.Fatal("rain remained after theme switch")
	}
}

func TestThemeEffectsMenu(t *testing.T) {
	for _, theme := range []string{"claude", "matrix"} {
		t.Run(theme, func(t *testing.T) {
			h := newHarness(t, nil, Options{Theme: theme})
			defer h.ed.Close()
			statePath := filepath.Join(t.TempDir(), "state.json")
			h.ed.SetState(config.LoadState(statePath))
			var find func([]*mygo.MenuItem) *mygo.MenuItem
			find = func(items []*mygo.MenuItem) *mygo.MenuItem {
				for _, it := range items {
					if it.Label == "装饰效果" {
						return it
					}
					if v := find(it.Submenu); v != nil {
						return v
					}
				}
				return nil
			}
			item := find(h.f.menu().Items())
			if item == nil || !item.Checked || item.Type != mygo.MenuItemCheckbox {
				t.Fatal("missing checked menu option")
			}
			item.Click(item, nil)
			h.settle()
			if h.ed.ThemeEffects() || h.f.rain.moving || h.f.mascot.moving {
				t.Fatal("disabled effects still active")
			}
			if _, ok := h.tt.Find("status-clawd"); ok {
				t.Fatal("disabled mascot still present")
			}
			if _, ok := h.tt.Find("砚"); !ok {
				t.Fatal("disabled effects removed welcome logo")
			}
			if find(h.f.menu().Items()).Checked {
				t.Fatal("menu check not updated")
			}
			cfg := config.Default()
			config.LoadState(statePath).Apply(cfg)
			if cfg.ThemeEffects {
				t.Fatal("disabled choice not persisted")
			}
			h.ed.Run("yan.toggleThemeEffects")
			h.settle()
			if !h.f.mascot.moving && !h.f.rain.moving {
				t.Fatal("enable did not restore theme effect")
			}
		})
	}
}

// Adapted from Mayoi's pose timing regression (MIT).
func TestMascotPoses(t *testing.T) {
	cases := []struct {
		ms   int64
		busy bool
		pose mascotPose
		hold time.Duration
	}{
		{10, false, mascotPose{blink: true}, 130 * time.Millisecond},
		{4700, false, mascotPose{}, 3700 * time.Millisecond},
		{2110, false, mascotPose{lift: .5}, 250 * time.Millisecond},
		{500, false, mascotPose{}, 1600 * time.Millisecond},
		{0, true, mascotPose{}, mascotHop},
		{130, true, mascotPose{lift: 1, step: 1}, mascotHop},
	}
	for _, tc := range cases {
		p, d := poseAt(time.UnixMilli(tc.ms), tc.busy)
		if p != tc.pose || d != tc.hold {
			t.Errorf("at %d busy %v: %+v for %v", tc.ms, tc.busy, p, d)
		}
	}
}
