package text

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"unicode/utf8"
)

const (
	// NormalMax 以下全部功能开启。
	NormalMax = 8 << 20
	// LargeMax 以上拒绝打开。
	LargeMax = 256 << 20
)

var bom = []byte{0xEF, 0xBB, 0xBF}

// ErrTooLarge 表示文件超过 LargeMax。
var ErrTooLarge = errors.New("文件超过 256 MB，Yan 不打开")

// Load 只读一次文件，识别编码与换行，切成叶子。文件不存在时返回空的新文档。
// 叶子直接切自读入的缓冲区（零拷贝）；之后的修改只会新建叶子，绝不写回这块缓冲区。
func Load(path string) (Snapshot, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Snapshot{}, err
	}
	m := &Meta{Path: abs}
	fi, err := os.Stat(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return newSnapshot(newFlat(nil), m), nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	if fi.IsDir() {
		return Snapshot{}, fmt.Errorf("%s 是目录（文件树在 M3 实现）", path)
	}
	if fi.Size() > LargeMax {
		return Snapshot{}, ErrTooLarge
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return Snapshot{}, err
	}
	m.ModTime, m.Size = fi.ModTime(), fi.Size()
	if bytes.HasPrefix(data, bom) {
		m.Encoding = EncUTF8BOM
		data = data[len(bom):]
	}
	m.EOL, m.MixedEOL = detectEOL(data)
	if len(data) > NormalMax {
		m.Tier, m.ReadOnly = TierLarge, true
	}
	if !utf8.Valid(data) {
		m.ReadOnly = true // GB18030 等编码在 M1 实现；先只读，避免保存时损坏
	}
	return newSnapshot(newFlat(split(data)), m), nil
}

func detectEOL(b []byte) (EOL, bool) {
	lf := bytes.Count(b, []byte{'\n'})
	crlf := bytes.Count(b, []byte("\r\n"))
	if crlf > 0 && crlf*2 > lf {
		return EOLCRLF, crlf != lf
	}
	return EOLLF, crlf > 0
}

// Save 原子地把快照写到 path：同目录临时文件 → fsync → rename，保留原文件权限。
// 换行原样写回（rope 不做换行归一化），BOM 按 Meta 写回。
func Save(s Snapshot, path string) error { return SaveChecked(s, path, nil) }

// SaveChecked verifies the destination after preparing the replacement, immediately
// before rename. A rejected write removes its temporary file and leaves the destination intact.
// Filesystems offer no portable compare-and-swap rename; another writer must still cooperate.
func SaveChecked(s Snapshot, path string, verify func() error) error {
	mode := fs.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".yan-save-*")
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
	if s.meta.Encoding == EncUTF8BOM {
		if _, err := f.Write(bom); err != nil {
			return err
		}
	}
	for c := range s.Chunks(0, s.Len()) {
		if _, err := f.Write(c); err != nil {
			return err
		}
	}
	if err := f.Chmod(mode); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if verify != nil {
		if err := verify(); err != nil {
			return err
		}
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	ok = true
	return nil
}

// MatchesFile compares on-disk bytes, including BOM and line endings, with a saved
// snapshot. Streaming bounds scratch memory even for a large read-only document.
func MatchesFile(s Snapshot, path string) (bool, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("%s is not a regular file", path)
	}
	size := int64(s.Len())
	if s.meta.Encoding == EncUTF8BOM {
		size += int64(len(bom))
	}
	if info.Size() != size {
		return false, nil
	}
	buf := make([]byte, 32*1024)
	equal := func(data []byte) (bool, error) {
		for len(data) > 0 {
			n := min(len(data), len(buf))
			if _, err := io.ReadFull(f, buf[:n]); err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
					return false, nil
				}
				return false, err
			}
			if !bytes.Equal(data[:n], buf[:n]) {
				return false, nil
			}
			data = data[n:]
		}
		return true, nil
	}
	if s.meta.Encoding == EncUTF8BOM {
		if ok, err := equal(bom); !ok || err != nil {
			return ok, err
		}
	}
	for part := range s.Chunks(0, s.Len()) {
		if ok, err := equal(part); !ok || err != nil {
			return ok, err
		}
	}
	// A writer may append after Stat; matching the original prefix is insufficient.
	n, err := f.Read(buf[:1])
	if n > 0 {
		return false, nil
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return true, nil
}
