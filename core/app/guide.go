package app

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/git"
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
const (
	practiceDefinition = "package practice\n\n// 光标放在下面 area 的调用上，转到它的定义；再跳回来。\nfunc report(w, h int) string {\n\treturn label(area(w, h))\n}\n\nfunc area(w, h int) int {\n\treturn w * h\n}\n\nfunc label(n int) string {\n\treturn \"面积\"\n}\n"
	practiceMarkdown   = "# 说明\n\n双击预览里的这一段，直接改它。[^1]\n\n- 第一项\n- 第二项\n\n```mermaid\nflowchart LR\n  A[写] --> B[预览]\n```\n\n[^1]: 脚注写在文末，预览里显示成上标数字。\n"
	practiceConflict   = "package practice\n\nfunc greeting() string {\n\treturn \"你好\"\n}\n"
)

var guideLessons = []guideLesson{
	{
		id: "editing", module: "编辑", title: "编辑基础", feature: "basics", file: "编辑基础.go",
		practice: "package practice\n\n// 把光标放在下面任意一行，照侧边栏「学习」里的步骤做。\nfunc greet() string {\n\treturn \"你好\"\n}\n\nfunc farewell() string {\n\treturn \"再见\"\n}\n",
		steps: []guideStep{
			{text: "选中光标所在的整行", cmds: []string{"expandLineSelection"}},
			{text: "把这一行向下复制一份", cmds: []string{"editor.action.copyLinesDownAction", "editor.action.copyLinesUpAction"}},
			{text: "把这一行上下移动", cmds: []string{"editor.action.moveLinesUpAction", "editor.action.moveLinesDownAction"}},
			{text: "注释或取消注释这一行", cmds: []string{"editor.action.commentLine"}},
			{text: "删除这一行", cmds: []string{"editor.action.deleteLines"}},
			{text: "撤销刚才的修改", cmds: []string{"undo"}},
		},
	},
	{
		id: "multicursor", module: "编辑", title: "多光标", feature: "basics", file: "多光标.go",
		practice: "package practice\n\n// 把 count 改个名字：先把光标放在第一个 count 上。\nfunc total(items []int) int {\n\tcount := 0\n\tfor range items {\n\t\tcount++\n\t}\n\treturn count\n}\n",
		steps: []guideStep{
			{text: "选中光标所在的词，再连按两次，选中后面两个相同的词", cmds: []string{"editor.action.addSelectionToNextFindMatch"}, times: 3},
			{text: "直接输入新名字：几处一起改", check: func(e *Editor) bool {
				t := e.cur()
				return t != nil && t.doc.buf.Dirty() && len(t.doc.buf.Selections().Ranges) > 1
			}},
			{text: "回到一个光标", cmds: []string{"removeSecondaryCursors"}},
		},
	},
	{
		id: "navigation", module: "导航", title: "找文件、找命令", feature: "basics", file: "导航.go",
		practice: "package practice\n\n// 这个文件里有几个函数，用「转到符号」跳过去。\nfunc alpha() {}\n\nfunc beta() {}\n\nfunc gamma() {}\n",
		steps: []guideStep{
			{text: "快速打开一个文件（输入文件名的几个字母）", cmds: []string{"workbench.action.quickOpen"}},
			{text: "打开命令面板：所有功能都能在这里按名字找到", cmds: []string{"workbench.action.showCommands"}},
			{text: "跳到某一行", cmds: []string{"workbench.action.gotoLine"}},
			{text: "跳到文件里的函数（符号）", cmds: []string{"workbench.action.gotoSymbol"}},
			{text: "在这个文件里查找", cmds: []string{"actions.find"}},
			{text: "在整个文件夹里搜索", cmds: []string{"workbench.action.findInFiles"}},
		},
	},
	{
		id: "definition", module: "导航", title: "转到定义", feature: "definition", file: "转到定义.go",
		practice: practiceDefinition,
		steps: []guideStep{
			{text: "把光标放在 report 里的 area 上，转到它的定义", cmds: []string{"editor.action.revealDefinition"}},
			{text: "跳回刚才的位置", cmds: []string{"workbench.action.navigateBack"}},
			{text: "按住 {link} 点击 label：鼠标也能转到定义", cmds: []string{"editor.action.revealDefinition"}, gui: true},
		},
	},
	{
		id: "layout", module: "布局", title: "列与终端", feature: "basics", file: "布局.go",
		practice: "package practice\n\n// 这一课把窗口分成几列，再开一个终端。\nfunc layout() {}\n",
		steps: []guideStep{
			{text: "向右拆分出一列", cmds: []string{"workbench.action.splitEditor"}},
			{text: "把这个编辑器移到另一列", cmds: []string{"workbench.action.moveEditorToNextGroup", "workbench.action.moveEditorToPreviousGroup"}},
			{text: "在列之间切换焦点", cmds: []string{"workbench.action.focusNextGroup", "workbench.action.focusPreviousGroup"}},
			{text: "锁定这一列：之后打开的文件去别的列", cmds: []string{"view.lockColumn"}, feature: "columnLock"},
			{text: "打开或收起终端面板", cmds: []string{"workbench.action.terminal.toggleTerminal"}, gui: true},
			{text: "在编辑区开一个终端标签", cmds: []string{"workbench.action.createTerminalEditor"}, gui: true, feature: "editorTerminal"},
			{text: "把终端移到右侧锁定列", cmds: []string{"view.terminalLockRight"}, gui: true, feature: "columnLock"},
		},
	},
	{
		id: "maximize", module: "布局", title: "最大化编辑区", feature: "maximize", file: "最大化.go",
		practice: "package practice\n\n// 先拆出两列，再把这一列放大到整个窗口，看完还原。\nfunc focus() {}\n",
		steps: []guideStep{
			{text: "向右拆分出一列", cmds: []string{"workbench.action.splitEditor"}, feature: "basics"},
			{text: "最大化这一列：侧边栏、终端和别的列先让开", show: cmdMaximize, check: func(e *Editor) bool { return e.maximized != nil }},
			{text: "还原：列宽、锁定和光标都和之前一样", show: cmdMaximize, check: func(e *Editor) bool { return e.maximized == nil }},
		},
	},
	{
		id: "git", module: "Git", title: "源代码管理", feature: "basics", file: "笔记.md",
		folder: func(dir string) error {
			return git.PracticeChanges(dir, map[string]string{"笔记.md": "# 笔记\n\n第一行。\n"},
				map[string]string{"笔记.md": "# 笔记\n\n第一行。\n第二行：这一行还没提交。\n", "新文件.txt": "一个新文件。\n"})
		},
		steps: []guideStep{
			{text: "打开源代码管理视图：练习仓库里有两处改动", cmds: []string{"workbench.view.scm"}},
			{text: "看当前文件改了什么", cmds: []string{"git.openChange", "git.openAllChanges"}},
			{text: "暂存改动", cmds: []string{"git.stage", "git.stageAll", "git.stageSelectedRanges"}},
			{text: "写一句提交说明，然后提交", cmds: []string{"git.commit", "git.commitAndPush"}},
		},
	},
	{
		id: "merge", module: "Git", title: "三方合并", feature: "merge",
		folder: func(dir string) error {
			return git.PracticeConflict(dir, "冲突.go", practiceConflict,
				strings.Replace(practiceConflict, "你好", "你好，世界", 1), strings.Replace(practiceConflict, "你好", "早上好", 1))
		},
		steps: []guideStep{
			{text: "打开源代码管理视图：练习仓库正在合并，冲突.go 有冲突", cmds: []string{"workbench.view.scm"}},
			{text: "打开冲突.go 的三方合并：左边是我们的，右边是他们的，中间是结果", cmds: []string{"yan.merge.open"}, check: func(e *Editor) bool {
				return mergeOf(e.cur()) != nil
			}},
			{text: "给这处冲突选一边（或两边都要）", show: "yan.merge.acceptOurs", check: func(e *Editor) bool {
				mv := mergeOf(e.cur())
				return mv != nil && mv.unresolved() == 0
			}},
			{text: "标记为已解决：保存并暂存", cmds: []string{"yan.merge.markResolved", "yan.merge.applyAndNext", "yan.merge.saveAndClose"}},
		},
	},
	{
		id: "markdown", module: "写作", title: "Markdown 预览与编辑", feature: "markdownEdit", file: "说明.md",
		practice: practiceMarkdown,
		steps: []guideStep{
			{text: "在右侧打开预览", cmds: []string{"markdown.showPreviewToSide"}, gui: true, feature: "markdownPreview"},
			{text: "在预览里双击一段，直接编辑它", check: func(e *Editor) bool { return e.guide.previewEdited }, gui: true},
			{text: "加一个脚注：正文里写 [^2]，文末写 [^2]: 和它的内容", feature: "footnotes", check: func(e *Editor) bool {
				t := practiceText(e, "说明.md")
				return strings.Count(t, "[^2]") >= 2 && strings.Contains(t, "[^2]:")
			}},
			{text: "在 mermaid 图里加一条边，比如 B --> C[保存]：预览跟着重画", feature: "mermaid", check: func(e *Editor) bool {
				return strings.Count(practiceText(e, "说明.md"), "-->") > strings.Count(practiceMarkdown, "-->")
			}},
			{text: "保存", cmds: []string{"workbench.action.files.save"}},
		},
	},
	{
		id: "settings", module: "设置", title: "个性化", feature: "basics",
		steps: []guideStep{
			{text: "打开设置文件 config.toml", cmds: []string{"yan.config.open"}},
			{text: "打开快捷键设置 keybindings.json", cmds: []string{"workbench.action.openGlobalKeybindingsFile"}},
			{text: "把侧边栏移到另一侧（再按一次移回来）", cmds: []string{"workbench.action.toggleSidebarPosition"}},
			{text: "从 VS Code、Cursor 导入设置（没有可跳过）", cmds: []string{"yan.config.importVSCode", "yan.config.importSettingsFile"}, feature: "importSettings"},
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
	"按 {workbench.action.quickOpen} 后输入「:行号」直接跳到那一行，输入「@」列出符号。",
	"{editor.action.addSelectionToNextFindMatch} 连按，一次选中一处相同的词，然后一起改。",
	"关错了标签？{workbench.action.reopenClosedEditor} 按原位置重新打开。",
	"鼠标中键点一下标签就能关掉它，不会切过去。",
	"把搜索结果拖到一列上，在那里打开并选中这一处。",
	"锁定一列后，打开的文件都去别的列：适合钉住终端或参考文件。",
	"面包屑里的文件夹点一下，就能在弹出的树里找同目录的文件。",
	"keybindings.json 支持 VS Code 的写法，包括 when 条件和 runCommands。",
	"Markdown 编辑框里在行首输入 /，可以插入标题、列表、表格、代码块。",
	"在 config.toml 的 [terminal] 里写 position = \"right\"，终端面板就放到右边。",
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
	e.status = "接着上次的「" + l.title + "」：第 " + strconv.Itoa(g.step+1) + " 步"
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
	e.status = "课程「" + l.title + "」：照着侧边栏的步骤做"
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
		if err := os.WriteFile(path, []byte(l.practice), 0o644); err != nil {
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
		return userError("这个窗口打开练习文件夹之前没有文件夹：用「打开文件夹」换回你的项目")
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
			e.status = "下一步：" + e.expandTip(next.text)
			if id := next.command(); e.keyHint[id] != "" {
				e.status += "（" + e.keyHint[id] + "）"
			}
		}
		return
	}
	if e.state != nil && !slices.Contains(e.state.GuideDone, l.id) {
		e.state.GuideDone = append(e.state.GuideDone, l.id)
		e.saveState()
	}
	e.status = "完成课程「" + l.title + "」"
	if e.ws != nil && inPractice(e.ws.root) && e.state != nil && e.state.GuideHome != "" {
		e.status += "：用「" + e.commands["yan.guide.leave"].Title + "」回到 " + filepath.Base(e.state.GuideHome)
		return
	}
	if next := e.nextLesson(g.active); next >= 0 {
		e.status += "：在侧边栏开始下一课「" + guideLessons[next].title + "」"
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
	e.status = "功能导览的进度已清空"
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
		dst.Lessons = append(dst.Lessons, view.LearnLesson{Module: l.module, Title: l.title, Done: d, Steps: len(l.steps),
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
			k = "「" + e.commands[id].Title + "」"
		}
		tip = tip[:i] + k + tip[j+1:]
	}
}
