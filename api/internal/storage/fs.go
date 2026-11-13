package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"time"
)

// Filesystem layout: files live at ab/cd/<key> (sharded by the key's first two byte pairs, so
// no directory grows huge); writes go to .tmp/ first and are renamed into place.
const (
	tmpDir       = ".tmp"
	tmpSuffix    = ".part"
	probeName    = "probe" + tmpSuffix
	shardWidth   = 2
	dirPerm      = 0o700
	filePerm     = 0o600
	createFlags  = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	readAllNames = -1
)

// FS stores files in a local directory. Every path goes through os.Root, so nothing outside
// the directory can be reached even if a key were crafted.
type FS struct {
	root *os.Root
}

var _ Store = (*FS)(nil)

// NewFS opens (creating if needed) the directory and checks it is writable, so a bad
// STORAGE_DIR fails at startup instead of on the first upload.
func NewFS(dir string) (*FS, error) {
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return nil, fmt.Errorf("storage dir: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("storage dir: %w", err)
	}
	s := &FS{root: root}
	if err := s.probe(); err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("storage dir %s is not writable: %w", dir, err)
	}
	return s, nil
}

func (s *FS) probe() error {
	if err := s.root.MkdirAll(tmpDir, dirPerm); err != nil {
		return err
	}
	name := path.Join(tmpDir, probeName)
	_ = s.root.Remove(name) // left over from a crash
	if err := s.root.WriteFile(name, nil, filePerm); err != nil {
		return err
	}
	return s.root.Remove(name)
}

func filePath(key string) string {
	return path.Join(key[:shardWidth], key[shardWidth:2*shardWidth], key)
}

// Put writes to a temporary file, syncs it, then renames it into place and syncs the
// directory, so a crash never leaves a half-written file under a real key.
func (s *FS) Put(_ context.Context, key string, r io.Reader) (int64, error) {
	if !ValidKey(key) {
		return 0, ErrInvalidKey
	}
	tmp := path.Join(tmpDir, key+tmpSuffix)
	f, err := s.root.OpenFile(tmp, createFlags, filePerm)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(f, r)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = s.root.Remove(tmp)
		return 0, err
	}
	final := filePath(key)
	dir := path.Dir(final)
	if err := s.root.MkdirAll(dir, dirPerm); err != nil {
		_ = s.root.Remove(tmp)
		return 0, err
	}
	if err := s.root.Rename(tmp, final); err != nil {
		_ = s.root.Remove(tmp)
		return 0, err
	}
	s.syncDir(dir)
	return n, nil
}

// syncDir makes the rename durable. Some platforms (macOS) can't fsync a directory; the
// file itself is already synced, so this is best effort.
func (s *FS) syncDir(dir string) {
	d, err := s.root.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}

func (s *FS) Open(_ context.Context, key string) (io.ReadSeekCloser, int64, error) {
	if !ValidKey(key) {
		return nil, 0, ErrInvalidKey
	}
	f, err := s.root.Open(filePath(key))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, err
	}
	return f, info.Size(), nil
}

func (s *FS) Delete(_ context.Context, key string) error {
	if !ValidKey(key) {
		return ErrInvalidKey
	}
	if err := s.root.Remove(filePath(key)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (s *FS) PurgeTemp(_ context.Context, olderThan time.Duration) (int, error) {
	d, err := s.root.Open(tmpDir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	entries, err := d.ReadDir(readAllNames)
	_ = d.Close()
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-olderThan)
	removed := 0
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || e.IsDir() || !info.ModTime().Before(cutoff) {
			continue
		}
		if err := s.root.Remove(path.Join(tmpDir, e.Name())); err == nil {
			removed++
		}
	}
	return removed, nil
}

func (s *FS) Close() error { return s.root.Close() }
