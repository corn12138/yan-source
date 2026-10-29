package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
)

// The feature guide, as the IDE Features Trainer of PyCharm and the other JetBrains IDEs: a
// sidebar view lists lessons by module; a lesson opens a practice file outside the project and
// walks through steps, each naming the key the user's keymap has for it. A step completes when
// the user really does it — its command runs, by key, menu or palette, or the editor reaches
// the state it asks for — and the guide moves on by itself. Progress is kept in state.json.

// guideStep is one thing a lesson asks for.
type guideStep struct {
	text string
	// cmds completes the step when any of them runs, times times; check completes it when it
	// holds after an input. A step may have either.
	cmds  []string
	times int
	check func(e *Editor) bool
	// gui marks a step only the window version can do: the terminal edition skips it.
	gui bool
	// feature is the features.json entry the step teaches, when not the lesson's.
	feature string
	// show is the command the guide names for a step done by check, without waiting for it.
	show string
}

// guideLesson is a lesson: its module, its title, the feature of features.json it teaches,
// the practice file it opens and its steps. A lesson with a folder opens a practice
// repository, made afresh by folder, in place of the window's folder (Git must not touch the
// user's projects); file is then the file in it to open first.
type guideLesson struct {
	id, module, title, feature string
	file, practice             string
	folder                     func(dir string) error
	steps                      []guideStep
}

// Practice texts some steps compare against.
var (
	practiceDefinition = i18n.N("package practice\n\n// Place the cursor on the area call, go to its definition, then go back.\nfunc report(w, h int) string {\n\treturn label(area(w, h))\n}\n\nfunc area(w, h int) int {\n\treturn w * h\n}\n\nfunc label(n int) string {\n\treturn \"Area\"\n}\n")
	practiceMarkdown   = i18n.N("# Guide\n\nDouble-click this paragraph in the preview to edit it.[^1]\n\n- First item\n- Second item\n\n```mermaid\nflowchart LR\n  A[Write] --> B[Preview]\n```\n\n[^1]: Footnotes are defined at the end and shown as superscript numbers in the preview.\n")
	practiceConflict   = i18n.N("package practice\n\nfunc greeting() string {\n\treturn \"Hello\"\n}\n")
)

