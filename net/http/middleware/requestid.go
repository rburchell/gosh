// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
)

// A unique ID for a client making HTTP requests
// See TagWithRequestID.
type CID string

// A unique ID for an individual HTTP request
// See TagWithRequestID.
type RID string

// TagWithRequestID tags requests with CID and RIDs, for later access during request processing.
//
// NOTE: CID is passed back to the client as a cookie, so it is *INSECURE*.
// You *MUST NOT* rely on it for anything security-related.
// The client may (intentionally or not) lose the CID, may forge the CID, or similar.
// If the CID is missing, or malformed, a new CID will be allocated.
func TagWithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const cookieCID = "cid"
		const idLength = 6

		isValidClientID := func(s string) bool {
			if len(s) != idLength {
				return false
			}
			for _, c := range s {
				if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
					return false
				}
			}
			return true
		}

		cidCookie, err := r.Cookie(cookieCID)
		var cid string
		if err != nil || !isValidClientID(cidCookie.Value) {
			cid = randomHex(idLength)
			http.SetCookie(w, &http.Cookie{Name: cookieCID, Value: cid, Path: "/"})
		} else {
			cid = cidCookie.Value
		}

		// Generate new request ID
		rid := randomHex(idLength)

		// Store IDs in context for easy access
		ctx := r.Context()
		ctx = context.WithValue(ctx, idsKey, ids{cid: CID(cid), rid: RID(rid)})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func randomHex(n int) string {
	b := make([]byte, (n+1)/2) // halve the length because hex doubles the size.
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Context keys
type ctxKey int

const idsKey ctxKey = iota

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
