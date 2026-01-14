// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package uuidv4

import (
	"testing"
)

const uuid1 = "a6075bc7-1a09-443a-b1c0-64de253fb2d6"
const uuid2 = "7d301ddd-8360-4aa3-9d23-71504d03b6e2"

func TestFromString_Valid(t *testing.T) {
	u, err := FromString(uuid1)
	if err != nil {
		t.Fatalf("expected no err, got %v", err)
	}
	if u.String() != uuid1 {
		t.Fatalf("expected %q, got %q", uuid1, u.String())
	}
}

func TestFromString_Uppercase(t *testing.T) {
	u := MustFromString("7D301DDD-8360-4AA3-9D23-71504D03B6E2")
	if u.String() != uuid2 {
		t.Fatalf("expected %q, got %q", uuid1, u.String())
	}
}

func TestFromString_Invalid(t *testing.T) {
	_, err := FromString("not-a-uuid")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMustFromString(t *testing.T) {
	u := MustFromString(uuid2)
	if u.String() != uuid2 {
		t.Fatalf("expected %q, got %q", uuid2, u.String())
	}
}

func TestFromBytes_Valid(t *testing.T) {
	u1 := MustFromString(uuid1)
	b := u1.Bytes()
	u2, err := FromBytes(b)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !u1.Equal(u2) {
		t.Fatal("UUIDs from string and bytes should be equal")
	}
}

func TestFromBytes_Invalid(t *testing.T) {
	_, err := FromBytes([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("expected error for invalid bytes, got nil")
	}
}

func TestMustFromBytes(t *testing.T) {
	u1 := MustFromString(uuid2)
	u2 := MustFromBytes(u1.Bytes())
	if !u1.Equal(u2) {
		t.Fatal("UUIDs from MustFromBytes and original should be equal")
	}
}

func TestUUID_Bytes(t *testing.T) {
	u := MustFromString(uuid1)
	b := u.Bytes()
	if len(b) != 16 {
		t.Fatalf("expected 16 bytes, got %d", len(b))
	}
}

func TestUUID_Equal(t *testing.T) {
	u1 := MustFromString(uuid1)
	u2 := MustFromString(uuid1)
	u3 := MustFromString(uuid2)
	if !u1.Equal(u1) {
		t.Fatal("equal UUIDs not equal")
	}
	if !u1.Equal(u2) {
		t.Fatal("equal UUIDs not equal")
	}
	if u1.Equal(u3) {
		t.Fatal("unequal UUIDs claimed equal")
	}
}

func TestUUID_String(t *testing.T) {
	u := MustFromString(uuid2)
	s := u.String()
	if s != uuid2 {
		t.Fatalf("expected %q, got %q", uuid2, s)
	}
}

func TestMay(t *testing.T) {
	u, err := May()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	s := u.String()
	if len(s) != 36 {
		t.Fatalf("expected UUID string length 36, got %d", len(s))
	}

	// since it's random, we can't do much here except to verify it looks valid-ish.
	// test version
	if s[14] != '4' {
		t.Fatalf("expected version 4, got %v", s[14])
	}
	// test variant
	v := s[19]
	if v != '8' && v != '9' && v != 'a' && v != 'b' {
		t.Fatalf("expected RFC4122 variant, got %v", v)
	}
}

func TestRandom(t *testing.T) {
	rands := make(map[string]struct{})
	for _ = range 100 {
		u := Must()
		rands[u.String()] = struct{}{}
	}
	if len(rands) != 100 {
		t.Fatalf("expected 100 unique UUID, only got %d", len(rands))
	}
}

func TestMust(t *testing.T) {
	u := Must()
	s := u.String()
	if len(s) != 36 {
		t.Fatalf("expected UUID string length 36, got %d", len(s))
	}
}
