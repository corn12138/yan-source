// Package releasechannel promotes already verified update manifests without
// deleting the only recoverable copy of a channel's current manifest.
package releasechannel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/corn12138/yan-source/internal/updatepack"
)

var ErrNotFound = errors.New("release not found")

type Asset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	API  string `json:"url"`
}

type Release struct {
	ID     int64   `json:"id"`
	Tag    string  `json:"tag_name"`
	Draft  bool    `json:"draft"`
	Assets []Asset `json:"assets"`
}

type Backend interface {
	Release(context.Context, string) (Release, error)
	Create(context.Context, string) (Release, error)
	Upload(context.Context, string, string, []byte) error
	Rename(context.Context, int64, string) error
	// Read returns this asset's own bytes. Reads by name can briefly return a
	// renamed asset's old bytes, which would misjudge the current version.
	Read(context.Context, Asset) ([]byte, error)
	// Public returns what installed clients download under the asset's name.
	Public(context.Context, Asset) ([]byte, error)
}

// publicWaits bounds how long a promotion waits for the public download name
// to serve the new bytes after a rename.
var publicWaits = []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second, 30 * time.Second}

func verifyPublic(ctx context.Context, b Backend, a Asset, data []byte) error {
	var last error
	for attempt := 0; ; attempt++ {
		public, err := b.Public(ctx, a)
		if err == nil && bytes.Equal(public, data) {
			return nil
		}
		last = err
		if attempt == len(publicWaits) {
			break
		}
		if err := delay(ctx, publicWaits[attempt]); err != nil {
			return err
		}
	}
	return errors.Join(last, errors.New("channel manifest is updated, but its public download still serves other bytes; rerun to confirm"))
}

func named(r Release, name string) (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

func manifestVersion(data []byte) (string, error) {
	var m updatepack.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return "", err
	}
	_, err := updatepack.Channel("v" + m.Version)
	return m.Version, err
}

// Promote accepts a manifest whose numbered archive has already passed signature
// verification. Publishers must run serially; GitHub asset renames have no CAS.
// Backups stay on the release so a later run can recover after process termination.
func Promote(ctx context.Context, b Backend, channel, tag, name string, data []byte) (result string, err error) {
	kind, err := updatepack.Channel(tag)
	if err != nil {
		return "", err
	}
	if channel != "stable" && channel != "preview" || kind == "preview" && channel == "stable" {
		return "", errors.New("invalid update channel for release")
	}
	if !strings.HasPrefix(name, "update-") || !strings.HasSuffix(name, ".json") || strings.ContainsAny(name, "/\\") {
		return "", errors.New("invalid manifest filename")
	}
	version, err := manifestVersion(data)
	if err != nil || version != strings.TrimPrefix(tag, "v") {
		return "", errors.New("manifest version differs from release tag")
	}
	r, err := b.Release(ctx, channel)
	created := errors.Is(err, ErrNotFound)
	if created {
		r, err = b.Create(ctx, channel)
		if err != nil {
			// A lost response does not prove the create failed. Observe before retrying.
			observed, readErr := b.Release(ctx, channel)
			if readErr != nil {
				return "", errors.Join(err, readErr)
			}
			r, err = observed, nil
		}
	}
	if err != nil {
		return "", err
	}
	if r.Draft || r.Tag != channel {
		return "", errors.New("channel is not a published release")
	}
	old, exists := named(r, name)
	if !exists && !created {
		// Restore the newest retained manifest before judging a requested version;
		// an interrupted newer publication must not turn into a downgrade.
		old, err = recoverMissing(ctx, b, r, name)
		if err != nil {
			return "", err
		}
		exists = true
	}
	var previous []byte
	if exists {
		previous, err = b.Read(ctx, old)
		if err != nil {
			return "", err
		}
		current, readErr := manifestVersion(previous)
		if readErr != nil {
			return "", fmt.Errorf("invalid current manifest: %w", readErr)
		}
		advance, checkErr := updatepack.CanPromote(current, tag)
		if checkErr != nil {
			return "", checkErr
		}
		if !advance {
			return "kept newer " + current, nil
		}
		if current == version {
			if !bytes.Equal(previous, data) {
				return "", errors.New("same version has different manifest bytes")
			}
			if err := verifyPublic(ctx, b, old, data); err != nil {
				return "", err
			}
			return "already at " + version, nil
		}
	}
	stageName := fmt.Sprintf("%s.pending-%x", name, sha256.Sum256(data))
	stage, ok := named(r, stageName)
	if !ok {
		uploadErr := b.Upload(ctx, channel, stageName, data)
		r, err = b.Release(ctx, channel)
		if err != nil {
			return "", errors.Join(uploadErr, err)
		}
		stage, ok = named(r, stageName)
		if !ok {
			return "", errors.Join(uploadErr, errors.New("staged manifest was not uploaded; current channel is unchanged"))
		}
	}
	staged, err := b.Read(ctx, stage)
	if err != nil || !bytes.Equal(staged, data) {
		return "", errors.Join(err, errors.New("staged manifest verification failed; current channel is unchanged"))
	}
	r, err = b.Release(ctx, channel)
	if err != nil {
		return "", err
	}
	current, present := named(r, name)
	if present != exists || present && current.ID != old.ID {
		return "", errors.New("channel changed during staging; retry with its current version")
	}
	// Recovery uses a fresh bounded context: cancellation must not strand the
	// channel between the two renames. An unrelated publisher's asset is untouched.
	if exists {
		defer func() {
			if err == nil {
				return
			}
			repair, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			r, checkErr := b.Release(repair, channel)
			if checkErr != nil {
				err = errors.Join(err, fmt.Errorf("could not inspect recovery state: %w", checkErr))
				return
			}
			if _, present := named(r, name); !present {
				if restoreErr := rename(repair, b, channel, old.ID, name); restoreErr != nil {
					err = errors.Join(err, fmt.Errorf("restore retained manifest on next run: %w", restoreErr))
				}
			}
		}()
		if err = rename(ctx, b, channel, old.ID, fmt.Sprintf("%s.previous-%d", name, old.ID)); err != nil {
			return "", err
		}
	}
	if err = rename(ctx, b, channel, stage.ID, name); err != nil {
		return "", err
	}
	r, err = b.Release(ctx, channel)
	if err != nil {
		return "", err
	}
	current, ok = named(r, name)
	if !ok || current.ID != stage.ID {
		return "", errors.New("published channel identity could not be verified")
	}
	installed, err := b.Read(ctx, current)
	if err != nil || !bytes.Equal(installed, data) {
		return "", errors.Join(err, errors.New("channel manifest bytes could not be verified; retained assets are available for retry"))
	}
	if err := verifyPublic(ctx, b, current, data); err != nil {
		return "", err
	}
	return "updated to " + version, nil
}

