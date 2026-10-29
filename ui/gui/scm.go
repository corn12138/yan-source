package gui

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

// 源代码管理视图，样子照着 Mayoi（cmd/mayoi/scm.go）：分支和拉取推送按钮，提交说明，提交按钮
// （旁边的箭头里有提交并推送、修改上次提交、撤销上次提交），下面是分组的改动。

// sidebarSwitch 是侧边栏顶上切换视图的按钮；count 大于 0 时右上角显示数字。
// sidebarSwitch 是侧边栏顶上的一个视图：名字，改动的数目；label 带着快捷键，指针停着时显示。
func (f *Frontend) sidebarSwitch(c *ui.Context, name, label string, on bool, count int, v view.SideView) {
	p := f.pal
	b := ui.Row(c.Key(label)).Height(24).Padding(0, 8).Gap(5).Radius(5).Shrink(0).AlignItems(ui.Center).Label(label).Tooltip(label)
	col := p.muted
	if on {
		b.Background(p.active)
		col = p.text
	} else if b.Hovered() {
		b.Background(p.hover)
		col = p.text
	}
	b.Children(func() {
		ui.Text(c, name).FontSize(13).SingleLine().TextColor(col)
		if count > 0 {
			label := fmt.Sprint(count)
			if count > 99 {
				label = "99+"
			}
			ui.Text(c, label).FontSize(11).FontWeight(500).Height(16).Padding(0, 4).Radius(8).MinWidth(16).
				TextAlign(ui.Center).Background(p.accent.Alpha(0.14)).TextColor(p.accent)
		}
	})
	if b.Clicked() {
		f.ed.ShowSidebarView(int(v))
	}
}

func (f *Frontend) scmPanel(c *ui.Context, s *view.SCM, multi bool) {
	ed := repositoryEditor{Editor: f.ed, id: s.RepositoryID}
	state := f.scmState(s.RepositoryID)
	p := f.pal
	if s.Busy != "" {
		ui.Progress(c.Key("scm-activity"), -1).Height(2).Radius(0).Label("git 运行中")
	} else {
		ui.Box(c.Key("scm-activity")).Height(2)
	}
	if s.NoRepo {
		if f.nestedRepositories(c, s) {
			return
		}
		ui.Column(c).Padding(28, 20, 16, 20).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconBranch).FontSize(22).TextColor(p.muted)
			ui.Text(c, "这个文件夹不在 git 仓库里").FontSize(13).FontWeight(600).TextColor(p.text)
			b := ui.Box(c.Key("git-init")).Height(28).Padding(0, 14).Radius(6).Justify(ui.Center).AlignItems(ui.Center).
				Background(c.Theme().Accent).Label("初始化仓库")
			if b.Hovered() {
				b.Background(c.Theme().AccentHover)
			}
			b.Children(func() { ui.Text(c, "初始化仓库").FontSize(12.5).FontWeight(600).TextColor(c.Theme().AccentText) })
			if b.Clicked() {
				ed.Run("git.init")
			}
		})
		return
	}
	f.scmBranchRow(c, s)
	ui.Column(c).Padding(2, 12, 10, 12).Gap(8).Children(func() {
		if s.Op != "" {
			f.scmOpBanner(c, s)
		}
		if s.Amend {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(4, 4, 4, 8).Radius(5).Background(p.accent.Alpha(0.12)).Children(func() {
				ui.Icon(c, iconPencil).FontSize(12).TextColor(p.accent)
				ui.Text(c, "正在修改上次提交").FontSize(11.5).TextColor(p.text).Grow(1).SingleLine().Shrink(1)
			})
		}
		if state.scmMessage != s.Message && !state.scmEditing {
			state.scmMessage = s.Message
		}
		state.scmEditing = false
		hint := "提交说明（" + f.modKeys(keymap.KeyEnter) + " 提交）"
		if s.Branch != "" {
			hint = fmt.Sprintf("提交说明（%s 提交到 %s）", f.modKeys(keymap.KeyEnter), s.Branch)
		}
		label := "提交说明"
		if multi {
			label += " · " + f.repositoryLabel(s.Root)
		}
		box := ui.TextArea(c.Key("commit-message"), &state.scmMessage).Bind(&state.scmMessageIn).Placeholder(hint).FontSize(12.5).
			MinHeight(60).MaxHeight(160).Label(label)
		if box.Changed() {
			state.scmEditing = true
			ed.SetCommitMessage(state.scmMessage)
		}
		// 内核要把键盘交给提交说明（修改上次提交、终端里按 Tab）时先要焦点；之后焦点在哪儿以输入框为准。
		switch focused := box.Focused(); {
		case focused && !s.MessageFocused:
			ed.FocusCommitMessage(true)
		case !focused && s.MessageFocused && !state.scmFocusAsked:
			state.scmMessageIn.Focus(c)
			state.scmFocusAsked = true
		case !focused && s.MessageFocused:
			ed.FocusCommitMessage(false)
			state.scmFocusAsked = false
		default:
			state.scmFocusAsked = false
		}
		if box.Shortcut(scmSubmitModifier(f.Profile()), ui.KeyEnter) {
			ed.Run("git.commit")
		}
		f.subjectLength(c, s.Message)
		f.commitButton(c, s)
	})
	drawList := func() {
		if len(s.Groups) == 0 {
			ui.Text(c, "没有改动").FontSize(12).TextColor(p.muted).Padding(8, 12)
		} else { // 在一个标签里看所有改了的文件（照着 Mayoi 的 Changes 标签）
			b := ui.Row(c.Key("all-changes")).Height(22).Padding(0, 8).Gap(6).AlignItems(ui.Center).Radius(4).Label("打开所有改动").
				Tooltip("在一个标签里看所有改了的文件：j k 上下，] [ 改动，. , 文件，s 暂存，z 折起")
			col := p.muted
			if b.Hovered() {
				b.Background(p.hover)
				col = p.text
			}
			b.Children(func() {
				ui.Icon(c, iconColumns).FontSize(12).TextColor(col)
				ui.Text(c, "打开所有改动").FontSize(12).TextColor(col)
			})
			if b.Clicked() {
				ed.Run("git.openAllChanges")
			}
		}
		i := 0
		for _, g := range s.Groups {
			f.scmGroupHeader(c, ed.id, g)
			for _, r := range g.Rows {
				f.scmRow(c, ed.id, g, r, i, i == s.Selected)
				i++
			}
		}
		if !multi {
			f.scmHistory(c, s)
		}
		f.scmRepositoryPanel(c, s)
	}
	if multi {
		ui.Column(c.Key("scm-list")).Padding(0, 6, 8, 6).Children(drawList)
	} else {
		ui.Scroll(c.Key("scm-list")).Grow(1).Padding(0, 6, 8, 6).Children(drawList)
	}
}

