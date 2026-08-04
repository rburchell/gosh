// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package middleware

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// logCapture is a slog.Handler that records everything, for asserting on
// access-log output.
type logCapture struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *logCapture) Enabled(ctx context.Context, lvl slog.Level) bool { return true }
func (h *logCapture) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}
func (h *logCapture) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *logCapture) WithGroup(name string) slog.Handler       { return h }

func (h *logCapture) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.records)
}

// message returns the log message of record i.
func (h *logCapture) message(i int) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.records[i].Message
}

// attr returns the value of the named attribute on record i, if present.
func (h *logCapture) attr(i int, key string) (slog.Value, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	var v slog.Value
	found := false
	h.records[i].Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			v = a.Value
			found = true
			return false
		}
		return true
	})
	return v, found
}

// captureRequestLog swaps the package logger for a capturing one for the
// duration of the test.
func captureRequestLog(t *testing.T) *logCapture {
	t.Helper()
	capture := &logCapture{}
	old := log
	log = slog.New(capture)
	t.Cleanup(func() { log = old })
	return capture
}

// serveExpectingPanic serves the request, recovering and returning the
// handler's panic value (nil if it returned normally).
func serveExpectingPanic(h http.Handler, w http.ResponseWriter, r *http.Request) (recovered any) {
	defer func() { recovered = recover() }()
	h.ServeHTTP(w, r)
	return nil
}

func TestLogRequests_EmitsOneRecord(t *testing.T) {
	capture := captureRequestLog(t)
	h := LogRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	if capture.count() != 1 {
		t.Fatalf("expected 1 access record, got %d", capture.count())
	}
	if v, ok := capture.attr(0, "status"); !ok || v.Int64() != int64(http.StatusTeapot) {
		t.Errorf("status attr = %v, want %d", v, http.StatusTeapot)
	}
}

func TestWithoutRequestLog_Suppresses(t *testing.T) {
	for _, status := range []int{200, 404, 500} {
		capture := captureRequestLog(t)
		h := LogRequests(WithoutRequestLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		})))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

		if capture.count() != 0 {
			t.Errorf("status %d: expected no access record, got %d", status, capture.count())
		}
		if rec.Code != status {
			t.Errorf("status %d: response altered, got %d", status, rec.Code)
		}
	}
}

func TestWithoutRequestLog_HandlerLogsRemainVisible(t *testing.T) {
	capture := captureRequestLog(t)
	h := LogRequests(WithoutRequestLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info("explicit handler log")
	})))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	if capture.count() != 1 {
		t.Fatalf("expected only the explicit handler log, got %d records", capture.count())
	}
}

func TestWithoutRequestLog_RequestLocal(t *testing.T) {
	capture := captureRequestLog(t)
	quiet := WithoutRequestLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	loud := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	mux := http.NewServeMux()
	mux.Handle("/quiet", quiet)
	mux.Handle("/loud", loud)
	h := LogRequests(mux)

	// Suppressed then normal: the second request must log.
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/quiet", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/loud", nil))
	if capture.count() != 1 {
		t.Fatalf("expected exactly 1 record after quiet+loud, got %d", capture.count())
	}

	// Concurrent suppressed and normal requests must not interfere.
	const n = 50
	var wg sync.WaitGroup
	for range n {
		wg.Add(2)
		go func() {
			defer wg.Done()
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/quiet", nil))
		}()
		go func() {
			defer wg.Done()
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/loud", nil))
		}()
	}
	wg.Wait()
	if got := capture.count(); got != 1+n {
		t.Errorf("expected %d records after concurrent requests, got %d", 1+n, got)
	}
}

func TestWithoutRequestLog_Nesting(t *testing.T) {
	capture := captureRequestLog(t)
	h := LogRequests(WithoutRequestLog(WithoutRequestLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if capture.count() != 0 {
		t.Errorf("nested WithoutRequestLog must stay suppressed, got %d records", capture.count())
	}
}

func TestLogRequests_NestedShareSuppression(t *testing.T) {
	capture := captureRequestLog(t)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	// Without suppression, both nested loggers emit.
	h := LogRequests(LogRequests(inner))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if capture.count() != 2 {
		t.Fatalf("expected 2 records from nested loggers, got %d", capture.count())
	}

	// With suppression, both stay silent.
	capture2 := captureRequestLog(t)
	h2 := LogRequests(LogRequests(WithoutRequestLog(inner)))
	h2.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if capture2.count() != 0 {
		t.Errorf("nested loggers must share suppression state, got %d records", capture2.count())
	}
}

func TestWithoutRequestLog_WithoutLogger(t *testing.T) {
	called := false
	h := WithoutRequestLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if !called || rec.Code != http.StatusNoContent {
		t.Error("WithoutRequestLog without LogRequests must call the handler normally")
	}
}

func TestWithoutRequestLog_IDsStillAvailable(t *testing.T) {
	captureRequestLog(t)
	var gotErr error
	h := TagWithRequestID(LogRequests(WithoutRequestLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _, gotErr = IDs(r)
	}))))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if gotErr != nil {
		t.Errorf("IDs unavailable inside suppressed route: %v", gotErr)
	}
}

