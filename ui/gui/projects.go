package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/app"
)

// 项目标签，照着 Mayoi v0.8.0–v0.8.1（MIT，cmd/mayoi 的 project tabs 和它的 rail）：「显示 › 项目标签」打开后，
// 所有文件夹都在一个窗口里，左边一列方块，一个文件夹一个，方块里是它名字的头一两个字。点方块、⌃⌘] ⌃⌘[、
// ⌃⌘1…9 换项目，各自的标签、终端、资源管理器、侧边栏和面板都和离开时一样（一个项目就是原来一个窗口的
// 编辑器和前端）。打开文件夹、新建窗口、把文件夹拖到方块那一列上，都加一个项目；方块的右键菜单关项目。
// 有没保存的修改的项目，方块角上有个点。打开项目标签时所有窗口并进前面那个，关掉时每个项目分成一个窗口。
// 下次从访达、Dock 打开时，上次那些项目都回来。

// group 是一个原生窗口和它里面的项目（只在主线程上用）。
type group struct {
	win        *mygo.Window
	projects   []*Frontend
	active     int
	confirming bool
	moving     bool                      // 关这个窗口只是为了把项目挪走：别关项目
	icons      map[string]ui.ImageSource // decoded pictures chosen for squares, by file
}

// groups 是开着的窗口，按窗口找。
var groups = map[*mygo.Window]*group{}

func (g *group) cur() *Frontend { return g.projects[g.active] }

// tabsShown 报告要不要画项目那一列：开了项目标签，或者窗口里不止一个项目。
func (g *group) tabsShown() bool { return len(g.projects) > 1 || g.cur().ed.ProjectTabs() }

// view 是窗口的内容：项目那一列和现在这个项目。
func (g *group) view(c *ui.Context) {
	f := g.cur()
	if !g.tabsShown() {
		f.view(c)
		return
	}
	desktopTheme := c.Theme()
	ui.Row(c.Key("projects")).Fill().Children(func() {
		g.rail(c)
		// The rail applies its palette too; it must not become the editor's system appearance.
		c.SetTheme(desktopTheme)
		// 每个项目一个键：换项目时界面状态（输入框、滚动）不串到别的项目。
		ui.Column(c.Key(fmt.Sprintf("project:%p", f))).Grow(1).MinWidth(0).AlignSelf(ui.Stretch).Children(func() { f.view(c) })
	})
}

// railW 是项目那一列的宽度，railSq 是方块的边长。
const (
	railW  = 48
	railSq = 32
)

// rail 画项目那一列：每个项目一个方块，现在这个左边有一条强调色的竖线；最后是打开文件夹的「＋」。
func (g *group) rail(c *ui.Context) {
	f := g.cur()
	p := f.applyTheme(c)
	col := ui.Column(c.Key("rail")).Width(railW).AlignSelf(ui.Stretch).Padding(titleBarH, 0, 8, 0).Gap(6).AlignItems(ui.Center).
		Background(p.chrome).BorderWidth(0, 1, 0, 0).BorderColor(p.border).Label("项目")
	folders := transfer.DropOptions{Formats: []transfer.Format{transfer.FileList}}
	if _, over := ui.DataDragOver(col, folders); over {
		col.Background(p.accent.Alpha(0.12))
	}
	if drop, ok := ui.DropData(col, folders); ok { // 把文件夹拖进来：每个一个项目
		if paths, err := drop.Data.Files(); err == nil {
			for _, path := range paths {
				if fi, err := os.Stat(path); err == nil && fi.IsDir() {
					g.add(path)
				}
			}
		}
	}
	col.Children(func() {
		names := make([]string, len(g.projects))
		for i, pf := range g.projects {
			names[i] = projectName(pf.ed.Root())
		}
		labels := railLabels(names)
		for i, pf := range g.projects {
			g.square(c, p, i, pf, labels[i])
		}
		add := ui.Box(c.Key("rail:add")).Size(railSq, railSq).Radius(8).Justify(ui.Center).AlignItems(ui.Center).
			Label("打开文件夹…").Tooltip("打开文件夹…（加一个项目）")
		fg := p.muted
		if add.Hovered() {
			add.Background(p.hover)
			fg = p.text
		}
		add.Children(func() { ui.Icon(c, iconPlus).FontSize(15).TextColor(fg) })
		if add.Clicked() {
			g.chooseFolder()
		}
	})
}

