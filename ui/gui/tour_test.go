package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/config"
)

// tourHarness is a window with its own state.json, as the app has, in a throwaway config.
func tourHarness(t *testing.T, opts Options) *harness {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := newHarness(t, sample, opts)
	h.ed.SetState(config.LoadState(filepath.Join(t.TempDir(), "state.json")))
	return h
}

// The first launch offers the guide in a corner; declining hides it for good.
func TestGuideOfferCard(t *testing.T) {
	h := tourHarness(t, Options{})
	h.settle()
	if h.tt.HasText("第一次用砚？") {
		t.Fatal("offered without the app asking")
	}
	h.ed.OfferGuide()
	h.settle()
	if !h.tt.HasText("第一次用砚？") {
		t.Fatal("no offer")
	}
	if err := h.tt.Click("不用了"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if h.tt.HasText("第一次用砚？") || h.f.frame.Guide.Visible {
		t.Fatal("the offer stayed")
	}
	h.ed.OfferGuide()
	h.settle()
	if h.f.frame.Guide.Visible {
		t.Fatal("offered again")
	}
}

// The step says how to do it — key, palette name and menu path, read from the menu bar — and
// a feature the build's release lacks is badged in the guide, the palette and the menus.
func TestGuideStepHowAndUnreleasedBadges(t *testing.T) {
	defer app.PretendRelease("0.1.0-alpha.28")()
	h := tourHarness(t, Options{})
	h.ed.Run("yan.guide.open")
	h.settle()
	if err := h.tt.Click("课程 最大化编辑区"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	texts := strings.Join(h.tt.Texts(), "|")
	for _, want := range []string{"命令面板：「向右拆分编辑器」", "菜单：", "未发布"} {
		if !strings.Contains(texts, want) {
			t.Fatalf("missing %q in %s", want, texts)
		}
	}
	if s := h.f.frame.Sidebar.Learn.Steps; s[0].Unreleased || !s[1].Unreleased {
		t.Fatalf("badges %+v", s)
	}
	if got := h.f.menuPath("workbench.action.toggleMaximizeEditorGroup"); got == "" || strings.Contains(got, unreleasedSuffix()) {
		t.Fatalf("menu path %q", got)
	}
	labels := map[string]string{}
	menuCommands(h.f.menu().Items(), "", labels)
	if !strings.HasSuffix(labels["workbench.action.toggleMaximizeEditorGroup"], unreleasedSuffix()) {
		t.Fatal("the menu item is not marked unreleased")
	}
	// The real steps, from the menu bar's commands: split, maximize, restore.
	h.ed.Run("workbench.action.splitEditor")
	h.ed.Run("workbench.action.toggleMaximizeEditorGroup")
	h.settle()
	if !h.f.frame.Maximized || !strings.Contains(h.f.frame.Status.Message, "下一步：还原") {
		t.Fatalf("maximized: %v, status %q", h.f.frame.Maximized, h.f.frame.Status.Message)
	}
	h.ed.Run("workbench.action.toggleMaximizeEditorGroup")
	h.settle()
	if s := h.f.frame.Sidebar.Learn.Steps; !s[len(s)-1].Done {
		t.Fatalf("lesson not done: %+v", s)
	}
}

// The merge lesson opens its practice repository in the window and the guide offers the way
// back to the project.
func TestGuideMergeLessonWindow(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("没有 git")
	}
	h := tourHarness(t, Options{})
	h.ed.Run("yan.guide.open")
	h.settle()
	if err := h.tt.Click("课程 三方合并"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if !h.tt.HasText("练习文件夹 · 回到 " + filepath.Base(h.root)) {
		t.Fatalf("no way back: %v", h.tt.Texts())
	}
	if err := h.tt.Click("回到 " + filepath.Base(h.root)); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if h.f.frame.Sidebar.Learn.Home != "" || !strings.HasPrefix(h.f.frame.Title, filepath.Base(h.root)) {
		t.Fatalf("still in practice: %q", h.f.frame.Title)
	}
}

// TestTourShots renders the guide's offer, a lesson with an unreleased step, the merge lesson
// in its practice repository and the palette's badge, light and dark, at the compact size.
// The build pretends to be the alpha.28 release so the badges show.
func TestTourShots(t *testing.T) {
	if os.Getenv("SHOTS") == "" {
		t.Skip("SHOTS=目录 时才截图")
	}
	defer app.PretendRelease("0.1.0-alpha.28")()
	for _, appearance := range []string{"light", "dark"} {
		name := "tour-" + appearance
		h := tourHarness(t, Options{Appearance: appearance})
		h.tt.SetSize(1100, 720)
		h.open("main.go")
		h.ed.OfferGuide()
		h.settle()
		h.shot(name + "-offer")
		h.ed.Run("yan.guide.open")
		h.ed.StartLesson(lessonByTitle(h, "最大化编辑区"))
		h.ed.Run("workbench.action.splitEditor")
		h.settle()
		h.shot(name + "-maximize-step")
		h.ed.StartLesson(lessonByTitle(h, "三方合并"))
		for i := 0; i < 50 && !h.tt.HasText("冲突.go"); i++ {
			time.Sleep(50 * time.Millisecond)
			h.settle()
		}
		h.ed.Run("workbench.view.scm")
		openConflict(t, h)
		h.ed.Run("yan.guide.open")
		h.settle()
		h.shot(name + "-merge")
		h.ed.Run("workbench.action.showCommands")
		h.settle()
		h.f.ovInput = "最大化"
		h.ed.SetOverlayInput("最大化")
		h.settle()
		h.shot(name + "-palette")
		h.ed.CloseOverlay()
		h.ed.Run("yan.guide.leave")
		h.settle()
	}
}

func lessonByTitle(h *harness, title string) int {
	h.settle()
	for i, l := range h.f.frame.Sidebar.Learn.Lessons {
		if l.Title == title {
			return i
		}
	}
	h.t.Fatalf("no lesson %s", title)
	return -1
}

// openConflict opens the practice repository's conflicted file from Source Control.
func openConflict(t *testing.T, h *harness) {
	t.Helper()
	for i := 0; i < 50; i++ {
		if c := h.f.frame.Columns; len(c) == 3 {
			return
		}
		if gs := h.f.frame.Sidebar.SCM.Groups; len(gs) > 0 && len(gs[0].Rows) > 0 && gs[0].Rows[0].Letter == 'C' {
			h.ed.SCMClick(0, false)
		}
		time.Sleep(50 * time.Millisecond)
		h.settle()
	}
	t.Fatalf("merge did not open: %v", h.f.frame.Sidebar.SCM.Groups)
}
