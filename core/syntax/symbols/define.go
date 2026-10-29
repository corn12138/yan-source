package symbols

import (
	"regexp"
	"slices"
	"strings"

	"github.com/corn12138/yan-source/core/text"
)

// Kinds that Definitions adds to Find's declarations. A local is a name that
// only means something inside the block it is in.
const (
	KindVariable = "variable" // var/let/const/val, :=, a top-level NAME =
	KindField    = "field"    // a member of a struct, class, interface or enum
	KindMacro    = "macro"    // #define
)

var (
	// binding declares names with a keyword: var a, b; let mut x; const {a, b} = …
	binding = regexp.MustCompile(`^(\s*)(?:for\s*\(\s*)?(?:(?:export|pub|declare|static|private|public|protected|readonly|internal)\s+)*(var|let|const|val|local)\s+(?:mut\s+)?(\{[^}]*\}|\[[^\]]*\]|[A-Za-z_$][\w$]*(?:\s*,\s*[A-Za-z_$][\w$]*)*)`)
	// short is Go's a, b := …, also after if, for and switch.
	short = regexp.MustCompile(`^(\s*)(?:(?:if|for|switch|select)\s+)?([A-Za-z_]\w*(?:\s*,\s*[A-Za-z_]\w*)*)\s*:=`)
	// group opens Go's const (, var ( and type (.
	group = regexp.MustCompile(`^(\s*)(?:const|var|type)\s*\(\s*(?://.*)?$`)
	// member is the first names of a line in a group or a block: Name, A, B, name?:, *gen(…
	member = regexp.MustCompile(`^(\s+)(?:(?:public|private|protected|static|readonly|abstract|override|async|get|set|pub(?:\([^)]*\))?|final|const|declare)\s+)*[*#]?([A-Za-z_$][\w$]*(?:\s*,\s*[A-Za-z_][\w]*)*)\s*([?!]?\s*[:(=<,{;]|\s+\S|$)`)
	// assign is a top-level NAME = …, as Python, Ruby and Lua define globals.
	assign = regexp.MustCompile(`^([A-Za-z_]\w*)\s*(?::[^=]*)?=[^=]`)
	define = regexp.MustCompile(`^\s*#\s*define\s+([A-Za-z_]\w*)`)
	word   = regexp.MustCompile(`[A-Za-z_$][\w$]*`)
)

// notMember are words a line in a block starts with that do not name a member.
var notMember = map[string]bool{
	"return": true, "if": true, "else": true, "for": true, "while": true, "switch": true, "case": true,
	"default": true, "break": true, "continue": true, "throw": true, "try": true, "catch": true,
	"do": true, "new": true, "delete": true, "yield": true, "await": true, "super": true, "this": true,
	"self": true, "go": true, "defer": true, "goto": true, "match": true, "loop": true, "where": true,
	"type": true, "func": true, "fn": true, "def": true, "class": true, "struct": true, "interface": true,
	"enum": true, "impl": true, "let": true, "var": true, "const": true, "use": true, "import": true,
	"export": true, "package": true, "from": true, "with": true, "pass": true, "raise": true,
	"public": true, "private": true, "protected": true,
}

// comment reports whether a trimmed line is empty or only a comment; #define is not one.
func comment(s string) bool {
	switch {
	case s == "", strings.HasPrefix(s, "//"), strings.HasPrefix(s, "/*"), strings.HasPrefix(s, "*"), strings.HasPrefix(s, "--"):
		return true
	case strings.HasPrefix(s, "#"):
		return !define.MatchString(s)
	}
	return false
}

