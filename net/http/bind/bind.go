// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bind provides an easy way to map a HTTP request parameters to a structs.
//
// Data sources are query parameters, form values, and JSON bodies.
//
// Supported struct tags are:
//   - `form`: The name of the formfield to decode.
//   - `binding:"required"`: Marks the field as required.
//
// If a required parameter is missing, an error is returned.
//
// Example usage:
//
//	type Input struct {
//	    Name  string  `form:"name" binding:"required"`
//	    Age   *int    `form:"age"`
//	}
//
//	var in Input
//	if err := bind.BindForm(r, &in); err != nil {
//	    // Handle error (e.g., missing required fields)
//	}
package bind

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
)

// Validate that all fields on obj with a required binding were placed in writtenFields.
// The key of writtenFields must be the field name, not the tag, for easier lookup.
func validateRequired[T any](writtenFields map[string]struct{}, obj T) error {
	v := reflect.ValueOf(obj).Elem()
	t := v.Type()

	for i := range t.NumField() {
		f := t.Field(i)
		if f.Tag.Get("binding") != "required" {
			continue
		}
		if _, ok := writtenFields[f.Name]; !ok {
			return fmt.Errorf("%s is required", f.Name)
		}
	}
	return nil
}

// Look up each field and value on a given obj, and call the callback.
//
// The given tagKey is used to name the field by tag instead of using the field name, if it's set.
func forEachField(obj any, tagKey string, fn func(field reflect.StructField, fv reflect.Value, tag string) error) error {
	v := reflect.ValueOf(obj).Elem()
	t := v.Type()

	for i := range t.NumField() {
		f := t.Field(i)
		tag := f.Tag.Get(tagKey)
		if tag == "" {
			tag = f.Name
		}
		if err := fn(f, v.Field(i), tag); err != nil {
			return err
		}
	}
	return nil
}

// Reads form values from r and writes them to obj.
//
// The form field names are determined from the struct field names,
// but can be overridden by setting a "form" struct tag.
//
// For example:
//
//	struct Person {
//	    Age int `form:"age"`
//	}
//
// If the struct tag `binding:"required" is set,
// then if the field is not present, an error will be returned.`
func BindForm[T any](r *http.Request, obj *T) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	writtenFields := make(map[string]struct{})
	err := forEachField(obj, "form", func(field reflect.StructField, fv reflect.Value, tag string) error {
		values, present := r.Form[tag]
		if !present {
			return nil
		}
		if len(values) == 0 {
			panic("how is this present?")
		}
		value := values[0]
		if err := setFieldValue(field.Name, fv, value); err != nil {
			return err
		}
		writtenFields[field.Name] = struct{}{}
		return nil
	})

	if err != nil {
		return err
	}

	return validateRequired(writtenFields, obj)
}

// Reads query values from r and writes them to obj.
//
// The query field names are determined from the struct field names,
// but can be overridden by setting a "query" struct tag.
//
// For example:
//
//	struct Person {
//	    Age int `query:"age"`
//	}
//
// If the struct tag `binding:"required" is set,
// then if the field is not present, an error will be returned.`
func BindQuery[T any](r *http.Request, obj *T) error {
	q := r.URL.Query()

	writtenFields := make(map[string]struct{})
	err := forEachField(obj, "query", func(field reflect.StructField, fv reflect.Value, tag string) error {
		value, present := q.Get(tag), q.Has(tag)
		if !present {
			return nil
		}
		if err := setFieldValue(field.Name, fv, value); err != nil {
			return err
		}
		writtenFields[field.Name] = struct{}{}
		return nil
	})

	if err != nil {
		return err
	}

	return validateRequired(writtenFields, obj)
}

// Reads json values from r and writes them to obj.
//
// The json field names are determined from the struct field names,
// but can be overridden by setting a "json" struct tag.
//
// For example:
//
//	struct Person {
//	    Age int `json:"age"`
//	}
//
// If the struct tag `binding:"required" is set,
// then if the field is not present, an error will be returned.`
func BindJSON[T any](r *http.Request, obj *T) error {
	defer r.Body.Close()

	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		return err
	}

	writtenFields := make(map[string]struct{})
	err := forEachField(obj, "json", func(field reflect.StructField, fv reflect.Value, tag string) error {
		value, ok := data[tag]
		if !ok {
			return nil
		}
		if err := setFieldValue(field.Name, fv, value); err != nil {
			return err
		}
		writtenFields[field.Name] = struct{}{}
		return nil
	})

	if err != nil {
		return err
	}

	return validateRequired(writtenFields, obj)
}
