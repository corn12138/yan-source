package app

import (
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/git/linediff"
)

func TestFitCells(t *testing.T) {
	out, kept, offs := fitCells("\tab", 6, 4)
	if out != "    ab" || kept != 3 || offs[1] != 4 || offs[3] != 6 {
		t.Fatalf("tab: %q %d %v", out, kept, offs)
	}
	out, kept, _ = fitCells("中文字", 5, 4) // 每个字两格：放得下两个
	if out != "中文 " || kept != len("中文") {
		t.Fatalf("wide: %q %d", out, kept)
	}
}

// 并排：改过的行左右对着，左边红、右边绿，改了的词换到排过的位置；只加的行左边空着，没有左边的样式。
func TestSplitRows(t *testing.T) {
	base := []string{"same", "\told word", "end"}
	cur := []string{"same", "\tnew word", "added", "end"}
	hunks := linediff.Hunks(base, cur)
	var texts []string
	var lines []diffLine
	put := func(s string, l diffLine) { texts = append(texts, s); lines = append(lines, l) }
	fold := 0
	writeSplit(put, base, cur, hunks, 0, &fold, map[int]bool{}, diffOpts{splitW: 20, tabW: 4, context: 3})
	var changed diffLine
	var changedText string
	for i, l := range lines {
		if l.leftGone && l.rightAdded {
			changed, changedText = l, texts[i]
		}
		if l.split && l.rightAdded && !l.leftGone && strings.TrimSpace(texts[i][:l.rightAt-len(splitSep)]) != "" {
			t.Fatalf("added-only row has a left side: %q", texts[i])
		}
	}
	if changedText == "" || !strings.HasPrefix(changedText, "    old word") || !strings.Contains(changedText, " │     new word") {
		t.Fatalf("changed row %q", changedText)
	}
	if len(changed.words) == 0 || changedText[changed.words[0].Start:changed.words[0].End] != "old" {
		t.Fatalf("left word %+v in %q", changed.words, changedText)
	}
	if len(changed.rwords) == 0 || changedText[changed.rwords[0].Start:changed.rwords[0].End] != "new" {
		t.Fatalf("right word %+v in %q", changed.rwords, changedText)
	}
}
