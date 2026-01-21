// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package envkv provides functions for parsing and serializing a simple key-value format.
//
// The format is primarily intended to allow specifying environment variables in a file.
//
// Each line in the text input should be of the form "key = value", optionally allowing
// quoted values and comments.
//
// Comments (begun with `#`) are ignored.
//
// Keys must only contain alphanumeric characters.
// Duplicate keys are not allowed.
//
// Values may be quoted, supporting \" and \n escapes.
//
//	# Example envkv snippet
//	HOST=localhost
//	PORT=8080
//	DEBUG="true"
//	WELCOME_MESSAGE="Hello, \"Gopher\"!\nHave fun!"
package envkv

import (
	"bytes"
	"errors"
	"fmt"
)

// KV represents a key-value pair as used by Unmarshal and Marshal.
type KV struct {
	Key   string // The key
	Value string // The assocated value
}

// Unmarshal parses a byte slice of KV
// Returns an error describing the first encountered formatting issue, with line numbers.
func Unmarshal(b []byte) ([]KV, error) {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	lines := bytes.Split(b, []byte("\n"))

	seen := map[string]struct{}{}
	var out []KV

	for ln, line := range lines {
		i := 0

		skipWhitespace := func() {
			for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
				i++
			}
		}

		// Skip leading whitespace
		skipWhitespace()

		// Skip comments
		if i == len(line) || line[i] == '#' {
			continue
		}

		start := i
		for i < len(line) && isKeyChar(line[i]) {
			i++
		}
		if start == i {
			return nil, errf(ln, "empty or invalid key")
		}
		key := string(line[start:i])

		// Skip whitespace trailing key
		skipWhitespace()

		if i == len(line) || line[i] != '=' {
			return nil, errf(ln, "missing =")
		}
		i++

		// Skip whitespace trailing =
		skipWhitespace()

		var val string
		if i < len(line) && line[i] == '"' {
			i++
			var buf []byte
			for {
				if i >= len(line) {
					return nil, errf(ln, "unterminated quote")
				}
				if line[i] == '"' {
					i++
					break
				}
				if line[i] == '\\' {
					i++
					if i >= len(line) {
						return nil, errf(ln, "bad escape")
					}
					switch line[i] {
					case '"':
						buf = append(buf, '"')
					case 'n':
						buf = append(buf, '\n')
					default:
						return nil, errf(ln, "unknown escape")
					}
					i++
					continue
				}
				buf = append(buf, line[i])
				i++
			}
			val = string(buf)

			// Skip whitespace trailing value
			skipWhitespace()

			if i < len(line) && line[i] != '#' {
				return nil, errf(ln, "trailing characters after quoted value")
			}
		} else {
			start = i
			for i < len(line) && line[i] != '#' {
				if line[i] == ' ' || line[i] == '\t' {
					return nil, errf(ln, "whitespace in bare value")
				}
				if line[i] == '\\' {
					return nil, errf(ln, "backslash in bare value")
				}
				i++
			}
			val = string(line[start:i])
		}

		if _, ok := seen[key]; ok {
			return nil, errf(ln, "duplicate key")
		}
		seen[key] = struct{}{}
		out = append(out, KV{Key: key, Value: val})
	}

	return out, nil
}

// Marshal serializes a slice of KV in key=value format, one per line.
func Marshal(kv []KV) ([]byte, error) {
	seen := map[string]struct{}{}
	var buf bytes.Buffer

	for _, e := range kv {
		if e.Key == "" {
			return nil, errors.New("empty key")
		}
		for i := 0; i < len(e.Key); i++ {
			if !isKeyChar(e.Key[i]) {
				return nil, errors.New("invalid key")
			}
		}
		if _, ok := seen[e.Key]; ok {
			return nil, errors.New("duplicate key")
		}
		seen[e.Key] = struct{}{}

		buf.WriteString(e.Key)
		buf.WriteByte('=')

		if needsQuotes(e.Value) {
			buf.WriteByte('"')
			for i := 0; i < len(e.Value); i++ {
				switch e.Value[i] {
				case '"':
					buf.WriteString(`\"`)
				case '\n':
					buf.WriteString(`\n`)
				default:
					buf.WriteByte(e.Value[i])
				}
			}
			buf.WriteByte('"')
		} else {
			buf.WriteString(e.Value)
		}
		buf.WriteByte('\n')
	}

	return buf.Bytes(), nil
}

func isKeyChar(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

func needsQuotes(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '#', '"', '\n':
			return true
		}
	}
	return false
}

func errf(line int, msg string) error {
	return fmt.Errorf("line %d: %s", line, msg)
}
