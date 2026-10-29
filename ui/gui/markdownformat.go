package gui

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/corn12138/yan-source/core/i18n"
)

// Formatting keys of the Markdown edit box. They work on the box's text and its selection, in
// runes as MyGo reports them, and return the new text and selection; the box writes the text
// through to the document as for typing.

// mdEdit is a text and its selection after a formatting key.
type mdEdit struct {
	text       string
	start, end int
}

// mdToggleWrap wraps the selection in mark (** for bold, * italic, ~~ strike, ` code), or
// unwraps it when the marks already surround it, inside or just outside the selection. With
// nothing selected it inserts a pair and leaves the caret between them.
func mdToggleWrap(text string, start, end int, mark string) mdEdit {
	r := []rune(text)
	start, end = clampSel(len(r), start, end)
	m := []rune(mark)
	sel := string(r[start:end])
	n := len(m)
	switch {
	case len(sel) >= 2*len(mark) && strings.HasPrefix(sel, mark) && strings.HasSuffix(sel, mark) && !wrappedBy(sel, mark):
		inner := []rune(sel)[n : len([]rune(sel))-n]
		return mdEdit{string(r[:start]) + string(inner) + string(r[end:]), start, start + len(inner)}
	case start >= n && end+n <= len(r) && string(r[start-n:start]) == mark && string(r[end:end+n]) == mark &&
		!(mark == "*" && (start >= 2 && r[start-2] == '*' || end+1 < len(r) && r[end+1] == '*')):
		return mdEdit{string(r[:start-n]) + sel + string(r[end+n:]), start - n, end - n}
	}
	return mdEdit{string(r[:start]) + mark + sel + mark + string(r[end:]), start + n, end + n}
}

// wrappedBy reports whether sel, though it starts and ends with mark, only does so because the
// mark is part of a longer one: *x* inside **x** is not italic wrapping.
func wrappedBy(sel, mark string) bool {
	return mark == "*" && strings.HasPrefix(sel, "**") && strings.HasSuffix(sel, "**") && !strings.HasPrefix(sel, "***")
}

// mdLink makes the selection a link, [text](), with the caret where the address goes; with
// nothing selected it inserts [](), the caret inside the brackets.
func mdLink(text string, start, end int) mdEdit {
	r := []rune(text)
	start, end = clampSel(len(r), start, end)
	sel := string(r[start:end])
	out := string(r[:start]) + "[" + sel + "]()" + string(r[end:])
	if sel == "" {
		return mdEdit{out, start + 1, start + 1}
	}
	at := start + 1 + (end - start) + 2
	return mdEdit{out, at, at}
}

var mdListItem = regexp.MustCompile(`^(\s*)(?:([-*+])|(\d+)([.)]))(\s+)(\[[ xX]\]\s+)?`)
var mdQuote = regexp.MustCompile(`^(\s*(?:>\s?)+)`)

// mdEnter continues a list item, task or quote on a new line, as a document editor does: the
// next item gets the same marker (the next number, an unchecked box). Enter on an item with
// nothing in it ends the list instead. ok is false where a plain line break is due.
func mdEnter(text string, start, end int) (mdEdit, bool) {
	r := []rune(text)
	start, end = clampSel(len(r), start, end)
	ls := start
	for ls > 0 && r[ls-1] != '\n' {
		ls--
	}
	le := end
	for le < len(r) && r[le] != '\n' {
		le++
	}
	line := string(r[ls:le])
	var prefix, next string
	if m := mdListItem.FindStringSubmatch(line); m != nil {
		prefix = m[0]
		marker := m[2]
		if m[3] != "" {
			n, _ := strconv.Atoi(m[3])
			marker = strconv.Itoa(n+1) + m[4]
		}
		next = m[1] + marker + m[5]
		if m[6] != "" {
			next += "[ ] "
		}
	} else if m := mdQuote.FindStringSubmatch(line); m != nil {
		prefix, next = m[1], m[1]
	} else {
		return mdEdit{}, false
	}
	pl := len([]rune(prefix))
	if start-ls < pl {
		return mdEdit{}, false // the caret is in the marker itself
	}
	if strings.TrimSpace(line[len(prefix):]) == "" && start == end {
		// An empty item ends the list: the marker goes, the line stays.
		return mdEdit{string(r[:ls]) + string(r[ls+pl:]), ls, ls}, true
	}
	ins := "\n" + next
	out := string(r[:start]) + ins + string(r[end:])
	at := start + len([]rune(ins))
	return mdEdit{out, at, at}, true
}

