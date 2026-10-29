package app

// The regions of a three-way merge, after the behaviour of Mayoi PR #50 (Garfielk,
// https://github.com/sorrycc/Mayoi/pull/50, unmerged): the changes of ours and of theirs
// from the common base, joined where they touch in the base, as Git joins them; a region both
// sides changed differently is a conflict. Where a region stands in the result is worked out
// from the result's own text, so it follows any edit, undo and redo. Rewritten for yan.

import (
	"slices"
	"sort"

	"github.com/corn12138/yan-source/core/git/linediff"
)

// mergeRegion is a run of the base that ours, theirs or both changed: base[baseLo:baseHi] is
// ours[oursLo:oursHi] and theirs[theirsLo:theirsHi]. conflict says both changed it differently.
type mergeRegion struct {
	baseLo, baseHi     int
	oursLo, oursHi     int
	theirsLo, theirsHi int
	conflict           bool
	ours, theirs       bool // which sides changed it
}

// sideHunk is a change of one side from the base.
type sideHunk struct {
	linediff.Hunk
	theirs bool
}

// mergeRegions are the changed regions of a three-way merge, in the base's order.
func mergeRegions(base, ours, theirs []string) []mergeRegion {
	ho, ht := linediff.Hunks(base, ours), linediff.Hunks(base, theirs)
	var all []sideHunk
	for _, h := range ho {
		all = append(all, sideHunk{h, false})
	}
	for _, h := range ht {
		all = append(all, sideHunk{h, true})
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].A < all[j].A })
	var out []mergeRegion
	dOurs, dTheirs := 0, 0 // how far the sides have moved from the base before the region
	for i := 0; i < len(all); {
		lo, hi := all[i].A, all[i].A+all[i].ALen
		j := i + 1
		for j < len(all) && all[j].A <= hi { // touching or overlapping in the base
			hi = max(hi, all[j].A+all[j].ALen)
			j++
		}
		r := mergeRegion{baseLo: lo, baseHi: hi}
		inOurs, inTheirs := 0, 0
		for _, h := range all[i:j] {
			if h.theirs {
				r.theirs, inTheirs = true, inTheirs+h.BLen-h.ALen
			} else {
				r.ours, inOurs = true, inOurs+h.BLen-h.ALen
			}
		}
		r.oursLo, r.oursHi = lo+dOurs, hi+dOurs+inOurs
		r.theirsLo, r.theirsHi = lo+dTheirs, hi+dTheirs+inTheirs
		dOurs, dTheirs = dOurs+inOurs, dTheirs+inTheirs
		r.conflict = r.ours && r.theirs && !slices.Equal(ours[r.oursLo:r.oursHi], theirs[r.theirsLo:r.theirsHi])
		out = append(out, r)
		i = j
	}
	return out
}

// sideRange is where the base's lines [lo, hi) are in a text whose changes from the base are
// hs: the changes before them shift them, the changes within or inserted at their edges are
// theirs.
func sideRange(hs []linediff.Hunk, lo, hi int) (int, int) {
	before, in := 0, 0
	for _, h := range hs {
		switch {
		case h.ALen == 0 && lo <= h.A && h.A <= hi, h.ALen > 0 && h.A < hi && h.A+h.ALen > lo:
			in += h.BLen - h.ALen
			lo, hi = min(lo, h.A), max(hi, h.A+h.ALen)
		case h.A+h.ALen <= lo && (h.ALen > 0 || h.A < lo):
			before += h.BLen - h.ALen
		}
	}
	return lo + before, hi + before + in
}

// resultRanges are where the regions stand in the result: by the result's own changes from
// the base, as for a side.
func resultRanges(base, result []string, regions []mergeRegion) [][2]int {
	hs := linediff.Hunks(base, result)
	out := make([][2]int, len(regions))
	for i, r := range regions {
		lo, hi := sideRange(hs, r.baseLo, r.baseHi)
		out[i] = [2]int{max(min(lo, len(result)), 0), max(min(hi, len(result)), 0)}
	}
	return out
}

// regionState is how a conflict stands in the result.
type regionState uint8

const (
	regionUndecided regionState = iota // the base's lines, neither side taken nor ignored
	regionOurs                         // ours' lines
	regionTheirs                       // theirs' lines
	regionBoth                         // both sides' lines, one after the other
	regionIgnored                      // the base's lines on purpose
	regionEdited                       // lines of its own
)

func (s regionState) label() string {
	return [...]string{"未处理", "用了我们的", "用了他们的", "两边都用了", "忽略了两边的改动", "手改过"}[s]
}

// stateOf is how region r stands with lines in the result; ignored are the sides ignored.
func stateOf(base, ours, theirs, lines []string, r mergeRegion, ignored uint8) regionState {
	o, t, b := ours[r.oursLo:r.oursHi], theirs[r.theirsLo:r.theirsHi], base[r.baseLo:r.baseHi]
	switch {
	case slices.Equal(lines, o):
		return regionOurs
	case slices.Equal(lines, t):
		return regionTheirs
	case slices.Equal(lines, append(slices.Clone(o), t...)), slices.Equal(lines, append(slices.Clone(t), o...)):
		return regionBoth
	case slices.Equal(lines, b) && ignored != 0:
		return regionIgnored
	case slices.Equal(lines, b):
		return regionUndecided
	}
	return regionEdited
}

// autoMerged is the base with every region only one side changed taken from that side: the
// result a merge view starts from.
func autoMerged(base, ours, theirs []string, regions []mergeRegion) []string {
	out := make([]string, 0, len(base))
	at := 0
	for _, r := range regions {
		out = append(out, base[at:r.baseLo]...)
		switch {
		case r.conflict:
			out = append(out, base[r.baseLo:r.baseHi]...)
		case r.ours:
			out = append(out, ours[r.oursLo:r.oursHi]...)
		default:
			out = append(out, theirs[r.theirsLo:r.theirsHi]...)
		}
		at = r.baseHi
	}
	return append(out, base[at:]...)
}
