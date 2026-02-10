package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteFile(t *testing.T) {
	t.Parallel()

	t.Run("basic write", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "out.json")
		data := []byte(`{"hello": "world"}`)

		if err := writeFile(path, data, 0o644); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if !bytes.Equal(got, data) {
			t.Errorf("content mismatch:\ngot:  %s\nwant: %s", got, data)
		}
	})

	t.Run("overwrite existing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "out.json")
		os.WriteFile(path, []byte("old content"), 0o644)

		newData := []byte("new content")
		if err := writeFile(path, newData, 0o644); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := os.ReadFile(path)
		if !bytes.Equal(got, newData) {
			t.Errorf("content mismatch:\ngot:  %s\nwant: %s", got, newData)
		}
	})

	t.Run("preserves permissions", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "out.json")

		if err := writeFile(path, []byte("data"), 0o600); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		info, _ := os.Stat(path)
		if got := info.Mode().Perm(); got != 0o600 {
			t.Errorf("perm = %o, want 600", got)
		}
	})

	t.Run("no temp files on success", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "out.json")

		writeFile(path, []byte("data"), 0o644)

		matches, _ := filepath.Glob(filepath.Join(dir, "out.json.tmp.*"))
		if len(matches) != 0 {
			t.Errorf("temp files remain: %v", matches)
		}
	})

	t.Run("empty data", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "out.json")

		if err := writeFile(path, []byte{}, 0o644); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := os.ReadFile(path)
		if len(got) != 0 {
			t.Errorf("expected empty file, got %d bytes", len(got))
		}
	})

	t.Run("large data", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "out.json")
		data := bytes.Repeat([]byte("x"), 2*1024*1024)

		if err := writeFile(path, data, 0o644); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := os.ReadFile(path)
		if !bytes.Equal(got, data) {
			t.Error("large data content mismatch")
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join("/nonexistent", "dir", "out.json")
		err := writeFile(path, []byte("data"), 0o644)
		if err == nil {
			t.Fatal("expected error for nonexistent directory")
		}
	})

	t.Run("readonly directory", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("skip on windows")
		}

		dir := t.TempDir()
		os.Chmod(dir, 0o555)
		t.Cleanup(func() { os.Chmod(dir, 0o755) })

		path := filepath.Join(dir, "out.json")
		err := writeFile(path, []byte("data"), 0o644)
		if err == nil {
			t.Fatal("expected error for readonly directory")
		}

		matches, _ := filepath.Glob(filepath.Join(dir, "*.tmp.*"))
		if len(matches) != 0 {
			t.Errorf("temp files remain after error: %v", matches)
		}
	})

	t.Run("symlink is preserved", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		realPath := filepath.Join(dir, "real.json")
		linkPath := filepath.Join(dir, "link.json")

		os.WriteFile(realPath, []byte("original"), 0o644)
		os.Symlink(realPath, linkPath)

		newData := []byte("updated")
		if err := writeFile(linkPath, newData, 0o644); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		target, err := os.Readlink(linkPath)
		if err != nil {
			t.Fatal("symlink was replaced by regular file")
		}
		if target != realPath {
			t.Errorf("symlink target changed: got %s, want %s", target, realPath)
		}

		got, _ := os.ReadFile(realPath)
		if !bytes.Equal(got, newData) {
			t.Error("target content not updated")
		}
	})
}
