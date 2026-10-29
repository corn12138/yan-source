//go:build !darwin

package netproxy

import "context"

// Windows/Linux keep the same provider boundary. Environment proxies work on
// all platforms; native desktop proxy discovery can be added here independently.
func readSystem(context.Context) (settings, error) { return settings{}, nil }
