// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

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
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// LogRequests ... logs requests.
func LogRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recw := &statusRecorder{ResponseWriter: w, status: 200}
		start := time.Now()
		next.ServeHTTP(recw, r)
		duration := time.Since(start)

		cid, rid, err := IDs(r)
		cids := "??"
		rids := "??"
		if err == nil {
			cids = string(cid)
			rids = string(rid)
		}

		level := slog.LevelInfo
		if recw.status >= 500 {
			level = slog.LevelError
		} else if recw.status >= 400 {
			level = slog.LevelWarn
		}

		slog.Log(r.Context(), level, "http",
			slog.Int("status", recw.status),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Duration("duration", duration),
			slog.String("cid", cids),
			slog.String("rid", rids),
			slog.String("ip", getClientIP(r)),
		)
	})
}
