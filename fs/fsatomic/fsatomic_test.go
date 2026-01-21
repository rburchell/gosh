package fsatomic

import (
	"os"
	"path/filepath"
	"testing"
)

// These tests are really only best effort.
// We'd have to mock out an interface to allow fs ops to fail to "really" test this,
// but I'm not convinced that's worthwhile.

func TestWriteFileAtomicSuccess(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "test.txt")
	content := []byte("hello world")

	err := WriteFile(target, content, 0600)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	read, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(read) != string(content) {
		t.Errorf("Content mismatch: got %q, want %q", string(read), string(content))
	}
}

func TestWriteFileAtomicOverwrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "test.txt")
	initial := []byte("first")
	final := []byte("second")

	// First write
	err := WriteFile(target, initial, 0600)
	if err != nil {
		t.Fatalf("First WriteFile failed: %v", err)
	}

	// Overwrite
	err = WriteFile(target, final, 0600)
	if err != nil {
		t.Fatalf("Second WriteFile failed: %v", err)
	}

	read, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(read) != string(final) {
		t.Errorf("Content mismatch: got %q, want %q", string(read), string(final))
	}
}

func TestWriteFileAtomicBadPath(t *testing.T) {
	dir := t.TempDir()
	// Deliberately use a nonexistent subdir
	badfile := filepath.Join(dir, "nonexistent", "test.txt")
	err := WriteFile(badfile, []byte("data"), 0600)
	if err == nil {
		t.Fatal("Expected failure on bad path, got nil")
	}
}
