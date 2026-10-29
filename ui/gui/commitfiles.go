package gui

import (
	"path/filepath"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/view"
	"github.com/egoist/mygo/ui"
)

type commitListState struct {
	list          ui.ListState
	selected      int
	identity, top string
}

// The code handle belongs to the actual diff rect, excluding the file list, so
// pointer coordinates, candidate windows and wrap widths use the same geometry.
func (f *Frontend) commitDiffView(c *ui.Context, pane ui.Element, ci int) {
	cf := f.frame.Columns[ci].Pane.CommitFiles
	draw := func(p ui.Element) {
		p.Bind(&f.cols[ci].pane)
		f.codePane(c, p, ci)
		if ci == f.frame.Active && f.frame.Find.Visible {
			p.Children(func() { f.findView(c) })
		}
	}
	width := pane.Bounds().W
	if cf == nil || !cf.Visible || width < 380 {
		draw(pane)
		return
	}
	w := min(float32(cf.Width), width-240)
	pane.Children(func() {
		ui.Row(c.Key("commit-layout")).Fill().Children(func() {
			code := ui.Box(c.Key("commit-code")).Grow(1).MinWidth(0).FillHeight().Clip()
			draw(code)
			box := ui.Column(c.Key("commit-files")).Width(w).Shrink(0).FillHeight().Background(f.pal.chrome).
				BorderWidth(0, 0, 0, 1).BorderColor(f.pal.border).Label(i18n.T("Commit File List"))
			box.Children(func() {
				f.splitter(c, edgeLeft, func(dx float32) { f.ed.SetCommitFilesWidth(float64(w - dx)) })
				ui.Row(c).Height(28).Padding(0, 6, 0, 10).AlignItems(ui.Center).Children(func() {
					ui.Text(c, i18n.F("Commit Files · %d", cf.Count)).FontSize(11.5).TextColor(f.pal.muted).SingleLine().Grow(1)
					if f.smallButton(c, iconListTree, i18n.T("Toggle Commit File Tree/List")) {
						f.ed.CommitFilesToggleTree(ci, cf.Identity)
					}
					if f.smallButton(c, iconX, i18n.T("Hide Commit File List")) {
						f.ed.Run("git.toggleCommitFiles")
					}
				})
				if cf.Loading {
					ui.Text(c, i18n.T("Reading…")).Padding(10).TextColor(f.pal.muted)
					return
				}
				if cf.Error != "" {
					ui.Text(c, cf.Error).Padding(10).TextColor(f.pal.danger)
					return
				}
				if len(cf.Rows) == 0 {
					ui.Text(c, i18n.T("No file changes")).Padding(10).TextColor(f.pal.muted)
					return
				}
				st := &f.cols[ci].commitList
				if st.identity != cf.Identity {
					st.identity = cf.Identity
					st.top = ""
					st.selected = -1
				}
				if st.top != cf.Selected {
					st.top = cf.Selected
					if !st.list.FocusWithin(c) {
						st.selected = -1
						for i, r := range cf.Rows {
							if r.Path == cf.Selected {
								st.selected = i
								st.list.ScrollIntoView(i)
								break
							}
						}
					}
				}
				st.list.Selected = &st.selected
				st.list.Key = func(i int) any { return cf.Rows[i].Path }
				st.list.Label = func(i int) string { return cf.Rows[i].Name }
				base := c.Theme()
				quiet := *base
				quiet.Accent, quiet.AccentText, quiet.SurfaceHover = f.pal.active, f.pal.text, f.pal.hover
				c.SetTheme(&quiet)
				rowClick := -1
				list := ui.List(c.Key(cf.Identity), &st.list, len(cf.Rows), func(i int) {
					row := f.commitFileRow(c, cf.Rows[i])
					if row.Clicked() {
						rowClick = i
					}
				}).Grow(1)
				c.SetTheme(base)
				if rowClick >= 0 {
					st.selected = rowClick
					st.list.Focus(c)
					f.ed.CommitFileAction(ci, cf.Identity, cf.Rows[rowClick].Path, false)
				}
				if st.selected < 0 || st.selected >= len(cf.Rows) {
					return
				}
				r := cf.Rows[st.selected]
				if rowClick < 0 && list.Changed() && !r.Folder || list.Submitted() && (!r.Folder || rowClick < 0) {
					f.ed.CommitFileAction(ci, cf.Identity, r.Path, list.Submitted())
				}
				if list.Shortcut(0, ui.KeyRight) && r.Folder {
					if r.Collapsed {
						f.ed.CommitFileAction(ci, cf.Identity, r.Path, false)
					} else if st.selected+1 < len(cf.Rows) {
						st.selected++
						st.list.ScrollIntoView(st.selected)
						next := cf.Rows[st.selected]
						if !next.Folder {
							f.ed.CommitFileAction(ci, cf.Identity, next.Path, false)
						}
					}
				}
				if list.Shortcut(0, ui.KeyLeft) {
					if r.Folder && !r.Collapsed {
						f.ed.CommitFileAction(ci, cf.Identity, r.Path, false)
					} else {
						for i, parent := range cf.Rows {
							if parent.Folder && parent.Path == filepath.Dir(r.Path) {
								st.selected = i
								st.list.ScrollIntoView(i)
								break
							}
						}
					}
				}

			})
		})
	})
}

