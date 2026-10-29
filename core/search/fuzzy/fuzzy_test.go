package fuzzy

import (
	"context"
	"testing"
)

func TestMatchRanksShortAndContiguous(t *testing.T) {
	paths := []string{"core/app/editor.go", "docs/ARCHITECTURE.md", "ui/tui/tui.go", "core/edit/edit.go"}
	res := Match(context.Background(), "edit", paths, 10)
	if len(res) < 2 || res[0].Path != "core/edit/edit.go" {
		t.Fatalf("res = %+v", res)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if Match(ctx, "edit", paths, 10) != nil {
		t.Fatal("cancelled match should return nil")
	}
}
