// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slogx

import (
	"context"
	"log/slog"
)

// A categoryHandler provides a way to categorise output, automatically appending a category attr,
// as well as providing the ability to set per-category minimum levels.
type categoryHandler struct {
	base     slog.Handler
	minLevel slog.Level
}

func (h *categoryHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return lvl >= h.minLevel
}

func (h *categoryHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.base.Handle(ctx, r)
}

func (h *categoryHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &categoryHandler{
		base:     h.base.WithAttrs(attrs),
		minLevel: h.minLevel,
	}
}

func (h *categoryHandler) WithGroup(name string) slog.Handler {
	return &categoryHandler{
		base:     h.base.WithGroup(name),
		minLevel: h.minLevel,
	}
}

// Creates a logger with a fixed category and minLevel, and a given underlying base handler.
//
// Note that minLevel only applies to filtering done by this handler; 'base' may do its own filtering.
func NewCategory(category string, base slog.Handler, minLevel slog.Level) *slog.Logger {
	handler := &categoryHandler{
		base:     base,
		minLevel: minLevel,
	}
	return slog.New(handler).With("category", category)
}
