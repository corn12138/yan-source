package releasechannel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

const manifestName = "update-darwin-arm64.json"

var disconnected = errors.New("connection lost")

type fakeBackend struct {
	r            Release
	data         map[int64][]byte
	next         int64
	creates      int
	uploads      int
	renames      int
	missing      bool
	readRelease  func() error
	upload       func([]byte) ([]byte, error)
	beforeRename func(int64, string) (bool, error)
	afterRename  func(int64, string)
	read         func(Asset) error
	public       func(name string, current []byte) []byte
	publicReads  int
}

func manifest(version string) []byte { return []byte(`{"version":"` + version + `"}`) }

func fixture() *fakeBackend {
	b := &fakeBackend{r: Release{ID: 1, Tag: "preview"}, data: map[int64][]byte{}}
	b.add(manifestName, manifest("0.1.0-alpha.26"))
	return b
}

func (b *fakeBackend) add(name string, data []byte) Asset {
	b.next++
	a := Asset{ID: b.next, Name: name}
	b.r.Assets = append(b.r.Assets, a)
	b.data[a.ID] = bytes.Clone(data)
	return a
}

func (b *fakeBackend) Release(ctx context.Context, _ string) (Release, error) {
	if err := ctx.Err(); err != nil {
		return Release{}, err
	}
	if b.readRelease != nil {
		if err := b.readRelease(); err != nil {
			return Release{}, err
		}
	}
	if b.missing {
		return Release{}, ErrNotFound
	}
	r := b.r
	r.Assets = slices.Clone(r.Assets)
	return r, nil
}

func (b *fakeBackend) Create(_ context.Context, channel string) (Release, error) {
	b.creates++
	b.missing = false
	b.r.Tag = channel
	return b.r, nil
}

func (b *fakeBackend) Upload(_ context.Context, _ string, name string, data []byte) error {
	b.uploads++
	var err error
	if b.upload != nil {
		data, err = b.upload(data)
	}
	if data != nil {
		b.add(name, data)
	}
	return err
}

func (b *fakeBackend) Rename(ctx context.Context, id int64, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.renames++
	apply, err := true, error(nil)
	if b.beforeRename != nil {
		apply, err = b.beforeRename(id, name)
	}
	if apply {
		for _, a := range b.r.Assets {
			if a.ID != id && a.Name == name {
				return errors.New("name occupied")
			}
		}
		for i, a := range b.r.Assets {
			if a.ID == id {
				b.r.Assets[i].Name = name
			}
		}
		if b.afterRename != nil {
			b.afterRename(id, name)
		}
	}
	return err
}

func (b *fakeBackend) Read(ctx context.Context, a Asset) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b.read != nil {
		if err := b.read(a); err != nil {
			return nil, err
		}
	}
	return bytes.Clone(b.data[a.ID]), nil
}

// Public resolves by name, as installed clients do; public can serve other bytes.
func (b *fakeBackend) Public(ctx context.Context, a Asset) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.publicReads++
	current, ok := named(b.r, a.Name)
	if !ok {
		return nil, errors.New("HTTP 404")
	}
	data := bytes.Clone(b.data[current.ID])
	if b.public != nil {
		data = b.public(a.Name, data)
	}
	return data, nil
}

func shortPublicWaits(t *testing.T) {
	saved := publicWaits
	publicWaits = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { publicWaits = saved })
}

func promote(b *fakeBackend) (string, error) {
	return Promote(context.Background(), b, "preview", "v0.1.0-alpha.27", manifestName, manifest("0.1.0-alpha.27"))
}

func assertVersion(t *testing.T, b *fakeBackend, want string) {
	t.Helper()
	a, ok := named(b.r, manifestName)
	if !ok {
		t.Fatal("channel has no public manifest")
	}
	v, err := manifestVersion(b.data[a.ID])
	if err != nil || v != want {
		t.Fatalf("public version %q, want %s: %v", v, want, err)
	}
}