// sectionToggle is a section title of Source Control that folds and unfolds it: the chevron
// and the title. It grows to take the header's free width, so the buttons beside it stay buttons.
func (f *Frontend) sectionToggle(c *ui.Context, key, title string, folded bool, toggle func()) {
	p := f.pal
	label := "折起" + title
	if folded {
		label = "展开" + title
	}
	t := ui.Row(c.Key(key)).Gap(4).AlignItems(ui.Center).Grow(1).MinWidth(0).Height(22).Label(label).Cursor(ui.CursorPointer)
	t.Children(func() {
		icon := iconChevronDown
		if folded {
			icon = iconChevronRight
		}
		ui.Icon(c, icon).FontSize(12).TextColor(p.muted)
		ui.Text(c, title).FontSize(11).FontWeight(600).LetterSpacing(0.4).TextColor(p.muted).SingleLine().Shrink(1)
	})
	if t.Clicked() {
		toggle()
	}
}

// scmHistoryHeader 是提交历史的标题：旁边的按钮切换显示所有分支，还是只显示检出的分支。
func (f *Frontend) scmHistoryHeader(c *ui.Context, s *view.SCM) {
	ed := repositoryEditor{Editor: f.ed, id: s.RepositoryID}

	p := f.pal
	ui.Row(c.Key("group:commits")).Gap(4).AlignItems(ui.Center).Height(22).Padding(6, 4, 0, 4).Children(func() {
		f.sectionToggle(c, "fold:commits", "提交", s.CommitsFolded, func() { ed.SCMToggleSection("commits") })
		if s.HistoryLoading {
			ui.Text(c, "读取中…").FontSize(10.5).TextColor(p.faint)
		}
		ui.Spacer(c)
		label := "显示所有分支"
		if s.AllBranches {
			label = "只显示当前分支"
		}
		b := ui.Box(c.Key("all-branches")).Size(20, 20).Radius(4).Justify(ui.Center).AlignItems(ui.Center).Label(label).Tooltip(label)
		col := p.muted
		if s.AllBranches {
			col = p.accent
			b.Background(p.accent.Alpha(0.12))
		}
		if b.Hovered() {
			b.Background(p.hover)
			col = p.text
		}
		b.Children(func() { ui.Icon(c, iconBranch).FontSize(13).TextColor(col) })
		if b.Clicked() {
			ed.Run("yan.scm.toggleAllBranches")
		}
	})
}

