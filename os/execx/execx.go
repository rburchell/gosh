// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package execx provides some helpers for os/exec.
package execx

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// Runs a given cmd, and reads all stdout/stderr from it.
func Slurp(cmd *exec.Cmd) ([]byte, []byte, error) {
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("slurp: %s: can't get stderr: %s", cmd.String(), err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("slurp: %s: can't get stdout: %s", cmd.String(), err)
	}
	stderrbuf := []byte{}
	stdoutbuf := []byte{}
	var wg sync.WaitGroup
	wg.Add(2)

	slurper := func(buf *[]byte, reader io.ReadCloser) {
		*buf, _ = io.ReadAll(reader)
		wg.Done()
	}

	go slurper(&stderrbuf, stderr)
	go slurper(&stdoutbuf, stdout)

	if err := cmd.Start(); err != nil {
		return stdoutbuf, stderrbuf, fmt.Errorf("slurp: %s: can't start: %s", cmd.String(), err)
	}
	wg.Wait()
	if err := cmd.Wait(); err != nil {
		return stdoutbuf, stderrbuf, fmt.Errorf("slurp: %s: can't wait: %s", cmd.String(), err)
	}

	return stdoutbuf, stderrbuf, nil
}

// Runs a given cmd synchronously.
// stderr and stdout are redirected to os.Stderr/Stdout
func ExecSync(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("wait: %w", err)
	}

	return nil
}

// Runs a given cmd asynchronously.
// stderr and stdout are redirected to os.Stderr/Stdout
func ExecAsync(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	return nil
}
