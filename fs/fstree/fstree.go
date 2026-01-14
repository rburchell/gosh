// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fstree provides functionality to gather a directory tree.
//
// A tree looks like the output of tree(1), like:
//
//	.
//	├── go.mod
//	└── README.md
//
// The interface is designed to be as minimal as possible, and to that
// end, there are presently no configuration knobs, options, or anything.
//
// The primary usecase that is being served here is to make debugging tests
// easier, or for use in small one-off tools.
package fstree

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Simple helper to retrieve a directory tree.
func tree(path string) ([]string, error) {
	var lines []string

	var walk func(dir string, prefix string)
	walk = func(dir string, prefix string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		sort.Slice(entries, func(i, j int) bool {
			return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
		})

		for i, e := range entries {
			last := i == len(entries)-1

			connector := "├── "
			childPrefix := prefix + "│   "

			if last {
				connector = "└── "
				childPrefix = prefix + "    "
			}

			lines = append(lines, prefix+connector+e.Name())

			if e.IsDir() {
				walk(filepath.Join(dir, e.Name()), childPrefix)
			}
		}
	}

	lines = append(lines, filepath.Base(path))
	walk(path, "")

	return lines, nil
}

// Builds a fs tree, and returns it.
// Each entry is joined together in a newline-delimited string.
func Sprint(path string) (string, error) {
	tree, err := tree(path)
	if err != nil {
		return "", err
	}
	return strings.Join(tree, "\n"), nil
}

// Builds a fs tree, and writes to w.
// It returns the number of bytes written and any write error encountered.
func Fprint(w io.Writer, path string) (int, error) {
	s, err := Sprint(path)
	if err != nil {
		return 0, err
	}
	return fmt.Fprint(w, s)
}

// Write tree lines to stdout, return bytes written
func Print(path string) (int, error) {
	s, err := Sprint(path)
	if err != nil {
		return 0, err
	}
	return fmt.Print(s)
}