// graphLaneW 是提交图两道之间隔多远，graphMaxLanes 是最多画几道：再往右的贴着最后一道画（照着 Mayoi）。
const (
	graphLaneW    = 10
	graphMaxLanes = 6
)

// scmCommit 是提交历史的一行：提交图，分支和标签，提交说明的第一行，没推送的标记，多久以前。
// 单击在预览标签里打开它（说明、作者、日期，再是它改的每个文件）；右键是能对它做的事。
func (f *Frontend) scmCommit(c *ui.Context, id uint64, cm *view.SCMCommit, i int, selected bool, lanes int) {
	ed := repositoryEditor{Editor: f.ed, id: id}
	if cm.File != nil {
		f.scmCommitFile(c, ed, cm, i, selected)
		return
	}

	p := f.pal
	tip := fmt.Sprintf("%s\n%s · %s · %s", cm.Subject, cm.Author, cm.When, cm.Short)
	if cm.Unpushed {
		tip += " · 还没推送"
	}
	if cm.Merge {
		tip += " · 合并"
	}
	row := ui.Row(c.Key("commit:"+cm.Hash)).Gap(6).AlignItems(ui.Center).Height(22).Padding(0, 4, 0, 2).Radius(4).Tooltip(tip)
	switch {
	case selected && f.frame.Focus == view.FocusSidebar:
		row.Background(p.active)
	case row.Hovered():
		row.Background(p.hover)
	}
	if row.Clicked() {
		ed.act(func(e *app.Editor) { e.SCMOpenCommit(cm.Hash, false) })
	}
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item("打开").Chosen() {
			ed.act(func(e *app.Editor) { e.SCMOpenCommit(cm.Hash, true) })
		}
		if m.Item("检出、合并、变基、重置、还原、拣选…").Chosen() {
			ed.SCMCommitCommand(cm.Hash, "yan.scm.commitActions")
		}
		m.Separator()
		for _, action := range []struct{ id, label string }{
			{"yan.git.exportPatch", "导出补丁…"},
			{"yan.git.copyPatch", "复制改动补丁"},
			{"yan.git.compareHeadToCommit", "比较 HEAD → 此提交"},
			{"yan.git.compareCommitToHead", "比较此提交 → HEAD"},
			{"yan.git.selectCompare", "选为比较起点"},
			{"yan.git.compareSelected", "与选定起点比较"},
			{"yan.git.openCommitRemote", "在远端打开提交"},
		} {
			disabled := (action.id == "yan.git.compareHeadToCommit" || action.id == "yan.git.compareCommitToHead") && cm.Head
			if action.id == "yan.git.compareSelected" {
				start := ed.ComparisonStart()
				disabled = start == "" || start == cm.Hash
			}
			if m.Item(action.label).Disabled(disabled).Chosen() {
				ed.SCMCommitCommand(cm.Hash, action.id)
			}
		}

	})
	lane := p.lane(cm.Color)
	row.Children(func() {
		arrow := iconChevronRight
		if cm.Expanded {
			arrow = iconChevronDown
		}
		if f.smallButton(c, arrow, "展开/折叠提交文件 "+cm.Short) {
			f.root.Focus(c)
			ed.act(func(e *app.Editor) { e.SCMToggleCommitFiles(cm.Hash) })
		}
		if cm.FilesLoading {
			ui.Text(c, "读取中…").FontSize(10).TextColor(p.muted)
		}
		if cm.FilesError != "" {
			ui.Text(c, cm.FilesError).FontSize(10).TextColor(p.danger)
		}
		ui.Box(c).Size(graphLaneW*float32(lanes-1)+12, 22).Shrink(0).Draw(func(pt *ui.Painter, b ui.Rect) {
			f.paintGraphRow(pt, b, cm)
		})
		const shown = 2
		for k, r := range cm.Refs {
			if k == shown {
				ui.Text(c, fmt.Sprintf("+%d", len(cm.Refs)-k)).FontSize(10.5).TextColor(p.muted).Padding(0, 4).
					Height(16).Radius(4).Background(p.hover).Shrink(0)
				break
			}
			f.refBadge(c, r, lane)
		}
		col := p.text
		switch {
		case cm.Merge:
			col = p.muted
		case !cm.Unpushed && !cm.Head:
			col = p.text.Alpha(0.75)
		}
		ui.Text(c, cm.Subject).FontSize(12.5).TextColor(col).SingleLine().Shrink(1).MinWidth(24)
		ui.Spacer(c)
		if cm.Unpushed {
			ui.Icon(c, iconArrowUp).FontSize(11).TextColor(p.accent).Shrink(0)
		}
		ui.Text(c, cm.When).FontSize(11).TextColor(p.faint).SingleLine().Shrink(0)
	})
}

