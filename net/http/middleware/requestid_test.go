// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// runTagged sends one request through the given tagger, capturing the IDs
// seen by the handler and the response.
type tagResult struct {
	cid CID
	rid RID
	err error
	rec *httptest.ResponseRecorder
}

func runTagged(t *testing.T, mw func(http.Handler) http.Handler, mutate func(*http.Request)) tagResult {
	t.Helper()
	var res tagResult
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res.cid, res.rid, res.err = IDs(r)
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	if mutate != nil {
		mutate(req)
	}
	res.rec = httptest.NewRecorder()
	handler.ServeHTTP(res.rec, req)
	if res.err != nil {
		t.Fatalf("unexpected error fetching IDs: %v", res.err)
	}
	return res
}

// cidCookie returns the cid cookie set on the response, or nil.
func cidCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "cid" {
			return c
		}
	}
	return nil
}

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

func TestValidID(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"18b04f", true},
		{"000000", true},
		{"ffffff", true},
		{"18b04", false},   // too short
		{"18b04f0", false}, // too long
		{"18B04F", false},  // uppercase
		{"18b04g", false},  // non-hex
		{"", false},        // empty
		{" 18b04f", false}, // leading whitespace
		{"18b04f ", false}, // trailing whitespace
		{"18b04f,29c15a", false},
	}
	for _, tt := range tests {
		if got := validID(tt.value); got != tt.want {
			t.Errorf("validID(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

// The default tagger must ignore the gosh tracing headers even when valid.
func TestTagWithRequestID_IgnoresForwardedHeaders(t *testing.T) {
	res := runTagged(t, TagWithRequestID, func(r *http.Request) {
		r.Header.Set(ClientIDHeader, "18b04f")
		r.Header.Set(RequestIDHeader, "ca04a1")
	})
	if res.cid == "18b04f" {
		t.Error("default tagger used forwarded CID header")
	}
	if res.rid == "ca04a1" {
		t.Error("default tagger used forwarded RID header")
	}
	if !validID(string(res.cid)) || !validID(string(res.rid)) {
		t.Errorf("expected generated valid IDs, got cid=%q rid=%q", res.cid, res.rid)
	}
	if cidCookie(res.rec) == nil {
		t.Error("expected a generated CID to set a cookie")
	}
}

func TestTagWithForwardedRequestID(t *testing.T) {
	t.Run("reuses both valid headers, no cookie set", func(t *testing.T) {
		res := runTagged(t, TagWithForwardedRequestID, func(r *http.Request) {
			r.Header.Set(ClientIDHeader, "18b04f")
			r.Header.Set(RequestIDHeader, "ca04a1")
		})
		if res.cid != "18b04f" || res.rid != "ca04a1" {
			t.Errorf("got cid=%q rid=%q, want forwarded values", res.cid, res.rid)
		}
		if cidCookie(res.rec) != nil {
			t.Error("header-supplied CID must not set a cookie")
		}
	})

	t.Run("header CID wins over different cookie", func(t *testing.T) {
		res := runTagged(t, TagWithForwardedRequestID, func(r *http.Request) {
			r.Header.Set(ClientIDHeader, "18b04f")
			r.AddCookie(&http.Cookie{Name: "cid", Value: "29c15a"})
		})
		if res.cid != "18b04f" {
			t.Errorf("got cid=%q, want header value over cookie", res.cid)
		}
	})

	t.Run("invalid header CID falls back to valid cookie", func(t *testing.T) {
		res := runTagged(t, TagWithForwardedRequestID, func(r *http.Request) {
			r.Header.Set(ClientIDHeader, "NOPE")
			r.AddCookie(&http.Cookie{Name: "cid", Value: "29c15a"})
		})
		if res.cid != "29c15a" {
			t.Errorf("got cid=%q, want cookie value", res.cid)
		}
		if cidCookie(res.rec) != nil {
			t.Error("valid cookie must not be replaced")
		}
	})

	t.Run("no valid source generates CID and sets cookie", func(t *testing.T) {
		res := runTagged(t, TagWithForwardedRequestID, func(r *http.Request) {
			r.Header.Set(ClientIDHeader, "NOPE")
			r.AddCookie(&http.Cookie{Name: "cid", Value: "ALSONO"})
		})
		if !validID(string(res.cid)) {
			t.Errorf("expected generated CID, got %q", res.cid)
		}
		c := cidCookie(res.rec)
		if c == nil || c.Value != string(res.cid) {
			t.Errorf("expected cookie with generated CID, got %v", c)
		}
	})

	t.Run("valid header CID does not repair invalid cookie", func(t *testing.T) {
		res := runTagged(t, TagWithForwardedRequestID, func(r *http.Request) {
			r.Header.Set(ClientIDHeader, "18b04f")
			r.AddCookie(&http.Cookie{Name: "cid", Value: "BROKEN"})
		})
		if res.cid != "18b04f" {
			t.Errorf("got cid=%q, want header value", res.cid)
		}
		if cidCookie(res.rec) != nil {
			t.Error("backend must not repair a gateway-owned cookie")
		}
	})

	t.Run("invalid RID header generates RID; CID and RID independent", func(t *testing.T) {
		res := runTagged(t, TagWithForwardedRequestID, func(r *http.Request) {
			r.Header.Set(ClientIDHeader, "18b04f")
			r.Header.Set(RequestIDHeader, "TOOBIG!")
		})
		if res.cid != "18b04f" {
			t.Errorf("invalid RID header must not invalidate CID header, got cid=%q", res.cid)
		}
		if !validID(string(res.rid)) || res.rid == "TOOBIG!" {
			t.Errorf("expected generated RID, got %q", res.rid)
		}
	})

	t.Run("valid RID header with no CID header", func(t *testing.T) {
		res := runTagged(t, TagWithForwardedRequestID, func(r *http.Request) {
			r.Header.Set(RequestIDHeader, "ca04a1")
		})
		if res.rid != "ca04a1" {
			t.Errorf("got rid=%q, want header value", res.rid)
		}
		if !validID(string(res.cid)) {
			t.Errorf("expected generated CID, got %q", res.cid)
		}
	})
}

// Header validation edge cases, including multi-value ambiguity, evaluated
// through the trusted tagger's selection rules.
func TestTagWithForwardedRequestID_HeaderValidation(t *testing.T) {
	tests := []struct {
		name   string
		values []string
	}{
		{"too short", []string{"18b04"}},
		{"too long", []string{"18b04f0"}},
		{"uppercase", []string{"18B04F"}},
		{"non-hex", []string{"18b04g"}},
		{"empty", []string{""}},
		{"leading whitespace", []string{" 18b04f"}},
		{"trailing whitespace", []string{"18b04f "}},
		{"comma-combined", []string{"18b04f, 29c15a"}},
		{"two lines, first valid", []string{"18b04f", "29c15a"}},
		{"two lines, second valid", []string{"NOPE", "29c15a"}},
		{"repeated identical", []string{"18b04f", "18b04f"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := runTagged(t, TagWithForwardedRequestID, func(r *http.Request) {
				// Direct map construction: real header parsing would strip
				// surrounding whitespace before we ever see it.
				for _, v := range tt.values {
					r.Header[ClientIDHeader] = append(r.Header[ClientIDHeader], v)
					r.Header[RequestIDHeader] = append(r.Header[RequestIDHeader], v)
				}
			})
			// Both must fall back to generated (and thus valid) IDs that
			// match none of the supplied values.
			for _, v := range tt.values {
				if string(res.cid) == v {
					t.Errorf("CID header value %q was used", v)
				}
				if string(res.rid) == v {
					t.Errorf("RID header value %q was used", v)
				}
			}
			if !validID(string(res.cid)) || !validID(string(res.rid)) {
				t.Errorf("expected generated IDs, got cid=%q rid=%q", res.cid, res.rid)
			}
			if cidCookie(res.rec) == nil {
				t.Error("fallback-generated CID should set a cookie")
			}
		})
	}
}

func TestForwardRequestIDs(t *testing.T) {
	// A source request that has passed through the tagger.
	tagged := func(t *testing.T) *http.Request {
		t.Helper()
		var out *http.Request
		h := TagWithRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			out = r
		}))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		return out
	}

	t.Run("copies both IDs as single header values", func(t *testing.T) {
		src := tagged(t)
		cid, rid, _ := IDs(src)
		dst := httptest.NewRequest("GET", "http://backend/", nil)
		dst.Header.Set("X-Unrelated", "keep")
		if err := ForwardRequestIDs(dst, src); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := dst.Header.Values(ClientIDHeader); len(got) != 1 || got[0] != string(cid) {
			t.Errorf("CID header = %v, want [%s]", got, cid)
		}
		if got := dst.Header.Values(RequestIDHeader); len(got) != 1 || got[0] != string(rid) {
			t.Errorf("RID header = %v, want [%s]", got, rid)
		}
		if dst.Header.Get("X-Unrelated") != "keep" {
			t.Error("unrelated header modified")
		}
	})

	t.Run("replaces existing and multiply assigned values", func(t *testing.T) {
		src := tagged(t)
		cid, _, _ := IDs(src)
		dst := httptest.NewRequest("GET", "http://backend/", nil)
		dst.Header.Add(ClientIDHeader, "stale1")
		dst.Header.Add(ClientIDHeader, "stale2")
		if err := ForwardRequestIDs(dst, src); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := dst.Header.Values(ClientIDHeader); len(got) != 1 || got[0] != string(cid) {
			t.Errorf("CID header = %v, want exactly [%s]", got, cid)
		}
	})

	t.Run("initializes nil destination header", func(t *testing.T) {
		src := tagged(t)
		dst := &http.Request{}
		if err := ForwardRequestIDs(dst, src); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dst.Header.Get(ClientIDHeader) == "" {
			t.Error("expected header to be set on initialized map")
		}
	})

	t.Run("does not modify source", func(t *testing.T) {
		src := tagged(t)
		dst := httptest.NewRequest("GET", "http://backend/", nil)
		if err := ForwardRequestIDs(dst, src); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(src.Header.Values(ClientIDHeader)) != 0 || len(src.Header.Values(RequestIDHeader)) != 0 {
			t.Error("source request headers were modified")
		}
	})

	t.Run("errors without panicking", func(t *testing.T) {
		src := tagged(t)
		untagged := httptest.NewRequest("GET", "/", nil)
		dst := httptest.NewRequest("GET", "http://backend/", nil)

		if err := ForwardRequestIDs(nil, src); err == nil {
			t.Error("expected error for nil destination")
		}
		if err := ForwardRequestIDs(dst, nil); err == nil {
			t.Error("expected error for nil source")
		}
		if err := ForwardRequestIDs(dst, untagged); err == nil {
			t.Error("expected error for untagged source")
		}
		if dst.Header.Get(ClientIDHeader) != "" {
			t.Error("failed forward must not set headers")
		}
	})
}

