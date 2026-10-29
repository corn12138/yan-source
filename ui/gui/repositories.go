package gui

import (
	"fmt"
	"path/filepath"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
	"github.com/egoist/mygo/ui"
)

type scmInputState struct {
	scmMessage                string
	scmEditing, scmFocusAsked bool
	scmMessageIn              ui.Handle
}

func (f *Frontend) scmState(id uint64) *scmInputState {
	if f.scmStates == nil {
		f.scmStates = map[uint64]*scmInputState{}
	}
	if f.scmStates[id] == nil {
		f.scmStates[id] = &scmInputState{}
	}
	return f.scmStates[id]
}
func scmSubmitModifier(profile keymap.Profile) ui.Modifiers {
	if profile.MacKeys() {
		return ui.Cmd
	}
	return ui.Ctrl
}

// The frame ID travels into menu/dialog closures. Reading the currently selected
// repository when a delayed callback runs would act on the wrong index.
type repositoryEditor struct {
	*app.Editor
	id uint64
}

func (e repositoryEditor) act(fn func(*app.Editor)) { e.WithRepository(e.id, fn) }
func (e repositoryEditor) Run(command string)       { e.act(func(ed *app.Editor) { ed.Run(command) }) }
func (e repositoryEditor) SetCommitMessage(s string) {
	e.act(func(ed *app.Editor) { ed.SetCommitMessage(s) })
}
func (e repositoryEditor) FocusCommitMessage(on bool) {
	e.act(func(ed *app.Editor) { ed.FocusCommitMessage(on) })
}
func (e repositoryEditor) SCMClick(i int, open bool) {
	e.act(func(ed *app.Editor) { ed.SCMClick(i, open) })
}
func (e repositoryEditor) SCMPick(i int, extend, toggle bool) {
	e.act(func(ed *app.Editor) { ed.SCMPick(i, extend, toggle) })
}
func (e repositoryEditor) SCMPicked() (rows []view.SCMRow) {
	e.InRepository(e.id, func(ed *app.Editor) { rows = ed.SCMPicked() })
	return
}
func (e repositoryEditor) SCMStageRows(rows []view.SCMRow) {
	e.act(func(ed *app.Editor) { ed.SCMStageRows(rows) })
}
func (e repositoryEditor) SCMStage(path string, unstage bool) {
	e.act(func(ed *app.Editor) { ed.SCMStage(path, unstage) })
}
func (e repositoryEditor) SCMToggleSection(key string) {
	e.act(func(ed *app.Editor) { ed.SCMToggleSection(key) })
}
func (e repositoryEditor) SCMGroupFiles(kind view.SCMGroupKind) (rows []view.SCMRow) {
	e.InRepository(e.id, func(ed *app.Editor) { rows = ed.SCMGroupFiles(kind) })
	return
}
func (e repositoryEditor) SCMStageGroup(kind view.SCMGroupKind) {
	e.act(func(ed *app.Editor) { ed.SCMStageGroup(kind) })
}
func (e repositoryEditor) SCMDiscard(rows []view.SCMRow) {
	e.act(func(ed *app.Editor) { ed.SCMDiscard(rows) })
}
func (e repositoryEditor) SCMCommitCommand(hash, cmd string) {
	e.act(func(ed *app.Editor) { ed.SCMCommitCommand(hash, cmd) })
}
func (e repositoryEditor) SCMStageFolder(i int, unstage bool) {
	e.act(func(ed *app.Editor) { ed.SCMStageFolder(i, unstage) })
}
func (e repositoryEditor) SCMFolderFiles(i int) (rows []view.SCMRow) {
	e.InRepository(e.id, func(ed *app.Editor) { rows = ed.SCMFolderFiles(i) })
	return
}
func (e repositoryEditor) ComparisonStart() (s string) {
	e.InRepository(e.id, func(ed *app.Editor) { s = ed.ComparisonStart() })
	return
}

func (f *Frontend) scmView(c *ui.Context) {
	s := &f.frame.Sidebar.SCM
	multi := len(s.Panels) > 1 || len(s.Panels) == 1 && s.Root != f.ed.Root()
	if !multi {
		f.scmPanel(c, s, false)
		return
	}
	ui.Row(c.Key("repository-picker")).Padding(6, 12).Gap(8).Children(func() {
		label := i18n.T("Select Repository")
		for _, panel := range s.Panels {
			if panel.State.RepositoryID == s.RepositoryID {
				label = panel.Name
			}
		}
		if s.AutoRepository {
			label += i18n.T(" · Auto")
		}
		b := ui.Row(c.Key("choose-repository")).Padding(4, 6).Radius(4).Label(i18n.T("Select Repository")).Tooltip(i18n.T("Choose the repository for commit history and Git commands")).Grow(1)
		b.Children(func() { ui.Text(c, label).SingleLine().Shrink(1).TextColor(f.pal.text).FontSize(12) })
		if b.Clicked() {
			f.ed.Run("yan.scm.chooseRepository")
		}
		if f.smallButton(c, iconRefresh, i18n.T("Refresh Repository List")) {
			f.ed.Run("yan.scm.refreshRepositories")
		}
	})
	if s.RepositoryError != "" {
		ui.Text(c, s.RepositoryError).FontSize(11).TextColor(f.pal.muted)
	}
	live := map[uint64]bool{}
	ui.Scroll(c.Key("repository-panels")).Grow(1).MinHeight(0).Children(func() {
		for _, panel := range s.Panels {
			state := panel.State
			live[state.RepositoryID] = true
			ui.Column(c.Key(fmt.Sprint("repository:", state.RepositoryID))).Children(func() {
				h := ui.Row(c.Key("repository-header")).Height(30).Padding(0, 10).Gap(6).AlignItems(ui.Center).Label(i18n.F("Repository %s", panel.Name)).Tooltip(state.Root)
				if state.RepositoryID == s.RepositoryID {
					h.Background(f.pal.active)
				}
				h.Children(func() {
					icon := iconChevronDown
					if state.Collapsed {
						icon = iconChevronRight
					}
					ui.Icon(c, icon).FontSize(12).TextColor(f.pal.muted)
					ui.Text(c, panel.Name).SingleLine().Shrink(1).Grow(1).FontSize(12).FontWeight(600).TextColor(f.pal.text)
					count := 0
					for _, g := range state.Groups {
						count += g.Count
					}
					ui.Text(c, fmt.Sprint(count)).FontSize(11).TextColor(f.pal.muted)
				})
				h.ContextMenu(func(m *ui.Menu) {
					if m.Item(i18n.T("Open as a Separate Project")).Chosen() {
						f.ed.SCMOpenRepository(state.Root)
					}
				})
				if h.Clicked() {
					f.ed.ToggleRepository(state.RepositoryID)
				}
				if !state.Collapsed {
					f.scmPanel(c, &state, true)
				}
			})
		}
		f.scmHistory(c.Key(fmt.Sprint("history:", s.RepositoryID)), s)
	})
	for id := range f.scmStates {
		if !live[id] {
			delete(f.scmStates, id)
		}
	}
}

func (f *Frontend) repositoryLabel(root string) string {
	if rel, err := filepath.Rel(f.ed.Root(), root); err == nil && rel != "." {
		return filepath.ToSlash(rel)
	}
	return filepath.Base(root)
}
