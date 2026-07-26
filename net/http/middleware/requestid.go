// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
)

// A unique ID for a client making HTTP requests
// See TagWithRequestID.
type CID string

// A unique ID for an individual HTTP request
// See TagWithRequestID.
type RID string

// Canonical header names used to carry request IDs between gosh HTTP
// processes. See ForwardRequestIDs (to send) and TagWithForwardedRequestID
// (to receive).
//
// Both headers are untrusted: any caller can forge them, just as it can
// forge the cid cookie. They are suitable only for log correlation and
// diagnostics, never for authentication or authorization.
const (
	ClientIDHeader  = "X-Gosh-Client-Id"
	RequestIDHeader = "X-Gosh-Request-Id"
)

const cookieCID = "cid"
const idLength = 6

// validID reports whether value is a well-formed gosh ID: exactly six bytes
// of lowercase hexadecimal. No normalization is performed; uppercase or
// surrounding whitespace is invalid.
func validID(value string) bool {
	if len(value) != idLength {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// headerID returns the ID carried in the named header, if the header has
// exactly one valid value. A header repeated across multiple field lines is
// ambiguous and treated as absent.
func headerID(r *http.Request, name string) (string, bool) {
	values := r.Header.Values(name)
	if len(values) == 1 && validID(values[0]) {
		return values[0], true
	}
	return "", false
}

// cookieID returns the CID carried in the cid cookie, if present and valid.
// If multiple cid cookies are present, the first one wins (per existing
// http.Request.Cookie behavior).
func cookieID(r *http.Request) (string, bool) {
	c, err := r.Cookie(cookieCID)
	if err != nil || !validID(c.Value) {
		return "", false
	}
	return c.Value, true
}

// TagWithRequestID tags requests with CID and RIDs, for later access during request processing.
//
// The CID identifies a logical client for diagnostics. It is taken from a
// valid cid cookie, or generated (in which case a replacement cookie is set).
// The RID identifies a single HTTP request, and is always generated.
//
// Gosh tracing headers (ClientIDHeader, RequestIDHeader) are deliberately
// ignored, even when syntactically valid, so that callers of internet-facing
// listeners cannot select their own tracing IDs. Use
// TagWithForwardedRequestID on listeners behind a trusted forwarder.
//
// NOTE: CID is passed back to the client as a cookie, so it is *INSECURE*.
// You *MUST NOT* rely on it for anything security-related.
// The client may (intentionally or not) lose the CID, may forge the CID, or similar.
// If the CID is missing, or malformed, a new CID will be allocated.
func TagWithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tagRequest(w, r, next, false)
	})
}

// TagWithForwardedRequestID tags requests with CID and RIDs like
// TagWithRequestID, but additionally trusts the gosh tracing headers
// (ClientIDHeader, RequestIDHeader) so a request forwarded by another gosh
// process (see ForwardRequestIDs) keeps its IDs across the hop.
//
// The CID is taken from a valid ClientIDHeader, else a valid cid cookie,
// else generated. The RID is taken from a valid RequestIDHeader, else
// generated. CID and RID are evaluated independently; an invalid value in
// one header does not affect the other. An invalid or ambiguous header
// (including one repeated across multiple field lines) is treated as absent.
//
// When the CID comes from the header, no cid cookie is set or repaired,
// even if the request carries an invalid cookie: the cookie belongs to the
// client-facing server that issued it, and a backend must not try to fix a
// browser cookie through a proxied response. A broken cookie is only
// replaced by a server that selects the CID from the cookie boundary.
//
// Enable this ONLY when everything that can reach the listener is trusted to
// supply tracing metadata — e.g. a service bound to loopback behind a gosh
// gateway — or when a trusted ingress already strips and replaces the
// headers. On a public listener it would let any caller choose its own IDs.
// Note that this grants the forwarding peer control over diagnostic
// correlation: a buggy or hostile peer can reuse one RID across many
// requests, so RID uniqueness must never be relied upon for correctness or
// security.
func TagWithForwardedRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tagRequest(w, r, next, true)
	})
}