func TestPromotionStagesBeforeReplacing(t *testing.T) {
	b := fixture()
	b.beforeRename = func(id int64, name string) (bool, error) {
		if id == 1 {
			stage, ok := named(b.r, fmt.Sprintf("%s.pending-%x", manifestName, sha256.Sum256(manifest("0.1.0-alpha.27"))))
			if !ok || !bytes.Equal(b.data[stage.ID], manifest("0.1.0-alpha.27")) {
				t.Fatal("current manifest moved before staged bytes were available")
			}
		}
		return true, nil
	}
	if _, err := promote(b); err != nil {
		t.Fatal(err)
	}
	assertVersion(t, b, "0.1.0-alpha.27")
	if _, ok := named(b.r, manifestName+".previous-1"); !ok {
		t.Fatal("old manifest was not retained")
	}
	uploads, renames := b.uploads, b.renames
	if _, err := promote(b); err != nil || b.uploads != uploads || b.renames != renames {
		t.Fatalf("same-version retry mutated the channel: %v", err)
	}
}

func TestPromotionFailuresPreserveCurrent(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*fakeBackend)
	}{
		{"lookup disconnected", func(b *fakeBackend) { b.readRelease = func() error { return disconnected } }},
		{"lookup forbidden", func(b *fakeBackend) { b.readRelease = func() error { return errors.New("HTTP 403") } }},
		{"upload failed", func(b *fakeBackend) { b.upload = func([]byte) ([]byte, error) { return nil, disconnected } }},
		{"staging corrupt", func(b *fakeBackend) {
			b.upload = func([]byte) ([]byte, error) { return manifest("0.1.0-alpha.9"), nil }
		}},
		{"activation failed", func(b *fakeBackend) {
			b.beforeRename = func(id int64, name string) (bool, error) {
				if id != 1 && name == manifestName {
					return false, disconnected
				}
				return true, nil
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := fixture()
			tc.setup(b)
			if _, err := promote(b); err == nil {
				t.Fatal("failure reported success")
			}
			assertVersion(t, b, "0.1.0-alpha.26")
			if b.creates != 0 {
				t.Fatal("query failure created another release")
			}
		})
	}
}

func TestPromotionObservesLostMutationResponses(t *testing.T) {
	for _, phase := range []string{"upload", "backup", "activate"} {
		t.Run(phase, func(t *testing.T) {
			b := fixture()
			if phase == "upload" {
				b.upload = func(data []byte) ([]byte, error) { return data, disconnected }
			}
			b.beforeRename = func(id int64, name string) (bool, error) {
				if phase == "backup" && id == 1 || phase == "activate" && name == manifestName {
					return true, disconnected
				}
				return true, nil
			}
			if _, err := promote(b); err != nil {
				t.Fatal(err)
			}
			assertVersion(t, b, "0.1.0-alpha.27")
			if b.uploads != 1 || b.renames != 2 {
				t.Fatal("lost response repeated an observed mutation")
			}
		})
	}
}

func TestPromotionCancellationRestoresOldManifest(t *testing.T) {
	b := fixture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b.afterRename = func(_ int64, name string) {
		if strings.Contains(name, ".previous-") {
			cancel()
		}
	}
	if _, err := Promote(ctx, b, "preview", "v0.1.0-alpha.27", manifestName, manifest("0.1.0-alpha.27")); err == nil {
		t.Fatal("cancelled promotion reported success")
	}
	assertVersion(t, b, "0.1.0-alpha.26")
}

func TestPromotionRecoversAcrossRuns(t *testing.T) {
	b := fixture()
	b.beforeRename = func(_ int64, name string) (bool, error) {
		if name == manifestName {
			return false, disconnected
		}
		return true, nil
	}
	if _, err := promote(b); err == nil || !strings.Contains(err.Error(), "restore retained") {
		t.Fatalf("missing recovery diagnostic: %v", err)
	}
	if _, ok := named(b.r, manifestName); ok {
		t.Fatal("failure injection did not leave the recovery case")
	}
	b.beforeRename = nil
	if _, err := promote(b); err != nil {
		t.Fatal(err)
	}
	assertVersion(t, b, "0.1.0-alpha.27")
	if b.uploads != 1 {
		t.Fatal("retry did not reuse the retained manifest")
	}
}

