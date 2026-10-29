package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

// pretendRelease makes the build report release v for the test.
func pretendRelease(t *testing.T, v string) {
	t.Helper()
	t.Cleanup(PretendRelease(v))
}

// Every lesson and step names a feature of features.json, every feature's command is in the
// command registry, and every since is a version CHANGELOG.md has, or "unreleased" while it
// has a "## Unreleased" section (scripts/bump.sh stamps both at once).
func TestFeaturesMatchRegistryAndChangelog(t *testing.T) {
	e, _ := New(config.Default(), fakeLayouter{}, &fakeHost{}, macGUI)
	for _, l := range guideLessons {
		for _, s := range l.steps {
			if _, ok := features[stepFeature(l, s)]; !ok {
				t.Errorf("%s: step %q teaches unknown feature %q", l.id, s.text, stepFeature(l, s))
			}
		}
	}
	for id, f := range features {
		for _, c := range f.Commands {
			if _, ok := e.commands[c]; !ok {
				t.Errorf("feature %s names unknown command %s", id, c)
			}
		}
	}
	b, err := os.ReadFile("../../CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	headings := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^## (\S+)$`).FindAllStringSubmatch(string(b), -1) {
		headings[m[1]] = true
	}
	for id, f := range features {
		switch {
		case f.Since == sinceUnreleased && !headings["Unreleased"]:
			t.Errorf("feature %s is unreleased but CHANGELOG.md has no ## Unreleased", id)
		case f.Since != sinceUnreleased && !headings[f.Since]:
			t.Errorf("feature %s since %q: no such version in CHANGELOG.md", id, f.Since)
		}
	}
}

// scripts/bump.sh stamps the version cut into features.json, as it does into CHANGELOG.md.
func TestBumpStampsFeatures(t *testing.T) {
	b, err := os.ReadFile("../../scripts/bump.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "core/app/features.json") || !strings.Contains(string(b), `"since": "`+sinceUnreleased+`"`) {
		t.Fatal("bump.sh no longer stamps features.json")
	}
}

func TestVersionOrder(t *testing.T) {
	for _, tc := range []struct {
		a, b  string
		later bool
	}{
		{"0.1.0-alpha.31", "0.1.0-alpha.28", true},
		{"0.1.0-alpha.9", "0.1.0-alpha.10", false},
		{"0.1.0", "0.1.0-alpha.31", true},
		{"v0.2.0-alpha.1", "0.1.0", true},
		{"0.1.0-alpha.28", "0.1.0-alpha.28", false},
	} {
		if got := versionLater(tc.a, tc.b); got != tc.later {
			t.Errorf("%s later than %s: %v", tc.a, tc.b, got)
		}
	}
}

// What a build lacks gets the badge: in the guide's lessons and steps, the command palette,
// and through CommandUnreleased the menus. A development build has every feature, so only the
// ones under "## Unreleased" are badged; an official release is compared by version.
func TestUnreleasedBadges(t *testing.T) {
	// Release stamping changes real features' versions, so the pending case needs
	// its own fixture to keep covering unreleased badges after the next release.
	const pendingCommand = "test.pendingFeature"
	commandSince[pendingCommand] = sinceUnreleased
	t.Cleanup(func() { delete(commandSince, pendingCommand) })
	pretendRelease(t, "0.1.0-alpha.28")
	e := guideEditor(t, macGUI)
	e.Run("yan.guide.open")
	e.StartLesson(lessonIndex("maximize"))
	var f view.Frame
	e.Frame(&f)
	lg := f.Sidebar.Learn
	badged := map[string]bool{}
	for _, l := range lg.Lessons {
		badged[l.Title] = l.Unreleased
	}
	if !badged["最大化编辑区"] || !badged["Markdown 预览与编辑"] || badged["转到定义"] || badged["编辑基础"] {
		t.Fatalf("lesson badges %v", badged)
	}
	if s := lg.Steps; s[0].Unreleased || !s[1].Unreleased || s[1].Since != "0.1.0-alpha.31" {
		t.Fatalf("steps %+v", s)
	}
	if !CommandUnreleased(cmdMaximize) || CommandUnreleased("editor.action.revealDefinition") || !CommandUnreleased("yan.guide.back") {
		t.Fatal("command badges")
	}
	e.Run("workbench.action.showCommands")
	typeText(e, "最大化")
	e.Frame(&f)
	if len(f.Overlay.Items) == 0 || !f.Overlay.Items[0].Unreleased {
		t.Fatalf("palette %+v", f.Overlay.Items)
	}

	pretendRelease(t, "") // a development build
	if CommandUnreleased(cmdMaximize) || CommandUnreleased("yan.guide.restart") || !CommandUnreleased(pendingCommand) {
		t.Fatal("development build badges")
	}
	pretendRelease(t, "0.1.0-alpha.31")
	if CommandUnreleased(cmdMaximize) {
		t.Fatal("the release that has it")
	}
	pretendRelease(t, "0.1.0-alpha.32")
	if CommandUnreleased("yan.guide.restart") || !CommandUnreleased(pendingCommand) {
		t.Fatal("released tour and pending feature badges")
	}
}