// square 是第 i 个项目的方块。
func (g *group) square(c *ui.Context, p *palette, i int, pf *Frontend, label string) {
	root := pf.ed.Root()
	name := projectName(root)
	mark, color := pf.ed.ProjectLook(root)
	if mark != "" {
		label = mark
	}
	// A chosen mark wins, then a chosen picture, then the folder's own logo, then the initials.
	var pic ui.ImageSource
	if mark == "" {
		if pic = g.iconImage(pf.ed.ProjectIcon(root)); pic == nil {
			pic = g.logoFor(pf, root)
		}
	}
	on := i == g.active
	tip := name
	if root != "" {
		tip += "\n" + root
	}
	if pf.attention != "" {
		tip += "\n" + pf.attention
	}
	if k := projectKey(pf, i); k != "" {
		tip += "\n" + k
	}
	ui.Row(c.Key(fmt.Sprintf("rail:%p", pf))).Width(railW).Height(railSq).AlignItems(ui.Center).Children(func() {
		bar := ui.Box(c).Width(3).Height(18).Radius(2) // 现在这个：左边一条竖线
		if on {
			bar.Background(p.accent)
		}
		ui.Box(c).Width((railW - railSq - 6) / 2)
		sq := ui.Box(c).Size(railSq, railSq).Radius(8).Justify(ui.Center).AlignItems(ui.Center).Label("项目 " + name).Tooltip(tip).
			Drag(railDrag{pf}) // 拖到别的方块上：挪到那里
		if d, over := ui.DragOver[railDrag](sq); over && d.f != pf {
			bar.Background(p.accent.Alpha(0.6))
		}
		if d, ok := ui.Drop[railDrag](sq); ok {
			g.move(d.f, i)
		}
		// A picture dropped on a square becomes its icon; folders still open as projects.
		files := transfer.DropOptions{Formats: []transfer.Format{transfer.FileList}}
		if _, over := ui.DataDragOver(sq, files); over {
			bar.Background(p.accent.Alpha(0.6))
		}
		if drop, ok := ui.DropData(sq, files); ok {
			if paths, err := drop.Data.Files(); err == nil {
				for _, path := range paths {
					if fi, err := os.Stat(path); err == nil && fi.IsDir() {
						g.add(path)
					} else if len(paths) == 1 && root != "" && app.IsIconFile(path) {
						g.setProjectIcon(pf, root, path)
					}
				}
			}
		}
		if pf.attention != "" { // 终端响铃、发了通知：描上强调色
			sq.Border(2, p.accent)
		}
		lane := projectColor(root)
		if color > 0 {
			lane = color - 1
		}
		tint := p.lane(lane)
		switch {
		case on:
			sq.Background(tint.Alpha(0.9))
		case sq.Hovered():
			sq.Background(tint.Alpha(0.55))
		default:
			sq.Background(tint.Alpha(0.3))
		}
		sq.Children(func() {
			fg := p.text
			if on {
				fg = p.inverseText
			}
			if pic != nil {
				ui.Image(c, pic).Size(railSq-10, railSq-10).Fit(ui.Contain)
			} else {
				ui.Text(c, label).FontSize(12).FontWeight(700).TextColor(fg)
			}
			if pf.ed.Dirty() { // 有没保存的修改：角上一个点
				ui.Box(c).Absolute().Top(-2).Right(-2).Size(9, 9).Radius(4.5).Background(p.modified).Border(1.5, p.chrome)
			}
		})
		if sq.Clicked() {
			g.show(i)
		}
		sq.ContextMenu(func(m *ui.Menu) {
			if root != "" {
				if m.Item("在" + app.FileManager(hostOS) + "中显示").Chosen() {
					mygo.Shell.ShowItemInFolder(root)
				}
				if m.Item("复制路径").Chosen() {
					host{pf}.SetClipboard(root)
				}
				m.Separator()
			}
			if root != "" && m.Item("自定义标识与颜色…").Chosen() {
				g.show(i)
				pf.ed.Run("yan.project.customize")
			}
			if root != "" && m.Item("选择图标…").Chosen() {
				g.chooseProjectIcon(pf, root)
			}
			if pf.ed.ProjectIcon(root) != "" && m.Item("重置图标").Chosen() {
				pf.ed.ClearProjectIcon(root)
				g.icons = nil
			}
			if m.Item("关闭项目").Chosen() {
				g.close(i)
			}
			if m.Item("关闭其他项目").Disabled(len(g.projects) < 2).Chosen() {
				g.closeOthers(i)
			}
		})
	})
}

// projectKey 是换到第 i 个项目的键的写法。
func projectKey(f *Frontend, i int) string {
	if !f.Profile().MacKeys() || i >= 9 {
		return "" // Windows、Linux 上还没有键（Ctrl+Alt 是 AltGr）
	}
	return fmt.Sprintf("⌃⌘%d", i+1)
}

