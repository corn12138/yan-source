package app

import (
	"os"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/i18n"
)

// 后退、前进（VS Code 的 workbench.action.navigateBack / navigateForward）：转到定义前记下的位置。

// maxNav 是后退最多记几个位置。
const maxNav = 50

// navPlace 是一个文件里的位置。
type navPlace struct {
	path string
	pos  int
}

type navHistory struct{ back, forward []navPlace }

// here 是光标现在的位置；没有文件（终端、改动视图、没存过的文档）时 ok 为假。
func (e *Editor) here() (navPlace, bool) {
	t := e.cur()
	if t == nil || t.term != nil || t.diff != nil || t.mergeSide != nil || t.doc.buf.Path() == "" {
		return navPlace{}, false
	}
	return navPlace{t.doc.buf.Path(), e.sels(t).Main().Head}, true
}

func pushPlace(list []navPlace, p navPlace) []navPlace {
	if n := len(list); n > 0 && list[n-1] == p {
		return list
	}
	list = append(list, p)
	if len(list) > maxNav {
		list = list[len(list)-maxNav:]
	}
	return list
}

// pushNav 在跳走之前记下现在的位置；前进的记录作废。
func (e *Editor) pushNav() {
	if p, ok := e.here(); ok {
		e.nav.back = pushPlace(e.nav.back, p)
		e.nav.forward = e.nav.forward[:0]
	}
}

// navigate 回到上一个（back 为假时是下一个）记下的位置；文件已经不在的位置跳过。
func (e *Editor) navigate(back bool) error {
	from, to := &e.nav.back, &e.nav.forward
	if !back {
		from, to = to, from
	}
	for len(*from) > 0 {
		p := (*from)[len(*from)-1]
		*from = (*from)[:len(*from)-1]
		if p2, ok := e.here(); ok && p2 == p {
			continue
		}
		if _, err := os.Stat(p.path); err != nil && e.docOf(p.path) == nil {
			continue
		}
		if cur, ok := e.here(); ok {
			*to = pushPlace(*to, cur)
		}
		if t := e.cur(); t == nil || t.doc.buf.Path() != p.path {
			if err := e.openPath(p.path); err != nil {
				return err
			}
			e.sync()
		}
		t := e.cur()
		if t == nil {
			return nil
		}
		b := t.doc.buf
		s := b.Snapshot()
		pos := min(p.pos, s.Len())
		b.SetSelections(edit.Single(edit.Cursor(pos)))
		e.centerLine(t, s.LineAt(pos))
		e.area = areaEditor
		e.reveal()
		return nil
	}
	if back {
		e.status = i18n.T("No position to go back to")
	} else {
		e.status = i18n.T("No position to go forward to")
	}
	return nil
}
