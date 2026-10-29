package tui

import (
	"fmt"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/view"
	"strings"
)

// Rows retain the repository ID so scrolling and discovery cannot retarget a click.
func (f *Frontend) drawRepositories(s *view.SCM, changes int, r view.Rect, focused bool) (caret [2]int, ok bool) {
	type row struct {
		text    string
		style   style.ID
		hit     sideLine
		active  bool
		message string
	}
	var rows []row
	selected := 0
	for _, panel := range s.Panels {
		p := panel.State
		active := p.RepositoryID == s.RepositoryID
		mark := "▾"
		if p.Collapsed {
			mark = "▸"
		}
		h := row{text: fmt.Sprintf(" %s %s · %s", mark, panel.Name, p.Branch), style: style.Dim, hit: sideLine{repositoryID: p.RepositoryID, kind: sideRepository}}
		if active {
			h.style = style.StatusAccent
			selected = len(rows)
		}
		rows = append(rows, h)
		if p.Collapsed {
			continue
		}
		lines := strings.Split(p.Message, "\n")
		for i, l := range lines {
			on := active && p.MessageFocused && i == len(lines)-1
			if on {
				selected = len(rows)
			}
			rows = append(rows, row{text: i18n.F("  Message %s", l), message: l, active: on, style: style.Text, hit: sideLine{repositoryID: p.RepositoryID, kind: sideRepositoryMessage}})
		}
		rows = append(rows, row{text: i18n.F("  Tab to write the message · ⌃K C %s", p.CommitLabel), style: style.Dim})
		if p.Busy != "" {
			rows = append(rows, row{text: "  " + p.Busy, style: style.StatusAccent})
		}
		if p.Op != "" {
			rows = append(rows, row{text: i18n.F("  %s: continue or abort from the Command Palette", p.Op), style: style.StatusAccent})
		}
		i := 0
		for _, g := range p.Groups {
			rows = append(rows, row{text: fmt.Sprintf("  %s (%d)", g.Title, g.Count), style: style.Dim})
			for _, file := range g.Rows {
				st := style.Text
				if active && i == p.Selected && !p.MessageFocused {
					selected = len(rows)
					if focused {
						st = style.TreeFocused
					}
				}
				label := file.Name
				if file.Dir != "" {
					label += "  " + file.Dir
				}
				if file.Folder {
					label += "/"
				}
				rows = append(rows, row{text: fmt.Sprintf("   %c %s%s", file.Letter, strings.Repeat("  ", file.Depth), label), style: st, hit: sideLine{repositoryID: p.RepositoryID, kind: sideSCMRow, index: i}})
				i++
			}
		}
	}
	i := 0
	for _, g := range s.Groups {
		i += len(g.Rows)
	}
	if len(s.Commits) > 0 {
		rows = append(rows, row{text: i18n.T("  Commit history · current repository"), style: style.Dim})
	}
	for _, commit := range s.Commits {
		st := style.Text
		if i == s.Selected && !s.MessageFocused {
			selected = len(rows)
			if focused {
				st = style.TreeFocused
			}
		}
		rows = append(rows, row{text: "   " + commit.Short + " " + commit.Subject, style: st, hit: sideLine{repositoryID: s.RepositoryID, kind: sideSCMRow, index: i}})
		i++
	}
	w, x0 := int(r.W), int(r.X)
	f.drawText(x0, 0, w, i18n.F(" Source Control (%d) · multiple repositories", changes), style.Tab)
	f.drawText(x0, 1, w, i18n.T(" Command Palette: Git: Select Repository"), style.Dim)
	count := max(0, int(r.H)-2)
	top := max(0, selected-count+1)
	for i := top; i < len(rows) && i-top < count; i++ {
		y := i - top + 2
		row := rows[i]
		f.sideLines[y] = row.hit
		if row.active && focused {
			x, on := f.field(x0+1, y, w-2, i18n.T("Message "), row.message, i18n.T("Write a commit message"), true)
			caret, ok = [2]int{x, y}, on
		} else {
			f.drawText(x0, y, w, row.text, row.style)
		}
	}
	return
}
