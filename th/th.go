// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package th provides some simple type helpers.
package th

// Must(T, error) takes any T, panics if there is an error, and returns T.
func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
