// Package updatehttp supplies desktop proxy support only for Yan's update hosts.
package updatehttp

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/corn12138/yan-source/platform/netproxy"
)

// Client preserves the original client and transport for unrelated requests.
// MyGo's updater uses http.DefaultClient for manifests and redirected archives;
// its signature and HTTPS verification remain in the original updater.
func Client(base *http.Client) *http.Client {
	client := *base
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = netproxy.Proxy
	fallback := base.Transport
	if fallback == nil {
		fallback = http.DefaultTransport
	}
	client.Transport = scopedTransport{fallback, transport}
	return &client
}

type scopedTransport struct{ fallback, updates http.RoundTripper }

func (t scopedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodGet && updateURL(req.URL) {
		return t.updates.RoundTrip(req)
	}
	return t.fallback.RoundTrip(req)
}

func updateURL(u *url.URL) bool {
	if u.Scheme != "https" {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "github.com":
		return strings.HasPrefix(u.Path, "/corn12138/yan-releases/releases/download/")
	case "release-assets.githubusercontent.com", "objects.githubusercontent.com":
		return true
	}
	return false
}
