// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package slogx contains some extensions to slog.
//
// [NewTextHandler] returns a handler which pretty-prints categorised log output.
// For convenience, there is also a global [TextHandler] instance.
//
// [NewCategory] returns a category handler, which puts a `category` attribute
// in each of the [slog.Record] it creates, as well as allowing you to set the minimum
// level to display for each of the categories independently.
//
// Using both of these functionalities might look like this:
//
//		// In library/app code: create two categories, ideally as package level vars
//	    var db *slog.Logger  = slogx.NewCategory("slogx", slogx.TextHandler, slog.LevelInfo)
//	    var net *slog.Logger = slogx.NewCategory("slogx", slogx.TextHandler, slog.LevelDebug)
//
//		// And log to them
//		db.Debug("debug dropped 1")     // dropped; db logging is LevelInfo+
//		net.Debug("debug shown 2")      // shown; net is LevelDebug+
//		db.Warn("warn shown 1")         // shown
//		net.Warn("warn shown 2")        // shown
//
// It is an explicit non-goal to provide the kitchen sink in this package.
// Just the simple stuff you want to use all the time.
package slogx

import (
	"log/slog"
	"os"
)

// A global TextHandler instance
//
// This avoids any issues around initialisation order,
// so that there is always an output available to send categorised log output to.
var TextHandler slog.Handler = NewTextHandler(os.Stderr)
