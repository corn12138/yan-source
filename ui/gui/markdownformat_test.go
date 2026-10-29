package gui

import "testing"

func TestMarkdownToggleWrap(t *testing.T) {
	for _, tc := range []struct {
		text       string
		start, end int
		mark, want string
		ws, we     int
	}{
		{"加粗这里", 2, 4, "**", "加粗**这里**", 4, 6},
		{"加粗**这里**", 4, 6, "**", "加粗这里", 2, 4},      // marks just outside the selection
		{"加粗**这里**", 2, 8, "**", "加粗这里", 2, 4},      // marks inside the selection
		{"ab", 1, 1, "*", "a**b", 2, 2},             // nothing selected: a pair, caret inside
		{"**bold**", 2, 6, "*", "***bold***", 3, 7}, // italic inside bold is added, not stripped
		{"x `code` y", 3, 7, "`", "x code y", 2, 6},
	} {
		got := mdToggleWrap(tc.text, tc.start, tc.end, tc.mark)
		if got.text != tc.want || got.start != tc.ws || got.end != tc.we {
			t.Errorf("%q [%d,%d] %s = %q [%d,%d], want %q [%d,%d]", tc.text, tc.start, tc.end, tc.mark, got.text, got.start, got.end, tc.want, tc.ws, tc.we)
		}
	}
}

func TestMarkdownLink(t *testing.T) {
	if got := mdLink("see docs", 4, 8); got.text != "see [docs]()" || got.start != 11 {
		t.Fatalf("%+v", got)
	}
	if got := mdLink("x", 1, 1); got.text != "x[]()" || got.start != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestMarkdownEnterContinues(t *testing.T) {
	for _, tc := range []struct {
		text  string
		caret int
		want  string
		at    int
		ok    bool
	}{
		{"- 苹果", 4, "- 苹果\n- ", 7, true},
		{"  * a", 5, "  * a\n  * ", 10, true},
		{"9. nine", 7, "9. nine\n10. ", 12, true},
		{"1) one", 6, "1) one\n2) ", 10, true},
		{"- [x] done", 10, "- [x] done\n- [ ] ", 17, true},
		{"> quote", 7, "> quote\n> ", 10, true},
		{"- a\n- ", 6, "- a\n", 4, true}, // an empty item ends the list
		{"- ab", 3, "- a\n- b", 6, true}, // in the middle of an item: split it
		{"plain", 5, "", 0, false},
		{"- a", 1, "", 0, false}, // the caret in the marker
	} {
		got, ok := mdEnter(tc.text, tc.caret, tc.caret)
		if ok != tc.ok || ok && (got.text != tc.want || got.start != tc.at) {
			t.Errorf("%q@%d = %q@%d %v, want %q@%d %v", tc.text, tc.caret, got.text, got.start, ok, tc.want, tc.at, tc.ok)
		}
	}
}

func TestMarkdownIndentList(t *testing.T) {
	got, ok := mdIndent("- a\n- b\n1. c", 4, 8, false)
	if !ok || got.text != "- a\n  - b\n   1. c" || got.start != 6 || got.end != 13 {
		t.Fatalf("indent %+v %v", got, ok)
	}
	back, ok := mdIndent(got.text, got.start, got.end, true)
	if !ok || back.text != "- a\n- b\n1. c" {
		t.Fatalf("outdent %+v %v", back, ok)
	}
	if _, ok := mdIndent("plain", 0, 0, false); ok {
		t.Fatal("Tab outside a list must stay the box's own")
	}
}

func TestMarkdownSlashMenu(t *testing.T) {
	q, at, ok := mdSlash("text\n/he", 8)
	if !ok || q != "he" || at != 5 {
		t.Fatalf("slash %q %d %v", q, at, ok)
	}
	if _, _, ok := mdSlash("a/b", 3); ok {
		t.Fatal("a slash inside a line is text")
	}
	if _, _, ok := mdSlash("/two words", 10); ok {
		t.Fatal("a space ends the command")
	}
	ms := mdBlockMatches("he")
	if len(ms) != 3 || ms[0].label != "一级标题" {
		t.Fatalf("matches %v", ms)
	}
	if got := mdInsertBlock("text\n/code", 5, 10, mdBlockMatches("code")[0]); got.text != "text\n```\n\n```" || got.start != 9 {
		t.Fatalf("code block %+v", got)
	}
	if got := mdInsertBlock("/biaoti", 0, 7, mdBlockMatches("biaoti")[1]); got.text != "## " || got.start != 3 {
		t.Fatalf("pinyin heading %+v", got)
	}
}
