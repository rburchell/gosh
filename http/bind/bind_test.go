// Copyright 2025 Robin Burchell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bind

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

// To avoid having to write huge, exhaustive type-specific tests for each of the Bind* variants, we have this ... lovely test.
// It tests setFieldValue, which all the Bind variants use.
func TestSetFieldValue(t *testing.T) {
	type TestStruct struct {
		Str      string
		StrPtr   *string
		Bool     bool
		BoolPtr  *bool
		Int      int
		IntPtr   *int
		Uint     uint
		UintPtr  *uint
		Float    float64
		FloatPtr *float64
		Slice    []int
		Map      map[string]int
	}

	strVal := "pointer"
	boolVal := true
	intVal := 42
	uintVal := uint(42)
	floatVal := 3.14

	tests := []struct {
		name      string
		field     string
		fieldType reflect.Type
		value     any
		want      any
		wantErr   bool
	}{
		// String cases
		{"string direct", "Str", reflect.TypeOf(""), "world", "world", false},
		{"string from bool", "Str", reflect.TypeOf(""), true, "", true},
		{"string ptr", "StrPtr", reflect.TypeOf((*string)(nil)), "pointer", &strVal, false},

		// Bool cases
		{"bool direct", "Bool", reflect.TypeOf(true), true, true, false},
		{"bool from string true", "Bool", reflect.TypeOf(true), "true", true, false},
		{"bool from string false", "Bool", reflect.TypeOf(true), "false", false, false},
		{"bool ptr", "BoolPtr", reflect.TypeOf((*bool)(nil)), true, &boolVal, false},
		{"bool wrong type", "Bool", reflect.TypeOf(true), "notabool", false, true},

		// Int cases
		{"int direct", "Int", reflect.TypeOf(0), 5, 5, false},
		{"int from string", "Int", reflect.TypeOf(0), "42", 42, false},
		{"int from float", "Int", reflect.TypeOf(0), 5.8, 5, false},
		{"int wrong type", "Int", reflect.TypeOf(0), true, 0, true},
		{"int ptr", "IntPtr", reflect.TypeOf((*int)(nil)), 42, &intVal, false},

		// Uint cases
		{"uint direct", "Uint", reflect.TypeOf(uint(0)), uint(9), uint(9), false},
		{"uint from string", "Uint", reflect.TypeOf(uint(0)), "123", uint(123), false},
		{"uint negative int", "Uint", reflect.TypeOf(uint(0)), -2, uint(0), true},
		{"uint wrong type", "Uint", reflect.TypeOf(uint(0)), true, uint(0), true},
		{"uint ptr", "UintPtr", reflect.TypeOf((*uint)(nil)), uint(42), &uintVal, false},

		// Float cases
		{"float direct", "Float", reflect.TypeOf(0.0), 1.5, 1.5, false},
		{"float from string", "Float", reflect.TypeOf(0.0), "2.5", 2.5, false},
		{"float from int", "Float", reflect.TypeOf(0.0), 3, 3.0, false},
		{"float wrong type", "Float", reflect.TypeOf(0.0), true, 0.0, true},
		{"float ptr", "FloatPtr", reflect.TypeOf((*float64)(nil)), 3.14, &floatVal, false},

		// Slice cases
		{"slice int", "Slice", reflect.TypeOf([]int{}), []int{1, 2, 3}, []int{1, 2, 3}, false},
		{"slice from wrong type", "Slice", reflect.TypeOf([]int{}), []string{"a"}, nil, true},

		// Map cases
		{"map basic", "Map", reflect.TypeOf(map[string]int{}), map[string]int{"a": 1}, map[string]int{"a": 1}, false},
		{"map from wrong key", "Map", reflect.TypeOf(map[string]int{}), map[int]int{1: 2}, nil, true},
		{"map from wrong value", "Map", reflect.TypeOf(map[string]int{}), map[string]string{}, nil, true},

		// Conversion cases
		{"int assignable", "Int", reflect.TypeOf(int(0)), int64(6), 6, false},
		{"uint assignable", "Uint", reflect.TypeOf(uint(0)), uint64(9), uint(9), false},
		{"float assignable", "Float", reflect.TypeOf(float64(0)), float32(7.3), 7.3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s TestStruct
			sf := reflect.ValueOf(&s).Elem().FieldByName(tt.field)
			err := setFieldValue(tt.field, sf, tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			actual := sf.Interface()
			if sf.Kind() == reflect.Pointer && !sf.IsNil() {
				actual = sf.Elem().Interface()
			}

			switch want := tt.want.(type) {
			case *string:
				if sf.Kind() != reflect.Pointer || sf.IsNil() || sf.Elem().Interface() != *want {
					t.Fatalf("got %#v, want %#v", actual, *want)
				}
			case *bool:
				if sf.Kind() != reflect.Pointer || sf.IsNil() || sf.Elem().Interface() != *want {
					t.Fatalf("got %#v, want %#v", actual, *want)
				}
			case *int:
				if sf.Kind() != reflect.Pointer || sf.IsNil() || sf.Elem().Interface() != *want {
					t.Fatalf("got %#v, want %#v", actual, *want)
				}
			case *uint:
				if sf.Kind() != reflect.Pointer || sf.IsNil() || sf.Elem().Interface() != *want {
					t.Fatalf("got %#v, want %#v", actual, *want)
				}
			case *float64:
				if sf.Kind() != reflect.Pointer || sf.IsNil() || math.Abs(sf.Elem().Interface().(float64)-*want) > 1e-6 {
					t.Fatalf("got %#v, want %#v", actual, *want)
				}
			case float32:
				if math.Abs(float64(actual.(float32)-want)) > 1e-6 {
					t.Fatalf("got %#v (%T), want %#v (%T)", actual, actual, tt.want, tt.want)
				}
			case float64:
				if math.Abs(actual.(float64)-want) > 1e-6 {
					t.Fatalf("got %#v (%T), want %#v (%T)", actual, actual, tt.want, tt.want)
				}
			default:
				if !reflect.DeepEqual(actual, tt.want) {
					t.Fatalf("got %#v (%T), want %#v (%T)", actual, actual, tt.want, tt.want)
				}
			}
		})
	}
}