// projectName 是项目的名字：文件夹的名字，没打开文件夹时是「未打开」。
func projectName(root string) string {
	if root == "" {
		return "未打开"
	}
	return filepath.Base(root)
}

// projectInitials 是方块里的字：几个词的名字取头两个词的头一个字母，一个词的取头两个字母，
// 中文取头一个字（照着 Mayoi 的 railInitials，MIT，46990a1）。
func projectInitials(name string) string {
	label, _ := projectLabel(name)
	return label
}

// projectLabel returns a project's label on its own, and the letters of its name in lower case,
// which railLabels tells clashing labels apart by.
func projectLabel(name string) (string, []rune) {
	words := strings.Fields(projectWords(name))
	var letters []rune
	for _, w := range words {
		for _, r := range w {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				letters = append(letters, unicode.ToLower(r))
			}
		}
	}
	switch {
	case len(letters) == 0:
		r, _ := utf8.DecodeRuneInString(name)
		return string(r), letters
	case unicode.Is(unicode.Han, letters[0]):
		return string(letters[0]), letters
	case len(words) >= 2:
		var out []rune
		for _, w := range words {
			for _, r := range w {
				if unicode.IsLetter(r) || unicode.IsDigit(r) {
					out = append(out, unicode.ToUpper(r))
					break
				}
			}
			if len(out) == 2 {
				break
			}
		}
		return string(out), letters
	case len(letters) >= 2:
		return string(unicode.ToUpper(letters[0])) + string(letters[1]), letters
	}
	return string(unicode.ToUpper(letters[0])), letters
}

// railLabels are the labels of projects named names, in order. Where two would read the same,
// each takes its first letter and the first after it that tells it apart, as "Dw" and "Dc" for
// Downloads and Documents; names that are the same keep the same label. Ported from Mayoi
// (MIT, cmd/mayoi/projects.go railLabels at 46990a1).
func railLabels(names []string) []string {
	labels := make([]string, len(names))
	letters := make([][]rune, len(names))
	same := map[string][]int{}
	for i, name := range names {
		labels[i], letters[i] = projectLabel(name)
		same[labels[i]] = append(same[labels[i]], i)
	}
	for _, group := range same {
		if len(group) < 2 {
			continue
		}
		for _, i := range group {
			for j := 1; j < len(letters[i]); j++ {
				r := letters[i][j]
				clash := false
				for _, o := range group {
					if slices.Equal(letters[o], letters[i]) {
						continue
					}
					if j < len(letters[o]) && letters[o][j] == r {
						clash = true
						break
					}
				}
				if !clash {
					labels[i] = string(unicode.ToUpper(letters[i][0])) + string(r)
					break
				}
			}
		}
	}
	return labels
}

// projectWords spells a folder's name as words: "my-app" as "my app", "mayoiEditor" as
// "mayoi Editor".
func projectWords(name string) string {
	var b strings.Builder
	prev := rune(0)
	for _, r := range name {
		switch {
		case r == '-' || r == '_' || r == '.':
			r = ' '
		case r >= 'A' && r <= 'Z' && prev >= 'a' && prev <= 'z':
			b.WriteRune(' ')
		}
		b.WriteRune(r)
		prev = r
	}
	return b.String()
}

// projectColor 是项目方块的颜色序号：同一个文件夹总是同一种。
func projectColor(root string) int {
	h := 0
	for _, b := range []byte(root) {
		h = h*31 + int(b)
	}
	return max(h, -h) % 6
}

// markAttention 在项目不在眼前时记下它要人看一眼的原因（终端响铃、通知）。
func (f *Frontend) markAttention(why string) {
	if g := f.g; g != nil && g.cur() != f {
		f.attention = why
	}
}

// railDrag 是在项目那一列里拖着的方块（只在这个应用里拖）。
type railDrag struct{ f *Frontend }

// move 把项目 f 挪到第 to 个位置（拖方块排序），现在这个跟着它走。
func (g *group) move(f *Frontend, to int) {
	from := slices.Index(g.projects, f)
	if from < 0 || to < 0 || to >= len(g.projects) || from == to {
		return
	}
	cur := g.cur()
	g.projects = slices.Delete(g.projects, from, from+1)
	g.projects = slices.Insert(g.projects, to, f)
	g.active = slices.Index(g.projects, cur)
	g.saveProjects()
}

