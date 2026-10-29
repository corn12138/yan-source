package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func conflictsOf(rs []mergeRegion) []string {
	var out []string
	for _, r := range rs {
		if r.conflict {
			out = append(out, fmt.Sprintf("base [%d,%d) ours [%d,%d) theirs [%d,%d)", r.baseLo, r.baseHi, r.oursLo, r.oursHi, r.theirsLo, r.theirsHi))
		}
	}
	return out
}

func TestMergeRegions(t *testing.T) {
	l := func(s string) []string { return strings.Fields(s) }
	for _, c := range []struct {
		name               string
		base, ours, theirs string
		want               []string
	}{
		{"same line", "l1 l2 l3 l4", "l1 l2 O l4", "l1 l2 T l4", []string{"base [2,3) ours [2,3) theirs [2,3)"}},
		{"touching hunks join", "l1 l2 l3 l4 l5", "l1 l2 O l4 l5", "l1 l2 T T4 l5", []string{"base [2,4) ours [2,4) theirs [2,4)"}},
		{"a line apart", "l1 l2 l3 l4 l5 l6", "l1 O l3 l4 l5 l6", "l1 l2 l3 l4 T l6", nil},
		{"both insert", "l1 l2 l3", "l1 O O2 l2 l3", "l1 T l2 l3", []string{"base [1,1) ours [1,3) theirs [1,2)"}},
		{"delete against change", "l1 l2 l3 l4 l5 l6", "l1 l6", "l1 l2 T T4 l5 l6", []string{"base [1,5) ours [1,1) theirs [1,5)"}},
		{"two, shifted", "a b c d e f g h i j", "a O O2 O3 d e f g H I j", "a T T2 d e f g T7 T8 j", []string{"base [1,3) ours [1,4) theirs [1,3)", "base [7,9) ours [8,10) theirs [7,9)"}},
		{"same change both sides", "a b c", "a X c", "a X c", nil},
	} {
		if got := conflictsOf(mergeRegions(l(c.base), l(c.ours), l(c.theirs))); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	// One-sided changes are regions too, and start the result.
	base, ours, theirs := l("a b c d e f"), l("A b c d e f"), l("a b c d e F")
	rs := mergeRegions(base, ours, theirs)
	if len(rs) != 2 || rs[0].conflict || rs[1].conflict {
		t.Fatalf("one-sided: %+v", rs)
	}
	if got := strings.Join(autoMerged(base, ours, theirs, rs), " "); got != "A b c d e F" {
		t.Fatalf("auto merged: %q", got)
	}
}

// The regions stand in the result where its text has them, whatever was taken or typed.
func TestMergeResultRanges(t *testing.T) {
	l := func(s string) []string { return strings.Fields(s) }
	base, ours, theirs := l("a b c d e f g h i j k l"), l("a O O2 d e f g H I j k l"), l("a T T2 d e f g T7 T8 j k l")
	rs := mergeRegions(base, ours, theirs)
	at := func(result string) string {
		res := l(result)
		var out []string
		for i, rg := range resultRanges(base, res, rs) {
			st := stateOf(base, ours, theirs, res[rg[0]:rg[1]], rs[i], 0)
			out = append(out, fmt.Sprintf("%s [%d,%d) %s", st.label(), rg[0], rg[1], strings.Join(res[rg[0]:rg[1]], "|")))
		}
		return strings.Join(out, "; ")
	}
	for _, c := range []struct{ result, want string }{
		{"a b c d e f g h i j k l", "未处理 [1,3) b|c; 未处理 [7,9) h|i"},
		{"a O O2 d e f g h i j k l", "用了我们的 [1,3) O|O2; 未处理 [7,9) h|i"},
		{"a O O2 X d e f g T7 T8 j k l", "手改过 [1,4) O|O2|X; 用了他们的 [8,10) T7|T8"},
		{"a O O2 T T2 d e f g h i j k l", "两边都用了 [1,5) O|O2|T|T2; 未处理 [9,11) h|i"},
		{"a b c d E f g h i j k l", "未处理 [1,3) b|c; 未处理 [7,9) h|i"}, // an edit between them
	} {
		if got := at(c.result); got != c.want {
			t.Errorf("%q: %s, want %s", c.result, got, c.want)
		}
	}
}
