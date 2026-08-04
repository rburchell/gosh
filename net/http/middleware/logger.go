// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package middleware

import (
	"bufio"
	"context"
	"github.com/rburchell/gosh/log/slogx"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
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
	req      *http.Request    // request being served, for the log record
	state    *requestLogState // shared suppression marker
	start    time.Time        // when the logger began serving the request
	status   int
	started  bool // has the response started (explicitly or via Write)?
	hijacked bool // did the handler take over the connection?
}

// loggedCloseConn emits the connection-lifetime record when the handler closes
// a hijacked connection. Close may be called more than once or concurrently,
// so the record is guarded independently of the underlying connection.
type loggedCloseConn struct {
	net.Conn
	recorder *statusRecorder
	once     sync.Once
}

func (c *loggedCloseConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() {
		if !c.recorder.state.suppressed {
			c.recorder.logRecord(
				"Closed",
				c.recorder.status,
				time.Since(c.recorder.start),
			)
		}
	})
	return err
}

// levelForStatus maps an HTTP status onto the slog level its access-log
// record should carry.
func levelForStatus(status int) slog.Level {
	switch {
	case status >= 500:
		return slog.LevelError
	case status >= 400:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

// logRecord emits one access-log line for the request. duration is the span
// the caller wants to report (time-to-hijack for a takeover, total lifetime
// for a close). Callers must check state.suppressed first.
func (r *statusRecorder) logRecord(msg string, status int, duration time.Duration, extra ...any) {
	cid, rid, err := IDs(r.req)
	cids := "??"
	rids := "??"
	if err == nil {
		cids = string(cid)
		rids = string(rid)
	}

	attrs := []any{
		slog.Int("status", status),
		slog.String("method", r.req.Method),
		slog.String("path", r.req.URL.Path),
		slog.Duration("duration", duration),
		slog.String("cid", cids),
		slog.String("rid", rids),
		slog.String("ip", getClientIP(r.req)),
	}
	attrs = append(attrs, extra...)
	log.Log(r.req.Context(), levelForStatus(status), msg, attrs...)
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

// Hijack preserves websocket and other connection takeover support through
// the recorder. ResponseController follows any further Unwrap methods in the
// writer chain and reports http.ErrNotSupported when the original writer
// cannot hijack.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(r.ResponseWriter).Hijack()
	if err == nil {
		// The handler has taken over the connection; the recorder can no
		// longer observe anything it writes on the raw socket. If no response
		// had started, leave the status unknown rather than assuming this was
		// a protocol upgrade: Hijacker also supports ordinary HTTP responses.
		// Emit a "Hijacked" record now so the takeover is visible even if the
		// connection then lives for a long time, and hand back a wrapper that
		// records its lifetime when the handler closes it.
		conn = &loggedCloseConn{Conn: conn, recorder: r}
		if !r.started {
			r.status = 0
		}
		r.hijacked = true
		if !r.state.suppressed {
			r.logRecord("Hijacked", r.status, time.Since(r.start))
		}
	}
	return conn, rw, err
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
// A handler that hijacks the connection (e.g. a websocket upgrade) gets two
// records instead of one: "Hijacked" at takeover, and "Closed" when the
// handler closes the returned connection. In this case, "Closed" reports the
// connection lifetime rather than the ServeHTTP lifetime.
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

		start := time.Now()
		recw := &statusRecorder{ResponseWriter: w, req: r, state: state, start: start, status: 200}

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

				// Hijacked connections log their completion when the returned
				// connection is closed, which may be after ServeHTTP returns.
				if !recw.hijacked {
					recw.logRecord("Finished", status, duration)
				}
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