// show 换到第 i 个项目：离开的那个像窗口失去焦点一样自动保存，换上的那个看看文件在外面有没有被改过。
func (g *group) show(i int) {
	if i < 0 || i >= len(g.projects) || i == g.active {
		return
	}
	old := g.cur()
	g.active = i
	f := g.cur()
	f.attention = ""
	if g.win != nil {
		windows[g.win] = f
		setAppMenu(f.menu())
	}
	old.focusChanged(false)
	f.focusChanged(true)
}

// attach 把项目 f（编辑器 ed）放进这个窗口，接上它的后台通知。在主线程上调用。
func (g *group) attach(f *Frontend, ed *app.Editor) {
	f.ed, f.g, f.win = ed, g, g.win
	f.winp.Store(g.win)
	if f.done != nil {
		return // 从别的窗口挪过来：通知已经接着，跟着 winp 走
	}
	f.done = make(chan struct{})
	done := f.done
	go func() {
		// 后台 job 用 Post 把结果寄回内核；这里把「门铃」转成项目所在窗口主线程上的一次 RunPending 和重画。
		for {
			select {
			case <-ed.Wake():
				if w := f.winp.Load(); w != nil {
					w.Update(ed.RunPending)
				}
			case <-done:
				return
			}
		}
	}()
}

// add 加一个项目，打开文件夹 root（"" 是欢迎页），换过去。
func (g *group) add(root string) {
	if makeWindow == nil {
		return
	}
	for i, pf := range g.projects {
		if root != "" && sameFolder(pf.ed.Root(), root) {
			g.show(i) // 已经开着
			return
		}
	}
	if root != "" {
		if found := g.cur().folderProject(root); found != nil {
			found.bringForward()
			return
		}
		if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
			g.cur().ed.Notify("无法打开文件夹：" + root)
			return
		}
	}
	ed, f := makeWindow()
	if root != "" {
		if err := ed.SwitchFolder(root); err != nil {
			g.cur().ed.Notify(err.Error())
			ed.Close()
			return
		}
	}
	g.attach(f, ed)
	g.projects = append(g.projects, f)
	g.show(len(g.projects) - 1)
	g.saveProjects()
	g.cur().refreshDock()
}

// chooseFolder 选一个文件夹，加成项目。
func (g *group) chooseFolder() {
	f := g.cur()
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: g.win, Title: "打开文件夹", ButtonLabel: "打开", Directory: true, CreateDirectories: true,
		})
		if err == nil && len(paths) > 0 {
			f.do(func() { g.add(paths[0]) })
		}
	}()
}

// close 关掉第 i 个项目（有没保存的修改时先问）；只剩它时关窗口。
func (g *group) close(i int) {
	if i < 0 || i >= len(g.projects) {
		return
	}
	if len(g.projects) == 1 {
		if g.win != nil {
			g.win.Close()
		}
		return
	}
	f := g.projects[i]
	if f.ed.AutoSave() {
		f.ed.Run("workbench.action.files.saveAll")
	}
	dirty, lost := f.ed.Unsaved()
	if !dirty {
		g.remove(f)
		return
	}
	detail := "不保存的话，修改会丢失。"
	if lost != nil {
		detail = "未命名的文件没能留到下次（" + lost.Error() + "）：先另存为它们，或者选「不保存」放弃。" + detail
	}
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent: g.win, Type: mygo.MessageWarning,
			Message: projectName(f.ed.Root()) + " 里有文件还没保存，要先保存吗？",
			Detail:  detail,
			Buttons: []string{"全部保存", "不保存", "取消"}, DefaultButton: 0, CancelButton: 2,
		})
		if err != nil || res.Button == 2 {
			return
		}
		f.do(func() {
			if res.Button == 0 {
				f.ed.Run("workbench.action.files.saveAll")
				if dirty, _ := f.ed.Unsaved(); dirty {
					return // 有文件保存失败：状态栏里有原因
				}
			}
			g.remove(f)
		})
	}()
}

// closeOthers 关掉第 i 个以外的项目（没保存的那些各问一次）。
func (g *group) closeOthers(i int) {
	keep := g.projects[i]
	for _, pf := range slices.Clone(g.projects) {
		if pf != keep {
			g.close(slices.Index(g.projects, pf))
		}
	}
}

// remove 关掉项目 f：它的终端、会话、后台的 git 和搜索。
func (g *group) remove(f *Frontend) {
	i := slices.Index(g.projects, f)
	if i < 0 {
		return
	}
	shown := i == g.active
	g.projects = slices.Delete(g.projects, i, i+1)
	if g.active > i || g.active >= len(g.projects) {
		g.active = max(g.active-1, 0)
	}
	f.shutdown()
	if g.win != nil {
		windows[g.win] = g.cur()
		setAppMenu(g.cur().menu())
	}
	if shown {
		g.cur().focusChanged(true)
	}
	g.saveProjects()
}

