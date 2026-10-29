package workspace

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, c := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func fixture(t *testing.T) string {
	root := t.TempDir()
	write(t, root, map[string]string{
		".gitignore":          "dist/\n*.log\n",
		"main.go":             "",
		"debug.log":           "",
		"dist/bundle.js":      "",
		"node_modules/x/i.js": "",
		"web/.gitignore":      "cache/\n!keep.log\n",
		"web/app.ts":          "",
		"web/cache/c.txt":     "",
		"web/keep.log":        "",
		"docs/中文说明.md":        "",
		".git/HEAD":           "",
		"Zeta/readme.txt":     "",
		"alpha/b.txt":         "",
	})
	return root
}

func TestWalkRespectsGitignore(t *testing.T) {
	root := fixture(t)
	var mu sync.Mutex
	var got []string
	err := Walk(context.Background(), root, NewIgnore(root), func(b []string) {
		mu.Lock()
		got = append(got, b...)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	want := []string{".gitignore", "Zeta/readme.txt", "alpha/b.txt", "docs/中文说明.md", "main.go", "web/.gitignore", "web/app.ts", "web/keep.log"}
	if !slices.Equal(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestTreeOrderAndReveal(t *testing.T) {
	root := fixture(t)
	tr := NewTree(root, NewIgnore(root))
	var names []string
	for _, n := range tr.Root.Children {
		names = append(names, n.Name)
	}
	// 目录在前、不区分大小写排序；.git、node_modules 不显示；被忽略的 dist 显示但变暗
	want := []string{"alpha", "dist", "docs", "web", "Zeta", ".gitignore", "debug.log", "main.go"}
	if !slices.Equal(names, want) {
		t.Fatalf("children = %v", names)
	}
	if !tr.Root.Children[1].Ignored || tr.Root.Children[0].Ignored {
		t.Fatal("dist should be dimmed, alpha not")
	}
	n := tr.Reveal("web/cache/c.txt")
	if n == nil || !n.Ignored {
		t.Fatalf("reveal: %+v", n)
	}
	// 根下 8 项 + web 下 4 项（cache .gitignore app.ts keep.log）+ cache 下 1 项
	if got := len(tr.Visible(nil)); got != 13 {
		t.Fatalf("visible = %d, want 13", got)
	}
}