// End-to-end: gateway (default) → ForwardRequestIDs → backend.
func TestTrustBoundaryPropagation(t *testing.T) {
	// The gateway ignores caller-supplied tracing headers and assigns local IDs.
	var gwCID CID
	var gwRID RID
	gateway := TagWithRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gwCID, gwRID, _ = IDs(r)
	}))
	inbound := httptest.NewRequest("GET", "/", nil)
	inbound.Header.Set(ClientIDHeader, "deadbe") // hostile caller
	inbound.Header.Set(RequestIDHeader, "deadbe")
	gateway.ServeHTTP(httptest.NewRecorder(), inbound)

	if gwCID == "deadbe" || gwRID == "deadbe" {
		t.Fatal("gateway trusted caller-supplied tracing headers")
	}

	// Re-run the gateway capturing the tagged request, then forward.
	var taggedReq *http.Request
	gateway2 := TagWithRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		taggedReq = r
	}))
	gateway2.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	gwCID, gwRID, _ = IDs(taggedReq)

	outbound := httptest.NewRequest("GET", "http://backend/resource", nil)
	if err := ForwardRequestIDs(outbound, taggedReq); err != nil {
		t.Fatalf("forward failed: %v", err)
	}

	// A trusted backend reuses exactly the gateway pair.
	var beCID CID
	var beRID RID
	trusted := TagWithForwardedRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		beCID, beRID, _ = IDs(r)
	}))
	trusted.ServeHTTP(httptest.NewRecorder(), outbound)
	if beCID != gwCID || beRID != gwRID {
		t.Errorf("trusted backend got cid=%q rid=%q, want gateway's %q/%q", beCID, beRID, gwCID, gwRID)
	}

	// A default backend ignores the forwarded pair.
	outbound2 := httptest.NewRequest("GET", "http://backend/resource", nil)
	if err := ForwardRequestIDs(outbound2, taggedReq); err != nil {
		t.Fatalf("forward failed: %v", err)
	}
	deflt := TagWithRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		beCID, beRID, _ = IDs(r)
	}))
	deflt.ServeHTTP(httptest.NewRecorder(), outbound2)
	if beCID == gwCID && beRID == gwRID {
		t.Error("default backend reused forwarded IDs; it must generate local ones")
	}

	// RID reuse across requests is mechanically possible in trusted mode —
	// uniqueness is advisory, not guaranteed.
	rids := map[RID]int{}
	for range 2 {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(RequestIDHeader, "ca04a1")
		reuse := TagWithForwardedRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, rid, _ := IDs(r)
			rids[rid]++
		}))
		reuse.ServeHTTP(httptest.NewRecorder(), req)
	}
	if rids["ca04a1"] != 2 {
		t.Errorf("expected RID reuse across trusted requests, got %v", rids)
	}
}