// refBadge 是提交上的分支或标签，用它那一道的颜色：远程分支淡一些，标签用普通的颜色。
func (f *Frontend) refBadge(c *ui.Context, r view.SCMRef, lane ui.Color) {
	p := f.pal
	fg, bg, icon := lane, lane.Alpha(0.16), iconBranch
	switch r.Kind {
	case view.RefRemote:
		fg, bg = lane.Alpha(0.7), lane.Alpha(0.08)
	case view.RefTag:
		fg, bg, icon = p.text, p.hover, iconTag
	case view.RefHead:
		icon = nil
	}
	weight := 500
	if r.Current || r.Kind == view.RefHead {
		weight = 700
	}
	label := r.Name
	if len(r.Remotes) > 0 {
		label += " ⇄ " + strings.Join(r.Remotes, ", ")
	}
	ui.Row(c).Gap(3).AlignItems(ui.Center).Height(16).Padding(0, 5, 0, 4).Radius(4).Background(bg).
		MaxWidth(110).Shrink(0).Children(func() {
		if icon != nil {
			ui.Icon(c, icon).FontSize(10).TextColor(fg).Shrink(0)
		}
		ui.Text(c, label).FontSize(11).FontWeight(weight).TextColor(fg).SingleLine().Shrink(1)
	})
}

// paintGraphRow 在 b 里画一行的提交图：通到这个提交的道连到它的点，通到父提交的从点上分出去，
// 别的直通；再画点：HEAD 是圈，合并是空心的点（照着 Mayoi 的 paintGraphRow）。
func (f *Frontend) paintGraphRow(pt *ui.Painter, b ui.Rect, cm *view.SCMCommit) {
	p := f.pal
	x := func(i int) float32 { return b.X + 6 + graphLaneW*float32(min(i, graphMaxLanes-1)) }
	top, mid, bottom := b.Y, b.Y+b.H/2, b.Y+b.H
	line := func(x1, y1, x2, y2 float32, color int) {
		var path ui.Path
		path.MoveTo(x1, y1)
		if x1 == x2 {
			path.LineTo(x2, y2)
		} else {
			ym := (y1 + y2) / 2
			path.CubeTo(x1, ym, x2, ym, x2, y2)
		}
		pt.StrokePath(&path, 1.5, p.lane(color))
	}
	for i, color := range cm.Above {
		if color < 0 {
			continue
		}
		if cm.Joins[i] {
			line(x(i), top, x(cm.Lane), mid, color)
		} else {
			line(x(i), top, x(i), mid, color)
		}
	}
	for i, color := range cm.Below {
		if color < 0 {
			continue
		}
		if cm.Through[i] || !cm.From[i] {
			line(x(i), mid, x(i), bottom, color)
		}
		if cm.From[i] {
			line(x(cm.Lane), mid, x(i), bottom, color)
		}
	}
	col := p.lane(cm.Color)
	cx := x(cm.Lane)
	switch {
	case cm.Head:
		dot := ui.Rect{X: cx - 5, Y: mid - 5, W: 10, H: 10}
		pt.Fill(dot, p.chrome, 5)
		pt.Stroke(dot, col, 5, 2)
	case cm.Merge:
		dot := ui.Rect{X: cx - 3.5, Y: mid - 3.5, W: 7, H: 7}
		pt.Fill(dot, p.chrome, 3.5)
		pt.Stroke(dot, col, 3.5, 1.5)
	default:
		pt.Fill(ui.Rect{X: cx - 4, Y: mid - 4, W: 8, H: 8}, col, 4)
	}
}

