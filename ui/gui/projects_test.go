package gui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/config"
)

func TestProjectInitials(t *testing.T) {
	for name, want := range map[string]string{
		"editors": "Ed", "my-app": "MA", "go_web server": "GW", "砚": "砚", "中文项目": "中", ".dotfiles": "Do", "未打开": "未",
		"mayoiEditor": "ME", "x": "X",
	} {
		if got := projectInitials(name); got != want {
			t.Errorf("%q: %q, want %q", name, got, want)
		}
	}
}

// Clashing labels are told apart by the first letter that differs; same names stay the same.
func TestRailLabels(t *testing.T) {
	got := railLabels([]string{"Downloads", "Documents", "editors", "smart-emergency-portal", "smart-entry", "智慧应急", "智能平台", "editors"})
	want := []string{"Dw", "Dc", "Ed", "Sm", "Sn", "智慧", "智能", "Ed"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("label %d = %q, want %q (all %v)", i, got[i], want[i], got)
		}
	}
}

// 一个窗口里两个项目：左边一列方块，点方块换项目，各自的标签留着；关掉一个，剩下的接着显示。
func TestProjectTabs(t *testing.T) {
	one, other := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(one, "a.txt"), []byte("a\n"), 0o644)
	os.WriteFile(filepath.Join(other, "b.txt"), []byte("b\n"), 0o644)
	os.WriteFile(filepath.Join(one, "favicon.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><circle cx="8" cy="8" r="7" fill="#e5533d"/></svg>`), 0o644)
	saved := makeWindow
	defer func() { makeWindow = saved }()
	SetWindowMaker(func() (*app.Editor, *Frontend) {
		f := New(Options{})
		cfg := config.Default()
		off := false
		cfg.AutoSave = &off
		ed, _ := app.New(cfg, f.Layouter(), f.Host(), f.Profile())
		f.ed = ed
		return ed, f
	})
	ed, a := makeWindow()
	if err := ed.OpenFolder(one); err != nil {
		t.Fatal(err)
	}
	ed.OpenFile(filepath.Join(one, "a.txt"))
	g := &group{}
	g.attach(a, ed)
	g.projects = []*Frontend{a}
	g.add(other)
	if len(g.projects) != 2 || g.active != 1 || g.cur().ed.Root() != other {
		t.Fatalf("after add: %d projects, active %d", len(g.projects), g.active)
	}
	b := g.cur()
	b.ed.OpenFile(filepath.Join(other, "b.txt"))
	g.add(other) // 已经开着：换过去，不再加
	if len(g.projects) != 2 {
		t.Fatalf("added the same folder twice")
	}

	tt := ui.NewTester(g.view, 1200, 780)
	tt.Frame()
	// The first folder's favicon shows in its square once found in the background.
	for end := time.Now().Add(3 * time.Second); a.logo.src == nil && time.Now().Before(end); {
		// The group's own loop takes the wake-ups and, without a window, drops them.
		time.Sleep(20 * time.Millisecond)
		a.ed.RunPending()
		tt.Frame()
	}
	if a.logo.src == nil {
		t.Fatalf("the favicon was not found: %+v", a.logo)
	}
	tt.Frame()
	if dir := os.Getenv("SHOTS"); dir != "" {
		out, _ := os.Create(filepath.Join(dir, "rail-logo.png"))
		png.Encode(out, tt.Image())
		out.Close()
	}
	if err := tt.Click("项目 " + filepath.Base(one)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if g.cur() != a {
		t.Fatalf("click did not switch: active %d", g.active)
	}
	if a.frame.Columns[0].Tabs[0].Name != "a.txt" {
		t.Fatalf("project a shows %q", a.frame.Columns[0].Tabs[0].Name)
	}
	host{a}.StepProject(1)
	tt.Frame()
	if g.cur() != b || b.frame.Columns[0].Tabs[0].Name != "b.txt" {
		t.Fatalf("next project: active %d", g.active)
	}
	host{a}.StepProject(1) // 到头了回到第一个
	if g.cur() != a {
		t.Fatalf("wrap: active %d", g.active)
	}
	g.close(1)
	tt.Frame()
	if len(g.projects) != 1 || g.cur() != a || b.done != nil {
		t.Fatalf("after close: %d projects", len(g.projects))
	}
}

// 拖方块排序：挪的是那个项目，现在显示的还是原来那个；看不见的项目响铃时做记号，换过去就清掉。
func TestProjectReorderAndAttention(t *testing.T) {
	saved := makeWindow
	defer func() { makeWindow = saved }()
	SetWindowMaker(func() (*app.Editor, *Frontend) {
		f := New(Options{})
		ed, _ := app.New(config.Default(), f.Layouter(), f.Host(), f.Profile())
		f.ed = ed
		return ed, f
	})
	dirs := []string{t.TempDir(), t.TempDir(), t.TempDir()}
	ed, a := makeWindow()
	ed.OpenFolder(dirs[0])
	g := &group{}
	g.attach(a, ed)
	g.projects = []*Frontend{a}
	g.add(dirs[1])
	g.add(dirs[2])
	b, c := g.projects[1], g.projects[2]
	g.show(1) // 显示 b
	g.move(c, 0)
	if g.projects[0] != c || g.projects[1] != a || g.cur() != b {
		t.Fatalf("after move: %v, active %d", g.projects, g.active)
	}
	c.markAttention("终端响铃：zsh")
	b.markAttention("看得见的项目不做记号")
	if c.attention == "" || b.attention != "" {
		t.Fatalf("attention c %q, b %q", c.attention, b.attention)
	}
	g.show(0)
	if c.attention != "" {
		t.Fatalf("attention kept after showing: %q", c.attention)
	}
}
