// Package updatepack writes the signed archive format consumed by MyGo's updater.
// Archive encoding is adapted from MyGo internal/update (MIT); see THIRD_PARTY_NOTICES.md.
package updatepack

import (
	"archive/tar"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const Repository = "corn12138/yan-releases"

var tagPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-alpha\.[0-9]+)?$`)
var targetPattern = regexp.MustCompile(`^darwin-(arm64|amd64|universal)$`)

type Manifest struct {
	Version   string `json:"version"`
	Notes     string `json:"notes"`
	Date      string `json:"date"`
	URL       string `json:"url"`
	Size      int64  `json:"size"`
	Signature string `json:"signature"`
}

func Channel(tag string) (string, error) {
	if !tagPattern.MatchString(tag) {
		return "", errors.New("expected vX.Y.Z or vX.Y.Z-alpha.N")
	}
	if strings.Contains(tag, "-alpha.") {
		return "preview", nil
	}
	return "stable", nil
}

func Feed(channel string) string {
	return "https://github.com/" + Repository + "/releases/download/" + channel
}

// Key checks the public half too: a wrong key must fail before artifacts are published.
func Key(private, public string) (ed25519.PrivateKey, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(private))
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid update signing key")
	}
	// Re-derive from the seed instead of trusting the public bytes stored in a 64-byte key.
	derived := ed25519.NewKeyFromSeed(key[:ed25519.SeedSize])
	if base64.StdEncoding.EncodeToString(derived.Public().(ed25519.PublicKey)) != strings.TrimSpace(public) {
		return nil, errors.New("update signing key does not match the embedded public key")
	}
	return derived, nil
}

func Pack(bundle, out, tag, target, notes string, key ed25519.PrivateKey) (Manifest, error) {
	var m Manifest
	if _, err := Channel(tag); err != nil {
		return m, err
	}
	if !targetPattern.MatchString(target) {
		return m, errors.New("unsupported update target")
	}
	if len(key) != ed25519.PrivateKeySize {
		return m, errors.New("invalid update signing key")
	}
	if fi, err := os.Stat(bundle); err != nil || !fi.IsDir() {
		return m, errors.New("update input must be an application directory")
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		return m, err
	}
	name := "yan-" + tag + "-" + target + ".update.tar.gz"
	f, err := os.CreateTemp(out, ".archive-*")
	if err != nil {
		return m, err
	}
	defer os.Remove(f.Name())
	sum := sha256.New()
	err = writeArchive(io.MultiWriter(f, sum), bundle)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return m, err
	}
	fi, err := os.Stat(f.Name())
	if err != nil {
		return m, err
	}
	m = Manifest{Version: strings.TrimPrefix(tag, "v"), Notes: notes, Date: time.Now().UTC().Format(time.RFC3339),
		URL:  "https://github.com/" + Repository + "/releases/download/" + url.PathEscape(tag) + "/" + name,
		Size: fi.Size(), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, sum.Sum(nil)))}
	if err = os.Rename(f.Name(), filepath.Join(out, name)); err != nil {
		return m, err
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	err = os.WriteFile(filepath.Join(out, "update-"+target+".json"), append(data, '\n'), 0644)
	return m, err
}

func Verify(path string, m Manifest, public string) error {
	pub, err := base64.StdEncoding.DecodeString(strings.TrimSpace(public))
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return errors.New("invalid update public key")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sum := sha256.New()
	n, err := io.Copy(sum, f)
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil || n != m.Size || !ed25519.Verify(pub, sum.Sum(nil), sig) {
		return errors.New("update size or signature verification failed")
	}
	return nil
}

func writeArchive(w io.Writer, bundle string) error {
	gz, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	root := filepath.Dir(filepath.Clean(bundle))
	err = filepath.WalkDir(bundle, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !d.IsDir() && info.Mode()&fs.ModeSymlink == 0 {
			return fmt.Errorf("unsupported archive entry: %s", p)
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		link := ""
		if info.Mode()&fs.ModeSymlink != 0 {
			link, err = os.Readlink(p)
			if err != nil {
				return err
			}
			// The updater refuses escaping links. Reject them before publishing the archive too.
			within, err := filepath.Rel(bundle, filepath.Join(filepath.Dir(p), link))
			if err != nil || filepath.IsAbs(link) || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
				return errors.New("update archive has an escaping symlink")
			}
		}
		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if d.IsDir() {
			hdr.Name += "/"
		}
		hdr.Uid, hdr.Gid, hdr.Uname, hdr.Gname = 0, 0, "", ""
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			f, err := os.Open(p)
			if err != nil {
				return err
			}
			_, err = io.Copy(tw, f)
			f.Close()
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err = tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// CanPromote permits retries of the same version, but never a downgrade.
func CanPromote(previous, tag string) (bool, error) {
	if _, err := Channel(tag); err != nil {
		return false, err
	}
	if _, err := Channel("v" + strings.TrimPrefix(previous, "v")); err != nil {
		return false, err
	}
	parts := func(v string) []int64 {
		fields := strings.FieldsFunc(strings.TrimPrefix(v, "v"), func(r rune) bool { return r == '.' || r == '-' })
		out := []int64{0, 0, 0, 1, 0}
		for i := range 3 {
			out[i], _ = strconv.ParseInt(fields[i], 10, 64)
		}
		if len(fields) > 3 {
			out[3] = 0
			out[4], _ = strconv.ParseInt(fields[4], 10, 64)
		}
		return out
	}
	a, b := parts(tag), parts(previous)
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i], nil
		}
	}
	return true, nil
}
