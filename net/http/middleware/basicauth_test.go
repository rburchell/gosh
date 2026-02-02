// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBasicAuth(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	auther := StaticBasicAuthenticator{
		"alice": "13",
		"bob":   "builder",
	}

	tests := []struct {
		name           string
		username       string
		password       string
		wantStatus     int
		wantNextCalled bool
	}{
		{
			name:           "Valid credentials",
			username:       "alice",
			password:       "13",
			wantStatus:     http.StatusOK,
			wantNextCalled: true,
		},
		{
			name:           "Unknown user",
			username:       "eve",
			password:       "password",
			wantStatus:     http.StatusUnauthorized,
			wantNextCalled: false,
		},
		{
			name:           "Wrong password",
			username:       "bob",
			password:       "notbuilder",
			wantStatus:     http.StatusUnauthorized,
			wantNextCalled: false,
		},
		{
			name:           "Missing credentials",
			username:       "",
			password:       "",
			wantStatus:     http.StatusUnauthorized,
			wantNextCalled: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nextCalled = false
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			if tc.username != "" || tc.password != "" {
				req.SetBasicAuth(tc.username, tc.password)
			}

			handler := BasicAuth(auther, next)
			handler.ServeHTTP(rec, req)

			if got := rec.Code; got != tc.wantStatus {
				t.Errorf("got HTTP status = %v, want %v", got, tc.wantStatus)
			}
			if nextCalled != tc.wantNextCalled {
				t.Errorf("next handler called = %v, want %v", nextCalled, tc.wantNextCalled)
			}
		})
	}
}
