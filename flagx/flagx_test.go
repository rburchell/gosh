// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flagx

import (
	"os"
	"testing"
)

func TestFromEnvkv(t *testing.T) {
	defer clearVars()

	var s string
	var b bool
	var i int

	StringVar(&s, "str", "def", "help")
	BoolVar(&b, "bool", false, "help")
	IntVar(&i, "int", 1, "help")

	os.WriteFile(".envkv", []byte("STR=fromenvkv\nBOOL=false\nINT=999\n"), 0644)
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
}

func TestFromEnvironment(t *testing.T) {
	defer clearVars()

	var s string
	var b bool
	var i int

	StringVar(&s, "str", "def", "help")
	BoolVar(&b, "bool", false, "help")
	IntVar(&i, "int", 1, "help")

	os.Setenv("STR", "fromenv")
	os.Setenv("BOOL", "true")
	os.Setenv("INT", "2")
	defer os.Unsetenv("STR")
	defer os.Unsetenv("BOOL")
	defer os.Unsetenv("INT")

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
}

func TestFromFlag(t *testing.T) {
	defer clearVars()

	var s string
	var b bool
	var i int

	StringVar(&s, "str", "def", "help")
	BoolVar(&b, "bool", false, "help")
	IntVar(&i, "int", 1, "help")

	origArgs := os.Args
	os.Args = []string{"cmd", "-str=fromcmd", "-bool=true", "-int=42"}
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
}
