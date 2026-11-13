package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newFS(t *testing.T) (*FS, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := NewFS(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, dir
}

func TestRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, dir := newFS(t)
	key := NewKey()
	data := []byte("hello attachment")
	n, err := s.Put(ctx, key, bytes.NewReader(data))
	if err != nil || n != int64(len(data)) {
		t.Fatalf("Put = %d, %v", n, err)
	}
	f, size, err := s.Open(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(f)
	_ = f.Close()
	if size != int64(len(data)) || !bytes.Equal(got, data) {
		t.Fatalf("Open = %q (%d)", got, size)
	}
	info, err := os.Stat(filepath.Join(dir, key[:2], key[2:4], key))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != filePerm {
		t.Errorf("mode = %v, want %v", info.Mode().Perm(), os.FileMode(filePerm))
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Open(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: err = %v", err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("second delete: %v", err)
	}
}

type failingReader struct{ sent bool }

var errBroken = errors.New("connection reset")

func (r *failingReader) Read(p []byte) (int, error) {
	if r.sent {
		return 0, errBroken
	}
	r.sent = true
	return copy(p, "partial"), nil
}

func TestFailedPutLeavesNothing(t *testing.T) {
	ctx := context.Background()
	s, dir := newFS(t)
	key := NewKey()
	if _, err := s.Put(ctx, key, &failingReader{}); !errors.Is(err, errBroken) {
		t.Fatalf("err = %v", err)
	}
	if _, _, err := s.Open(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("file exists after failed put: %v", err)
	}
	tmp, _ := os.ReadDir(filepath.Join(dir, tmpDir))
	if len(tmp) != 0 {
		t.Fatalf("temp files left: %v", tmp)
	}
}

func TestInvalidKeys(t *testing.T) {
	ctx := context.Background()
	s, _ := newFS(t)
	for _, key := range []string{"", "../../etc/passwd", strings.Repeat("A", 32), strings.Repeat("a", 31), "ab/cd"} {
		if _, err := s.Put(ctx, key, strings.NewReader("x")); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Put(%q): err = %v", key, err)
		}
		if _, _, err := s.Open(ctx, key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Open(%q): err = %v", key, err)
		}
		if err := s.Delete(ctx, key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Delete(%q): err = %v", key, err)
		}
	}
	if a, b := NewKey(), NewKey(); a == b || !ValidKey(a) {
		t.Fatalf("NewKey gave %q and %q", a, b)
	}
}

func TestPurgeTemp(t *testing.T) {
	ctx := context.Background()
	s, dir := newFS(t)
	old := filepath.Join(dir, tmpDir, "old"+tmpSuffix)
	fresh := filepath.Join(dir, tmpDir, "fresh"+tmpSuffix)
	for _, p := range []string{old, fresh} {
		if err := os.WriteFile(p, []byte("x"), filePerm); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	n, err := s.PurgeTemp(ctx, time.Hour)
	if err != nil || n != 1 {
		t.Fatalf("PurgeTemp = %d, %v", n, err)
	}
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("old temp file kept: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("fresh temp file removed: %v", err)
	}
}

func TestNewFSRejectsUnwritableDir(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, filePerm); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFS(file); err == nil {
		t.Fatal("NewFS on a regular file succeeded")
	}
}
