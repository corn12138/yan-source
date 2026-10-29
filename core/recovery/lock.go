package recovery

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrInUse is returned by Acquire when another yan process owns the session: two writers of one
// checkpoint would each replace the other's drafts.
var ErrInUse = errors.New("another yan is keeping this session's untitled documents")

// Lock is held by the one editor that reads and writes a session's checkpoint.
type Lock struct{ f *os.File }

// Acquire takes the session id for this process until Release. It fails with ErrInUse when another
// process holds it; the operating system releases the lock if that process dies.
func Acquire(dir, id string) (*Lock, error) {
	if !idPattern.MatchString(id) {
		return nil, errors.New("invalid recovery id")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, id+".lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := lockFile(f); err != nil {
		f.Close()
		return nil, err
	}
	return &Lock{f: f}, nil
}

// Release gives the session up. The lock file stays: removing it could let a third process lock a
// new file while a second still holds the old one.
func (l *Lock) Release() {
	if l != nil && l.f != nil {
		unlockFile(l.f)
		l.f.Close()
		l.f = nil
	}
}