// shutdown 关掉一个项目：停后台通知，关终端，记下会话。
func (f *Frontend) shutdown() {
	if f.done != nil {
		close(f.done)
		f.done = nil
	}
	f.closeTerminals()
	f.ed.SaveSession()
	f.ed.Close()
}

// saveProjects 用项目标签时记下这个窗口里的文件夹，下次打开时都回来。
func (g *group) saveProjects() {
	f := g.cur()
	if !f.ed.ProjectTabs() {
		return
	}
	var roots []string
	for _, pf := range g.projects {
		if r := pf.ed.Root(); r != "" {
			roots = append(roots, r)
		}
	}
	f.ed.SaveProjects(roots)
}

// restoreProjects 用项目标签时把上次记下的文件夹都开回来（启动时）。
func (g *group) restoreProjects() {
	f := g.cur()
	if !f.ed.ProjectTabs() {
		return
	}
	first := f.ed.Root()
	for _, r := range f.ed.SavedProjects() {
		if fi, err := os.Stat(r); err != nil || !fi.IsDir() || r == first {
			continue
		}
		if first == "" { // 从访达、Dock 打开时第一个窗口什么都没开：它就是上次的第一个项目
			if err := f.ed.OpenFolder(r); err == nil {
				f.ed.RestoreSession()
				first = r
				continue
			}
		}
		g.add(r)
	}
	for i, pf := range g.projects {
		if pf.ed.Root() == first {
			g.show(i)
		}
	}
}

// restoreDrafts 找回没开文件夹的项目上次留下的未命名文件：每份备份一个项目，启动时没开文件夹的
// 那个先接第一份。开着文件夹的项目在打开文件夹时自己找回。
func (g *group) restoreDrafts() {
	ids, problems := app.RootlessDrafts()
	for _, err := range problems {
		g.cur().ed.Notify("未命名文件的备份读不出来，已移到一边：" + err.Error())
	}
	for i, id := range ids {
		if i > 0 || g.cur().ed.Root() != "" {
			n := len(g.projects)
			g.add("")
			if len(g.projects) == n {
				return // 开不了新项目（测试里没有窗口）
			}
		}
		g.cur().ed.AdoptDrafts(id)
	}
}

// focused 是前面那个窗口；没有时是随便一个。
func focusedGroup() *group {
	if g := groups[mygo.FocusedWindow()]; g != nil {
		return g
	}
	for _, g := range groups {
		return g
	}
	return nil
}

// setProjectTabs 打开时把所有窗口的项目并进前面那个窗口，关掉时每个项目分成一个窗口。
func setProjectTabs(on bool) {
	into := focusedGroup()
	if into == nil {
		return
	}
	if on {
		for _, g := range slices.Collect(func(yield func(*group) bool) {
			for _, g := range groups {
				if g != into && !yield(g) {
					return
				}
			}
		}) {
			for _, pf := range g.projects {
				into.attach(pf, pf.ed)
				into.projects = append(into.projects, pf)
			}
			g.projects, g.moving = nil, true
			g.win.Destroy()
		}
		into.saveProjects()
		return
	}
	x, y := into.win.Position()
	for _, g := range slices.Collect(func(yield func(*group) bool) {
		for _, g := range groups {
			if !yield(g) {
				return
			}
		}
	}) {
		keep := g.cur()
		for _, pf := range slices.Clone(g.projects) {
			if pf == keep {
				continue
			}
			g.projects = slices.DeleteFunc(g.projects, func(o *Frontend) bool { return o == pf })
			x, y = x+28, y+28
			pf.openWindow(pf.ed, x, y)
		}
		g.active = 0
		windows[g.win] = keep
	}
}

// ShowProject、StepProject、CloseProject、SetProjectTabs 实现 app.ProjectHost。
func (h host) ShowProject(i int) {
	if g := h.f.g; g != nil {
		if i < 0 {
			i = len(g.projects) - 1
		}
		g.show(i)
	}
}

func (h host) StepProject(delta int) {
	if g := h.f.g; g != nil && len(g.projects) > 1 {
		n := len(g.projects)
		g.show(((g.active+delta)%n + n) % n)
	}
}

func (h host) CloseProject() {
	if g := h.f.g; g != nil {
		g.close(g.active)
	}
}

func (h host) SetProjectTabs(on bool) { setProjectTabs(on) }
