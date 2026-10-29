package gui

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

func keyDown() app.KeyInput { return app.KeyInput{Chord: keymap.Chord{Key: keymap.KeyDown}} }

// 在文件中搜索的视图，样子照着 Mayoi（cmd/mayoi/search.go）：查询框（区分大小写、全字、正则），
// 替换框和全部替换，「⋯」打开包含、排除；下面是按文件分组的结果，匹配在行里标出来。

func (f *Frontend) searchView(c *ui.Context) {
	p := f.pal
	s := &f.frame.Sidebar.Search
	if s.Running && s.Query != "" {
		ui.Progress(c.Key("search-activity"), -1).Height(2).Radius(0).Label("搜索中")
	} else {
		ui.Box(c.Key("search-activity")).Height(2)
	}
	for i, v := range []string{s.Query, s.With, s.Include, s.Exclude} {
		if f.searchText[i] != v && f.searchEditing != i+1 {
			f.searchText[i] = v
		}
	}
	f.searchEditing = 0
	if s.Focus != f.searchFocus {
		f.searchFocus = s.Focus
		switch s.Focus {
		case view.SearchQuery, view.SearchReplace, view.SearchInclude, view.SearchExclude:
			f.searchIn[s.Focus-view.SearchQuery].Focus(c)
		case view.SearchResults:
			f.root.Focus(c)
		}
	}
	ui.Column(c).Padding(0, 12, 6, 6).Gap(5).Children(func() {
		ui.Row(c).Gap(2).AlignItems(ui.Stretch).Children(func() {
			arrow := iconChevronRight
			if s.Replacing {
				arrow = iconChevronDown
			}
			b := ui.Box(c.Key("toggle-replace")).Width(18).Radius(4).Justify(ui.Center).AlignItems(ui.Center).
				Label("切换替换").Tooltip("切换替换 (" + f.ed.KeyHint("workbench.action.replaceInFiles") + ")")
			if b.Hovered() {
				b.Background(p.hover)
			}
			b.Children(func() { ui.Icon(c, arrow).FontSize(13).TextColor(p.muted) })
			if b.Clicked() {
				f.ed.ToggleSearchOption(3)
			}
			ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
				in := f.searchField(c, 0, "搜索", func() {
					f.searchToggle(c, iconCase, "区分大小写", s.MatchCase, 0)
					f.searchToggle(c, iconWord, "全字匹配", s.Word, 1)
					f.searchToggle(c, iconRegex, "使用正则表达式", s.Regex, 2)
				})
				if in.Shortcut(0, ui.KeyDown) && len(s.Rows) > 0 {
					f.ed.FocusSearch(int(view.SearchResults) - 1)
					f.ed.Handle(keyDown()) // 选中并预览第一处
				}
				if !s.Replacing {
					return
				}
				with := f.searchField(c, 1, "替换", func() {
					if f.smallButton(c, iconReplaceAll, "全部替换") {
						f.confirmReplaceAll(s)
					}
				})
				if with.Submitted() {
					f.confirmReplaceAll(s)
				}
			})
		})
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(0, 0, 0, 20).Children(func() {
			col := p.muted
			if s.Error {
				col = p.danger
			}
			ui.Text(c, s.Summary).FontSize(11.5).TextColor(col).Grow(1).SingleLine().Shrink(1)
			if f.iconButton(c, iconMore, "显示包含、排除的文件", s.Details).Size(22, 18).Clicked() {
				f.ed.ToggleSearchOption(4)
			}
		})
		if !s.Details {
			return
		}
		ui.Column(c).Gap(4).Padding(0, 0, 0, 20).Children(func() {
			ui.Text(c, "包含的文件").FontSize(11).TextColor(p.muted)
			f.searchField(c, 2, "例如 *.go, src/**", nil)
			ui.Text(c, "排除的文件").FontSize(11).TextColor(p.muted).Margin(2, 0, 0, 0)
			f.searchField(c, 3, "例如 *.test.ts, testdata", nil)
		})
	})
	f.searchResults(c, s)
}

// searchField 是搜索视图的一个输入框：k 是 0 查询、1 替换、2 包含、3 排除。
func (f *Frontend) searchField(c *ui.Context, k int, placeholder string, after func()) ui.Element {
	p := f.pal
	var in ui.Element
	field := ui.Row(c).Height(26).Padding(0, 4, 0, 8).Gap(2).AlignItems(ui.Center).Radius(5).
		Background(p.editor).Border(1, p.border)
	field.Children(func() {
		in = ui.TextInputBase(c.Key(fmt.Sprint("search-field", k)), &f.searchText[k]).Bind(&f.searchIn[k]).Grow(1).MinWidth(0).
			FontSize(12.5).Placeholder(placeholder).Label(placeholder)
		if in.Focused() {
			field.Border(1, p.accentDim)
			if f.frame.Sidebar.Search.Focus != view.SearchField(k)+view.SearchQuery {
				f.ed.FocusSearch(k)
			}
		}
		if in.Changed() {
			f.searchEditing = k + 1
			f.ed.SetSearchField(k, f.searchText[k])
		}
		if after != nil {
			after()
		}
	})
	return in
}

func (f *Frontend) searchToggle(c *ui.Context, icon *ui.SVG, label string, on bool, opt int) {
	p := f.pal
	b := ui.Box(c.Key(label)).Size(22, 20).Radius(4).Justify(ui.Center).AlignItems(ui.Center).Label(label).Tooltip(label)
	col := p.muted
	if on {
		b.Background(p.active).Border(1, p.accentDim)
		col = p.text
	} else if b.Hovered() {
		b.Background(p.hover)
	}
	b.Children(func() { ui.Icon(c, icon).FontSize(13).TextColor(col) })
	if b.Clicked() {
		f.ed.ToggleSearchOption(opt)
	}
}

