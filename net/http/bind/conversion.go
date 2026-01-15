// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bind

import (
	"fmt"
	"reflect"
	"strconv"
)

// Writes 'value' to 'fv' (named field 'fieldName').
//
// The exception is if 'value' is nil: the field is not written.
// However, this should not happen.
//
// Returns an error if the value cannot be written (e.g, wrong type).
//
// FIXME: add fieldName to all logging.
func setFieldValue(fieldName string, fv reflect.Value, value any) error {
	// Apologies in advance ... Abandon all hope all ye who enter here ...
	if value == nil {
		panic("setFieldValue was given nil!")
	}

	// Handle pointers
	if fv.Kind() == reflect.Pointer {
		ptrVal := reflect.New(fv.Type().Elem())
		if err := setFieldValue(fieldName, ptrVal.Elem(), value); err != nil {
			return err
		}
		fv.Set(ptrVal)
		return nil
	}

	if !fv.CanSet() {
		return fmt.Errorf("field %s is not settable", fieldName)
	}

	rv := reflect.ValueOf(value)
	kind := fv.Kind()

	switch v := value.(type) {
	case string:
		str := v
		switch kind {
		case reflect.String:
			fv.SetString(str)
			return nil
		case reflect.Bool:
			b, err := strconv.ParseBool(str)
			if err != nil {
				return fmt.Errorf("cannot convert %q to bool: %w", str, err)
			}
			fv.SetBool(b)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			i, err := strconv.ParseInt(str, 10, 64)
			if err != nil {
				return fmt.Errorf("cannot convert %q to int: %w", str, err)
			}
			fv.SetInt(i)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			u, err := strconv.ParseUint(str, 10, 64)
			if err != nil {
				return fmt.Errorf("cannot convert %q to uint: %w", str, err)
			}
			fv.SetUint(u)
		case reflect.Float32, reflect.Float64:
			f, err := strconv.ParseFloat(str, 64)
			if err != nil {
				return fmt.Errorf("cannot convert %q to float: %w", str, err)
			}
			fv.SetFloat(f)

		default:
			return fmt.Errorf("unsupported kind %s for string input", kind)
		}
		return nil
	case bool:
		if kind == reflect.Bool {
			fv.SetBool(v)
		} else {
			return fmt.Errorf("cannot assign bool to %s", kind)
		}
		return nil
	case int, int8, int16, int32, int64:
		i := reflect.ValueOf(v).Int()
		switch kind {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			fv.SetInt(i)
		case reflect.Float32, reflect.Float64:
			fv.SetFloat(float64(i))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if i < 0 {
				return fmt.Errorf("cannot assign negative int to uint")
			}
			fv.SetUint(uint64(i))
		default:
			return fmt.Errorf("cannot assign int to %s", kind)
		}
		return nil
	case uint, uint8, uint16, uint32, uint64:
		u := reflect.ValueOf(v).Uint()
		switch kind {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			fv.SetUint(u)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			fv.SetInt(int64(u))
		case reflect.Float32, reflect.Float64:
			fv.SetFloat(float64(u))
		default:
			return fmt.Errorf("cannot assign uint to %s", kind)
		}
		return nil
	case float32, float64:
		f := reflect.ValueOf(v).Float()
		switch kind {
		case reflect.Float32, reflect.Float64:
			fv.SetFloat(f)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			fv.SetInt(int64(f))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if f < 0 {
				return fmt.Errorf("cannot assign negative float to uint")
			}
			fv.SetUint(uint64(f))
		default:
			return fmt.Errorf("cannot assign float to %s", kind)
		}
		return nil
	}

	// Handle slices
	if kind == reflect.Slice && rv.Kind() == reflect.Slice {
		slice := reflect.MakeSlice(fv.Type(), rv.Len(), rv.Len())
		for i := range rv.Len() {
			if err := setFieldValue(fmt.Sprintf("%s[%d]", fieldName, i), slice.Index(i), rv.Index(i).Interface()); err != nil {
				return err
			}
		}
		fv.Set(slice)
		return nil
	}

	// Handle maps
	if kind == reflect.Map && rv.Kind() == reflect.Map {
		if fv.Type().Key() != rv.Type().Key() {
			return fmt.Errorf("cannot assign map with key type %s to map with key type %s", rv.Type().Key(), fv.Type().Key())
		}
		if fv.Type().Elem() != rv.Type().Elem() {
			return fmt.Errorf("cannot assign map with value type %s to map with value type %s", rv.Type().Elem(), fv.Type().Elem())
		}

		mp := reflect.MakeMap(fv.Type())
		for _, key := range rv.MapKeys() {
			val := reflect.New(fv.Type().Elem()).Elem()
			if err := setFieldValue(fmt.Sprintf("%s[%v]", fieldName, key.Interface()), val, rv.MapIndex(key).Interface()); err != nil {
				return err
			}
			mp.SetMapIndex(key.Convert(fv.Type().Key()), val)
		}
		fv.Set(mp)
		return nil
	}

	// fallback to assignable/convertible
	if rv.Type().AssignableTo(fv.Type()) {
		fv.Set(rv)
		return nil
	} else if rv.Type().ConvertibleTo(fv.Type()) {
		fv.Set(rv.Convert(fv.Type()))
		return nil
	}

	// give up and go home
	return fmt.Errorf("cannot assign %T to %s", value, fv.Type())
}
