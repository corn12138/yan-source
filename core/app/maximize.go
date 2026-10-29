package app

import (
	"errors"
	"slices"
)

// 最大化编辑区：当前这一列，或者合并视图的三栏，单独占满窗口，侧边栏和终端面板不画。
// 只是不画别的：列、标签、光标、滚动、宽度、锁定和终端都原样留着，所以还原时一点不差，
// 不画的列里的终端也照样在跑。焦点去了没画出来的列（在那里打开了文件、换了列）或者侧边栏，
// 画出来的列关掉了，或者合并视图结束了，就自动还原：不在看不见的地方做事。

// maximizeState 是最大化时画出来的列；merge 不为 nil 时它们是这个合并视图的。
type maximizeState struct {
	cols  []*column
	merge *mergeView
}

// toggleMaximize 最大化编辑区，或者还原。
func (e *Editor) toggleMaximize() error {
	if e.maximized != nil {
		e.restoreLayout("已还原布局")
		return nil
	}
	t := e.cur()
	if t == nil {
		return errors.New("没有打开的编辑器")
	}
	m := &maximizeState{cols: []*column{e.column()}}
	if mv := mergeOf(t); mv != nil {
		m.merge = mv
		m.cols = e.mergeColumns(mv)
	}
	e.maximized = m
	e.area = areaEditor
	e.status = "已最大化编辑区：再执行一次「" + e.hintOr(cmdMaximize, "最大化／还原编辑区") + "」还原"
	return nil
}

const cmdMaximize = "workbench.action.toggleMaximizeEditorGroup"

// mergeColumns 是显示合并视图的列，按从左到右的顺序。
func (e *Editor) mergeColumns(mv *mergeView) []*column {
	var out []*column
	for _, c := range e.cols {
		if slices.ContainsFunc(c.tabs, func(t *tab) bool { return t == mv.result || t == mv.oursTab || t == mv.theirsTab }) {
			out = append(out, c)
		}
	}
	return out
}

// restoreLayout 结束最大化：什么都没动过，画回来就是原样。
func (e *Editor) restoreLayout(status string) {
	e.maximized = nil
	if status != "" {
		e.status = status
	}
}

// checkMaximized 在出帧前看最大化是否还成立，不成立就还原（理由写在状态栏）。
func (e *Editor) checkMaximized() {
	m := e.maximized
	if m == nil {
		return
	}
	m.cols = slices.DeleteFunc(m.cols, func(c *column) bool { return !slices.Contains(e.cols, c) })
	switch {
	case len(m.cols) == 0:
		e.restoreLayout("已还原布局：最大化的列关掉了")
	case m.merge != nil && (!e.tabOpen(m.merge.result) || mergeOf(m.merge.result) == nil): // closed, or resolved and detached
		e.restoreLayout("已还原布局：合并视图已结束")
	case e.area == areaTree:
		e.restoreLayout("已还原布局")
	case !slices.Contains(m.cols, e.column()):
		e.restoreLayout("已还原布局：打开了别的列")
	}
}

// hiddenColumn reports whether column c is not drawn because the editor area is maximized.
func (e *Editor) hiddenColumn(c *column) bool {
	return e.maximized != nil && !slices.Contains(e.maximized.cols, c)
}
