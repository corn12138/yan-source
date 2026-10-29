package releasechannel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubMissingRequiresHTTP404(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		missing      bool
	}{
		{"explicit404", "HTTP/1.1 404 Not Found\nContent-Type: application/json\r\n\r\n{}", true},
		{"forbidden", "HTTP/2.0 403 Forbidden\r\n\r\n{}", false},
		{"disconnected", "", false},
		{"textOnly", "gh: Not Found (HTTP 404)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			g := GitHub{run: func(context.Context, []string, []byte) ([]byte, error) {
				cancel()
				return []byte(tc.output), disconnected
			}}
			_, err := g.Release(ctx, "preview")
			if err == nil || errors.Is(err, ErrNotFound) != tc.missing {
				t.Fatalf("classification: %v", err)
			}
		})
	}
}

func TestGitHubListsAllRecoveryAssets(t *testing.T) {
	pages := 0
	g := GitHub{run: func(_ context.Context, args []string, _ []byte) ([]byte, error) {
		path := args[len(args)-1]
		var response any
		if strings.Contains(path, "/tags/") {
			response = Release{ID: 42, Tag: "preview"}
		} else {
			pages++
			count := 100
			if pages == 2 {
				count = 1
			}
			assets := make([]Asset, count)
			for i := range assets {
				assets[i] = Asset{ID: int64((pages-1)*100 + i + 1)}
			}
			response = assets
		}
		data, _ := json.Marshal(response)
		return append([]byte("HTTP/2.0 200 OK\nContent-Type: application/json\r\n\r\n"), data...), nil
	}}
	r, err := g.Release(context.Background(), "preview")
	if err != nil || pages != 2 || len(r.Assets) != 101 || r.Assets[100].ID != 101 {
		t.Fatalf("pagination: %d pages, %d assets, %v", pages, len(r.Assets), err)
	}
}

func TestGitHubAssetListing404IsNotMissingChannel(t *testing.T) {
	g := GitHub{run: func(_ context.Context, args []string, _ []byte) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "/tags/") {
			return []byte("HTTP/1.1 200 OK\r\n\r\n{\"id\":42,\"tag_name\":\"preview\"}"), nil
		}
		return []byte("HTTP/1.1 404 Not Found\r\n\r\n{}"), disconnected
	}}
	if _, err := g.Release(context.Background(), "preview"); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("asset listing erased an existing channel: %v", err)
	}
}

func TestPublicManifestReadHasNoAuthentication(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("public read included credentials")
		}
		if r.URL.Path == "/large" {
			fmt.Fprint(w, strings.Repeat("x", (1<<20)+1))
			return
		}
		fmt.Fprint(w, `{"version":"0.1.0-alpha.27"}`)
	}))
	defer s.Close()
	if data, err := readPublic(context.Background(), s.URL, ""); err != nil || string(data) != string(manifest("0.1.0-alpha.27")) {
		t.Fatalf("public read: %q %v", data, err)
	}
	if _, err := readPublic(context.Background(), s.URL+"/large", ""); err == nil {
		t.Fatal("oversized manifest accepted")
	}
}

// Identity reads go to the asset's own API URL; only Public goes through the
// download name, which can lag behind a rename.
func TestGitHubReadsAssetByIdentity(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("manifest read included credentials")
		}
		switch {
		case r.URL.Path == "/assets/7" && r.Header.Get("Accept") == "application/octet-stream":
			fmt.Fprint(w, `{"version":"0.1.0-alpha.28"}`)
		case r.URL.Path == "/download/preview/update-darwin-arm64.json":
			fmt.Fprint(w, `{"version":"0.1.0-alpha.27"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer s.Close()
	a := Asset{ID: 7, Name: manifestName, URL: s.URL + "/download/preview/update-darwin-arm64.json", API: s.URL + "/assets/7"}
	if data, err := (GitHub{}).Read(context.Background(), a); err != nil || string(data) != string(manifest("0.1.0-alpha.28")) {
		t.Fatalf("identity read: %q %v", data, err)
	}
	if data, err := (GitHub{}).Public(context.Background(), a); err != nil || string(data) != string(manifest("0.1.0-alpha.27")) {
		t.Fatalf("public read: %q %v", data, err)
	}
	if _, err := (GitHub{}).Read(context.Background(), Asset{URL: a.URL}); err == nil {
		t.Fatal("read an asset without its API URL through its name")
	}
}
