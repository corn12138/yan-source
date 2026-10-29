// Package recovery keeps the text of untitled documents across quits: one checkpoint file per
// editor session, replaced atomically, so a crash at any point leaves either the previous or the
// next complete checkpoint.
package recovery

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	schema = 1
	// The limits keep a damaged or hostile file from taking the startup down.
	maxDraft  = 16 << 20
	maxFile   = 64 << 20
	maxDrafts = 500
)

var idPattern = regexp.MustCompile(`^[fr]-[0-9a-f]{16}$`)

// Draft is one untitled document: its name, text and where its tab was.
type Draft struct {
	Title  string `json:"title"`
	Text   string `json:"text"`
	EOL    int    `json:"eol,omitempty"`
	Column int    `json:"column,omitempty"`
	Anchor int    `json:"anchor,omitempty"`
	Head   int    `json:"head,omitempty"`
}

// Checkpoint is everything kept for one editor session. Gen orders checkpoints of one session.
type Checkpoint struct {
	Schema int     `json:"schema"`
	ID     string  `json:"id"`
	Root   string  `json:"root,omitempty"`
	Gen    uint64  `json:"gen"`
	Drafts []Draft `json:"drafts"`
}

// FolderID is the identity of the session of a folder: one folder is open in one project at a time.
func FolderID(root string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(root)))
	return "f-" + hex.EncodeToString(sum[:8])
}

// NewRootlessID is a fresh identity for a project without a folder.
func NewRootlessID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand does not fail on supported platforms
	}
	return "r-" + hex.EncodeToString(b[:])
}

// Rootless reports whether id names a session without a folder.
func Rootless(id string) bool { return strings.HasPrefix(id, "r-") }

func path(dir, id string) (string, error) {
	if !idPattern.MatchString(id) {
		return "", fmt.Errorf("invalid recovery id %q", id)
	}
	return filepath.Join(dir, id+".json"), nil
}

// Write replaces the checkpoint of cp.ID with cp, or removes it when cp keeps no drafts. The new
// file is complete and synced before it takes the name.
func Write(dir string, cp Checkpoint) error {
	p, err := path(dir, cp.ID)
	if err != nil {
		return err
	}
	if len(cp.Drafts) > maxDrafts {
		return fmt.Errorf("too many untitled documents to keep (%d)", len(cp.Drafts))
	}
	for _, d := range cp.Drafts {
		if len(d.Text) > maxDraft {
			return fmt.Errorf("%s is too large to keep (%d MB)", d.Title, len(d.Text)>>20)
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if len(cp.Drafts) == 0 {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return syncDir(dir)
	}
	cp.Schema = schema
	b, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	if len(b) > maxFile {
		return fmt.Errorf("untitled documents too large to keep (%d MB)", len(b)>>20)
	}
	f, err := os.CreateTemp(dir, "."+cp.ID+"-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			f.Close()
			os.Remove(tmp)
		}
	}()
	if err := f.Chmod(0o600); err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		return err
	}
	ok = true
	return syncDir(dir)
}

// syncDir makes a rename or removal in dir durable. Windows cannot open a directory to sync it;
// there the rename itself is what the file system offers.
func syncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// Load reads every checkpoint in dir. A file that cannot be read as a checkpoint is moved aside
// (its name gets ".corrupt-<time>") and reported in problems; it never hides the others.
func Load(dir string) (cps []Checkpoint, problems []error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []error{err}
	}
	for _, e := range entries {
		name := e.Name()
		id, isJSON := strings.CutSuffix(name, ".json")
		if !isJSON || !e.Type().IsRegular() {
			continue // temp files of an interrupted write, files moved aside
		}
		cp, err := read(filepath.Join(dir, name), id)
		if err != nil {
			problems = append(problems, isolate(dir, name, err))
			continue
		}
		cps = append(cps, cp)
	}
	return cps, problems
}

// Exists reports whether id has a checkpoint.
func Exists(dir, id string) bool {
	p, err := path(dir, id)
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Read reads the checkpoint of id; a missing one reads as no drafts.
func Read(dir, id string) (Checkpoint, error) {
	p, err := path(dir, id)
	if err != nil {
		return Checkpoint{}, err
	}
	cp, err := read(p, id)
	if errors.Is(err, os.ErrNotExist) {
		return Checkpoint{ID: id}, nil
	}
	if err != nil {
		return Checkpoint{ID: id}, isolate(dir, filepath.Base(p), err)
	}
	return cp, nil
}

// isolate moves an unreadable checkpoint aside, so that it stays for inspection but no longer
// stands in for the session's drafts.
func isolate(dir, name string, cause error) error {
	err := fmt.Errorf("%s: %w", name, cause)
	aside := filepath.Join(dir, name+".corrupt-"+time.Now().Format("20060102-150405.000"))
	if renameErr := os.Rename(filepath.Join(dir, name), aside); renameErr != nil {
		return errors.Join(err, renameErr)
	}
	return err
}

func read(p, id string) (Checkpoint, error) {
	if !idPattern.MatchString(id) {
		return Checkpoint{}, errors.New("not a recovery file name")
	}
	f, err := os.Open(p)
	if err != nil {
		return Checkpoint{}, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxFile+1))
	if err != nil {
		return Checkpoint{}, err
	}
	if len(b) > maxFile {
		return Checkpoint{}, errors.New("recovery file too large")
	}
	var cp Checkpoint
	if err := json.Unmarshal(b, &cp); err != nil {
		return Checkpoint{}, err
	}
	switch {
	case cp.Schema != schema:
		return Checkpoint{}, fmt.Errorf("unknown recovery schema %d", cp.Schema)
	case cp.ID != id:
		return Checkpoint{}, errors.New("recovery file names another session")
	case len(cp.Drafts) > maxDrafts:
		return Checkpoint{}, errors.New("too many drafts")
	}
	for _, d := range cp.Drafts {
		if len(d.Text) > maxDraft {
			return Checkpoint{}, errors.New("draft too large")
		}
	}
	return cp, nil
}