// Definitions lists what a document defines: Find's declarations, and the
// names that var/let/const/:= and the like bind, the members of Go groups and
// of struct, class, interface and enum blocks, top-level assignments and
// #define. It reads lines only, not a syntax tree, so it is a heuristic: a
// name it finds is one that a line looks like it defines.
func Definitions(snap text.Snapshot) []Symbol {
	if snap.LineCount() > maxLines {
		return nil
	}
	var out []Symbol
	add := func(line int, text, name, kind string, depth int, from int) {
		if name == "" || notMember[name] {
			return
		}
		col := from + wordIndex(text[from:], name)
		if col < from {
			return
		}
		for _, o := range out {
			if o.Line == line && o.Col == col {
				return
			}
		}
		out = append(out, Symbol{Line: line, Name: name, Kind: kind, Depth: depth, Col: col})
	}
	// A block whose lines are members: Go's const (…), a struct {…}, a class {…}.
	type block struct {
		indent int // the opening line's
		inner  int // the members', from the first one; -1 before it
		paren  bool
	}
	var blk *block
	for i := range snap.LineCount() {
		s, e := snap.LineRange(i)
		if e-s > 400 {
			continue
		}
		line := strings.TrimSuffix(snap.String(s, e), "\r")
		trimmed := strings.TrimSpace(line)
		if comment(trimmed) {
			continue
		}
		indent := columns(line[:len(line)-len(strings.TrimLeft(line, " \t"))])
		if blk != nil {
			closing := trimmed[0] == ')' && blk.paren || trimmed[0] == '}' && !blk.paren
			switch {
			case indent <= blk.indent:
				blk = nil
			case closing && blk.inner < 0:
				blk = nil
				continue
			case blk.inner < 0 || indent == blk.inner:
				if m := member.FindStringSubmatchIndex(line); m != nil {
					blk.inner = indent
					for _, n := range strings.Split(line[m[4]:m[5]], ",") {
						kind := KindField
						if blk.paren {
							kind = KindVariable
						}
						add(i, line, strings.TrimSpace(n), kind, indent, m[4])
					}
				}
			}
		}
		if sym, ok := declare(line); ok {
			sym.Line = i
			add(i, line, sym.Name, sym.Kind, sym.Depth, max(wordIndex(line, sym.Name), 0))
			if n := len(out); n > 0 && out[n-1].Line == i {
				out[n-1].Detail = sym.Detail
			}
			if strings.HasSuffix(trimmed, "{") && slices.Contains([]string{"class", "interface", "enum", "type"}, sym.Kind) {
				blk = &block{indent: indent, inner: -1}
			}
			continue
		}
		if m := group.FindStringSubmatch(line); m != nil {
			blk = &block{indent: columns(m[1]), inner: -1, paren: true}
			continue
		}
		if m := binding.FindStringSubmatchIndex(line); m != nil {
			names := line[m[6]:m[7]]
			if names[0] == '{' || names[0] == '[' {
				for _, part := range strings.Split(names[1:len(names)-1], ",") {
					// {a: b} binds b; {a = 1} and [a = 1] bind a.
					part, _, _ = strings.Cut(part, "=")
					if _, renamed, ok := strings.Cut(part, ":"); ok {
						part = renamed
					}
					part = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(part), "..."))
					if word.MatchString(part) && word.FindString(part) == part {
						add(i, line, part, KindVariable, indent, m[6])
					}
				}
			} else {
				for _, n := range strings.Split(names, ",") {
					add(i, line, strings.TrimSpace(n), KindVariable, indent, m[6])
				}
			}
			continue
		}
		if m := short.FindStringSubmatchIndex(line); m != nil {
			for _, n := range strings.Split(line[m[4]:m[5]], ",") {
				if n = strings.TrimSpace(n); n != "_" {
					add(i, line, n, KindVariable, indent, m[4])
				}
			}
			continue
		}
		if m := define.FindStringSubmatchIndex(line); m != nil {
			add(i, line, line[m[2]:m[3]], KindMacro, indent, m[2])
			continue
		}
		if m := assign.FindStringSubmatch(line); m != nil {
			add(i, line, m[1], KindVariable, 0, 0)
		}
	}
	return out
}

// Local reports whether a definition only means something in its block: a
// variable inside a function, not one at the top of a file.
func (s Symbol) Local() bool { return s.Kind == KindVariable && s.Depth > 0 }

// Param finds name among the parameters of a declaration's line, after the
// name it declares, or as a Go method's receiver: the byte where it starts,
// or -1.
func Param(line string, sym Symbol, name string) int {
	for at := 0; at < len(line); {
		open := strings.IndexByte(line[at:], '(')
		if open < 0 {
			break
		}
		open += at
		end := strings.IndexByte(line[open:], ')')
		if end < 0 {
			end = len(line) - open
		}
		if k := wordIndex(line[open:open+end], name); k >= 0 && open+k != sym.Col {
			return open + k
		}
		at = open + end
	}
	return -1
}

// wordIndex finds name in s as a whole word: the byte where it starts, or -1.
func wordIndex(s, name string) int {
	for at := 0; ; {
		k := strings.Index(s[at:], name)
		if k < 0 {
			return -1
		}
		k += at
		before := k == 0 || !isNameByte(s[k-1])
		after := k+len(name) >= len(s) || !isNameByte(s[k+len(name)])
		if before && after {
			return k
		}
		at = k + 1
	}
}

func isNameByte(b byte) bool {
	return b == '_' || b == '$' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}
