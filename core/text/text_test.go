package text

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 对照测试：对 Snapshot 和普通 string 做同样的随机修改，结果必须一致。
func TestRandomEditsMatchString(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	pieces := []string{"a", "中文", "\n", "hello world\n", "😀", "\r\n", strings.Repeat("x", 3000)}
	ref := ""
	s := FromString("", Meta{})
	for i := 0; i < 800; i++ {
		start := r.IntN(len(ref) + 1)
		end := start + r.IntN(min(len(ref)-start, 50)+1)
		start, end = runeAlign(ref, start), runeAlign(ref, end)
		if end < start {
			end = start
		}
		txt := ""
		if r.IntN(3) > 0 {
			txt = pieces[r.IntN(len(pieces))]
		}
		cs := ChangeSet{{Start: start, End: end, Text: txt}}
		old := s
		s = cs.Apply(s)
		ref = ref[:start] + txt + ref[end:]
		if got := s.String(0, s.Len()); got != ref {
			t.Fatalf("step %d: content mismatch", i)
		}
		if back := cs.Invert(old).Apply(s); back.String(0, back.Len()) != old.String(0, old.Len()) {
			t.Fatalf("step %d: invert failed", i)
		}
		if i%50 == 0 {
			checkLines(t, s, ref)
		}
	}
}

func runeAlign(s string, i int) int {
	for i > 0 && i < len(s) && (s[i]&0xC0) == 0x80 {
		i--
	}
	return i
}

func checkLines(t *testing.T, s Snapshot, ref string) {
	t.Helper()
	lines := strings.Split(ref, "\n")
	if s.LineCount() != len(lines) {
		t.Fatalf("LineCount = %d, want %d", s.LineCount(), len(lines))
	}
	off := 0
	for i, l := range lines {
		if got := s.LineStart(i); got != off {
			t.Fatalf("LineStart(%d) = %d, want %d", i, got, off)
		}
		if got := s.LineAt(off); got != i {
			t.Fatalf("LineAt(%d) = %d, want %d", off, got, i)
		}
		off += len(l) + 1
	}
}

func TestLineRangeStripsCRLF(t *testing.T) {
	s := FromString("ab\r\ncd\n\nef", Meta{})
	want := []string{"ab", "cd", "", "ef"}
	for i, w := range want {
		a, b := s.LineRange(i)
		if got := s.String(a, b); got != w {
			t.Errorf("line %d = %q, want %q", i, got, w)
		}
	}
}

func TestMapPos(t *testing.T) {
	cs := ChangeSet{{Start: 2, End: 4, Text: "XYZ"}}
	cases := []struct {
		pos  int
		a    Assoc
		want int
	}{{0, AssocAfter, 0}, {2, AssocBefore, 2}, {2, AssocAfter, 5}, {3, AssocAfter, 5}, {4, AssocAfter, 5}, {6, AssocAfter, 7}}
	for _, c := range cases {
		if got := cs.MapPos(c.pos, c.a); got != c.want {
			t.Errorf("MapPos(%d,%d) = %d, want %d", c.pos, c.a, got, c.want)
		}
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	orig := "\xEF\xBB\xBFline1\r\n中文\r\n"
	if err := os.WriteFile(p, []byte(orig), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	m := s.Meta()
	if m.Encoding != EncUTF8BOM || m.EOL != EOLCRLF || m.ReadOnly {
		t.Fatalf("meta = %+v", m)
	}
	if err := Save(s, p); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != orig {
		t.Fatalf("round trip changed bytes: %q", got)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", fi.Mode().Perm())
	}
}
