// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package middleware

import (
	"crypto/subtle"
	"errors"
	"net/http"
)

// BasicAuthenticator is an interface for retrieving a user's password.
// It defines a single method, Password, which returns the expected password for a given username.
type BasicAuthenticator interface {
	// Password returns the expected password for the provided username.
	// It returns an error if the password cannot be retrieved.
	// Passwords must be returned in plain text.
	// Any error is treated as an authentication failure, but not returned to the client.
	Password(username string) (string, error)
}

// StaticBasicAuthenticator is a BasicAuthenticator implementation that authenticates
// against a hardcoded map of usernames to passwords.
//
// Use it for simple scenarios where credentials are known in advance and do
// not need to be dynamic. Not suitable for production with sensitive data.
type StaticBasicAuthenticator map[string]string

func (s StaticBasicAuthenticator) Password(username string) (string, error) {
	pass, ok := s[username]
	if !ok {
		return "", errors.New("user not found")
	}
	return pass, nil
}

// BasicAuth is HTTP middleware for Basic Authentication.
// It checks the Authorization header, and calls the next handler if authentication succeeds.
// If authentication fails, it responds with HTTP 401 Unauthorized.
func BasicAuth(getter BasicAuthenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		expectedPass, err := getter.Password(user)
		if err != nil || subtle.ConstantTimeCompare([]byte(pass), []byte(expectedPass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
