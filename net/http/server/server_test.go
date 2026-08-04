// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rburchell/gosh/net/http/middleware"
)

func TestBuilder_HandleFunc(t *testing.T) {
	builder := Build(nil)
	builder.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	handler := builder.Build()

	req := httptest.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if body := w.Body.String(); body != "pong" {
		t.Fatalf(`expected body "pong", got %q`, body)
	}
}

func TestBuilder_EnableLoggingForTests(t *testing.T) {
	b := Build(nil)
	if b.requestLogging {
		t.Fatal("request logging should default to disabled under go test")
	}

	b.Build()
	b.EnableLoggingForTests()
	if !b.requestLogging {
		t.Fatal("EnableLoggingForTests did not enable request logging")
	}
	if b.wrapped != nil {
		t.Fatal("EnableLoggingForTests must invalidate the cached handler")
	}
}

func TestBuilder_RequestLoggerPreservesHijacker(t *testing.T) {
	b := Build(nil).EnableLoggingForTests()
	b.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("response writer does not implement http.Hijacker")
			return
		}
		// Exercise the takeover, not just the interface: a Hijack that
		// returned http.ErrNotSupported would still satisfy the assertion
		// above. Write a minimal response over the raw connection so the
		// client's request completes.
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack through request logger: %v", err)
			return
		}
		defer conn.Close()
		io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nhi")
	})

	server := httptest.NewServer(b.Build())
	defer server.Close()

	response, err := http.Get(server.URL + "/ws")
	if err != nil {
		t.Fatalf("request through builder: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read hijacked response: %v", err)
	}
	if string(body) != "hi" {
		t.Errorf("body over hijacked connection = %q, want %q", body, "hi")
	}
}

// buildAndServe runs one request through a built handler, capturing the IDs
// the route observed.
func buildAndServe(t *testing.T, b *Builder, mutate func(*http.Request)) (middleware.CID, middleware.RID) {
	t.Helper()
	var cid middleware.CID
	var rid middleware.RID
	b.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		var err error
		cid, rid, err = middleware.IDs(r)
		if err != nil {
			t.Errorf("IDs unavailable in route: %v", err)
		}
	})
	handler := b.Build()
	req := httptest.NewRequest("GET", "/probe", nil)
	if mutate != nil {
		mutate(req)
	}
	handler.ServeHTTP(httptest.NewRecorder(), req)
	return cid, rid
}

func TestBuilder_DefaultIgnoresForwardedIDs(t *testing.T) {
	cid, rid := buildAndServe(t, Build(nil), func(r *http.Request) {
		r.Header.Set(middleware.ClientIDHeader, "18b04f")
		r.Header.Set(middleware.RequestIDHeader, "ca04a1")
	})
	if cid == "18b04f" || rid == "ca04a1" {
		t.Errorf("default builder trusted forwarded headers: cid=%q rid=%q", cid, rid)
	}
}

func TestBuilder_TrustForwardedRequestIDs(t *testing.T) {
	cid, rid := buildAndServe(t, Build(nil).TrustForwardedRequestIDs(), func(r *http.Request) {
		r.Header.Set(middleware.ClientIDHeader, "18b04f")
		r.Header.Set(middleware.RequestIDHeader, "ca04a1")
	})
	if cid != "18b04f" || rid != "ca04a1" {
		t.Errorf("trusted builder ignored forwarded headers: cid=%q rid=%q", cid, rid)
	}
}

// Enabling trust after Build must invalidate the cached handler so a
// subsequent Build (or ListenAndServe) serves the trusted stack.
func TestBuilder_TrustAfterBuildInvalidatesCache(t *testing.T) {
	b := Build(nil)
	var cid middleware.CID
	b.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		cid, _, _ = middleware.IDs(r)
	})
	b.Build()
	b.TrustForwardedRequestIDs()
	if b.wrapped != nil {
		t.Fatal("TrustForwardedRequestIDs must invalidate the cached handler")
	}
	handler := b.Build()

	req := httptest.NewRequest("GET", "/probe", nil)
	req.Header.Set(middleware.ClientIDHeader, "18b04f")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if cid != "18b04f" {
		t.Errorf("rebuilt handler is not trusted: cid=%q", cid)
	}
}

func TestBuilder_WithoutRequestLogRouteStillTagged(t *testing.T) {
	b := Build(nil)
	var cid middleware.CID
	var rid middleware.RID
	b.Handle("/quiet", middleware.WithoutRequestLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cid, rid, _ = middleware.IDs(r)
	})))
	handler := b.Build()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/quiet", nil))
	if cid == "" || rid == "" {
		t.Errorf("suppressed route missing IDs: cid=%q rid=%q", cid, rid)
	}
}
