package gui

// Review interaction is adapted from Mayoi (MIT), cmd/mayoi/review.go.

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
	"github.com/egoist/mygo/ui"
)

func (f *Frontend) reviewButton(c *ui.Context, label, cmd string) {
	// Narrow columns shorten the scope crumbs before this button: its label never wraps.
	b := ui.Box(c.Key(cmd)).Height(20).Padding(0, 6).Radius(4).Shrink(0).Justify(ui.Center).AlignItems(ui.Center).Label(label)
	if b.Hovered() {
		b.Background(f.pal.hover)
	}
	b.Children(func() { ui.Text(c, label).FontSize(12).SingleLine().TextColor(f.pal.accent) })
	if b.Clicked() {
		f.ed.Run(cmd)
	}
}

func (f *Frontend) reviewView(c *ui.Context) {
	d := f.frame.Review
	if d == nil {
		f.reviewID = 0
		return
	}
	if f.reviewID != d.ID {
		f.reviewID = d.ID
		f.reviewInput = d.Text
	}
	p := f.pal
	ui.Overlay(c, func() {
		ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).AlignItems(ui.Center).Modal().Children(func() {
			box := f.floating(c, ui.Column(c.Key(d.ID)).Width(680).MaxWidthPercent(94).Margin(70, 0, 0, 0).Padding(14).Gap(10).Radius(8)).Role(ui.RoleDialog).Label(i18n.T("Review Comments"))
			box.Children(func() {
				ui.Text(c, d.Ref).FontSize(12).TextColor(p.muted)
				ui.Scroll(c).MaxHeight(140).Children(func() { ui.Text(c, d.Snippet).Font(f.opts.FontFamily).FontSize(12).TextColor(p.text) })
				input := ui.TextArea(c.Key("review-input"), &f.reviewInput).Height(126).AutoFocus().Label(i18n.T("Comment")).Placeholder(i18n.T("Write a suggestion; after saving, all comments can be sent to the terminal together"))
				if input.Changed() {
					f.ed.SetReviewInput(f.reviewInput)
				}
				mod := ui.Ctrl
				if slices.Contains(f.Profile().Mods, keymap.ModSuper) {
					mod = ui.Cmd
				}
				if input.Shortcut(mod, ui.KeyEnter) {
					f.ed.SetReviewInput(f.reviewInput)
					f.ed.Run("review.save")
				}
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					if d.Saved {
						f.reviewButton(c, i18n.T("Delete Comment"), "review.delete")
					}
					ui.Spacer(c)
					f.reviewButton(c, i18n.T("Cancel"), "review.cancel")
					f.reviewButton(c, i18n.T("Save Comment"), "review.save")
				})
			})
			if box.OverlayShortcut(0, ui.KeyEscape) {
				f.ed.Run("review.cancel")
			}
		})
	})
}

func (f *Frontend) reviewLineButtons(c *ui.Context, ci int, pf *view.PaneFrame) {
	for i, row := range pf.Rows {
		if i < len(pf.Sticky) || row.ReviewID == 0 && i != f.cols[ci].reviewHover {
			continue
		}
		label := i18n.T("Add Review Comment")
		if row.ReviewID != 0 {
			label = i18n.F("Edit Comment %d", row.ReviewID)
		}
		b := ui.Box(c.Key(fmt.Sprintf("review-line-%d", i))).Absolute().Left(0).Top(codePadY + float32(i)*f.font.lineH).Width(18).Height(f.font.lineH).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).Label(label).Tooltip(label)
		b.Children(func() {
			word := "+"
			if row.ReviewID != 0 {
				word = "●"
			}
			ui.Text(c, word).FontSize(12).TextColor(f.pal.accent)
		})
		if b.Clicked() {
			f.ed.ReviewLine(ci, int(row.Line))
		}
	}
}

func (f *Frontend) reviewTerminal() *termSession {
	live := func(s *termSession) bool {
		return s != nil && s.reviewRoot == f.ed.Root() && s.t != nil && !s.failed && !s.closed && (slices.Contains(f.terms, s) || s.id != 0 && f.tabTerms[s.id] == s)
	}
	if live(f.lastReviewTerm) {
		return f.lastReviewTerm
	}
	if f.termActive >= 0 && f.termActive < len(f.terms) && live(f.terms[f.termActive]) {
		return f.terms[f.termActive]
	}
	// Frame order is stable; map iteration must not choose a different tab on each send.
	for _, col := range f.frame.Columns {
		for _, t := range col.Tabs {
			if s := f.tabTerms[t.Terminal]; live(s) {
				return s
			}
		}
	}
	return nil
}

func (h host) PasteReview(value string) (bool, error) {
	f := h.f
	s := f.reviewTerminal()
	if s == nil {
		return false, nil
	}
	// Without mode 2004, multiline Paste translates line breaks to Enter. The
	// exported snapshot includes terminal modes; keep the review on the clipboard
	// when the receiving program has not requested bracketed paste.
	if !bytes.Contains(s.t.Snapshot(), []byte("\x1b[?2004h")) {
		return false, nil
	}
	// Send one bracketed paste, with no submit keystroke. Strip controls that could
	// terminate the bracket or act as keys; source newlines and tabs remain literal.
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' || r == 127 {
			return -1
		}
		return r
	}, value)
	s.t.Send([]byte("\x1b[200~" + value + "\x1b[201~"))
	f.lastReviewTerm = s
	if s.id != 0 {
		for ci, col := range f.frame.Columns {
			for i, t := range col.Tabs {
				if t.Terminal == s.id {
					f.ed.ClickTab(ci, i, true)
					f.shownTerm = 0
					f.focusTerm = false
					f.focusRoot = false
					return true, nil
				}
			}
		}
	}
	f.termActive = slices.Index(f.terms, s)
	f.panelOpen = true
	f.focusTerm = true
	return true, nil
}
