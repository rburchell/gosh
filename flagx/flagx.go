// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// package flagx allows retrieving flags from the environment, or envkv, as well as the command line.
//
// The order of value lookup is:
//
//  1. flag
//  2. environment
//  3. envkv
//
// When looking up keys in the environment or envkv, keys are forced to uppercase, to match convention.
//
// The API is a subset of the stdlib's flag package. It can be used either through
// the package-level functions, which operate on a default [FlagSet] ([CommandLine]):
//
//	func main() {
//	    var flagvar string
//	    flagx.StringVar(&flagvar, "flagname", "1234", "help message for flagname")
//	    flagx.Parse()
//	}
//
// or through an explicit [FlagSet], which allows parsing an arbitrary argument
// slice (mirroring stdlib's [flag.FlagSet]):
//
//	func main() {
//	    fs := flagx.NewFlagSet("mycmd", flagx.ContinueOnError)
//	    var flagvar string
//	    fs.StringVar(&flagvar, "flagname", "1234", "help message for flagname")
//	    if err := fs.Parse(os.Args[1:]); err != nil {
//	        // handle err
//	    }
//	}
//
// The implementation is not exhaustive; new API can be added as needed.
package flagx

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rburchell/gosh/log/slogx"
	"github.com/rburchell/gosh/text/envkv"
)

var log *slog.Logger = slogx.NewCategory("flagx", slogx.TextHandler, slog.LevelDebug)

// ErrorHandling defines how [FlagSet.Parse] behaves if parsing fails. It is an
// alias for [flag.ErrorHandling] so the constants below can be passed straight
// through to the underlying flag package.
type ErrorHandling = flag.ErrorHandling

// These constants mirror [flag]'s error handling modes. See [flag.ErrorHandling].
const (
	ContinueOnError = flag.ContinueOnError
	ExitOnError     = flag.ExitOnError
	PanicOnError    = flag.PanicOnError
)

type varRec struct {
	key        string
	val        any
	defaultVal any
	help       string
}

// FlagSet represents a set of defined flags. It mirrors the subset of
// [flag.FlagSet] that flagx supports, adding lookup of values from the
// environment and envkv in addition to the command line.
type FlagSet struct {
	fs   *flag.FlagSet
	vars []varRec
}

// CommandLine is the default set of command-line flags, parsed from os.Args by
// [Parse]. The package-level functions are wrappers for the methods of
// CommandLine. It mirrors [flag.CommandLine].
var CommandLine = NewFlagSet(os.Args[0], ExitOnError)

// NewFlagSet returns a new, empty flag set with the specified name and error
// handling property. See [flag.NewFlagSet].
func NewFlagSet(name string, eh ErrorHandling) *FlagSet {
	return &FlagSet{fs: flag.NewFlagSet(name, eh)}
}

func clearVars() {
	CommandLine = NewFlagSet(os.Args[0], ExitOnError)
}

// StringVar defines a string flag. See [flag.FlagSet.StringVar].
func (f *FlagSet) StringVar(val *string, key string, defaultVal string, help string) {
	f.vars = append(f.vars, varRec{key, val, defaultVal, help})
	f.fs.StringVar(val, key, defaultVal, help)
}

// BoolVar defines a bool flag. See [flag.FlagSet.BoolVar].
func (f *FlagSet) BoolVar(val *bool, key string, defaultVal bool, help string) {
	f.vars = append(f.vars, varRec{key, val, defaultVal, help})
	f.fs.BoolVar(val, key, defaultVal, help)
}

// IntVar defines an int flag. See [flag.FlagSet.IntVar].
func (f *FlagSet) IntVar(val *int, key string, defaultVal int, help string) {
	f.vars = append(f.vars, varRec{key, val, defaultVal, help})
	f.fs.IntVar(val, key, defaultVal, help)
}

// DurationVar defines a time.Duration flag. See [flag.FlagSet.DurationVar].
func (f *FlagSet) DurationVar(val *time.Duration, key string, defaultVal time.Duration, help string) {
	f.vars = append(f.vars, varRec{key, val, defaultVal, help})
	f.fs.DurationVar(val, key, defaultVal, help)
}

// Duration defines a time.Duration flag and returns a pointer to its value. See
// [flag.FlagSet.Duration].
func (f *FlagSet) Duration(key string, defaultVal time.Duration, help string) *time.Duration {
	val := new(time.Duration)
	f.DurationVar(val, key, defaultVal, help)
	return val
}

// Args returns the non-flag arguments. See [flag.FlagSet.Args].
func (f *FlagSet) Args() []string { return f.fs.Args() }

// Arg returns the i'th non-flag argument. See [flag.FlagSet.Arg].
func (f *FlagSet) Arg(i int) string { return f.fs.Arg(i) }