func (f *Frontend) searchResults(c *ui.Context, s *view.Search) {
	p := f.pal
	if len(s.Rows) == 0 {
		if s.Query == "" {
			ui.Column(c).Padding(10, 14, 0, 26).Gap(6).Children(func() {
				ui.Text(c, "在文件夹的文件里找文字。").FontSize(12).TextColor(p.muted)
				for _, k := range [][2]string{{"↓", "到结果里"}, {"↩", "打开匹配"}, {f.ed.KeyHint("workbench.action.replaceInFiles"), "替换"}} {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, k[0]).FontSize(11).TextColor(p.muted).Padding(0, 5).Radius(4).Background(p.hover).MinWidth(22).TextAlign(ui.Center)
						ui.Text(c, k[1]).FontSize(11.5).TextColor(p.faint)
					})
				}
			})
		}
		return
	}
	focused := s.Focus == view.SearchResults && f.frame.Focus == view.FocusSidebar
	if s.Selected != f.searchSel {
		f.searchSel = s.Selected
		if focused {
			f.searchList.ScrollIntoView(s.Selected)
		}
	}
	ui.List(c.Key("search-results"), &f.searchList, len(s.Rows), func(i int) {
		r := &s.Rows[i]
		row := ui.Row(c).Gap(6).AlignItems(ui.Center).Height(22).Radius(4)
		hovered := row.Hovered()
		switch {
		case i == s.Selected && focused:
			row.Background(p.active)
		case hovered:
			row.Background(p.hover)
		}
		if r.Match { // 拖到一列上：在那里打开，选中这一处
			row.Drag(tabDrag{f: f, match: &searchRef{row: i, line: r.Line}})
		}
		if row.DoubleClicked() {
			f.ed.SearchClick(i, true)
		} else if row.Clicked() {
			f.ed.SearchClick(i, false)
		}
		if !r.Match {
			row.Background(p.chrome).Children(func() {
				icon := iconChevronDown
				if r.Collapsed {
					icon = iconChevronRight
				}
				ui.Icon(c, icon).FontSize(12).TextColor(p.muted)
				fi, col := fileIcon(p, r.Name)
				ui.Icon(c, fi).FontSize(14).TextColor(col)
				ui.Text(c, r.Name).FontSize(13).TextColor(p.text).SingleLine().Shrink(0)
				if r.Dir != "" {
					ui.Text(c, r.Dir).FontSize(11.5).TextColor(p.faint).SingleLine().Shrink(1)
				}
				ui.Spacer(c)
				if hovered && s.Replacing && f.smallButton(c, iconReplaceAll, "替换这个文件里的全部") {
					f.ed.ReplaceSearch(i)
				}
				ui.Text(c, fmt.Sprint(r.Count)).FontSize(10.5).TextColor(p.muted).Padding(0, 5).Radius(8).Background(p.hover)
			})
			return
		}
		row.Padding(0, 0, 0, 30).Children(func() {
			ui.RichText(c, f.matchPreview(r)...).FontSize(12.5).SingleLine().Grow(1).Shrink(1).MinWidth(0)
			if hovered && s.Replacing {
				if f.smallButton(c, iconReplace, "替换") {
					f.ed.ReplaceSearch(i)
				}
			} else {
				ui.Text(c, strconv.Itoa(r.Line+1)).FontSize(10.5).TextColor(p.faint).SingleLine().Shrink(0).Padding(0, 4, 0, 0)
			}
		})
	}).Grow(1).Padding(0, 6, 8, 6)
}

// previewLead 是匹配前最多显示几个字，窄的侧边栏里也看得见匹配。
const previewLead = 28

// matchPreview 是匹配所在行的样子：匹配标出来；替换时划掉，后面跟着换成的文字（照着 Mayoi）。
func (f *Frontend) matchPreview(r *view.SearchRow) []ui.Span {
	p := f.pal
	pre := strings.TrimLeft(r.Pre, " \t")
	if n := utf8.RuneCountInString(pre); n > previewLead {
		cut := 0
		for range n - previewLead {
			_, size := utf8.DecodeRuneInString(pre[cut:])
			cut += size
		}
		pre = "…" + pre[cut:]
	}
	oneLine := func(t string) string { return strings.ReplaceAll(strings.ReplaceAll(t, "\t", " "), "\n", "⏎") }
	spans := []ui.Span{{Text: oneLine(pre), Color: p.text}}
	if !r.Replacing {
		spans = append(spans, ui.Span{Text: oneLine(r.Hit), Color: p.text, Background: p.matchNow})
	} else {
		spans = append(spans, ui.Span{Text: oneLine(r.Hit), Color: p.text, Background: p.removed.Alpha(0.22), Strikethrough: true})
		if r.With != "" {
			spans = append(spans, ui.Span{Text: oneLine(r.With), Color: p.text, Background: p.added.Alpha(0.22)})
		}
	}
	return append(spans, ui.Span{Text: oneLine(r.Post), Color: p.text})
}

// confirmReplaceAll 全部替换前先问一声：没开的文件直接写回磁盘，不能撤销。
func (f *Frontend) confirmReplaceAll(s *view.Search) {
	if len(s.Rows) == 0 {
		return
	}
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent: f.win, Type: mygo.MessageWarning,
			Message: "全部替换吗？", Detail: s.Summary + "。没开的文件会直接改写，不能撤销。",
			Buttons: []string{"全部替换", "取消"}, DefaultButton: 0, CancelButton: 1,
		})
		if err == nil && res.Button == 0 {
			f.do(func() { f.ed.ReplaceSearch(-1) })
		}
	}()
}
