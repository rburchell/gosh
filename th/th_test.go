// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package th

import (
	"errors"
	"testing"
)

func TestMust_Ok(t *testing.T) {
	want := 42
	got := Must(want, nil)
	if got != want {
		t.Fatalf("Must() = %v, want %v", got, want)
	}
}

func TestMust_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Must() did not panic on error")
		}
	}()
	Must(0, errors.New("fail"))
}
