// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package envkv

import (
	"testing"
)

func TestUnmarshalMarshal(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      []KV
		wantErr   bool
		marshalOK bool
	}{
		{
			name:  "simple key=value",
			input: "FOO=bar",
			want:  []KV{{Key: "FOO", Value: "bar"}},
		},
		{
			name:  "spaces around =",
			input: "FOO = bar",
			want:  []KV{{Key: "FOO", Value: "bar"}},
		},
		{
			name:  "quoted value",
			input: `FOO="bar"`,
			want:  []KV{{Key: "FOO", Value: "bar"}},
		},
		{
			name:  "quoted value with escape",
			input: `FOO="b\"a\nr"`,
			want:  []KV{{Key: "FOO", Value: "b\"a\nr"}},
		},
		{
			name:  "bare value with comment",
			input: `FOO=bar# comment`,
			want:  []KV{{Key: "FOO", Value: "bar"}},
		},
		{
			name:  "quoted value with #",
			input: `FOO="bar#baz"`,
			want:  []KV{{Key: "FOO", Value: "bar#baz"}},
		},
		{
			name:  "empty value with =",
			input: `FOO=`,
			want:  []KV{{Key: "FOO", Value: ""}},
		},
		{
			name:  "empty quoted value",
			input: `FOO=""`,
			want:  []KV{{Key: "FOO", Value: ""}},
		},
		{
			name:    "invalid key",
			input:   `=value`,
			wantErr: true,
		},
		{
			name:    "whitespace in bare value",
			input:   `FOO=bar baz`,
			wantErr: true,
		},
		{
			name:    "unterminated quote",
			input:   `FOO="bar`,
			wantErr: true,
		},
		{
			name:    "unknown escape",
			input:   `FOO="\t"`,
			wantErr: true,
		},
		{
			name:    "literal newline in quoted",
			input:   "FOO=\"bar\nbaz\"",
			wantErr: true,
		},
		{
			name:    "duplicate keys",
			input:   "FOO=bar\nFOO=baz",
			wantErr: true,
		},
		{
			name: "comments and blank lines",
			input: `
# comment
FOO=bar

# another
BAZ="qux"
`,
			want: []KV{
				{Key: "FOO", Value: "bar"},
				{Key: "BAZ", Value: "qux"},
			},
		},
		{
			name:  "trailing whitespace after quoted value",
			input: `FOO="bar"   `,
			want:  []KV{{Key: "FOO", Value: "bar"}},
		},
		{
			name:    "invalid backslash",
			input:   `FOO=\bar`,
			wantErr: true,
		},
		{
			name:    "UTF-8 key",
			input:   `æøå=FOO`,
			wantErr: true,
		},
		{
			name:  "UTF-8 value",
			input: `FOO="æøå"`,
			want:  []KV{{Key: "FOO", Value: "æøå"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Unmarshal([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Fatalf("Unmarshal() got %v entries, want %v", len(got), len(tt.want))
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("Unmarshal()[%d] = %+v, want %+v", i, got[i], tt.want[i])
					}
				}

				// test Marshal roundtrip
				out, err := Marshal(got)
				if err != nil {
					t.Fatalf("Marshal() error: %v", err)
				}

				// Unmarshal the output and compare
				got2, err := Unmarshal(out)
				if err != nil {
					t.Fatalf("Unmarshal(Marshal()) error: %v", err)
				}
				if !equalKV(got, got2) {
					t.Errorf("roundtrip failed: got=%+v, got2=%+v", got, got2)
				}
			}
		})
	}
}

func equalKV(a, b []KV) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
