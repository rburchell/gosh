// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package server provides a convenience method to construct and serve HTTP
// in a straightforward, repeatable way.
//
// It ensures that all of the right framework is inserted for you for e.g. logging.
//
// For example:
//
//	func handlePingPong(w http.ResponseWriter, r *http.Request) {
//		w.Write([]byte("pong"))
//	}
//
//	server.Build(nil).
//	HandleFunc("/ping", handlePingPong).
//	ListenAndServeOrDie(":8080")
//
// The snippet above will respond to /ping on :8080, otherwise, terminate if it can't listen.
package server

import (
	"github.com/rburchell/gosh/log/slogx"
	"github.com/rburchell/gosh/net/http/middleware"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

var log *slog.Logger = slogx.NewCategory("http", slogx.TextHandler, slog.LevelDebug)

// Builds a http.Handler, and optionally serves it.
type Builder struct {
	mux     *http.ServeMux
	routes  []any
	wrapped http.Handler
}

// Starts a Builder using the base 'mux'. If nil is provided, uses http.NewServeMux().
func Build(mux *http.ServeMux) *Builder {
	if mux == nil {
		mux = http.NewServeMux()
	}
	return &Builder{mux: mux}
}

// Adds a single route (pattern and handler) to the Builder.
func (b *Builder) Handle(pattern string, handler http.Handler) *Builder {
	b.mux.Handle(pattern, handler)
	b.routes = append(b.routes, pattern)
	return b
}

func (b *Builder) HandleFunc(pattern string, handler http.HandlerFunc) *Builder {
	b.mux.Handle(pattern, handler)
	b.routes = append(b.routes, pattern)
	return b
}

// Constructs the final http.Handler.
//
// If you want to use it right away, ListenAndServeOrDie might be useful.
func (b *Builder) Build() http.Handler {
	// Wrap in middleware.
	// Remember that these are called bottom-up.. Order matters.
	var wrapped http.Handler = b.mux
	wrapped = middleware.LogRequests(wrapped)
	wrapped = middleware.TagWithRequestID(wrapped)
	b.wrapped = wrapped
	return wrapped
}

// Constructs the final http.Handler (i.e. does Build()), and listens to the provided addr.
func (b *Builder) ListenAndServe(addr string) error {
	if b.wrapped == nil {
		b.Build()
	}
	friendlyAddr := addr
	if strings.HasPrefix(addr, ":") {
		friendlyAddr = "localhost" + addr
	}
	log.Debug("Hosting routes", "count", len(b.routes), "addr", "http://"+friendlyAddr)
	return http.ListenAndServe(addr, b.wrapped)
}

// The same as ListenAndServe, but fatally exits if ListenAndServe returns an error.
func (b *Builder) ListenAndServeOrDie(addr string) {
	err := b.ListenAndServe(addr)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}
