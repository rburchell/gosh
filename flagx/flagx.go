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
// The API is a subset of the stdlib's flag package, i.e:
//
//	func main() {
//	    var flagvar string
//	    flagx.StringVar(&flagvar, "flagname", "1234", "help message for flagname")
//	    flagx.Parse()
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

type varRec struct {
	key        string
	val        any
	defaultVal any
	help       string
}

var allVars []varRec

func clearVars() {
	allVars = []varRec{}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

}

// See [flag.StringVar]
func StringVar(val *string, key string, defaultVal string, help string) {
	allVars = append(allVars, varRec{key, val, defaultVal, help})
	flag.StringVar(val, key, defaultVal, help)
}

// See [flag.BoolVar]
func BoolVar(val *bool, key string, defaultVal bool, help string) {
	allVars = append(allVars, varRec{key, val, defaultVal, help})
	flag.BoolVar(val, key, defaultVal, help)
}

// See [flag.IntVar]
func IntVar(val *int, key string, defaultVal int, help string) {
	allVars = append(allVars, varRec{key, val, defaultVal, help})
	flag.IntVar(val, key, defaultVal, help)
}

// See [flag.DurationVar]
func DurationVar(val *time.Duration, key string, defaultVal time.Duration, help string) {
	allVars = append(allVars, varRec{key, val, defaultVal, help})
	flag.DurationVar(val, key, defaultVal, help)
}

// See [flag.Duration]
func Duration(key string, defaultVal time.Duration, help string) *time.Duration {
	val := new(time.Duration)
	DurationVar(val, key, defaultVal, help)
	return val
}

// See [flag.Parse]
//
// The one difference here is that values are also looked for in envkv (as a .envkv file),
// and environment. Flag vars are searched for in envkv and environment as uppercase keys.
func Parse() {
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

	toBool := func(v string) bool {
		if v == "false" || v == "" {
			return false
		}
		return true
	}

	// assign parses raw into the value pointed to by valPtr. On a parse
	// error the existing value (i.e. the default, or whatever an earlier
	// source set) is left untouched and the error is logged.
	assign := func(source, key, raw string, valPtr any) {
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

	helplines := []string{}

	for _, v := range allVars {
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

	usage := flag.Usage
	flag.Usage = func() {
		usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Command line options also fall back to the environment:\n")

		for _, help := range helplines {
			fmt.Fprintf(flag.CommandLine.Output(), "  %s\n", help)
		}
	}

	// Step 3: overwrite with flag
	flag.Parse()
}
