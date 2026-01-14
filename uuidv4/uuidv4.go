// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// package uuidv4 is for generating and manipulating UUIDs
//
// All UUIDs are V4, RFC 4122 variant.
//
// To generate UUIDs, the entry points are May() and Must().
// They generate the same UUID type, but Must() will panic
// if generation ever fails (however unlikely that may be).
package uuidv4

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/rburchell/gosh/th"
)

type UUID [16]byte

// Generate a UUID, or return error.
func May() (UUID, error) {
	var u UUID
	if _, err := rand.Read(u[:]); err != nil {
		return UUID{}, err
	}

	// set version to 4
	u[6] = (u[6] & 0x0f) | 0x40
	// set variant to RFC4122
	u[8] = (u[8] & 0x3f) | 0x80

	return u, nil
}

// Generate a UUID, panic if generation failures.
func Must() UUID {
	return th.Must(May())
}

var _ fmt.Stringer = UUID{}

// Returns a string representation of UUID.
//
// xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.
func (u UUID) String() string {
	var buf [36]byte
	hex.Encode(buf[0:8], u[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], u[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], u[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], u[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], u[10:16])
	return string(buf[:])
}

// Returns the raw underlying bytes of the UUID.
//
// Note that the slice is not copied, so modifying the UUID will also copy the returned slice.
func (u UUID) Bytes() []byte {
	return u[:]
}

// Returns true if the two UUID are equal.
func (u UUID) Equal(v UUID) bool {
	return bytes.Equal(u[:], v[:])
}

// Returns UUID from raw bytes, or error.
func FromBytes(b []byte) (UUID, error) {
	if len(b) != 16 {
		return UUID{}, fmt.Errorf("uuid: invalid length: %d", len(b))
	}
	var u UUID
	copy(u[:], b)
	return u, nil
}

// Returns UUID from raw bytes, or panic.
func MustFromBytes(b []byte) UUID {
	return th.Must(FromBytes(b))
}

// Returns UUID parsed from string representation, or error.
func FromString(s string) (UUID, error) {
	// TODO: It may make sense to be more permissive in our allowed formats here.
	if len(s) != 36 ||
		s[8] != '-' || s[13] != '-' ||
		s[18] != '-' || s[23] != '-' {
		return UUID{}, errors.New("uuid: invalid string format")
	}

	var u UUID
	_, err := hex.Decode(u[0:4], []byte(s[0:8]))
	if err != nil {
		return UUID{}, err
	}
	_, err = hex.Decode(u[4:6], []byte(s[9:13]))
	if err != nil {
		return UUID{}, err
	}
	_, err = hex.Decode(u[6:8], []byte(s[14:18]))
	if err != nil {
		return UUID{}, err
	}
	_, err = hex.Decode(u[8:10], []byte(s[19:23]))
	if err != nil {
		return UUID{}, err
	}
	_, err = hex.Decode(u[10:16], []byte(s[24:36]))
	if err != nil {
		return UUID{}, err
	}

	return u, nil
}

// Returns UUID parsed from string representation, or panic.
func MustFromString(s string) UUID {
	return th.Must(FromString(s))
}