func TestPromotionCannotDowngradeOrChangeSameVersion(t *testing.T) {
	b := fixture()
	b.r.Assets[0].Name = manifestName + ".previous-1"
	b.data[1] = manifest("0.1.0")
	if result, err := promote(b); err != nil || !strings.Contains(result, "kept newer") {
		t.Fatalf("missing newer-version guard: %s %v", result, err)
	}
	assertVersion(t, b, "0.1.0")
	if b.uploads != 0 {
		t.Fatal("older release uploaded over recovered newer version")
	}
	b = fixture()
	b.data[1] = []byte(`{"version":"0.1.0-alpha.27","notes":"different"}`)
	if _, err := promote(b); err == nil || b.uploads != 0 || b.renames != 0 {
		t.Fatal("same version accepted different bytes")
	}
}

func TestPromotionCreatesOnlyMissingRelease(t *testing.T) {
	b := fixture()
	b.r.Assets, b.missing = nil, true
	if _, err := promote(b); err != nil {
		t.Fatal(err)
	}
	assertVersion(t, b, "0.1.0-alpha.27")
	if b.creates != 1 {
		t.Fatal("missing release was not created exactly once")
	}
	b = fixture()
	b.r.Assets = nil
	if _, err := promote(b); err == nil || b.creates != 0 || b.uploads != 0 {
		t.Fatal("empty existing channel lost its unknown version boundary")
	}
	b = fixture()
	if _, err := Promote(context.Background(), b, "stable", "v0.1.0-alpha.27", manifestName, manifest("0.1.0-alpha.27")); err == nil || b.uploads != 0 {
		t.Fatal("alpha promoted to stable")
	}
}

func TestPromotionDoesNotReplaceConcurrentPublisher(t *testing.T) {
	b := fixture()
	b.upload = func(data []byte) ([]byte, error) {
		b.r.Assets = nil
		b.add(manifestName, manifest("0.1.0-alpha.28"))
		return data, nil
	}
	if _, err := promote(b); err == nil || b.renames != 0 {
		t.Fatal("concurrent publisher was overwritten")
	}
	assertVersion(t, b, "0.1.0-alpha.28")
}

// Right after a rename, the public download name can still serve the renamed
// asset's bytes. Promotion waits for it rather than reporting a failure.
func TestPromotionWaitsForThePublicName(t *testing.T) {
	shortPublicWaits(t)
	b := fixture()
	old := manifest("0.1.0-alpha.26")
	stale := 2
	b.public = func(_ string, current []byte) []byte {
		if stale > 0 {
			stale--
			return old
		}
		return current
	}
	if result, err := promote(b); err != nil || result != "updated to 0.1.0-alpha.27" {
		t.Fatalf("%q: %v", result, err)
	}
	assertVersion(t, b, "0.1.0-alpha.27")
}

// A name still stale after the wait is reported, and a rerun neither restages nor
// renames: the current version is read by asset, not through the stale name.
func TestPromotionRerunAfterPublicLag(t *testing.T) {
	shortPublicWaits(t)
	b := fixture()
	old := manifest("0.1.0-alpha.26")
	b.public = func(string, []byte) []byte { return old }
	if _, err := promote(b); err == nil || !strings.Contains(err.Error(), "rerun to confirm") {
		t.Fatalf("stale public name accepted: %v", err)
	}
	assertVersion(t, b, "0.1.0-alpha.27")
	uploads, renames := b.uploads, b.renames
	if _, err := promote(b); err == nil || b.uploads != uploads || b.renames != renames {
		t.Fatalf("rerun with a stale name mutated the channel or passed: %v", err)
	}
	b.public = nil
	if result, err := promote(b); err != nil || result != "already at 0.1.0-alpha.27" || b.uploads != uploads || b.renames != renames {
		t.Fatalf("%q: %v", result, err)
	}
}