// subjectLength 在提交说明第一行超过 50 个字时说它多长，超过 72 个字时用警告色（照着 Mayoi）。
func (f *Frontend) subjectLength(c *ui.Context, msg string) {
	subject, _, _ := strings.Cut(msg, "\n")
	n := utf8.RuneCountInString(strings.TrimRight(subject, " \t\r"))
	if n <= 50 {
		return
	}
	col := f.pal.muted
	if n > 72 {
		col = f.pal.modified
	}
	ui.Text(c, fmt.Sprintf("第一行 %d 个字", n)).FontSize(11).TextColor(col).TextAlign(ui.End)
}

func (f *Frontend) scmBranchRow(c *ui.Context, s *view.SCM) {
	ed := repositoryEditor{Editor: f.ed, id: s.RepositoryID}

	p := f.pal
	ui.Row(c).Height(30).Padding(0, 8, 0, 8).Gap(2).AlignItems(ui.Center).Children(func() {
		branch := s.Branch
		if branch == "" {
			branch = "没有分支"
		}
		b := ui.Row(c.Key("scm-branch")).Height(24).Padding(0, 6).Gap(5).Radius(5).AlignItems(ui.Center).
			Shrink(1).Label("分支 " + branch).Tooltip("切换分支…")
		if b.Hovered() || b.Pressed() {
			b.Background(p.hover)
		}
		b.Children(func() {
			ui.Icon(c, iconBranch).FontSize(13).TextColor(p.muted)
			ui.Text(c, branch).FontSize(12.5).FontWeight(500).TextColor(p.text).SingleLine().Shrink(1)
			ui.Icon(c, iconChevronDown).FontSize(11).TextColor(p.faint)
		})
		if b.Clicked() {
			ed.Run("git.checkout")
		}
		ui.Spacer(c)
		if f.syncButton(c, "fetch", iconFetch, "抓取", 0).Clicked() {
			ed.Run("git.fetch")
		}
		pull, push := "拉取", "推送"
		if !s.Upstream {
			push = "发布分支"
		}
		if s.Behind > 0 {
			pull = fmt.Sprintf("拉取 %d 个提交", s.Behind)
		}
		if s.Ahead > 0 {
			push = fmt.Sprintf("推送 %d 个提交", s.Ahead)
		}
		if f.syncButton(c, "pull", iconPull, pull, s.Behind).Clicked() {
			ed.Run("git.pull")
		}
		if f.syncButton(c, "push", iconPush, push, s.Ahead).Clicked() {
			ed.Run("git.push")
		}
	})
}

// syncButton 是拉取、推送这类按钮；有要传的提交时图标旁写着个数，画成强调色。
func (f *Frontend) syncButton(c *ui.Context, key string, icon *ui.SVG, label string, count int) ui.Element {
	p := f.pal
	b := ui.Row(c.Key("sync:"+key)).Height(24).MinWidth(26).Padding(0, 5).Gap(3).Radius(5).
		Justify(ui.Center).AlignItems(ui.Center).Label(label).Tooltip(label)
	col := p.muted
	if count > 0 {
		col = p.accent
		b.Background(p.accent.Alpha(0.12))
	}
	if b.Hovered() || b.Pressed() {
		b.Background(p.hover)
		if count > 0 {
			b.Background(p.accent.Alpha(0.2))
		} else {
			col = p.text
		}
	}
	b.Children(func() {
		ui.Icon(c, icon).FontSize(14).TextColor(col)
		if count > 0 {
			ui.Text(c, fmt.Sprint(count)).FontSize(11.5).FontWeight(600).TextColor(col)
		}
	})
	return b
}