// NArg returns the number of non-flag arguments. See [flag.FlagSet.NArg].
func (f *FlagSet) NArg() int { return f.fs.NArg() }

func toBool(v string) bool {
	if v == "false" || v == "" {
		return false
	}
	return true
}

// assign parses raw into the value pointed to by valPtr. On a parse error the
// existing value (i.e. the default, or whatever an earlier source set) is left
// untouched and the error is logged.
func assign(source, key, raw string, valPtr any) {
	switch tv := valPtr.(type) {
	case *string:
		*tv = raw
	case *bool:
		*tv = toBool(raw)
	case *int:
		ival, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			log.Error("invalid int", "source", source, "key", key, "value", raw, "err", err)
			return
		}
		*tv = int(ival)
	case *time.Duration:
		d, err := time.ParseDuration(raw)
		if err != nil {
			log.Error("invalid duration", "source", source, "key", key, "value", raw, "err", err)
			return
		}
		*tv = d
	default:
		panic(fmt.Sprintf("unsupported %s type: %T", source, valPtr))
	}
}

// Parse parses flag definitions from the argument list, which should not
// include the command name. It mirrors [flag.FlagSet.Parse].
//
// The one difference here is that values are also looked for in envkv (as a
// .envkv file), and environment. Flag vars are searched for in envkv and
// environment as uppercase keys.
func (f *FlagSet) Parse(args []string) error {
	bytes, err := os.ReadFile(".envkv")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Error("envkv: read", "err", err)
	}

	var envkvs []envkv.KV
	if err == nil {
		envkvs, err = envkv.Unmarshal(bytes)
		if err != nil {
			log.Error("envkv: unmarshal", "err", err)
		}
	}

	helplines := []string{}

	for _, v := range f.vars {
		upperKey := strings.ToUpper(v.key)
		if v.defaultVal != nil {
			helplines = append(helplines, fmt.Sprintf("%s - %s (default %q)", upperKey, v.help, v.defaultVal))
		} else {
			helplines = append(helplines, fmt.Sprintf("%s - %s", upperKey, v.help))
		}

		// 1. Write from envkv
		for _, val := range envkvs {
			if val.Key == upperKey {
				assign("envkv", upperKey, val.Value, v.val)
			}
		}

		// 2: Write from environment
		if val, ok := os.LookupEnv(upperKey); ok {
			assign("env", upperKey, val, v.val)
		}
	}

	prev := f.fs.Usage
	f.fs.Usage = func() {
		if prev != nil {
			prev()
		} else {
			fmt.Fprintf(f.fs.Output(), "Usage of %s:\n", f.fs.Name())
			f.fs.PrintDefaults()
		}
		fmt.Fprintf(f.fs.Output(), "\n")
		fmt.Fprintf(f.fs.Output(), "Command line options also fall back to the environment:\n")

		for _, help := range helplines {
			fmt.Fprintf(f.fs.Output(), "  %s\n", help)
		}
	}

	// Step 3: overwrite with flag
	return f.fs.Parse(args)
}

// StringVar defines a string flag on [CommandLine]. See [flag.StringVar].
func StringVar(val *string, key string, defaultVal string, help string) {
	CommandLine.StringVar(val, key, defaultVal, help)
}

// BoolVar defines a bool flag on [CommandLine]. See [flag.BoolVar].
func BoolVar(val *bool, key string, defaultVal bool, help string) {
	CommandLine.BoolVar(val, key, defaultVal, help)
}

// IntVar defines an int flag on [CommandLine]. See [flag.IntVar].
func IntVar(val *int, key string, defaultVal int, help string) {
	CommandLine.IntVar(val, key, defaultVal, help)
}

// DurationVar defines a time.Duration flag on [CommandLine]. See [flag.DurationVar].
func DurationVar(val *time.Duration, key string, defaultVal time.Duration, help string) {
	CommandLine.DurationVar(val, key, defaultVal, help)
}

// Duration defines a time.Duration flag on [CommandLine] and returns a pointer
// to its value. See [flag.Duration].
func Duration(key string, defaultVal time.Duration, help string) *time.Duration {
	return CommandLine.Duration(key, defaultVal, help)
}

// Args returns the non-flag command-line arguments. See [flag.Args].
func Args() []string { return CommandLine.Args() }

// Arg returns the i'th command-line argument. See [flag.Arg].
func Arg(i int) string { return CommandLine.Arg(i) }

// NArg returns the number of non-flag command-line arguments. See [flag.NArg].
func NArg() int { return CommandLine.NArg() }

// Parse parses the command-line flags from os.Args[1:]. See [flag.Parse].
//
// The one difference here is that values are also looked for in envkv (as a
// .envkv file), and environment. Flag vars are searched for in envkv and
// environment as uppercase keys.
func Parse() {
	CommandLine.Parse(os.Args[1:])
}