var guideLessons = []guideLesson{
	{
		id: "editing", module: i18n.N("Editing"), title: i18n.N("Editing Basics"), feature: "basics", file: "编辑基础.go",
		practice: i18n.N("package practice\n\n// Put the cursor on any line below and follow the steps in the sidebar's Learn view.\nfunc greet() string {\n\treturn \"Hello\"\n}\n\nfunc farewell() string {\n\treturn \"Goodbye\"\n}\n"),
		steps: []guideStep{
			{text: i18n.N("Select the whole line the cursor is on"), cmds: []string{"expandLineSelection"}},
			{text: i18n.N("Copy this line down"), cmds: []string{"editor.action.copyLinesDownAction", "editor.action.copyLinesUpAction"}},
			{text: i18n.N("Move this line up or down"), cmds: []string{"editor.action.moveLinesUpAction", "editor.action.moveLinesDownAction"}},
			{text: i18n.N("Comment or uncomment this line"), cmds: []string{"editor.action.commentLine"}},
			{text: i18n.N("Delete this line"), cmds: []string{"editor.action.deleteLines"}},
			{text: i18n.N("Undo the last change"), cmds: []string{"undo"}},
		},
	},
	{
		id: "multicursor", module: i18n.N("Editing"), title: i18n.N("Multiple Cursors"), feature: "basics", file: "多光标.go",
		practice: i18n.N("package practice\n\n// Rename count: first put the cursor on the first count.\nfunc total(items []int) int {\n\tcount := 0\n\tfor range items {\n\t\tcount++\n\t}\n\treturn count\n}\n"),
		steps: []guideStep{
			{text: i18n.N("Select the word at the cursor, then press twice more to select the next two occurrences"), cmds: []string{"editor.action.addSelectionToNextFindMatch"}, times: 3},
			{text: i18n.N("Type the new name: all places change together"), check: func(e *Editor) bool {
				t := e.cur()
				return t != nil && t.doc.buf.Dirty() && len(t.doc.buf.Selections().Ranges) > 1
			}},
			{text: i18n.N("Go back to one cursor"), cmds: []string{"removeSecondaryCursors"}},
		},
	},
	{
		id: "navigation", module: i18n.N("Navigation"), title: i18n.N("Finding Files and Commands"), feature: "basics", file: "导航.go",
		practice: i18n.N("package practice\n\n// This file has a few functions: jump to them with \"Go to Symbol\".\nfunc alpha() {}\n\nfunc beta() {}\n\nfunc gamma() {}\n"),
		steps: []guideStep{
			{text: i18n.N("Quick-open a file (type a few letters of its name)"), cmds: []string{"workbench.action.quickOpen"}},
			{text: i18n.N("Open the Command Palette: every feature can be found here by name"), cmds: []string{"workbench.action.showCommands"}},
			{text: i18n.N("Go to a line"), cmds: []string{"workbench.action.gotoLine"}},
			{text: i18n.N("Go to a function (symbol) in the file"), cmds: []string{"workbench.action.gotoSymbol"}},
			{text: i18n.N("Find in this file"), cmds: []string{"actions.find"}},
			{text: i18n.N("Search the whole folder"), cmds: []string{"workbench.action.findInFiles"}},
		},
	},
	{
		id: "definition", module: i18n.N("Navigation"), title: i18n.N("Go to Definition"), feature: "definition", file: "转到定义.go",
		practice: practiceDefinition,
		steps: []guideStep{
			{text: i18n.N("Place the cursor on area in report and go to its definition"), cmds: []string{"editor.action.revealDefinition"}},
			{text: i18n.N("Go back to the previous location"), cmds: []string{"workbench.action.navigateBack"}},
			{text: i18n.N("Hold {link} and click label to go to its definition"), cmds: []string{"editor.action.revealDefinition"}, gui: true},
		},
	},
	{
		id: "layout", module: i18n.N("Layout"), title: i18n.N("Columns and Terminals"), feature: "basics", file: "布局.go",
		practice: i18n.N("package practice\n\n// This lesson splits the window into columns, then opens a terminal.\nfunc layout() {}\n"),
		steps: []guideStep{
			{text: i18n.N("Split a column to the right"), cmds: []string{"workbench.action.splitEditor"}},
			{text: i18n.N("Move this editor to another column"), cmds: []string{"workbench.action.moveEditorToNextGroup", "workbench.action.moveEditorToPreviousGroup"}},
			{text: i18n.N("Move focus between columns"), cmds: []string{"workbench.action.focusNextGroup", "workbench.action.focusPreviousGroup"}},
			{text: i18n.N("Lock this column: files opened afterwards go to other columns"), cmds: []string{"view.lockColumn"}, feature: "columnLock"},
			{text: i18n.N("Open or hide the terminal panel"), cmds: []string{"workbench.action.terminal.toggleTerminal"}, gui: true},
			{text: i18n.N("Open a terminal tab in the editor area"), cmds: []string{"workbench.action.createTerminalEditor"}, gui: true, feature: "editorTerminal"},
			{text: i18n.N("Move the terminal to the locked column on the right"), cmds: []string{"view.terminalLockRight"}, gui: true, feature: "columnLock"},
		},
	},
	{
		id: "maximize", module: i18n.N("Layout"), title: i18n.N("Maximize Editor Area"), feature: "maximize", file: "最大化.go",
		practice: i18n.N("package practice\n\n// Split into two columns, maximize this column, then restore the layout.\nfunc focus() {}\n"),
		steps: []guideStep{
			{text: i18n.N("Split a column to the right"), cmds: []string{"workbench.action.splitEditor"}, feature: "basics"},
			{text: i18n.N("Maximize this column, hiding the sidebar, terminal panel and other columns"), show: cmdMaximize, check: func(e *Editor) bool { return e.maximized != nil }},
			{text: i18n.N("Restore the layout: column widths, locks and cursors stay as they were"), show: cmdMaximize, check: func(e *Editor) bool { return e.maximized == nil }},
		},
	},
	{
		id: "git", module: "Git", title: i18n.N("Source Control"), feature: "basics", file: "笔记.md",
		folder: func(dir string) error {
			return git.PracticeChanges(dir, map[string]string{"笔记.md": i18n.T("# Notes\n\nFirst line.\n")},
				map[string]string{"笔记.md": i18n.T("# Notes\n\nFirst line.\nSecond line: not committed yet.\n"), "新文件.txt": i18n.T("A new file.\n")})
		},
		steps: []guideStep{
			{text: i18n.N("Open Source Control: the practice repository has two changes"), cmds: []string{"workbench.view.scm"}},
			{text: i18n.N("See what changed in the current file"), cmds: []string{"git.openChange", "git.openAllChanges"}},
			{text: i18n.N("Stage changes"), cmds: []string{"git.stage", "git.stageAll", "git.stageSelectedRanges"}},
			{text: i18n.N("Write a commit message, then commit"), cmds: []string{"git.commit", "git.commitAndPush"}},
		},
	},
	{
		id: "merge", module: "Git", title: i18n.N("Three-Way Merge"), feature: "merge",
		folder: func(dir string) error {
			return git.PracticeConflict(dir, "冲突.go", i18n.T(practiceConflict),
				strings.Replace(i18n.T(practiceConflict), i18n.T("Hello"), i18n.T("Hello, world"), 1), strings.Replace(i18n.T(practiceConflict), i18n.T("Hello"), i18n.T("Good morning"), 1))
		},
		steps: []guideStep{
			{text: i18n.N("Open Source Control: the practice repository has a merge conflict in its Go file"), cmds: []string{"workbench.view.scm"}},
			{text: i18n.N("Open the conflicted Go file in a three-way merge: ours on the left, theirs on the right, the result in the middle"), cmds: []string{"yan.merge.open"}, check: func(e *Editor) bool {
				return mergeOf(e.cur()) != nil
			}},
			{text: i18n.N("Choose one side of this conflict, or keep both"), show: "yan.merge.acceptOurs", check: func(e *Editor) bool {
				mv := mergeOf(e.cur())
				return mv != nil && mv.unresolved() == 0
			}},
			{text: i18n.N("Mark as resolved: save and stage"), cmds: []string{"yan.merge.markResolved", "yan.merge.applyAndNext", "yan.merge.saveAndClose"}},
		},
	},
	{
		id: "markdown", module: i18n.N("Writing"), title: i18n.N("Markdown Preview and Editing"), feature: "markdownEdit", file: "说明.md",
		practice: practiceMarkdown,
		steps: []guideStep{
			{text: i18n.N("Open the preview to the side"), cmds: []string{"markdown.showPreviewToSide"}, gui: true, feature: "markdownPreview"},
			{text: i18n.N("Double-click a paragraph in the preview and edit it directly"), check: func(e *Editor) bool { return e.guide.previewEdited }, gui: true},
			{text: i18n.N("Add a footnote: write [^2] in the text and [^2]: followed by its content at the end"), feature: "footnotes", check: func(e *Editor) bool {
				t := practiceText(e, "说明.md")
				return strings.Count(t, "[^2]") >= 2 && strings.Contains(t, "[^2]:")
			}},
			{text: i18n.N("Add an edge to the Mermaid diagram, such as B --> C[Save]: the preview redraws"), feature: "mermaid", check: func(e *Editor) bool {
				return strings.Count(practiceText(e, "说明.md"), "-->") > strings.Count(practiceMarkdown, "-->")
			}},
			{text: i18n.N("Save"), cmds: []string{"workbench.action.files.save"}},
		},
	},
	{
		id: "settings", module: i18n.N("Settings"), title: i18n.N("Personalization"), feature: "basics",
		steps: []guideStep{
			{text: i18n.N("Open the settings file config.toml"), cmds: []string{"yan.config.open"}},
			{text: i18n.N("Open the keyboard shortcuts file keybindings.json"), cmds: []string{"workbench.action.openGlobalKeybindingsFile"}},
			{text: i18n.N("Move the sidebar to the other side (press again to move it back)"), cmds: []string{"workbench.action.toggleSidebarPosition"}},
			{text: i18n.N("Import settings from VS Code or Cursor (skip if you have neither)"), cmds: []string{"yan.config.importVSCode", "yan.config.importSettingsFile"}, feature: "importSettings"},
		},
	},
}