// commitButton 是提交按钮：主按钮提交，旁边的箭头打开更多提交方式。
func (f *Frontend) commitButton(c *ui.Context, s *view.SCM) {
	ed := repositoryEditor{Editor: f.ed, id: s.RepositoryID}

	t := c.Theme()
	committing := s.Busy == "提交中…"
	idle := s.Busy == "" && (s.Amend || len(s.Groups) > 0)
	bg, hover, pressed, fg := t.Accent, t.AccentHover, t.AccentPressed, t.AccentText
	if !idle && !committing {
		p := f.pal
		bg, fg = p.text.Alpha(0.06).Over(p.chrome), p.faint
		hover, pressed = p.text.Alpha(0.1).Over(p.chrome), p.text.Alpha(0.13).Over(p.chrome)
	}
	ui.Row(c.Key("commit")).Height(28).Children(func() {
		main := ui.Row(c.Key("commit-main")).Grow(1).Height(28).Radius(6, 0, 0, 6).Justify(ui.Center).AlignItems(ui.Center).Gap(6).
			Background(bg).Label(s.CommitLabel).Tooltip(s.CommitLabel + " (" + f.modKeys(keymap.KeyEnter) + ")")
		switch {
		case !idle:
		case main.Pressed():
			main.Background(pressed)
		case main.Hovered():
			main.Background(hover)
		}
		main.Children(func() {
			if committing {
				ui.Spinner(c).Size(12, 12)
				ui.Text(c, "提交中…").FontSize(12.5).FontWeight(600).TextColor(fg).SingleLine()
				return
			}
			ui.Icon(c, iconCheck).FontSize(14).TextColor(fg)
			ui.Text(c, s.CommitLabel).FontSize(12.5).FontWeight(600).TextColor(fg).SingleLine()
		})
		if main.Clicked() && idle {
			ed.Run("git.commit")
		}
		ui.Box(c).Width(1).Height(28).Background(fg.Alpha(0.28).Over(bg))
		more := ui.Row(c.Key("commit-more")).Width(28).Height(28).Radius(0, 6, 6, 0).Justify(ui.Center).AlignItems(ui.Center).
			Background(bg).Label("更多提交方式").Tooltip("更多提交方式")
		switch {
		case more.Pressed():
			more.Background(pressed)
		case more.Hovered():
			more.Background(hover)
		}
		more.Children(func() { ui.Icon(c, iconChevronDown).FontSize(14).TextColor(fg) })
		more.Menu(func(m *ui.Menu) {
			if m.Item("提交").Chosen() {
				ed.Run("git.commit")
			}
			if m.Item("提交并推送").Chosen() {
				ed.Run("git.commitAndPush")
			}
			m.Separator()
			if m.Item("修改上次提交").Checked(s.Amend).Chosen() {
				ed.Run("git.commitAmend")
			}
			if m.Item("撤销上次提交").Chosen() {
				ed.Run("git.undoCommit")
			}
		})
	})
}

func (f *Frontend) scmGroupHeader(c *ui.Context, id uint64, g view.SCMGroup) {
	ed := repositoryEditor{Editor: f.ed, id: id}

	p := f.pal
	row := ui.Row(c.Key("group:"+g.Title)).Gap(4).AlignItems(ui.Center).Height(22).Padding(0, 4)
	hovered := row.Hovered()
	row.Children(func() {
		f.sectionToggle(c, "fold:"+g.Title, g.Title, g.Folded, func() { ed.SCMToggleSection(app.SCMSectionKey(g.Kind)) })
		if hovered {
			switch g.Kind {
			case view.GroupStaged:
				if f.smallButton(c, iconMinus, "取消暂存全部") {
					ed.SCMStageGroup(g.Kind)
				}
			case view.GroupChanges, view.GroupUntracked:
				if f.smallButton(c, iconUndo, "放弃全部修改…") {
					f.confirmDiscard(ed.id, ed.SCMGroupFiles(g.Kind))
				}
				if f.smallButton(c, iconPlus, "暂存全部") {
					ed.SCMStageGroup(g.Kind)
				}
			case view.GroupMerge:
				if f.smallButton(c, iconPlus, "暂存全部合并的改动") {
					ed.SCMStageGroup(g.Kind)
				}
			}
		}
		ui.Text(c, fmt.Sprint(g.Count)).FontSize(10.5).TextColor(p.muted).Padding(0, 5).Radius(8).Background(p.hover)
	})
}

