// Package netproxy resolves explicit environment proxies and desktop proxy settings.
package netproxy

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http/httpproxy"
)

type settings struct {
	config        httpproxy.Config
	excludeSimple bool
}

var systemCache struct {
	sync.Mutex
	settings settings
	until    time.Time
}

// Proxy preserves explicit environment settings, including NO_PROXY. Desktop
// launches usually have no proxy environment, so they use the system provider.
func Proxy(req *http.Request) (*url.URL, error) {
	env := httpproxy.FromEnvironment()
	if env.HTTPProxy != "" || env.HTTPSProxy != "" {
		return env.ProxyFunc()(req.URL)
	}
	systemCache.Lock()
	defer systemCache.Unlock()
	if time.Now().After(systemCache.until) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		st, err := readSystem(ctx)
		if err != nil {
			return nil, err
		}
		systemCache.settings, systemCache.until = st, time.Now().Add(15*time.Second)
	}
	return systemCache.settings.proxy(req.URL, env.NoProxy)
}

func (s settings) proxy(u *url.URL, noProxy string) (*url.URL, error) {
	if s.excludeSimple && !strings.Contains(u.Hostname(), ".") && !strings.Contains(u.Hostname(), ":") {
		return nil, nil
	}
	cfg := s.config
	cfg.NoProxy = strings.Trim(cfg.NoProxy+","+noProxy, ",")
	return cfg.ProxyFunc()(u)
}