func TestLogRequests_ResponseController(t *testing.T) {
	captureRequestLog(t)
	var flushErr error
	h := LogRequests(WithoutRequestLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flushErr = http.NewResponseController(w).Flush()
	})))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if flushErr != nil {
		t.Errorf("ResponseController could not reach underlying writer: %v", flushErr)
	}
}

func TestLogRequests_Panic(t *testing.T) {
	t.Run("before response start logs 500 and rethrows", func(t *testing.T) {
		capture := captureRequestLog(t)
		h := LogRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("boom")
		}))
		recovered := serveExpectingPanic(h, httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		if recovered != "boom" {
			t.Fatalf("expected original panic value, got %v", recovered)
		}
		if capture.count() != 1 {
			t.Fatalf("expected 1 record, got %d", capture.count())
		}
		if v, _ := capture.attr(0, "status"); v.Int64() != 500 {
			t.Errorf("status = %d, want 500", v.Int64())
		}
	})

	t.Run("after WriteHeader logs started status", func(t *testing.T) {
		capture := captureRequestLog(t)
		h := LogRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
			panic("boom")
		}))
		serveExpectingPanic(h, httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		if v, _ := capture.attr(0, "status"); v.Int64() != 204 {
			t.Errorf("status = %d, want 204", v.Int64())
		}
	})

	t.Run("after Write logs implicit 200", func(t *testing.T) {
		capture := captureRequestLog(t)
		h := LogRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("partial"))
			panic("boom")
		}))
		serveExpectingPanic(h, httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		if v, _ := capture.attr(0, "status"); v.Int64() != 200 {
			t.Errorf("status = %d, want 200", v.Int64())
		}
	})

	t.Run("second WriteHeader does not replace recorded status", func(t *testing.T) {
		capture := captureRequestLog(t)
		h := LogRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		if v, _ := capture.attr(0, "status"); v.Int64() != 201 {
			t.Errorf("status = %d, want first-written 201", v.Int64())
		}
	})

	t.Run("suppressed panic emits nothing and still propagates", func(t *testing.T) {
		capture := captureRequestLog(t)
		h := LogRequests(WithoutRequestLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("boom")
		})))
		recovered := serveExpectingPanic(h, httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		if recovered != "boom" {
			t.Fatalf("suppression must not swallow the panic, got %v", recovered)
		}
		if capture.count() != 0 {
			t.Errorf("expected no records for suppressed panic, got %d", capture.count())
		}
	})

	t.Run("nested loggers honor suppression on panic path", func(t *testing.T) {
		capture := captureRequestLog(t)
		h := LogRequests(LogRequests(WithoutRequestLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("boom")
		}))))
		recovered := serveExpectingPanic(h, httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		if recovered != "boom" {
			t.Fatalf("expected panic to propagate through both loggers, got %v", recovered)
		}
		if capture.count() != 0 {
			t.Errorf("expected no records, got %d", capture.count())
		}
	})
}

// A handler that hijacks before starting a response should log the
// takeover immediately with an unknown status, then log the full lifetime
// when it closes the returned connection.
func TestLogRequests_HijackLifetime(t *testing.T) {
	capture := captureRequestLog(t)

	const (
		upgradeResp = "HTTP/1.1 101 Switching Protocols\r\n\r\n"
		payload     = "hello"
	)

	handlerDone := make(chan struct{})
	var readErr error
	var readN int
	h := LogRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(handlerDone)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			readErr = err
			return
		}
		defer conn.Close()
		io.WriteString(conn, upgradeResp)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, len(payload))
		readN, readErr = io.ReadFull(conn, buf)
	}))

	server := httptest.NewServer(h)
	defer server.Close()

	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	// A conforming client must wait for the server's handshake response before
	// sending any further data (RFC 6455 §4.1).
	io.WriteString(conn, "GET /ws HTTP/1.1\r\nHost: x\r\n\r\n")
	if _, err := io.ReadFull(conn, make([]byte, len(upgradeResp))); err != nil {
		t.Fatalf("read upgrade response: %v", err)
	}
	io.WriteString(conn, payload)

	<-handlerDone
	if readErr != nil {
		t.Fatalf("handler read %d bytes then errored: %v", readN, readErr)
	}

	// Wait for Close to emit the "Closed" record.
	deadline := time.Now().Add(2 * time.Second)
	for capture.count() < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("expected Hijacked+Closed records, got %d", capture.count())
		}
		time.Sleep(time.Millisecond)
	}

	// Takeover then close. The raw response status is not observable.
	if capture.count() != 2 {
		t.Fatalf("expected exactly Hijacked+Closed, got %d records", capture.count())
	}
	if m := capture.message(0); m != "Hijacked" {
		t.Errorf("record 0 message = %q, want Hijacked", m)
	}
	if m := capture.message(1); m != "Closed" {
		t.Errorf("record 1 message = %q, want Closed", m)
	}
	for i := range 2 {
		if v, _ := capture.attr(i, "status"); v.Int64() != 0 {
			t.Errorf("record %d status = %d, want unknown status 0", i, v.Int64())
		}
	}

	if duration, ok := capture.attr(1, "duration"); !ok || duration.Duration() <= 0 {
		t.Errorf("Closed duration = %v, want positive connection lifetime", duration)
	}
}