// command is the command the guide names for the step: the first it waits for, or the one it shows.
func (s guideStep) command() string {
	if len(s.cmds) > 0 {
		return s.cmds[0]
	}
	return s.show
}

// stepFeature is the feature a step of lesson l teaches.
func stepFeature(l guideLesson, s guideStep) string {
	if s.feature != "" {
		return s.feature
	}
	return l.feature
}

// lessonUnreleased reports whether lesson l teaches something the official release lacks.
func lessonUnreleased(l guideLesson) bool {
	if unreleased(FeatureSince(l.feature)) {
		return true
	}
	return slices.ContainsFunc(l.steps, func(s guideStep) bool { return unreleased(FeatureSince(stepFeature(l, s))) })
}

// learnDir is where the practice files and folders live: next to config.toml, never in a project.
func learnDir() string { return filepath.Join(filepath.Dir(config.UserPath()), "learn") }

// practicePath is where lesson l keeps its practice file or folder.
func practicePath(l guideLesson) string {
	if l.folder != nil {
		return filepath.Join(learnDir(), l.id)
	}
	return filepath.Join(learnDir(), l.file)
}

// inPractice reports whether path is one of the guide's practice files or folders.
func inPractice(path string) bool {
	rel, err := filepath.Rel(learnDir(), path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// practiceText is the text of the practice file named file, as edited.
func practiceText(e *Editor, file string) string {
	if d := e.docOf(filepath.Join(learnDir(), file)); d != nil {
		s := d.buf.Snapshot()
		return s.String(0, s.Len())
	}
	return ""
}

// guideTips are short tips shown under the lessons, one at a time; {id} becomes the key of
// command id in the user's keymap.
var guideTips = []string{
	i18n.N("Press {workbench.action.quickOpen}, then type \":line\" to jump to that line, or \"@\" to list symbols."),
	i18n.N("Press {editor.action.addSelectionToNextFindMatch} repeatedly to select one more occurrence each time, then edit them together."),
	i18n.N("Closed the wrong tab? {workbench.action.reopenClosedEditor} reopens it where it was."),
	i18n.N("Middle-click a tab to close it without switching to it."),
	i18n.N("Drag a search result onto a column to open it there with the match selected."),
	i18n.N("Once a column is locked, opened files go to other columns: good for pinning a terminal or a reference file."),
	i18n.N("Click a folder in the breadcrumbs to find files beside it in a pop-up tree."),
	i18n.N("keybindings.json accepts VS Code's format, including when conditions and runCommands."),
	i18n.N("In the Markdown editing box, type / at the start of a line to insert headings, lists, tables and code blocks."),
	i18n.N("Write position = \"right\" under [terminal] in config.toml to put the terminal panel on the right."),
}

// guideState is the guide's progress in this window.
type guideState struct {
	sel    int // the lesson chosen in the list
	active int // the lesson running, -1 for none
	step   int
	count  int // how many times the current step's command has run
	tip    int
	// previewEdited is set when a Markdown preview writes an edit, for the Markdown lesson.
	previewEdited bool
	// offer is set when the frontend offers the guide on launch (OfferGuide).
	offer bool
}

// OfferGuide asks the window to offer the guide once, on the first launch: until the user
// takes or declines it (state.json remembers which), and not to someone who has used it.
func (e *Editor) OfferGuide() { e.guide.offer = true }

func (e *Editor) guideOffered() bool {
	st := e.state
	return !e.guide.offer || st == nil || st.GuideOffered || len(st.GuideDone) > 0 || st.GuideLesson != ""
}

// dismissGuideOffer declines the offer; it will not come back.
func (e *Editor) dismissGuideOffer() error {
	e.guide.offer = false
	if e.state != nil && !e.state.GuideOffered {
		e.state.GuideOffered = true
		e.saveState()
	}
	return nil
}

// openGuide shows the guide in the sidebar, resuming the lesson left unfinished.
func (e *Editor) openGuide() error {
	e.dismissGuideOffer()
	e.sidebar, e.side, e.area = true, sideGuide, areaTree
	e.closeOverlay()
	if e.guide.active < 0 {
		return e.resumeLesson()
	}
	return nil
}

// resumeLesson goes back to the lesson and step state.json has, with what the user had
// practised: the practice file or folder is reopened as it was, only made when missing.
func (e *Editor) resumeLesson() error {
	if e.state == nil || e.state.GuideLesson == "" {
		return nil
	}
	i := slices.IndexFunc(guideLessons, func(l guideLesson) bool { return l.id == e.state.GuideLesson })
	if i < 0 {
		return nil
	}
	l := guideLessons[i]
	g := &e.guide
	g.sel, g.active, g.step, g.count = i, i, max(0, min(e.state.GuideStep, len(l.steps)-1)), 0
	if _, err := os.Stat(practicePath(l)); err != nil {
		if err := e.preparePractice(l); err != nil {
			return err
		}
	}
	if err := e.enterPractice(l); err != nil {
		return err
	}
	e.skipWindowSteps()
	e.status = i18n.F("Resume lesson %q at step %d", i18n.T(l.title), g.step+1)
	return nil
}

// StartLesson starts lesson i of the list: its practice file opens and its first step shows.
func (e *Editor) StartLesson(i int) {
	e.sync()
	defer e.sync()
	if err := e.startLesson(i); err != nil {
		e.status = err.Error()
	}
}

func (e *Editor) startLesson(i int) error {
	if i < 0 || i >= len(guideLessons) {
		return nil
	}
	e.dismissGuideOffer()
	l := guideLessons[i]
	if err := e.preparePractice(l); err != nil {
		return err
	}
	e.guide.sel, e.guide.active, e.guide.step, e.guide.count, e.guide.previewEdited = i, i, 0, 0, false
	if err := e.enterPractice(l); err != nil {
		e.guide.active = -1
		return err
	}
	e.skipWindowSteps()
	e.saveGuideProgress()
	e.status = i18n.F("Lesson %q: follow the steps in the sidebar", i18n.T(l.title))
	return nil
}

// preparePractice writes lesson l's practice file, or makes its practice folder, afresh.
func (e *Editor) preparePractice(l guideLesson) error {
	path := practicePath(l)
	switch {
	case l.folder != nil:
		if e.ws != nil && e.ws.root == path {
			// Making it afresh under the window's feet: reopen it after, as if from elsewhere.
			if err := e.leavePracticeFolder(); err != nil {
				return err
			}
		}
		if err := os.MkdirAll(learnDir(), 0o755); err != nil {
			return err
		}
		return l.folder(path)
	case l.file != "":
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(i18n.T(l.practice)), 0o644); err != nil {
			return err
		}
		if d := e.docOf(path); d != nil { // a fresh copy each time the lesson starts
			if s, err := text.Load(path); err == nil {
				d.buf.Reload(s)
			}
		}
	}
	return nil
}

// enterPractice opens lesson l's practice file, or switches the window to its practice folder,
// remembering the folder it replaces to go back to.
func (e *Editor) enterPractice(l guideLesson) error {
	path := practicePath(l)
	if l.folder != nil {
		if e.ws == nil || e.ws.root != path {
			home := ""
			if e.ws != nil && !inPractice(e.ws.root) {
				home = e.ws.root
			}
			if err := e.SwitchFolder(path); err != nil {
				return err
			}
			if e.state != nil && home != "" {
				e.state.GuideHome = home
				e.saveState()
			}
		}
		e.sidebar, e.side = true, sideGuide // the switch keeps the guide in view
		if l.file != "" {
			if err := e.openPath(filepath.Join(path, l.file)); err != nil {
				return err
			}
		}
		e.area = areaEditor
		return nil
	}
	if l.file == "" {
		return nil
	}
	if err := e.openPath(path); err != nil {
		return err
	}
	e.area = areaEditor
	return nil
}

// leavePracticeFolder brings the window back to the folder a practice folder replaced.
func (e *Editor) leavePracticeFolder() error {
	if e.ws == nil || !inPractice(e.ws.root) {
		return nil
	}
	home := ""
	if e.state != nil {
		home = e.state.GuideHome
	}
	if home == "" {
		return userError(i18n.T("This window had no folder before practice. Use Open Folder to return to your project"))
	}
	if err := e.SwitchFolder(home); err != nil {
		return err
	}
	e.state.GuideHome = ""
	e.saveState()
	e.sidebar, e.side = true, sideGuide
	return nil
}

// saveGuideProgress remembers the lesson running and its step, to resume after a restart.
func (e *Editor) saveGuideProgress() {
	if e.state == nil {
		return
	}
	g := &e.guide
	id, step := "", 0
	if g.active >= 0 && g.active < len(guideLessons) && g.step < len(guideLessons[g.active].steps) {
		id, step = guideLessons[g.active].id, g.step
	}
	if e.state.GuideLesson != id || e.state.GuideStep != step {
		e.state.GuideLesson, e.state.GuideStep = id, step
		e.saveState()
	}
}

// guideSaw tells the guide a command ran.
func (e *Editor) guideSaw(id string) {
	g := &e.guide
	if g.active < 0 || g.active >= len(guideLessons) {
		return
	}
	steps := guideLessons[g.active].steps
	if g.step >= len(steps) || !slices.Contains(steps[g.step].cmds, id) {
		return
	}
	g.count++
	if g.count >= max(steps[g.step].times, 1) {
		e.guideAdvance()
	}
}

// guideCheck completes a step whose condition now holds; it runs after each input.
func (e *Editor) guideCheck() {
	g := &e.guide
	if g.active < 0 || g.active >= len(guideLessons) {
		return
	}
	steps := guideLessons[g.active].steps
	if g.step < len(steps) && steps[g.step].check != nil && steps[g.step].check(e) {
		e.guideAdvance()
	}
}

func (e *Editor) guideAdvance() {
	g := &e.guide
	g.step, g.count = g.step+1, 0
	e.skipWindowSteps()
	e.saveGuideProgress()
	l := guideLessons[g.active]
	if g.step < len(l.steps) {
		if e.side != sideGuide || !e.sidebar || e.maximized != nil { // the guide is out of sight: say what is next
			next := l.steps[g.step]
			e.status = i18n.F("Next step: %s", e.expandTip(next.text))
			if id := next.command(); e.keyHint[id] != "" {
				e.status += i18n.F(" (%s)", e.keyHint[id])
			}
		}
		return
	}
	if e.state != nil && !slices.Contains(e.state.GuideDone, l.id) {
		e.state.GuideDone = append(e.state.GuideDone, l.id)
		e.saveState()
	}
	e.status = i18n.F("Completed lesson %q", i18n.T(l.title))
	if e.ws != nil && inPractice(e.ws.root) && e.state != nil && e.state.GuideHome != "" {
		e.status += i18n.F(": use %q to return to %s", e.commands["yan.guide.leave"].Title, filepath.Base(e.state.GuideHome))
		return
	}
	if next := e.nextLesson(g.active); next >= 0 {
		e.status += i18n.F(": start the next lesson \"%s\" in the sidebar", i18n.T(guideLessons[next].title))
		g.sel = next
	}
}

// skipWindowSteps passes over the steps only the window version can do, in the terminal one.
func (e *Editor) skipWindowSteps() {
	g := &e.guide
	steps := guideLessons[g.active].steps
	for !e.profile.GUI && g.step < len(steps) && steps[g.step].gui {
		g.step++
	}
}

// skipStep passes over the current step (a feature the user has no use for, as Git without a repository).
func (e *Editor) skipStep() error {
	if e.guide.active < 0 {
		return nil
	}
	e.guideAdvance()
	return nil
}

// backStep goes back to the step before, to do it again; after the last step, to the last.
func (e *Editor) backStep() error {
	g := &e.guide
	if g.active < 0 {
		return nil
	}
	steps := guideLessons[g.active].steps
	k := min(g.step, len(steps)) - 1
	for k > 0 && !e.profile.GUI && steps[k].gui {
		k--
	}
	g.step, g.count = max(k, 0), 0
	e.saveGuideProgress()
	return nil
}

// restartLesson starts the lesson running, or the one chosen, again from a fresh practice file.
func (e *Editor) restartLesson() error {
	i := e.guide.active
	if i < 0 {
		i = e.guide.sel
	}
	return e.startLesson(i)
}

// nextLesson returns the first lesson after i not yet done, -1 when all are.
func (e *Editor) nextLesson(i int) int {
	for k := 1; k <= len(guideLessons); k++ {
		j := (i + k) % len(guideLessons)
		if !e.lessonDone(guideLessons[j].id) {
			return j
		}
	}
	return -1
}

func (e *Editor) lessonDone(id string) bool {
	return e.state != nil && slices.Contains(e.state.GuideDone, id)
}

// resetGuide forgets the lessons done.
func (e *Editor) resetGuide() error {
	if e.state != nil {
		e.state.GuideDone, e.state.GuideLesson, e.state.GuideStep = nil, "", 0
		e.saveState()
	}
	e.guide = guideState{active: -1}
	e.status = i18n.T("Feature tour progress reset")
	return nil
}

func (e *Editor) nextTip() error {
	e.guide.tip = (e.guide.tip + 1) % len(guideTips)
	return nil
}

// handleGuideKey moves through the lesson list and starts the chosen lesson.
func (e *Editor) handleGuideKey(c keymap.Chord) bool {
	if c.Mods != 0 {
		return false
	}
	switch c.Key {
	case keymap.KeyUp:
		e.guide.sel = max(e.guide.sel-1, 0)
	case keymap.KeyDown:
		e.guide.sel = min(e.guide.sel+1, len(guideLessons)-1)
	case keymap.KeyEnter:
		if err := e.startLesson(e.guide.sel); err != nil {
			e.status = err.Error()
		}
	default:
		return false
	}
	return true
}

// guideOfferFrame is the launch offer for the frontends.
func (e *Editor) guideOfferFrame(dst *view.GuideOffer) {
	*dst = view.GuideOffer{Visible: !e.guideOffered(), Lessons: len(guideLessons)}
}

// guideFrame describes the guide for the frontends.
func (e *Editor) guideFrame(dst *view.Learn) {
	g := &e.guide
	dst.Lessons = dst.Lessons[:0]
	done := 0
	for _, l := range guideLessons {
		d := e.lessonDone(l.id)
		if d {
			done++
		}
		dst.Lessons = append(dst.Lessons, view.LearnLesson{Module: i18n.T(l.module), Title: i18n.T(l.title), Done: d, Steps: len(l.steps),
			Unreleased: lessonUnreleased(l), Folder: l.folder != nil})
	}
	dst.Selected, dst.Active, dst.Done = g.sel, g.active, done
	dst.Steps = dst.Steps[:0]
	if g.active >= 0 && g.active < len(guideLessons) {
		l := guideLessons[g.active]
		for i, s := range l.steps {
			since := FeatureSince(stepFeature(l, s))
			st := view.LearnStep{Text: e.expandTip(s.text), Done: i < g.step, Current: i == g.step, WindowOnly: s.gui && !e.profile.GUI,
				Since: since, Unreleased: unreleased(since)}
			if id := s.command(); id != "" {
				st.Command, st.Title = id, e.commands[id].Title
				st.Keys = e.keyHint[id]
				if s.times > 1 {
					st.Count = g.count
					st.Times = s.times
				}
			}
			dst.Steps = append(dst.Steps, st)
		}
	}
	dst.Home = ""
	if e.ws != nil && inPractice(e.ws.root) && e.state != nil && e.state.GuideHome != "" {
		dst.Home = filepath.Base(e.state.GuideHome)
	}
	dst.Tip = e.expandTip(guideTips[g.tip%len(guideTips)])
}

// expandTip writes the keys of the commands a tip or a step names; {link} is the modifier a
// link click holds on this platform.
func (e *Editor) expandTip(tip string) string {
	tip = i18n.T(tip)
	for {
		i := strings.Index(tip, "{")
		j := strings.Index(tip, "}")
		if i < 0 || j < i {
			return tip
		}
		id := tip[i+1 : j]
		k := e.keyHint[id]
		switch {
		case id == "link":
			k = modName(e.LinkModifier(), e.profile)
		case k == "":
			k = i18n.F("“%s”", e.commands[id].Title)
		}
		tip = tip[:i] + k + tip[j+1:]
	}
}
