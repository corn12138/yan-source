package netproxy

import (
	"context"
	"fmt"
	"os/exec"
)

func readSystem(ctx context.Context) (settings, error) {
	// Read the effective network service configuration, without changing it or
	// importing a cgo dependency into the future Windows/Linux frontends.
	out, err := exec.CommandContext(ctx, "/usr/sbin/scutil", "--proxy").Output()
	if err != nil {
		return settings{}, fmt.Errorf("reading system proxy: %w", err)
	}
	return parseMacSettings(string(out))
}
