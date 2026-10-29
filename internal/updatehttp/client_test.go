package updatehttp

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestUpdateTransportScope(t *testing.T) {
	used := ""
	mark := func(label string) transportFunc {
		return func(*http.Request) (*http.Response, error) {
			used = label
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
		}
	}
	tr := scopedTransport{mark("base"), mark("update")}
	for _, tc := range []struct{ method, url, want string }{
		{"GET", "https://github.com/corn12138/yan-releases/releases/download/preview/update-darwin-arm64.json", "update"},
		{"GET", "https://release-assets.githubusercontent.com/github-production-release-asset/123/asset", "update"},
		{"GET", "https://github.com/another/repo", "base"},
		{"GET", "https://api.github.com/repos/corn12138/yan", "base"},
		{"GET", "https://github.com.example/corn12138/yan-releases/releases/download/preview/a", "base"},
		{"GET", "http://github.com/corn12138/yan-releases/releases/download/preview/a", "base"},
		{"POST", "https://github.com/corn12138/yan-releases/releases/download/preview/a", "base"},
	} {
		req, _ := http.NewRequest(tc.method, tc.url, nil)
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if used != tc.want {
			t.Fatalf("%s %s routed to %s", tc.method, tc.url, used)
		}
	}
	base := &http.Client{Timeout: 3 * time.Second, Transport: mark("original")}
	client := Client(base)
	if client == base || client.Timeout != base.Timeout {
		t.Fatal("client settings not preserved")
	}
	resp, err := client.Get("https://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if used != "original" {
		t.Fatal("unrelated request transport replaced")
	}
}
