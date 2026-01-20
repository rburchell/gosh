// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slogx

import (
	"context"
	"log/slog"
	"testing"
)

type captureHandler struct {
	records []slog.Record
	attrs   []slog.Attr
}

func (h *captureHandler) Enabled(ctx context.Context, lvl slog.Level) bool { return true }
func (h *captureHandler) Handle(ctx context.Context, r slog.Record) error {
	r = r.Clone()
	r.AddAttrs(h.attrs...)
	h.records = append(h.records, r)
	return nil
}
func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.attrs = append(h.attrs, attrs...)
	return h
}
func (h *captureHandler) WithGroup(name string) slog.Handler { return h }

func TestNewCategory_AddsCategoryAndMinLevel(t *testing.T) {
	base := &captureHandler{}
	logger := NewCategory("mycat", base, slog.LevelWarn)

	logger.Info("should be filtered out")
	logger.Warn("should log warn")
	logger.Error("should log error")

	if len(base.records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(base.records))
	}
	for _, r := range base.records {
		hasCategory := false
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "category" && a.Value.String() == "mycat" {
				hasCategory = true
			}
			return true
		})
		if !hasCategory {
			t.Errorf("record missing category attr: %v", r)
		}
	}
}

func TestNewCategory_BaseHandlerFiltering(t *testing.T) {
	base := &captureHandler{}
	logger := NewCategory("x", base, slog.LevelDebug)

	logger.Debug("should be logged")
	if len(base.records) != 1 {
		t.Errorf("expected 1 record, got %d", len(base.records))
	}
}
