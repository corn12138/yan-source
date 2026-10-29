package releasechannel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/corn12138/yan-source/internal/updatepack"
)

// GitHub keeps credentials inside gh. Manifest reads are unauthenticated: by
// asset ID to judge identity, and by the download name installed clients use.
type GitHub struct {
	run func(context.Context, []string, []byte) ([]byte, error)
}

func (g GitHub) command(ctx context.Context, args []string, input []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if g.run != nil {
		return g.run(ctx, args, input)
	}
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Stdin = bytes.NewReader(input)
	return cmd.Output()
}

func githubResponse(raw []byte) (int, []byte, error) {
	// gh prints a status line followed by MIME headers and decoded body bytes;
	// treating this as a wire response would incorrectly decode chunked bodies again.
	end := bytes.Index(raw, []byte("\r\n\r\n"))
	sep := 4
	if end < 0 {
		end, sep = bytes.Index(raw, []byte("\n\n")), 2
	}
	line, _, _ := bytes.Cut(raw, []byte("\n"))
	fields := strings.Fields(string(line))
	if end < 0 || len(fields) < 2 || !strings.HasPrefix(fields[0], "HTTP/") {
		return 0, nil, errors.New("GitHub response has no HTTP status")
	}
	status, err := strconv.Atoi(fields[1])
	if err != nil || status < 100 || status > 599 {
		return 0, nil, errors.New("invalid GitHub HTTP status")
	}
	return status, raw[end+sep:], nil
}

func (g GitHub) api(ctx context.Context, method, path string, input any, output any) error {
	args := []string{"api", "--include", "--method", method, "repos/" + updatepack.Repository + path}
	var data []byte
	if input != nil {
		var err error
		data, err = json.Marshal(input)
		if err != nil {
			return err
		}
		args = append(args, "--input", "-")
	}
	var last error
	for attempt := range 3 {
		raw, cmdErr := g.command(ctx, args, data)
		status, body, parseErr := githubResponse(raw)
		if status == http.StatusNotFound {
			return ErrNotFound
		}
		if cmdErr == nil && parseErr == nil && status >= 200 && status < 300 {
			if output == nil {
				return nil
			}
			return json.Unmarshal(body, output)
		}
		cause := errors.Join(cmdErr, parseErr)
		if cause == nil {
			cause = errors.New(http.StatusText(status))
		}
		last = fmt.Errorf("GitHub %s %s failed (HTTP %d): %w", method, path, status, cause)
		if method != "GET" || status >= 400 && status < 500 || attempt == 2 {
			return last
		}
		if err := delay(ctx, time.Duration(attempt+1)*time.Second); err != nil {
			return err
		}
	}
	return last
}

func (g GitHub) Release(ctx context.Context, channel string) (Release, error) {
	var r Release
	if err := g.api(ctx, "GET", "/releases/tags/"+url.PathEscape(channel), nil, &r); err != nil {
		return r, err
	}
	// Retained manifests can outgrow a single asset-list page.
	r.Assets = nil
	for page := 1; ; page++ {
		var assets []Asset
		path := fmt.Sprintf("/releases/%d/assets?per_page=100&page=%d", r.ID, page)
		if err := g.api(ctx, "GET", path, nil, &assets); err != nil {
			if errors.Is(err, ErrNotFound) {
				// Only the tag lookup can establish that a channel is absent.
				return Release{}, errors.New("asset listing returned 404 for an existing channel")
			}
			return Release{}, err
		}
		r.Assets = append(r.Assets, assets...)
		if len(assets) < 100 {
			return r, nil
		}
	}
}

func (g GitHub) Create(ctx context.Context, channel string) (Release, error) {
	var repo struct {
		Branch string `json:"default_branch"`
	}
	if err := g.api(ctx, "GET", "", nil, &repo); err != nil {
		return Release{}, err
	}
	if repo.Branch == "" {
		return Release{}, errors.New("public repository has no default branch")
	}
	var r Release
	err := g.api(ctx, "POST", "/releases", map[string]any{
		"tag_name": channel, "target_commitish": repo.Branch, "prerelease": true, "make_latest": "false",
		"name": "yan " + channel + " update channel",
		"body": "Automatic-update manifest. Download installers from a numbered release.",
	}, &r)
	return r, err
}

func (g GitHub) Upload(ctx context.Context, channel, name string, data []byte) error {
	dir, err := os.MkdirTemp("", "yan-manifest-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, name)
	if err = os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	_, err = g.command(ctx, []string{"release", "upload", channel, "--repo", updatepack.Repository, path}, nil)
	return err
}

func (g GitHub) Rename(ctx context.Context, id int64, name string) error {
	return g.api(ctx, "PATCH", fmt.Sprintf("/releases/assets/%d", id), map[string]string{"name": name}, nil)
}

func (g GitHub) Read(ctx context.Context, asset Asset) ([]byte, error) {
	if asset.API == "" {
		return nil, errors.New("asset has no API URL")
	}
	return readRetrying(ctx, asset.API, "application/octet-stream")
}

func (g GitHub) Public(ctx context.Context, asset Asset) ([]byte, error) {
	return readRetrying(ctx, asset.URL, "")
}

func readRetrying(ctx context.Context, source, accept string) ([]byte, error) {
	var last error
	for attempt := range 3 {
		data, err := readPublic(ctx, source, accept)
		if err == nil {
			return data, nil
		}
		last = err
		if attempt == 2 {
			break
		}
		if err := delay(ctx, time.Duration(attempt+1)*time.Second); err != nil {
			return nil, err
		}
	}
	return nil, last
}

func readPublic(ctx context.Context, source, accept string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", source, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("public manifest download: HTTP %d", res.StatusCode)
	}
	const maxManifest = 1 << 20
	data, err := io.ReadAll(io.LimitReader(res.Body, maxManifest+1))
	if len(data) > maxManifest {
		return nil, errors.New("update manifest exceeds 1 MiB")
	}
	return data, err
}

func delay(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
