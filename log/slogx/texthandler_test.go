// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slogx

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestTextHandler(t *testing.T) {
	testMode = true
	var buf bytes.Buffer
	handler := NewTextHandler(&buf)
	logger := slog.New(handler)

	logger.Debug("debuglog", "category", "tst", "key", "value")
	logger.Info("infolog", "category", "tst", "key", "value")
	logger.Warn("warnlog", "category", "tst", "key", "value")
	logger.Warn("errorlog", "category", "tst", "key", "value")

	lines := strings.Split(buf.String(), "\n")
	if len(lines)-1 != 4 {
		t.Fatalf("expected %d lines, got %d", 4, len(lines))
	}
	want := []string{
		`[01;38;5;240mtst       [0mdebuglog [03;32mkey[0m=[01;32mvalue[0m [03;32mfile[0m=[01;32m/testmode.go:0[0m [03;32mfunc[0m=[01;32mTestTextHandler[0m`,
		`[01;38;5;245mtst       [0minfolog [03;32mkey[0m=[01;32mvalue[0m [03;32mfile[0m=[01;32m/testmode.go:0[0m [03;32mfunc[0m=[01;32mTestTextHandler[0m`,
		`[01;38;5;208mtst       [0mwarnlog [03;32mkey[0m=[01;32mvalue[0m [03;32mfile[0m=[01;32m/testmode.go:0[0m [03;32mfunc[0m=[01;32mTestTextHandler[0m`,
		`[01;38;5;208mtst       [0merrorlog [03;32mkey[0m=[01;32mvalue[0m [03;32mfile[0m=[01;32m/testmode.go:0[0m [03;32mfunc[0m=[01;32mTestTextHandler[0m`,
	}
	for idx, want := range want {
		got := lines[idx]
		if got != want {
			t.Errorf("want:\n%s\ngot:\n%s", want, got)
		}
	}
}