// tagRequest implements both taggers; trustHeaders selects whether the gosh
// tracing headers are consulted.
func tagRequest(w http.ResponseWriter, r *http.Request, next http.Handler, trustHeaders bool) {
	var cid string
	cidFromHeader := false
	if trustHeaders {
		cid, cidFromHeader = headerID(r, ClientIDHeader)
	}
	if !cidFromHeader {
		var ok bool
		if cid, ok = cookieID(r); !ok {
			cid = randomHex(idLength)
			http.SetCookie(w, &http.Cookie{Name: cookieCID, Value: cid, Path: "/"})
		}
	}

	var rid string
	ridFromHeader := false
	if trustHeaders {
		rid, ridFromHeader = headerID(r, RequestIDHeader)
	}
	if !ridFromHeader {
		rid = randomHex(idLength)
	}

	// Store IDs in context for easy access
	ctx := r.Context()
	ctx = context.WithValue(ctx, idsKey, ids{cid: CID(cid), rid: RID(rid)})
	next.ServeHTTP(w, r.WithContext(ctx))
}

// ForwardRequestIDs copies the CID and RID assigned to src (by
// TagWithRequestID or TagWithForwardedRequestID) onto dst as the gosh
// tracing headers, replacing any existing values so exactly one value is
// sent for each. Nothing else about either request is touched.
//
// It is intended for use in a reverse proxy, e.g. in a
// httputil.ReverseProxy Rewrite func: ForwardRequestIDs(pr.Out, pr.In).
// Forwarding is deliberately explicit — propagating the headers on every
// outbound request would leak internal tracing metadata to unrelated
// third parties.
//
// Callers must forward the RID belonging to the request currently being
// served, and must not deliberately reuse one RID across unrelated
// requests: the receiver correlates its logs on these values, and reuse
// smears unrelated requests together. Always pass the inbound request
// being proxied as src rather than caching IDs across requests.
//
// It returns an error, without panicking, if either request is nil or if
// src has not passed through request-ID middleware.
func ForwardRequestIDs(dst, src *http.Request) error {
	if dst == nil {
		return errors.New("ForwardRequestIDs: nil destination request")
	}
	if src == nil {
		return errors.New("ForwardRequestIDs: nil source request")
	}
	cid, rid, err := IDs(src)
	if err != nil {
		return fmt.Errorf("ForwardRequestIDs: %w", err)
	}
	if dst.Header == nil {
		dst.Header = make(http.Header)
	}
	dst.Header.Set(ClientIDHeader, string(cid))
	dst.Header.Set(RequestIDHeader, string(rid))
	return nil
}

func randomHex(n int) string {
	b := make([]byte, (n+1)/2) // halve the length because hex doubles the size.
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Context keys
type ctxKey int

const (
	idsKey ctxKey = iota
	logStateKey
)

type ids struct {
	cid CID
	rid RID
}

// Fetch CID associated with the request, or error.
// See TagWithRequestID.
func ClientID(r *http.Request) (CID, error) {
	c, _, err := IDs(r)
	return c, err
}

// Fetch RID associated with the request, or error.
// See TagWithRequestID.
func RequestID(r *http.Request) (RID, error) {
	_, rid, err := IDs(r)
	return rid, err
}

// Fetch CID/RID associated with the request, or error.
// See TagWithRequestID.
func IDs(r *http.Request) (CID, RID, error) {
	if v := r.Context().Value(idsKey); v != nil {
		if idsStruct, ok := v.(ids); ok {
			return idsStruct.cid, idsStruct.rid, nil
		}
	}

	// if this is hit, you are accessing the IDs either too early (before the tag handler),
	// or the tag handler isn't installed.
	return "", "", errors.New("IDs not found in request")
}
