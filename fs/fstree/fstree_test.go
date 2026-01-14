// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fstree

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func mustWriteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
}

func assertEqual(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Logf("Got:")
		for _, line := range got {
			t.Logf("%s", line)
		}
		t.Logf("Want:")
		for _, line := range want {
			t.Logf("%s", line)
		}
		t.Fatalf("got:\n%v\nwant:\n%v", got, want)
	}
}

// Main test for the tree() func itself.
func TestTree(t *testing.T) {
	type test struct {
		name     string
		before   func(string)
		expected func(string) []string
	}

	tests := []test{
		test{
			name: "simple",
			before: func(dir string) {
				mustWriteFile(t, filepath.Join(dir, "README.md"))
				mustWriteFile(t, filepath.Join(dir, "go.mod"))
			},
			expected: func(dir string) []string {
				return []string{
					filepath.Base(dir),
					"├── go.mod",
					"└── README.md",
				}
			},
		},
		test{
			name: "nested",
			before: func(dir string) {
				mustMkdir(t, filepath.Join(dir, "cmd"))
				mustWriteFile(t, filepath.Join(dir, "cmd", "a.go"))
				mustWriteFile(t, filepath.Join(dir, "cmd", "foobar.txt"))
				mustWriteFile(t, filepath.Join(dir, "README.md"))
				mustWriteFile(t, filepath.Join(dir, "another.txt"))
			},
			expected: func(dir string) []string {
				return []string{
					filepath.Base(dir),
					"├── another.txt",
					"├── cmd",
					"│   ├── a.go",
					"│   └── foobar.txt",
					"└── README.md",
				}
			},
		},
		test{
			name: "deep",
			before: func(dir string) {
				mustMkdir(t, filepath.Join(dir, "cmd"))
				mustMkdir(t, filepath.Join(dir, "cmd", "foobar"))
				mustMkdir(t, filepath.Join(dir, "cmd", "foobar", "bin"))
				mustWriteFile(t, filepath.Join(dir, "cmd", "foobar", "bin", "foobar-linux-arm64"))
			},
			expected: func(dir string) []string {
				return []string{
					filepath.Base(dir),
					"└── cmd",
					"    └── foobar",
					"        └── bin",
					"            └── foobar-linux-arm64",
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			tt.before(dir)

			got, err := tree(dir)
			if err != nil {
				t.Fatal(err)
			}

			want := tt.expected(dir)
			assertEqual(t, got, want)
		})
	}
}

/////////////////////////
// Tests for the public interface below.
/////////////////////////

func setupTestDir(t *testing.T) string {
	dir := t.TempDir()

	// Structure:
	// dir/
	//   a.txt
	//   b/
	//     c.txt
	//   d/
	//     e/
	//       f.txt
	mustWriteFile(t, filepath.Join(dir, "a.txt"))
	mustMkdir(t, filepath.Join(dir, "b"))
	mustWriteFile(t, filepath.Join(dir, "b", "c.txt"))
	mustMkdir(t, filepath.Join(dir, "d"))
	mustMkdir(t, filepath.Join(dir, "d", "e"))
	mustWriteFile(t, filepath.Join(dir, "d", "e", "f.txt"))

	return dir
}

func TestSprint(t *testing.T) {
	dir := setupTestDir(t)

	got, err := Sprint(dir)
	if err != nil {
		t.Fatalf("Sprint() error = %v", err)
	}

	base := filepath.Base(dir)
	want := base + `
├── a.txt
├── b
│   └── c.txt
└── d
    └── e
        └── f.txt`
	if got != want {
		t.Errorf("Sprint() got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFprint(t *testing.T) {
	dir := setupTestDir(t)
	var buf bytes.Buffer

	n, err := Fprint(&buf, dir)
	if err != nil {
		t.Fatalf("Fprint() error = %v", err)
	}
	got := buf.String()

	base := filepath.Base(dir)
	want := base + `
├── a.txt
├── b
│   └── c.txt
└── d
    └── e
        └── f.txt`
	if got != want {
		t.Errorf("Fprint() got:\n%s\nwant:\n%s", got, want)
	}
	if n != len(got) {
		t.Errorf("Fprint() bytes written %d, want %d", n, len(got))
	}
}

func TestPrint(t *testing.T) {
	dir := setupTestDir(t)
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	n, err := Print(dir)
	if err != nil {
		t.Fatalf("Print() error = %v", err)
	}

	w.Close()
	os.Stdout = origStdout

	out, _ := io.ReadAll(r)
	got := string(out)
	base := filepath.Base(dir)
	want := base + `
├── a.txt
├── b
│   └── c.txt
└── d
    └── e
        └── f.txt`
	if got != want {
		t.Errorf("Print() got:\n%s\nwant:\n%s", got, want)
	}
	if n != len(got) {
		t.Errorf("Print() bytes written %d, want %d", n, len(got))
	}
}
