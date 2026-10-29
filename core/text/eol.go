package text

import "strings"

// CaretOffset keeps a selection boundary before a CRLF pair rather than between its bytes.
func (s Snapshot) CaretOffset(off int) int {
	if off > 0 && off < s.Len() && s.ByteAt(off) == '\n' && s.ByteAt(off-1) == '\r' {
		return off - 1
	}
	return off
}

// ConvertEOL builds the text once; applying one replacement per line would
// repeatedly rebuild the flat tree for files with many short lines.
func (s Snapshot) ConvertEOL(eol EOL) (Snapshot, ChangeSet) {
	var out strings.Builder
	out.Grow(s.Len())
	var changes ChangeSet
	newline := "\n"
	if eol == EOLCRLF {
		newline = "\r\n"
	}
	for line := 0; line < s.LineCount(); line++ {
		start, end := s.LineRange(line)
		out.WriteString(s.String(start, end))
		if line+1 == s.LineCount() {
			break
		}
		next := s.LineStart(line + 1)
		old := s.String(end, next)
		out.WriteString(newline)
		if old != newline {
			if eol == EOLLF {
				changes = append(changes, Change{Start: end, End: end + 1})
			} else {
				changes = append(changes, Change{Start: end, End: end, Text: "\r"})
			}
		}
	}
	m := s.Meta()
	if len(changes) == 0 && m.EOL == eol && !m.MixedEOL {
		return s, nil
	}
	m.EOL, m.MixedEOL = eol, false
	return FromString(out.String(), m), changes
}