func recoverMissing(ctx context.Context, b Backend, r Release, name string) (Asset, error) {
	var best Asset
	var version string
	for _, a := range r.Assets {
		if !strings.HasPrefix(a.Name, name+".previous-") && !strings.HasPrefix(a.Name, name+".pending-") {
			continue
		}
		data, err := b.Read(ctx, a)
		if err != nil {
			return Asset{}, err
		}
		v, err := manifestVersion(data)
		if err != nil {
			return Asset{}, err
		}
		if strings.HasPrefix(a.Name, name+".pending-") && a.Name != fmt.Sprintf("%s.pending-%x", name, sha256.Sum256(data)) {
			return Asset{}, errors.New("retained staged manifest has different bytes")
		}
		newer := best.ID == 0
		if !newer {
			newer, err = updatepack.CanPromote(version, "v"+v)
			if err != nil {
				return Asset{}, err
			}
		}
		if newer {
			best, version = a, v
		}
	}
	if best.ID == 0 {
		return Asset{}, errors.New("channel has no manifest or retained recovery copy; inspect its release history before repairing")
	}
	if err := rename(ctx, b, r.Tag, best.ID, name); err != nil {
		return Asset{}, err
	}
	r, err := b.Release(ctx, r.Tag)
	if err != nil {
		return Asset{}, err
	}
	a, ok := named(r, name)
	if !ok || a.ID != best.ID {
		return Asset{}, errors.New("recovered manifest identity differs")
	}
	return a, nil
}

func rename(ctx context.Context, b Backend, channel string, id int64, name string) error {
	var last error
	for range 3 {
		if err := ctx.Err(); err != nil {
			return err
		}
		last = b.Rename(ctx, id, name)
		r, err := b.Release(ctx, channel)
		if err != nil {
			return errors.Join(last, err)
		}
		if a, ok := named(r, name); ok {
			if a.ID == id {
				return nil
			}
			return errors.New("another asset occupies the requested name")
		}
	}
	return errors.Join(last, errors.New("asset rename was not observed"))
}
