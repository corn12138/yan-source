//go:build selftest

package app

// SelfTestUntitled is the text of each untitled document with changes (native verification builds).
func (e *Editor) SelfTestUntitled() []string {
	var out []string
	for _, d := range e.docs {
		if d.untitled() && d.buf.Dirty() {
			s := d.buf.Snapshot()
			out = append(out, s.String(0, s.Len()))
		}
	}
	return out
}
