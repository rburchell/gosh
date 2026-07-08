// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flagx

import (
	"io"
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

func TestFlagSetVisit(t *testing.T) {
	fs := NewFlagSet("t", ContinueOnError)

	var s string
	var b bool
	var i int

	fs.StringVar(&s, "str", "def", "help")
	fs.BoolVar(&b, "bool", false, "help")
	fs.IntVar(&i, "int", 1, "help")

	if err := fs.Parse([]string{"-int=42", "-str=x"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Visit reports only the flags that were set, in lexical order.
	var seen []string
	fs.Visit(func(f *Flag) { seen = append(seen, f.Name+"="+f.Value.String()) })

	if len(seen) != 2 || seen[0] != "int=42" || seen[1] != "str=x" {
		t.Errorf("unexpected Visit result: %v", seen)
	}

	// VisitAll reports every flag, set or not, in lexical order.
	var all []string
	fs.VisitAll(func(f *Flag) { all = append(all, f.Name) })

	if len(all) != 3 || all[0] != "bool" || all[1] != "int" || all[2] != "str" {
		t.Errorf("unexpected VisitAll result: %v", all)
	}
}

func TestValueHelpers(t *testing.T) {
	defer clearVars()

	s := String("str", "def", "help")
	b := Bool("bool", false, "help")
	i := Int("int", 1, "help")
	d := Duration("dur", time.Second, "help")

	origArgs := os.Args
	os.Args = []string{"cmd", "-str=fromcmd", "-bool=true", "-int=42", "-dur=2h"}
	defer func() { os.Args = origArgs }()

	Parse()

	if *s != "fromcmd" {
		t.Errorf("expected 'fromcmd', got %q", *s)
	}
	if *b != true {
		t.Errorf("expected bool true, got %v", *b)
	}
	if *i != 42 {
		t.Errorf("expected int 42, got %d", *i)
	}
	if *d != 2*time.Hour {
		t.Errorf("expected 2h, got %v", *d)
	}
}

func TestFlagSetValueHelpers(t *testing.T) {
	fs := NewFlagSet("t", ContinueOnError)

	s := fs.String("str", "def", "help")
	b := fs.Bool("bool", false, "help")
	i := fs.Int("int", 1, "help")
	d := fs.Duration("dur", time.Second, "help")

	if err := fs.Parse([]string{"-str=fromcmd", "-bool=true", "-int=42", "-dur=2h"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if *s != "fromcmd" {
		t.Errorf("expected 'fromcmd', got %q", *s)
	}
	if *b != true {
		t.Errorf("expected bool true, got %v", *b)
	}
	if *i != 42 {
		t.Errorf("expected int 42, got %d", *i)
	}
	if *d != 2*time.Hour {
		t.Errorf("expected 2h, got %v", *d)
	}
}

func TestFlagSetParse(t *testing.T) {
	fs := NewFlagSet("t", ContinueOnError)

	var s string
	var b bool
	var i int
	var d time.Duration

	fs.StringVar(&s, "str", "def", "help")
	fs.BoolVar(&b, "bool", false, "help")
	fs.IntVar(&i, "int", 1, "help")
	fs.DurationVar(&d, "dur", time.Second, "help")

	if err := fs.Parse([]string{"-str=fromcmd", "-bool=true", "-int=42", "-dur=2h"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}

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

func TestFlagSetArgs(t *testing.T) {
	fs := NewFlagSet("t", ContinueOnError)

	var s string
	fs.StringVar(&s, "str", "def", "help")

	if err := fs.Parse([]string{"-str=x", "one", "two", "three"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got := fs.NArg(); got != 3 {
		t.Errorf("expected NArg 3, got %d", got)
	}
	if got := fs.Args(); len(got) != 3 || got[0] != "one" || got[2] != "three" {
		t.Errorf("unexpected Args: %v", got)
	}
	if got := fs.Arg(1); got != "two" {
		t.Errorf("expected Arg(1) 'two', got %q", got)
	}
}

func TestFlagSetParseError(t *testing.T) {
	fs := NewFlagSet("t", ContinueOnError)

	var i int
	fs.IntVar(&i, "int", 7, "help")

	// Silence the usage/error output that ContinueOnError writes on failure.
	fs.fs.SetOutput(io.Discard)

	// ContinueOnError returns the parse error rather than exiting the process.
	if err := fs.Parse([]string{"-int=notanint"}); err == nil {
		t.Fatal("expected error on invalid flag, got nil")
	}
}

func TestFlagSetEnvOverlay(t *testing.T) {
	fs := NewFlagSet("t", ContinueOnError)

	var s string
	var i int

	fs.StringVar(&s, "str", "def", "help")
	fs.IntVar(&i, "int", 1, "help")

	// envkv provides str; environment overrides int (and would override str,
	// but we leave str to prove envkv is consulted).
	os.WriteFile(".envkv", []byte("STR=fromenvkv\nINT=100\n"), 0644)
	defer os.Remove(".envkv")
	os.Setenv("INT", "200")
	defer os.Unsetenv("INT")

	if err := fs.Parse([]string{}); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if s != "fromenvkv" {
		t.Errorf("expected 'fromenvkv' from envkv, got %q", s)
	}
	if i != 200 {
		t.Errorf("expected int 200 from environment overriding envkv, got %d", i)
	}
}