// Simple test of BindForm
func TestBindFormBasics(t *testing.T) {
	type TestStruct struct {
		Name     string `form:"name" binding:"required"`
		Age      int    `form:"age"`
		Active   bool   `form:"active"`
		Optional string `form:"optional"`
	}

	tests := []struct {
		name      string
		formData  url.Values
		want      TestStruct
		wantError bool
	}{
		{
			name: "All fields present",
			formData: url.Values{
				"name":     {"Alice"},
				"age":      {"30"},
				"active":   {"true"},
				"optional": {"hello"},
			},
			want: TestStruct{
				Name:     "Alice",
				Age:      30,
				Active:   true,
				Optional: "hello",
			},
			wantError: false,
		},
		{
			name: "Missing optional field",
			formData: url.Values{
				"name":   {"Bob"},
				"age":    {"25"},
				"active": {"false"},
			},
			want: TestStruct{
				Name:   "Bob",
				Age:    25,
				Active: false,
			},
			wantError: false,
		},
		{
			name:      "Missing required field",
			formData:  url.Values{"age": {"40"}},
			want:      TestStruct{Age: 40},
			wantError: true,
		},
		{
			name: "Empty optional field",
			formData: url.Values{
				"name":     {"Carol"},
				"optional": {""},
			},
			want: TestStruct{
				Name:     "Carol",
				Optional: "",
			},
			wantError: false,
		},
		{
			name: "Boolean parsing",
			formData: url.Values{
				"name":   {"Dave"},
				"active": {"1"},
			},
			want: TestStruct{
				Name:   "Dave",
				Active: true,
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := strings.NewReader(tt.formData.Encode())
			req, _ := http.NewRequest("POST", "/", body)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			var got TestStruct
			err := BindForm(req, &got)
			if (err != nil) != tt.wantError {
				t.Errorf("Bind error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bind = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// Tests that binding pointers works.
func TestBindFormPointers(t *testing.T) {
	type TestStruct struct {
		Name     *string `form:"name" binding:"required"`
		Age      *int    `form:"age"`
		Active   *bool   `form:"active"`
		Optional *string `form:"optional"`
	}

	strPtr := func(s string) *string { return &s }
	intPtr := func(i int) *int { return &i }
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name      string
		formData  url.Values
		want      TestStruct
		wantError bool
	}{
		{
			name: "All fields present",
			formData: url.Values{
				"name":     {"Alice"},
				"age":      {"30"},
				"active":   {"true"},
				"optional": {"hello"},
			},
			want: TestStruct{
				Name:     strPtr("Alice"),
				Age:      intPtr(30),
				Active:   boolPtr(true),
				Optional: strPtr("hello"),
			},
			wantError: false,
		},
		{
			name: "Missing optional field",
			formData: url.Values{
				"name":   {"Bob"},
				"age":    {"25"},
				"active": {"false"},
			},
			want: TestStruct{
				Name:   strPtr("Bob"),
				Age:    intPtr(25),
				Active: boolPtr(false),
			},
			wantError: false,
		},
		{
			name:      "Missing required field",
			formData:  url.Values{"age": {"40"}},
			want:      TestStruct{Age: intPtr(40)},
			wantError: true,
		},
		{
			name: "Empty optional field",
			formData: url.Values{
				"name":     {"Carol"},
				"optional": {""},
			},
			want: TestStruct{
				Name:     strPtr("Carol"),
				Optional: strPtr(""),
			},
			wantError: false,
		},
		{
			name: "Boolean parsing",
			formData: url.Values{
				"name":   {"Dave"},
				"active": {"1"},
			},
			want: TestStruct{
				Name:   strPtr("Dave"),
				Active: boolPtr(true),
			},
			wantError: false,
		},
		{
			name: "Pointer remains nil when missing",
			formData: url.Values{
				"name": {"Eve"},
			},
			want: TestStruct{
				Name: strPtr("Eve"),
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := strings.NewReader(tt.formData.Encode())
			req, _ := http.NewRequest("POST", "/", body)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			var got TestStruct
			err := BindForm(req, &got)
			if (err != nil) != tt.wantError {
				t.Errorf("Bind error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bind = %+v, want %+v", got, tt.want)
			}
		})
	}
}

type (
	FormInput struct {
		Name  string `form:"name" binding:"required"`
		Age   int    `form:"age"`
		Email string // tagless, should match field name
	}
	QueryInput struct {
		Item string `query:"item" binding:"required"`
		ID   int    `query:"id"`
		Flag bool   // tagless
	}
	JSONInput struct {
		Title string `json:"title" binding:"required"`
		Num   int    `json:"num"`
		Flag  bool   // tagless
	}
)

func TestBindForm(t *testing.T) {
	tests := []struct {
		name    string
		form    url.Values
		want    FormInput
		wantErr bool
	}{
		{
			name: "all fields present",
			form: url.Values{"name": {"Alice"}, "age": {"32"}, "Email": {"alice@test.com"}},
			want: FormInput{Name: "Alice", Age: 32, Email: "alice@test.com"},
		},
		{
			name:    "missing required",
			form:    url.Values{"age": {"32"}, "Email": {"bob@test.com"}},
			want:    FormInput{Age: 32, Email: "bob@test.com"},
			wantErr: true,
		},
		{
			name: "no optional field",
			form: url.Values{"name": {"Bob"}},
			want: FormInput{Name: "Bob"},
		},
		{
			name: "tagless present",
			form: url.Values{"name": {"Carol"}, "Email": {"carol@test.com"}},
			want: FormInput{Name: "Carol", Email: "carol@test.com"},
		},
		{
			name: "extra unrelated fields",
			form: url.Values{"name": {"X"}, "unknown": {"foobar"}},
			want: FormInput{Name: "X"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{Form: tt.form}
			var got FormInput
			err := BindForm(r, &got)
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBindQuery(t *testing.T) {
	tests := []struct {
		name    string
		rawQS   string
		want    QueryInput
		wantErr bool
	}{
		{
			name:  "all fields present",
			rawQS: "item=foo&id=42&Flag=true",
			want:  QueryInput{Item: "foo", ID: 42, Flag: true},
		},
		{
			name:    "missing required",
			rawQS:   "id=7&Flag=false",
			want:    QueryInput{ID: 7, Flag: false},
			wantErr: true,
		},
		{
			name:  "no optional",
			rawQS: "item=bar",
			want:  QueryInput{Item: "bar"},
		},
		{
			name:  "tagless present",
			rawQS: "item=foo&Flag=true",
			want:  QueryInput{Item: "foo", Flag: true},
		},
		{
			name:  "extra field",
			rawQS: "item=test&extra=xyz",
			want:  QueryInput{Item: "test"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := &url.URL{RawQuery: tt.rawQS}
			r := &http.Request{URL: url}
			var got QueryInput
			err := BindQuery(r, &got)
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBindJSON(t *testing.T) {
	tests := []struct {
		name    string
		body    map[string]any
		want    JSONInput
		wantErr bool
	}{
		{
			name: "all fields",
			body: map[string]any{"title": "foo", "num": 1, "Flag": true},
			want: JSONInput{Title: "foo", Num: 1, Flag: true},
		},
		{
			name:    "missing required",
			body:    map[string]any{"num": 2, "Flag": false},
			want:    JSONInput{Num: 2, Flag: false},
			wantErr: true,
		},
		{
			name: "only required",
			body: map[string]any{"title": "bar"},
			want: JSONInput{Title: "bar"},
		},
		{
			name: "tagless present",
			body: map[string]any{"title": "baz", "Flag": true},
			want: JSONInput{Title: "baz", Flag: true},
		},
		{
			name: "extra field",
			body: map[string]any{"title": "zip", "extra": "xx"},
			want: JSONInput{Title: "zip"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := json.Marshal(tt.body)
			r := &http.Request{Body: io.NopCloser(bytes.NewReader(b))}
			var got JSONInput
			err := BindJSON(r, &got)
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
