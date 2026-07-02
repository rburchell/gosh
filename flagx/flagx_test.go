// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flagx

import (
	"os"
	"testing"
	"time"
)

func TestFromEnvkv(t *testing.T) {
	defer clearVars()

	var s string
	var b bool
	var i int
	var d time.Duration

	StringVar(&s, "str", "def", "help")
	BoolVar(&b, "bool", false, "help")
	IntVar(&i, "int", 1, "help")
	DurationVar(&d, "dur", time.Second, "help")

	os.WriteFile(".envkv", []byte("STR=fromenvkv\nBOOL=false\nINT=999\nDUR=1h30m\n"), 0644)
	defer os.Remove(".envkv")

	origArgs := os.Args
	os.Args = []string{"cmd"}
	defer func() { os.Args = origArgs }()

	Parse()

	if s != "fromenvkv" {
		t.Errorf("expected 'fromenvkv', got %q", s)
	}
	if b != false {
		t.Errorf("expected bool false, got %v", b)
	}
	if i != 999 {
		t.Errorf("expected int 999, got %d", i)
	}
	if d != 90*time.Minute {
		t.Errorf("expected 1h30m, got %v", d)
	}
}

func TestFromEnvironment(t *testing.T) {
	defer clearVars()

	var s string
	var b bool
	var i int
	var d time.Duration

	StringVar(&s, "str", "def", "help")
	BoolVar(&b, "bool", false, "help")
	IntVar(&i, "int", 1, "help")
	DurationVar(&d, "dur", time.Second, "help")

	os.Setenv("STR", "fromenv")
	os.Setenv("BOOL", "true")
	os.Setenv("INT", "2")
	os.Setenv("DUR", "5m")
	defer os.Unsetenv("STR")
	defer os.Unsetenv("BOOL")
	defer os.Unsetenv("INT")
	defer os.Unsetenv("DUR")

	origArgs := os.Args
	os.Args = []string{"cmd"}
	defer func() { os.Args = origArgs }()

	Parse()

	if s != "fromenv" {
		t.Errorf("expected 'fromenv', got %q", s)
	}
	if b != true {
		t.Errorf("expected bool true, got %v", b)
	}
	if i != 2 {
		t.Errorf("expected int 2, got %d", i)
	}
	if d != 5*time.Minute {
		t.Errorf("expected 5m, got %v", d)
	}
}

func TestFromFlag(t *testing.T) {
	defer clearVars()

	var s string
	var b bool
	var i int
	var d time.Duration

	StringVar(&s, "str", "def", "help")
	BoolVar(&b, "bool", false, "help")
	IntVar(&i, "int", 1, "help")
	DurationVar(&d, "dur", time.Second, "help")

	origArgs := os.Args
	os.Args = []string{"cmd", "-str=fromcmd", "-bool=true", "-int=42", "-dur=2h"}
	defer func() { os.Args = origArgs }()

	Parse()

	if s != "fromcmd" {
		t.Errorf("expected 'fromcmd', got %q", s)
	}
	if b != true {
		t.Errorf("expected bool true, got %v", b)
	}
	if i != 42 {
		t.Errorf("expected int 42, got %d", i)
	}
	if d != 2*time.Hour {
		t.Errorf("expected 2h, got %v", d)
	}
}

func TestInvalidValues(t *testing.T) {
	defer clearVars()

	var i int
	var d time.Duration

	IntVar(&i, "int", 7, "help")
	DurationVar(&d, "dur", 8*time.Second, "help")

	// Invalid values should be rejected, leaving the defaults in place.
	os.Setenv("INT", "notanint")
	os.Setenv("DUR", "notaduration")
	defer os.Unsetenv("INT")
	defer os.Unsetenv("DUR")

	origArgs := os.Args
	os.Args = []string{"cmd"}
	defer func() { os.Args = origArgs }()

	Parse()

	if i != 7 {
		t.Errorf("expected default 7 on invalid int, got %d", i)
	}
	if d != 8*time.Second {
		t.Errorf("expected default 8s on invalid duration, got %v", d)
	}
}

func TestDurationDefault(t *testing.T) {
	defer clearVars()

	d := Duration("dur", 3*time.Second, "help")

	origArgs := os.Args
	os.Args = []string{"cmd"}
	defer func() { os.Args = origArgs }()

	Parse()

	if *d != 3*time.Second {
		t.Errorf("expected default 3s, got %v", *d)
	}
}
