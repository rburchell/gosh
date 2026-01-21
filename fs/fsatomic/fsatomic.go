// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fsatomic provides os.WriteFile() that attempts to ensure atomic writing.
//
// Filesystem semantics mean that writing a file is not generally atomic.
// If a crash or power loss occurs during writing, the file content may be lost entirely,
// or end up inconsistent.
//
// fsatomic attempts to mitigate this by writing the content to a temporary file,
// and renaming it to the target location, as well as syncing the filesystem contents
// between steps to attempt to ensure that things happen consistently.
package fsatomic

import (
	"fmt"
	"os"
	"path"
)

// Writes 'file' atomically, such that either the old or the new content will always be completely present.
func WriteFile(file string, data []byte, perm os.FileMode) error {
	// Find a good temporary location in the target directory
	dir := path.Dir(file)
	tmpfile, err := os.CreateTemp(dir, path.Base(file)+".tmp-*")
	if err != nil {
		return fmt.Errorf("tmp create: %w", err)
	}
	tmp := tmpfile.Name()

	// Clean up the temp file if something went wrong.
	removeTemp := true
	defer func() {
		if removeTemp {
			os.Remove(tmp)
		}
	}()

	err = os.WriteFile(tmp, data, perm)
	if err != nil {
		return fmt.Errorf("tmp write: %w", err)
	}
	fh, err := os.Open(tmp)
	if err != nil {
		return fmt.Errorf("tmp open: %w", err)
	}
	// Sync to ensure the file contents end up on disk
	err = fh.Sync()
	if err != nil {
		fh.Close() // best effort..
		return fmt.Errorf("tmp sync: %w", err)
	}
	err = fh.Close()
	if err != nil {
		return fmt.Errorf("tmp close: %w", err)
	}

	// Now that we're relatively sure the content is on disk, we need to rename.
	err = os.Rename(tmp, file)
	if err != nil {
		return fmt.Errorf("tmp rename: %w", err)
	}
	removeTemp = false

	// Sync to ensure the rename ends up on disk
	dh, err := os.Open(path.Dir(file))
	if err != nil {
		return fmt.Errorf("dir open: %w", err)
	}
	err = dh.Sync()
	if err != nil {
		dh.Close() // best effort..
		return fmt.Errorf("dir sync: %w", err)
	}
	err = dh.Close()
	if err != nil {
		return fmt.Errorf("dir close: %w", err)
	}
	return nil
}
