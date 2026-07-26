// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"
	"github.com/rburchell/gosh/log/slogx"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

var log *slog.Logger = slogx.NewCategory("http", slogx.TextHandler, slog.LevelDebug)

// list of locations we will trust for reporting headers
var trustedNets []*net.IPNet

func init() {
	// FIXME: don't hardcode this.
	var trustedCIDRs = []string{
		"127.0.0.1/8",
		"100.0.0.0/8",
	}
	for _, cidr := range trustedCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			trustedNets = append(trustedNets, network)
		}
	}

}

// getClientIP gets the correct IP for the end client
// it also uses HTTP headers, if the request is from a trusted origin (see trustedNets).
func getClientIP(r *http.Request) string {
	remoteIPStr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteIPStr = r.RemoteAddr
	}
	remoteIP := net.ParseIP(remoteIPStr)
	if remoteIP == nil {
		return remoteIPStr
	}

	trusted := false
	for _, net := range trustedNets {
		if net.Contains(remoteIP) {
			trusted = true
			break
		}
	}

	if trusted {
		for _, h := range []string{"X-Forwarded-For", "X-Real-IP"} {
			if ip := r.Header.Get(h); ip != "" {
				// if multiple IPs, take the first
				if idx := strings.Index(ip, ","); idx != -1 {
					ip = ip[:idx]
				}
				ip = strings.TrimSpace(ip)

				// ensure it is valid...
				remoteIP := net.ParseIP(ip)
				if remoteIP != nil {
					return ip
				}
			}
		}
	}

	return remoteIP.String()
}

type statusRecorder struct {
	http.ResponseWriter
	status  int
	started bool // has the response started (explicitly or via Write)?
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.started {
		r.status = code
		r.started = true
	}
	// Always forward, even superfluous calls: the underlying writer ignores
	// them but logs a warning pointing at the offending handler.
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.started {
		// Write starts an implicit 200 OK.
		r.status = http.StatusOK
		r.started = true
	}
	return r.ResponseWriter.Write(b)
}

// This allows use in a http.ResponseController, which means that our wrapping is a little less of a pain.
// We still hide interfaces (i.e. http.Flusher), but the ResponseController allows hitting the underlying
// implementations anyway.
//
// This is pretty disgusting, but since I don't want to deal with the combinatorial explosion of interfaces,
// this feels like the path of least resistance.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// requestLogState is request-scoped mutable state shared between LogRequests
// and WithoutRequestLog. It is stored by pointer in the request context so a
// route wrapper running inside the logger can mark the request after the
// logger has already captured its request object. Never share it across
// requests; it needs no locking as it is only mutated within the synchronous
// handler chain of one request.
type requestLogState struct {
	suppressed bool
}

// LogRequests logs a completion record for each request, including its CID,
// RID (see TagWithRequestID), status, and duration.
//
// The record is also emitted when the handler panics: the panic is logged
// (with status 500 if no response had started, else the status actually
// started) and then rethrown, leaving panic recovery to the net/http server.
// Note that a response started only through the writer reached via Unwrap
// (e.g. an http.ResponseController flush) is invisible to the recorder, so
// a panic after such a start logs 500.
//
// Wrap a route in WithoutRequestLog to suppress its completion record.
func LogRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reuse existing state so nested LogRequests layers observe one
		// shared suppression marker.
		state, ok := r.Context().Value(logStateKey).(*requestLogState)
		if !ok {
			state = &requestLogState{}
			r = r.WithContext(context.WithValue(r.Context(), logStateKey, state))
		}

		recw := &statusRecorder{ResponseWriter: w, status: 200}
		start := time.Now()

		defer func() {
			recovered := recover()
			duration := time.Since(start)

			if !state.suppressed {
				status := recw.status
				if recovered != nil && !recw.started {
					// Nothing was sent; log a 500 but do not write one to
					// the response — that's the server's job.
					status = 500
				}

				cid, rid, err := IDs(r)
				cids := "??"
				rids := "??"
				if err == nil {
					cids = string(cid)
					rids = string(rid)
				}

				level := slog.LevelInfo
				if status >= 500 {
					level = slog.LevelError
				} else if status >= 400 {
					level = slog.LevelWarn
				}

				log.Log(r.Context(), level, "Finished",
					slog.Int("status", status),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Duration("duration", duration),
					slog.String("cid", cids),
					slog.String("rid", rids),
					slog.String("ip", getClientIP(r)),
				)
			}

			if recovered != nil {
				panic(recovered)
			}
		}()

		next.ServeHTTP(recw, r)
	})
}

// WithoutRequestLog suppresses the standard completion record LogRequests
// would emit for requests handled by next, regardless of status and also on
// panic. It affects only that record: request-ID tagging, logs written by
// the handler itself, and other middleware are untouched, and only the
// current request is affected.
//
// Use it for high-frequency operational routes — health checks, heartbeats —
// whose access logs are noise. If failures on such a route matter, log them
// explicitly in the handler.
//
// Without an enclosing LogRequests it simply invokes the handler. Nesting is
// harmless.
func WithoutRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if state, ok := r.Context().Value(logStateKey).(*requestLogState); ok {
			state.suppressed = true
		}
		next.ServeHTTP(w, r)
	})
}
