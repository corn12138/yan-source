//go:build !darwin && !linux

package process

import (
	"context"
	"time"
)

const supportsLoginShell = false

func discoverLoginPath(context.Context, []string, time.Duration) string { return "" }
