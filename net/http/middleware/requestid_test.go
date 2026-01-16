// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTagWithRequestID(t *testing.T) {
	var capturedCID CID
	var capturedRID RID

	handler := TagWithRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cid, rid, err := IDs(r)
		if err != nil {
			t.Errorf("unexpected error fetching IDs: %v", err)
			return
		}
		capturedCID = cid
		capturedRID = rid
		w.WriteHeader(http.StatusOK)
	}))

	// Test: No CID provided
	req1 := httptest.NewRequest("GET", "/", nil)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if capturedCID == "" || capturedRID == "" {
		t.Fatal("expected both CID and RID to be set on first request")
	}
	firstCID := capturedCID
	firstRID := capturedRID

	// Test: CID should stay stable across requests
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.AddCookie(&http.Cookie{Name: "cid", Value: string(firstCID)})
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if capturedCID != firstCID {
		t.Errorf("expected CID to stay the same, got %s, want %s", capturedCID, firstCID)
	}
	if capturedRID == firstRID {
		t.Errorf("expected RID to change between requests, but it didn't")
	}

	// Test: Invalid CID is replaced
	req3 := httptest.NewRequest("GET", "/", nil)
	req3.AddCookie(&http.Cookie{Name: "cid", Value: "INVALID"})
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)

	if capturedCID == "INVALID" || capturedCID == "" {
		t.Errorf("expected invalid CID to be replaced with a valid CID, got %s", capturedCID)
	}
}

// Tests that different clients get different CIDs.
func TestTagWithRequestID_DifferentClients(t *testing.T) {
	var cids []CID
	handler := TagWithRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cid, _, err := IDs(r)
		if err != nil {
			t.Errorf("unexpected error fetching IDs: %v", err)
			return
		}
		cids = append(cids, cid)
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest("GET", "/", nil)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	req2 := httptest.NewRequest("GET", "/", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if len(cids) != 2 {
		t.Errorf("expected two CIDs, got %s", cids)
	}
	if cids[0] == cids[1] {
		t.Errorf("expected different clients to have different CIDs, but got %s", cids)
	}
}