// mdIndent indents (or with out, outdents) the list items the selection touches, by the width
// of each item's marker, as CommonMark nests a list under the text of the item above.
func mdIndent(text string, start, end int, out bool) (mdEdit, bool) {
	r := []rune(text)
	start, end = clampSel(len(r), start, end)
	ls := start
	for ls > 0 && r[ls-1] != '\n' {
		ls--
	}
	lines := strings.Split(string(r[ls:]), "\n")
	last := ls
	var outLines []string
	shiftStart, shiftEnd := 0, 0
	changed := false
	for i, l := range lines {
		within := last <= end || i == 0
		m := mdListItem.FindStringSubmatch(l)
		if !within || m == nil {
			outLines = append(outLines, l)
			last += len([]rune(l)) + 1
			continue
		}
		width := len(m[2]) + len(m[3]) + len(m[4]) + len(m[5])
		var nl string
		delta := 0
		switch {
		case !out:
			nl, delta = strings.Repeat(" ", width)+l, width
		default:
			cut := min(width, len(m[1]))
			nl, delta = l[cut:], -cut
		}
		changed = changed || delta != 0
		if i == 0 {
			shiftStart = delta
		}
		shiftEnd += delta
		outLines = append(outLines, nl)
		last += len([]rune(l)) + 1
	}
	if !changed {
		return mdEdit{}, false
	}
	t := string(r[:ls]) + strings.Join(outLines, "\n")
	return mdEdit{t, max(ls, start+shiftStart), max(ls, end+shiftEnd)}, true
}

// mdBreak replaces the selection with a line break: Enter where nothing continues.
func mdBreak(text string, start, end int) mdEdit {
	r := []rune(text)
	start, end = clampSel(len(r), start, end)
	return mdEdit{string(r[:start]) + "\n" + string(r[end:]), start + 1, start + 1}
}

func clampSel(n, start, end int) (int, int) {
	start, end = min(max(start, 0), n), min(max(end, 0), n)
	if start > end {
		start, end = end, start
	}
	return start, end
}

// mdBlockKind is a block the slash menu inserts: what it is called, the words it is found by,
// and the Markdown that replaces the slash command, the caret going where "|" is.
type mdBlockKind struct {
	label, keys, insert string
}

// mdBlockKinds are in the interface language: names and the table's headers are Yan's.
func mdBlockKinds() []mdBlockKind {
	return []mdBlockKind{
		{i18n.T("Heading 1"), "h1 heading title biaoti", "# |"},
		{i18n.T("Heading 2"), "h2 heading biaoti", "## |"},
		{i18n.T("Heading 3"), "h3 heading biaoti", "### |"},
		{i18n.T("Bulleted List"), "bullet list ul liebiao", "- |"},
		{i18n.T("Numbered List"), "numbered list ol liebiao", "1. |"},
		{i18n.T("To-Do"), "todo task checkbox daiban", "- [ ] |"},
		{i18n.T("Quote"), "quote blockquote yinyong", "> |"},
		{i18n.T("Code Block"), "code fence daima", "```\n|\n```"},
		{i18n.T("Table"), "table biaoge", i18n.T("| Column 1 | Column 2 |\n| --- | --- |\n| | |")},
		{i18n.T("Divider"), "divider rule hr fengexian", "---\n|"},
	}
}

// mdSlash reports the slash command the caret ends, "/" and what follows it at the start of a
// line, and the rune offset where it starts; ok is false when the caret ends none.
func mdSlash(text string, caret int) (query string, at int, ok bool) {
	r := []rune(text)
	if caret < 1 || caret > len(r) {
		return "", 0, false
	}
	ls := caret
	for ls > 0 && r[ls-1] != '\n' {
		ls--
	}
	line := string(r[ls:caret])
	if !strings.HasPrefix(line, "/") || strings.ContainsAny(line, " \t") || len([]rune(line)) > 12 {
		return "", 0, false
	}
	return strings.ToLower(line[1:]), ls, true
}

// mdBlockMatches returns the blocks a slash query finds: by name, or by the start of a keyword.
func mdBlockMatches(query string) []mdBlockKind {
	var out []mdBlockKind
	for _, k := range mdBlockKinds() {
		hit := query == "" || strings.Contains(strings.ToLower(k.label), query)
		for _, w := range strings.Fields(k.keys) {
			hit = hit || strings.HasPrefix(w, query)
		}
		if hit {
			out = append(out, k)
		}
	}
	return out
}

// mdInsertBlock replaces the slash command from at to caret with block k.
func mdInsertBlock(text string, at, caret int, k mdBlockKind) mdEdit {
	r := []rune(text)
	ins := strings.Replace(k.insert, "|", "", 1)
	pos := at + len([]rune(k.insert[:strings.Index(k.insert, "|")]))
	return mdEdit{string(r[:at]) + ins + string(r[caret:]), pos, pos}
}