func (f *Frontend) scmRow(c *ui.Context, id uint64, g view.SCMGroup, r view.SCMRow, i int, selected bool) {
	ed := repositoryEditor{Editor: f.ed, id: id}

	if r.Folder {
		f.scmFolderRow(c, ed.id, g, r, i, selected)
		return
	}
	p := f.pal
	row := ui.Row(c.Key(fmt.Sprintf("%d:%s", g.Kind, r.Path))).Gap(6).AlignItems(ui.Center).Height(22).Padding(0, 0, 0, 14+float32(r.Depth)*14).Radius(4)
	hovered := row.Hovered()
	switch {
	case selected && f.frame.Focus == view.FocusSidebar, r.Picked:
		row.Background(p.active)
	case hovered:
		row.Background(p.hover)
	}
	if r.Letter != 'D' { // 和资源管理器一样的拖动内容：拖到一列上打开，拖到系统文件管理器里复制
		path := r.Path
		row.DragDataFrom(func() transfer.Data {
			data, _ := transfer.FileData(path)
			return data
		})
	}
	if row.DoubleClicked() {
		ed.SCMClick(i, true)
	} else if row.Clicked() {
		switch m := mods(row.ClickModifiers()); {
		case m&keymap.ModShift != 0:
			ed.SCMPick(i, true, false)
		case slices.Contains(f.Profile().Mods, m&(keymap.ModSuper|keymap.ModCtrl)) && m&(keymap.ModSuper|keymap.ModCtrl) != 0:
			ed.SCMPick(i, false, true)
		default:
			ed.SCMClick(i, false)
		}
	}
	// 选中了几个文件、这一行在里面时，行上的按钮对它们全部
	targets := []view.SCMRow{r}
	if r.Picked {
		if picked := ed.SCMPicked(); len(picked) > 1 {
			targets = picked
		}
	}
	row.Children(func() {
		icon, col := fileIcon(p, r.Name)
		ui.Icon(c, icon).FontSize(14).TextColor(col)
		label := ui.Text(c, r.Name).FontSize(13).SingleLine().Shrink(0).TextColor(gitColor(p, r.Letter))
		if r.Letter == 'D' {
			label.Strikethrough()
		}
		if r.Dir != "" {
			ui.Text(c, r.Dir).FontSize(11.5).TextColor(p.faint).SingleLine().Shrink(1)
		}
		ui.Spacer(c)
		if hovered {
			if g.Kind == view.GroupStaged {
				if f.smallButton(c, iconMinus, "取消暂存") {
					ed.SCMStageRows(targets)
				}
			} else {
				if g.Kind != view.GroupMerge && f.smallButton(c, iconUndo, "放弃修改…") {
					f.confirmDiscard(ed.id, targets)
				}
				if f.smallButton(c, iconPlus, "暂存") {
					ed.SCMStageRows(targets)
				}
			}
		}
		ui.Text(c, string(r.Letter)).FontSize(11).FontWeight(600).Font(f.opts.FontFamily).Padding(0, 4, 0, 2).
			TextColor(gitColor(p, r.Letter)).Shrink(0)
	})
}

// smallButton 是行上悬停时出现的小按钮。
func (f *Frontend) smallButton(c *ui.Context, icon *ui.SVG, label string) bool {
	p := f.pal
	b := ui.Box(c.Key(label)).Size(20, 20).Radius(4).Justify(ui.Center).AlignItems(ui.Center).Label(label).Tooltip(label)
	col := p.muted
	if b.Hovered() {
		b.Background(p.hover)
		col = p.text
	}
	b.Children(func() { ui.Icon(c, icon).FontSize(13).TextColor(col) })
	return b.Clicked()
}

// confirmDiscard 先问一声，再放弃这些文件的修改（不能撤销；未跟踪的文件移到废纸篓）。
func (f *Frontend) confirmDiscard(id uint64, rows []view.SCMRow) {
	ed := repositoryEditor{Editor: f.ed, id: id}

	msg := fmt.Sprintf("放弃 %d 个文件的修改吗？", len(rows))
	if len(rows) == 1 {
		msg = "放弃 " + rows[0].Name + " 的修改吗？"
	}
	for _, panel := range f.frame.Sidebar.SCM.Panels {
		if panel.State.RepositoryID == id {
			msg += " · " + panel.Name
			break
		}
	}
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent: f.win, Type: mygo.MessageWarning, Message: msg,
			Detail:  "这不能撤销。未跟踪的文件会移到" + app.TrashName(hostOS) + "。",
			Buttons: []string{"放弃修改", "取消"}, DefaultButton: 1, CancelButton: 1,
		})
		if err == nil && res.Button == 0 {
			f.do(func() { ed.SCMDiscard(rows) })
		}
	}()
}

