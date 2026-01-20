// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slogx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
)

// Returns a new slog.Handler which will pretty-print all records, and write them to w.
//
// Log output includes terminal escape codes unconditionally; the expectation is you are writing a command line tool.
func NewTextHandler(w io.Writer) slog.Handler {
	return textHandler{
		Writer: w,
	}
}

type textHandler struct {
	// The stream that bytes will be written to.
	Writer io.Writer
	attrs  []slog.Attr
}

func leftJustified(str string, width int) string {
	if len(str) >= width {
		return str[:width]
	}
	for len(str) < width {
		str += " "
	}
	return str
}

var testMode bool

func (h textHandler) Handle(ctx context.Context, r slog.Record) error {
	const (
		keyColor   = "\033[03;32m"
		valueColor = "\033[01;32m"
		resetColor = "\033[0m"
	)

	catStr := "<unknown>"
	forAllAttrs := func(callback func(attr slog.Attr) bool) {
		for _, attr := range h.attrs {
			if !callback(attr) {
				return
			}
		}
		r.Attrs(callback)
	}

	// Format attributes, and find category name
	// FIXME: If my understanding is correct, we should/could do this on the handler attrs once, rather than once per record.
	var kvstr string
	forAllAttrs(func(attr slog.Attr) bool {
		if attr.Key == "category" {
			if s, ok := attr.Value.Any().(string); ok && s != "" {
				catStr = s
				return true
			}
		}
		kvstr += fmt.Sprintf("%s%s%s=%s%s%s ", keyColor, attr.Key, resetColor, valueColor, attr.Value, resetColor)
		return true
	})

	// Append caller info if available
	if r.PC != 0 {
		// TODO: This could be replaced with Record.Source() if we assume Go 1.25
		frame, _ := runtime.CallersFrames([]uintptr{r.PC}).Next()
		fileName := frame.File

		// Drop homedir to make it slightly less awkward to read
		// It's a shame these paths are so awkwardly long.. I'd really like to make them shorter somehow.
		// FIXME: This doesn't really change, so we could cache it during init()?
		homeDir, err := os.UserHomeDir()
		if err == nil {
			fileName = strings.ReplaceAll(fileName, homeDir, "~")
		}
		funcName := frame.Function
		lastDot := strings.LastIndex(funcName, ".")
		funcName = funcName[lastDot+1:]
		if testMode {
			fileName = "/testmode.go"
			frame.Line = 0
		}
		kvstr += fmt.Sprintf("%sfile%s=%s%s:%d%s ", keyColor, resetColor, valueColor, fileName, frame.Line, resetColor)
		kvstr += fmt.Sprintf("%sfunc%s=%s%s%s ", keyColor, resetColor, valueColor, funcName, resetColor)
	}

	// Trim trailing space
	if len(kvstr) > 0 {
		kvstr = kvstr[:len(kvstr)-1]
	}

	// Determine message color by level
	var color string
	switch r.Level {
	case slog.LevelDebug:
		color = "\033[01;38;5;240m"
	case slog.LevelInfo:
		color = "\033[01;38;5;245m"
	case slog.LevelWarn:
		color = "\033[01;38;5;208m"
	case slog.LevelError:
		color = "\033[01;38;5;124m"
	default:
		color = resetColor
	}

	// Build and write the final line
	line := fmt.Sprintf("%s%s%s%s %s", color, leftJustified(catStr, 10), resetColor, r.Message, kvstr)
	fmt.Fprintln(h.Writer, line)
	return nil
}

func (h textHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (h textHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return textHandler{Writer: h.Writer, attrs: attrs}
}

func (h textHandler) WithGroup(name string) slog.Handler {
	// FIXME: Handle group somehow
	return textHandler{Writer: h.Writer, attrs: h.attrs}
}