// Back, restart and resume: progress is kept in state.json, and a new window opening the
// guide carries on at the same step with the practice file as the user left it.
func TestGuideBackRestartResume(t *testing.T) {
	e := guideEditor(t, macGUI)
	e.StartLesson(lessonIndex("editing"))
	e.Run("expandLineSelection")
	e.Run("editor.action.copyLinesDownAction")
	if e.guide.step != 2 || e.state.GuideLesson != "editing" || e.state.GuideStep != 2 {
		t.Fatalf("progress %+v, state %q %d", e.guide, e.state.GuideLesson, e.state.GuideStep)
	}
	e.Run("yan.guide.back")
	if e.guide.step != 1 || e.state.GuideStep != 1 {
		t.Fatalf("back: %+v", e.guide)
	}
	e.Run("workbench.action.files.save") // what a restart finds is the practice file on disk
	edited := content(e)

	again, _ := New(config.Default(), fakeLayouter{}, &fakeHost{}, macGUI)
	again.SetState(e.state)
	again.Run("yan.guide.open")
	if again.guide.active != lessonIndex("editing") || again.guide.step != 1 || content(again) != edited {
		t.Fatalf("resume: %+v, text %q", again.guide, content(again))
	}
	again.Run("yan.guide.restart")
	if again.guide.step != 0 || content(again) != i18n.T(guideLessons[0].practice) {
		t.Fatalf("restart: %+v", again.guide)
	}
	for _, id := range []string{"expandLineSelection", "editor.action.copyLinesDownAction", "editor.action.moveLinesUpAction", "editor.action.commentLine", "editor.action.deleteLines", "undo"} {
		again.Run(id)
	}
	if again.state.GuideLesson != "" || !again.lessonDone("editing") {
		t.Fatalf("a finished lesson is not resumed: %q", again.state.GuideLesson)
	}
	again.Run("yan.guide.back") // the last step again, after finishing
	if again.guide.step != len(guideLessons[0].steps)-1 {
		t.Fatalf("back after the end: %+v", again.guide)
	}
}

// The window offers the guide once on a first launch; taking or declining it is remembered.
func TestGuideOfferedOnce(t *testing.T) {
	e := guideEditor(t, macGUI)
	statePath := filepath.Join(t.TempDir(), "state.json")
	e.SetState(config.LoadState(statePath))
	var f view.Frame
	e.Frame(&f)
	if f.Guide.Visible {
		t.Fatal("offered without the frontend asking")
	}
	e.OfferGuide()
	e.Frame(&f)
	if !f.Guide.Visible || f.Guide.Lessons != len(guideLessons) {
		t.Fatalf("offer %+v", f.Guide)
	}
	e.Run("yan.guide.dismissOffer")
	e.Frame(&f)
	if f.Guide.Visible || !e.state.GuideOffered {
		t.Fatal("declining did not stick")
	}
	b, _ := os.ReadFile(statePath)
	if !strings.Contains(string(b), `"guideOffered": true`) {
		t.Fatalf("state.json %s", b)
	}
	next, _ := New(config.Default(), fakeLayouter{}, &fakeHost{}, macGUI)
	next.SetState(config.LoadState(statePath))
	next.OfferGuide()
	next.Frame(&f)
	if f.Guide.Visible {
		t.Fatal("offered again after a restart")
	}
	// Taking the offer counts too.
	fresh := guideEditor(t, macGUI)
	fresh.OfferGuide()
	fresh.Run("yan.guide.open")
	if fresh.Frame(&f); f.Guide.Visible || !fresh.state.GuideOffered {
		t.Fatal("opening the guide must end the offer")
	}
}