func (f *Frontend) commitFileRow(c *ui.Context, r view.CommitFile) ui.Element {
	return ui.Row(c).Height(24).Padding(0, 8, 0, float32(8+r.Depth*12)).Gap(5).AlignItems(ui.Center).Tooltip(r.Path).Label(i18n.F("Commit file tree %s", r.Name)).Children(func() {
		icon, tint := fileIcon(f.pal, r.Name)
		if r.Folder {
			arrow := iconChevronDown
			if r.Collapsed {
				arrow = iconChevronRight
			}
			ui.Icon(c, arrow).FontSize(11).TextColor(f.pal.muted)
			icon = iconFolder
			if !r.Collapsed {
				icon = iconFolderOpen
			}
			tint = f.pal.accent
		}
		ui.Icon(c, icon).FontSize(13).TextColor(tint)
		ui.Text(c, r.Name).FontSize(12).TextColor(f.pal.text).SingleLine().Grow(1)
		if r.Status != 0 {
			ui.Text(c, string(r.Status)).FontSize(10).TextColor(f.pal.muted)
		}
	})
}

func (f *Frontend) scmCommitFile(c *ui.Context, ed repositoryEditor, cm *view.SCMCommit, i int, selected bool) {
	r := *cm.File
	row := ui.Row(c.Key(cm.Hash+":"+r.Path)).Height(24).Padding(0, 4, 0, float32(16+r.Depth*12)).Gap(5).AlignItems(ui.Center).
		Label(i18n.F("Commit files %s", r.Name)).Tooltip(r.Path)
	if selected {
		row.Background(f.pal.active)
	} else if row.Hovered() {
		row.Background(f.pal.hover)
	}
	if row.Clicked() || row.DoubleClicked() {
		f.root.Focus(c)
		ed.act(func(e *app.Editor) { e.SCMCommitFile(cm.Hash, r.Path, row.DoubleClicked()) })
	}
	row.Children(func() {
		icon, tint := fileIcon(f.pal, r.Name)
		if r.Folder {
			icon = iconChevronDown
			if r.Collapsed {
				icon = iconChevronRight
			}
			tint = f.pal.muted
		}
		ui.Icon(c, icon).FontSize(13).TextColor(tint)
		ui.Text(c, r.Name).FontSize(12).TextColor(f.pal.text).SingleLine().Grow(1)
		if r.Status != 0 {
			ui.Text(c, string(r.Status)).FontSize(10).TextColor(f.pal.muted)
		}
	})
}