// A handler that starts a response before hijacking should retain that
// status in both lifecycle records and must not emit a Finished record
// when ServeHTTP returns.
func TestLogRequests_HijackPreservesStartedStatus(t *testing.T) {
	capture := captureRequestLog(t)
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	h := LogRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Fatalf("hijack: %v", err)
		}
		conn.Close()
	}))
	h.ServeHTTP(&hijackResponseWriter{Conn: serverConn}, httptest.NewRequest("GET", "/", nil))

	if capture.count() != 2 {
		t.Fatalf("expected exactly Hijacked+Closed, got %d records", capture.count())
	}
	if m := capture.message(0); m != "Hijacked" {
		t.Errorf("record 0 message = %q, want Hijacked", m)
	}
	if m := capture.message(1); m != "Closed" {
		t.Errorf("record 1 message = %q, want Closed", m)
	}
	for i := range 2 {
		if v, _ := capture.attr(i, "status"); v.Int64() != http.StatusAccepted {
			t.Errorf("record %d status = %d, want %d", i, v.Int64(), http.StatusAccepted)
		}
	}
}

type hijackResponseWriter struct {
	net.Conn
}

func (*hijackResponseWriter) Header() http.Header         { return make(http.Header) }
func (*hijackResponseWriter) WriteHeader(int)             {}
func (*hijackResponseWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w *hijackResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.Conn, bufio.NewReadWriter(bufio.NewReader(w.Conn), bufio.NewWriter(w.Conn)), nil
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		want       string
	}{
		{
			name:       "untrusted IP, ignore headers",
			remoteAddr: "8.8.8.8:12345",
			headers: map[string]string{
				"X-Forwarded-For": "1.2.3.4",
			},
			want: "8.8.8.8",
		},
		{
			name:       "trusted localhost, use X-Forwarded-For",
			remoteAddr: "127.0.0.1:54321",
			headers: map[string]string{
				"X-Forwarded-For": "1.2.3.4",
			},
			want: "1.2.3.4",
		},
		{
			name:       "trusted 100.x.x.x, use X-Real-IP",
			remoteAddr: "100.1.2.3:45678",
			headers: map[string]string{
				"X-Real-IP": "5.6.7.8",
			},
			want: "5.6.7.8",
		},
		{
			name:       "header with multiple IPs",
			remoteAddr: "127.0.0.1:54321",
			headers: map[string]string{
				"X-Forwarded-For": "9.9.9.9, 10.10.10.10",
			},
			want: "9.9.9.9",
		},
		{
			name:       "fallback to remote addr",
			remoteAddr: "192.168.1.1:1111",
			headers:    nil,
			want:       "192.168.1.1",
		},

		{
			name:       "untrusted localhost IPv6",
			remoteAddr: "[::1]:54321",
			headers: map[string]string{
				"X-Forwarded-For": "2001:db8::1",
			},
			want: "::1",
		},
		{
			name:       "trusted 100.x.x.x IPv4-mapped IPv6, use X-Real-IP",
			remoteAddr: "[::ffff:100.1.2.3]:12345",
			headers: map[string]string{
				"X-Real-IP": "5.6.7.8",
			},
			want: "5.6.7.8",
		},
		{
			name:       "untrusted IPv6, ignore headers",
			remoteAddr: "[2001:db8::1234]:12345",
			headers: map[string]string{
				"X-Forwarded-For": "1.2.3.4",
			},
			want: "2001:db8::1234",
		},
		{
			name:       "invalid RemoteAddr, use raw string",
			remoteAddr: "invalid-address",
			headers: map[string]string{
				"X-Forwarded-For": "9.9.9.9",
			},
			want: "invalid-address",
		},
		{
			name:       "invalid IP in header, fallback to RemoteAddr",
			remoteAddr: "127.0.0.1:1234",
			headers: map[string]string{
				"X-Forwarded-For": "not-an-ip",
			},
			want: "127.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: tt.remoteAddr,
				Header:     make(http.Header),
			}
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := getClientIP(req)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
