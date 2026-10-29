package app

import (
	"errors"
	"strconv"
)

// 项目标签，照着 Mayoi v0.8.0–v0.8.1（MIT，cmd/mayoi 的 project tabs）：窗口版可以把几个文件夹放在一个窗口里，
// 左边一列方块，一个文件夹一个，各自的标签、终端、侧边栏和面板都留着。这里只有命令和设置，
// 换项目、开项目、关项目都由前端做（ProjectHost）。终端版没有项目标签。

// ProjectHost 是能在一个窗口里放几个项目的前端。
type ProjectHost interface {
	ShowProject(i int)      // 第 i 个（从 0 数；-1 是最后一个）
	StepProject(delta int)  // 下一个、上一个
	CloseProject()          // 关掉现在这个（只剩一个时关窗口）
	SetProjectTabs(on bool) // 开：所有窗口并进现在这个；关：每个项目分成一个窗口
}

func (e *Editor) projectHost() (ProjectHost, error) {
	if h, ok := e.host.(ProjectHost); ok {
		return h, nil
	}
	return nil, errors.New("项目标签只在窗口版里有（yan-app）")
}

// ProjectTabs 报告是不是用项目标签（记在 state.json）。
func (e *Editor) ProjectTabs() bool {
	return e.state != nil && e.state.ProjectTabs != nil && *e.state.ProjectTabs
}

// SaveProjects 记下用项目标签时窗口里的文件夹，下次打开时都回来。
func (e *Editor) SaveProjects(roots []string) {
	if e.state == nil {
		return
	}
	e.state.Projects = roots
	e.saveState()
}

// SavedProjects 是上次记下的项目文件夹。
func (e *Editor) SavedProjects() []string {
	if e.state == nil {
		return nil
	}
	return e.state.Projects
}

func (e *Editor) toggleProjectTabs() error {
	h, err := e.projectHost()
	if err != nil {
		return err
	}
	on := !e.ProjectTabs()
	if e.state != nil {
		e.state.ProjectTabs = &on
		e.saveState()
	}
	e.settingsGen++
	h.SetProjectTabs(on)
	return nil
}

func registerProjects(e *Editor) {
	run := func(fn func(h ProjectHost)) func(*Editor) error {
		return func(e *Editor) error {
			h, err := e.projectHost()
			if err != nil {
				return err
			}
			fn(h)
			return nil
		}
	}
	e.Register(Command{ID: "yan.projectTabs.toggle", Title: "显示：项目标签（一个窗口里几个文件夹）", Run: (*Editor).toggleProjectTabs})
	e.Register(Command{ID: "yan.project.next", Title: "转到：下一个项目", Run: run(func(h ProjectHost) { h.StepProject(1) })})
	e.Register(Command{ID: "yan.project.previous", Title: "转到：上一个项目", Run: run(func(h ProjectHost) { h.StepProject(-1) })})
	e.Register(Command{ID: "yan.project.close", Title: "文件：关闭项目", Run: run(func(h ProjectHost) { h.CloseProject() })})
	for i := 1; i <= 9; i++ {
		n := i - 1
		if i == 9 {
			n = -1
		}
		e.Register(Command{ID: "yan.project.show" + strconv.Itoa(i), Hidden: true, Run: run(func(h ProjectHost) { h.ShowProject(n) })})
	}
}

// projectBindings 是项目的快捷键：⌃⌘] ⌃⌘[ 下一个、上一个，⌃⌘1…9 第几个（9 是最后一个）。
// ⇧⌘] ⇧⌘[ 在砚里是换标签，所以不像 Mayoi 那样用它们。
func projectBindings(g map[string]string) {
	g["ctrl+cmd+]"], g["ctrl+cmd+["] = "yan.project.next", "yan.project.previous"
	for i := 1; i <= 9; i++ {
		g["ctrl+cmd+"+strconv.Itoa(i)] = "yan.project.show" + strconv.Itoa(i)
	}
}
