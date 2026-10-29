package workspace

import (
	"context"
	"os"
	"testing"
)

// 用法：YAN_WALK_ROOT=~/go/pkg/mod go test -run XXX -bench Walk -cpuprofile cpu.out ./core/workspace
func BenchmarkWalk(b *testing.B) {
	root := os.Getenv("YAN_WALK_ROOT")
	if root == "" {
		b.Skip("set YAN_WALK_ROOT")
	}
	for b.Loop() {
		n := 0
		Walk(context.Background(), root, NewIgnore(root), func(p []string) { n += len(p) })
		b.ReportMetric(float64(n), "files")
	}
}