// scmOpBanner 说合并、变基、拣选或还原遇到冲突停下来了，给「继续」「中止」（照着 Mayoi v0.8.0）。
func (f *Frontend) scmOpBanner(c *ui.Context, s *view.SCM) {
	ed := repositoryEditor{Editor: f.ed, id: s.RepositoryID}

	p := f.pal
	ui.Row(c.Key("scm-op")).Gap(6).AlignItems(ui.Center).Padding(4, 4, 4, 8).Radius(5).Background(p.modified.Alpha(0.14)).Children(func() {
		ui.Icon(c, iconBranch).FontSize(12).TextColor(p.modified)
		ui.Text(c, s.Op+"遇到冲突").FontSize(11.5).TextColor(p.text).Grow(1).SingleLine().Shrink(1).
			Tooltip("解决冲突、暂存文件后点「继续」；「中止」回到" + s.Op + "之前")
		for _, b := range []struct{ label, cmd string }{{"中止", "yan.git.abortOp"}, {"继续", "yan.git.continueOp"}} {
			btn := ui.Box(c.Key("scm-op:"+b.cmd)).Height(20).Padding(0, 8).Radius(4).Justify(ui.Center).AlignItems(ui.Center).Label(b.label)
			col := p.text
			if btn.Hovered() {
				btn.Background(p.hover)
			}
			if b.cmd == "yan.git.continueOp" {
				btn.Background(p.accent.Alpha(0.18))
				col = p.accent
			}
			btn.Children(func() { ui.Text(c, b.label).FontSize(11.5).FontWeight(600).TextColor(col) })
			if btn.Clicked() {
				ed.Run(b.cmd)
			}
		}
	})
}

// scmFolderRow 是以树的形式查看时一个文件夹的一行：点它折起、展开；按钮暂存、取消暂存、放弃它下面这一组的文件。
func (f *Frontend) scmFolderRow(c *ui.Context, id uint64, g view.SCMGroup, r view.SCMRow, i int, selected bool) {
	ed := repositoryEditor{Editor: f.ed, id: id}

	p := f.pal
	row := ui.Row(c.Key(fmt.Sprintf("%d:dir:%s", g.Kind, r.Path))).Gap(6).AlignItems(ui.Center).Height(22).
		Padding(0, 0, 0, 2+float32(r.Depth)*14).Radius(4).Label("文件夹 " + r.Name)
	hovered := row.Hovered()
	switch {
	case selected && f.frame.Focus == view.FocusSidebar:
		row.Background(p.active)
	case hovered:
		row.Background(p.hover)
	}
	if row.Clicked() {
		ed.SCMClick(i, false)
	}
	row.Children(func() {
		chev, icon := iconChevronDown, iconFolderOpen
		if r.Collapsed {
			chev, icon = iconChevronRight, iconFolder
		}
		ui.Icon(c, chev).FontSize(12).TextColor(p.faint)
		ui.Icon(c, icon).FontSize(14).TextColor(p.accent)
		ui.Text(c, r.Name).FontSize(13).TextColor(p.text).SingleLine().Shrink(1)
		ui.Spacer(c)
		if !hovered {
			return
		}
		if g.Kind == view.GroupStaged {
			if f.smallButton(c, iconMinus, "取消暂存这个文件夹") {
				ed.SCMStageFolder(i, true)
			}
			return
		}
		if g.Kind != view.GroupMerge && f.smallButton(c, iconUndo, "放弃这个文件夹的修改…") {
			f.confirmDiscard(ed.id, ed.SCMFolderFiles(i))
		}
		if f.smallButton(c, iconPlus, "暂存这个文件夹") {
			ed.SCMStageFolder(i, false)
		}
	})
}

func (f *Frontend) scmHistory(c *ui.Context, s *view.SCM) {
	ed := repositoryEditor{Editor: f.ed, id: s.RepositoryID}
	p := f.pal
	i := 0
	for _, g := range s.Groups {
		i += len(g.Rows)
	}
	f.scmHistoryHeader(c, s)
	lanes := min(max(s.Lanes, 1), graphMaxLanes)
	for k := range s.Commits {
		f.scmCommit(c, ed.id, &s.Commits[k], i, i == s.Selected, lanes)
		i++
	}
	if s.MoreCommits {
		b := ui.Row(c.Key("more-commits")).Height(22).Padding(0, 8).Radius(4).AlignItems(ui.Center).Label("显示更多提交")
		col := p.muted
		if b.Hovered() {
			b.Background(p.hover)
			col = p.text
		}
		b.Children(func() { ui.Text(c, "显示更多提交…").FontSize(12).TextColor(col) })
		if b.Clicked() {
			ed.Run("yan.scm.moreCommits")
		}
	}
}
