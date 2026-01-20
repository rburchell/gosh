// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package middleware

import (
	"net/http"
	"testing"
)

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