// The three-way merge lesson runs in a practice repository that replaces the window's folder,
// never in the user's project, and the guide brings the window back to the project after.
func TestGuideMergeLessonInPracticeFolder(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("没有 git")
	}
	e := guideEditor(t, macGUI)
	home := e.ws.root
	e.StartLesson(lessonIndex("merge"))
	practice := practicePath(guideLessons[lessonIndex("merge")])
	if e.ws == nil || e.ws.root != practice || e.state.GuideHome != home || e.side != sideGuide {
		t.Fatalf("folder %v, home %q, status %q", e.ws, e.state.GuideHome, e.status)
	}
	pump(t, e, func() bool { return e.git.status != nil && e.git.status.Op != "" })
	e.Run("workbench.view.scm")
	if e.guide.step != 1 {
		t.Fatalf("scm step: %+v", e.guide)
	}
	if err := e.openPath(filepath.Join(practice, "冲突.go")); err != nil {
		t.Fatal(err)
	}
	e.Run("yan.merge.open")
	if mergeOf(e.cur()) == nil || e.guide.step != 2 {
		t.Fatalf("merge view: %+v, status %q", e.guide, e.status)
	}
	e.Run("yan.merge.acceptTheirs")
	if e.guide.step != 3 {
		t.Fatalf("resolving the conflict: %+v", e.guide)
	}
	e.Run("yan.merge.markResolved")
	if !e.lessonDone("merge") {
		t.Fatalf("lesson not done: %+v, status %q", e.guide, e.status)
	}
	if !strings.Contains(e.status, "回到") {
		t.Fatalf("status %q", e.status)
	}
	e.Run("yan.guide.open")
	var f view.Frame
	e.Frame(&f)
	if f.Sidebar.Learn.Home != filepath.Base(home) {
		t.Fatalf("home offered %q", f.Sidebar.Learn.Home)
	}
	e.Run("yan.guide.leave")
	if e.ws.root != home || e.state.GuideHome != "" {
		t.Fatalf("left to %s, status %q", e.ws.root, e.status)
	}
	if out, _ := exec.Command("git", "-C", home, "status", "--porcelain").CombinedOutput(); strings.Contains(string(out), "冲突") {
		t.Fatal("the practice touched the user's folder")
	}
}

// A link click with the platform's modifier completes "go to definition" as F12 does, and
// the step names that modifier the way the platform does.
func TestGuideDefinitionByLinkClick(t *testing.T) {
	for _, tc := range []struct {
		p    keymap.Profile
		name string
	}{
		{macGUI, "⌘"},
		{keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl}, GUI: true, OS: "windows"}, "Ctrl"},
		{keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl}, GUI: true, OS: "linux"}, "Ctrl"},
	} {
		e := guideEditor(t, tc.p)
		e.Run("yan.guide.open")
		e.StartLesson(lessonIndex("definition"))
		e.Run("yan.guide.skip")
		e.Run("yan.guide.skip")
		var f view.Frame
		e.Frame(&f)
		if s := f.Sidebar.Learn.Steps[2]; !strings.Contains(s.Text, "按住 "+tc.name+" 点击") || s.Command != "editor.action.revealDefinition" || s.Title == "" {
			t.Fatalf("%s: step %+v", tc.p.OS, s)
		}
		s := e.cur().doc.buf.Snapshot()
		g := e.gutter(s)
		at := strings.Index(content(e), "label(area")
		line := s.LineAt(at)
		e.Handle(MouseInput{Column: 0, X: g + view.Unit(at-s.LineStart(line)+1), Y: view.Unit(line), Mods: e.LinkModifier(), Kind: MousePress})
		if !e.lessonDone("definition") {
			t.Fatalf("%s: link click did not count: %+v, status %q", tc.p.OS, e.guide, e.status)
		}
	}
}

// The Markdown lesson's footnote and Mermaid steps complete by what the user writes.
func TestGuideMarkdownFootnoteAndMermaid(t *testing.T) {
	e := guideEditor(t, macGUI)
	e.StartLesson(lessonIndex("markdown"))
	e.Run("yan.guide.skip") // the preview steps, done with the mouse in the window
	e.Run("yan.guide.skip")
	e.GotoLine(3)
	e.Run("cursorEnd")
	typeText(e, "[^2]")
	if e.guide.step != 2 {
		t.Fatal("half a footnote completed the step")
	}
	e.Run("cursorBottom")
	typeText(e, "[^2]: 第二个脚注。\n")
	if e.guide.step != 3 {
		t.Fatalf("footnote: %+v\n%s", e.guide, content(e))
	}
	at := strings.Index(content(e), "B[预览]") + len("B[预览]")
	e.GotoLine(strings.Count(content(e)[:at], "\n") + 1)
	e.Run("cursorEnd")
	typeText(e, "\n  B --> C[保存]")
	if e.guide.step != 4 {
		t.Fatalf("mermaid: %+v\n%s", e.guide, content(e))
	}
}
