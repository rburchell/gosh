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
// The API a subset of the stdlib's flag package, i.e:
//
//	var flagvar string
//
//	func init() {
//	    flagx.StringVar(&flagvar, "flagname", "1234", "help message for flagname")
//	}
//
//	func main() {
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
	toInt := func(v string) int {
		var ival int64
		ival, err = strconv.ParseInt(v, 10, 64)
		return int(ival)
	}

	for _, v := range allVars {
		upperKey := strings.ToUpper(v.key)

		// 1. Write from envkv
		for _, val := range envkvs {
			if val.Key == upperKey {
				switch tv := v.val.(type) {
				case *string:
					*tv = val.Value
				case *bool:
					*tv = toBool(val.Value)
				case *int:
					*tv = toInt(val.Value)
				default:
					panic(fmt.Sprintf("unsupported envkv type: %T", v.val))
				}
			}
		}

		// 2: Write from environment
		val, ok := os.LookupEnv(upperKey)
		if ok {
			switch tv := v.val.(type) {
			case *string:
				*tv = val
			case *bool:
				*tv = toBool(val)
			case *int:
				*tv = toInt(val)
			default:
				panic(fmt.Sprintf("unsupported env type: %T", v.val))
			}
		}
	}

	// Step 3: overwrite with flag
	flag.Parse()
}
