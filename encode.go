// Package huml provides functionality for marshalling and unmarshalling HUML data.
package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Marshal returns the HUML encoding of v.
//
// This function works like json.Marshal, converting a Go value into a HUML
// formatted byte slice. It traverses the data structure recursively.
//
// The mapping from Go types to HUML is as follows:
//   - bool -> true | false
//   - int, float, etc. -> number
//   - string -> "quoted string" or ```multiline string```
//   - struct -> multi-line dictionary
//   - map -> multi-line dictionary
//   - slice, array -> multi-line list
//   - nil pointer or interface -> null
//
// Struct fields can be customized with `huml` tags. For example:
//
//	// Field appears as 'my_field' in HUML.
//	Field int `huml:"my_field"`
//
//	// Field is ignored.
//	Field int `huml:"-"`
func Marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := NewEncoder(&buf)
	// The HUML specification indicates that an optional version directive can be at the top.
	// We will add this by default for clarity and compliance.
	if _, err := buf.WriteString("%HUML v0.1.0\n"); err != nil {
		return nil, err
	}
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// An Encoder writes HUML values to an output stream.
type Encoder struct {
	w io.Writer
}

// NewEncoder returns a new encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// Encode writes the HUML encoding of v to the stream, followed by a newline.
// See the documentation for Marshal for details about the conversion of Go
// values to HUML.
func (enc *Encoder) Encode(v any) error {
	s := newState(enc.w)
	s.marshalValue(reflect.ValueOf(v), 0)
	if s.err == nil {
		// Ensure the document ends with a newline for POSIX compatibility.
		s.write("\n")
	}
	err := s.err
	putState(s)
	return err
}

// state holds the encoding state for a single Marshal or Encode call.
// It is used to pass state through the recursive encoding process without
// passing many arguments.
type state struct {
	w   io.Writer
	err error
}

var statePool = sync.Pool{
	New: func() any {
		return new(state)
	},
}

// newState retrieves a new state from the pool.
func newState(w io.Writer) *state {
	s := statePool.Get().(*state)
	s.w = w
	return s
}

// putState returns a state to the pool.
func putState(s *state) {
	s.w = nil
	s.err = nil
	statePool.Put(s)
}

// write is a helper to write a string to the output writer,
// stopping immediately if an error has occurred.
func (s *state) write(str string) {
	if s.err != nil {
		return
	}
	_, s.err = io.WriteString(s.w, str)
}

// marshalValue is the primary recursive function that dispatches to the
// appropriate marshalling function based on the value's kind.
func (s *state) marshalValue(v reflect.Value, indent int) {
	if s.err != nil {
		return
	}

	// Follow pointers and interfaces to find the concrete value.
	// If we encounter a nil pointer along the way, it represents a null value.
	v = indirect(v, &s.err)
	if s.err != nil {
		return
	}

	// A reflect.Invalid value, often from a nil pointer, is marshalled as null.
	if !v.IsValid() {
		s.write("null")
		return
	}

	switch v.Kind() {
	case reflect.Map:
		s.marshalMap(v, indent)
	case reflect.Struct:
		s.marshalStruct(v, indent)
	case reflect.Slice, reflect.Array:
		s.marshalSlice(v, indent)
	case reflect.String:
		s.marshalString(v.String(), indent)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		s.write(strconv.FormatInt(v.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		s.write(strconv.FormatUint(v.Uint(), 10))
	case reflect.Float32, reflect.Float64:
		f := v.Float()
		if math.IsNaN(f) {
			s.write("nan")
		} else if math.IsInf(f, 1) {
			s.write("inf")
		} else if math.IsInf(f, -1) {
			s.write("-inf")
		} else {
			// 'g' format is used for the most compact representation.
			s.write(strconv.FormatFloat(f, 'g', -1, 64))
		}
	case reflect.Bool:
		s.write(strconv.FormatBool(v.Bool()))
	default:
		// Any type we don't explicitly handle is unsupported.
		s.err = fmt.Errorf("huml: unsupported type: %s", v.Type())
	}
}

// marshalMap converts a Go map into a HUML multi-line dictionary.
func (s *state) marshalMap(v reflect.Value, indent int) {
	// An empty map is represented by the special empty dict marker.
	if v.Len() == 0 {
		s.write("{}")
		return
	}

	// The HUML spec requires string keys for dictionaries.
	if v.Type().Key().Kind() != reflect.String {
		s.err = fmt.Errorf("huml: map key type must be a string, not %s", v.Type().Key())
		return
	}

	// Sort map keys to ensure the output is deterministic. This is crucial
	// for consistency in tests, version control, and other automated processing.
	keys := v.MapKeys()
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].String() < keys[j].String()
	})

	for i, key := range keys {
		// Separate key-value pairs with a newline.
		if i > 0 {
			s.write("\n")
		}

		val := v.MapIndex(key)
		s.writeKVPair(key.String(), val, indent)
	}
}

// marshalStruct converts a Go struct into a HUML multi-line dictionary.
func (s *state) marshalStruct(v reflect.Value, indent int) {
	var fields []struct {
		name  string
		value reflect.Value
	}

	// Iterate over the struct fields to gather exported fields and their names.
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		// Skip unexported fields as they are not accessible.
		if !field.IsExported() {
			continue
		}

		// Parse the `huml` tag to determine the field name or if it should be skipped.
		tag := field.Tag.Get("huml")
		if tag == "-" {
			continue
		}
		fieldName := tag
		if fieldName == "" {
			fieldName = field.Name
		}

		fields = append(fields, struct {
			name  string
			value reflect.Value
		}{
			name:  fieldName,
			value: v.Field(i),
		})
	}

	// An empty struct (or one with no exported fields) is an empty dict.
	if len(fields) == 0 {
		s.write("{}")
		return
	}

	for i, field := range fields {
		if i > 0 {
			s.write("\n")
		}
		s.writeKVPair(field.name, field.value, indent)
	}
}

// marshalSlice converts a Go slice or array into a HUML multi-line list.
func (s *state) marshalSlice(v reflect.Value, indent int) {
	// An empty slice is represented by the special empty list marker.
	if v.Len() == 0 {
		s.write("[]")
		return
	}

	for i := 0; i < v.Len(); i++ {
		if i > 0 {
			s.write("\n")
		}
		elem := v.Index(i)

		s.write(strings.Repeat(" ", indent))
		s.write("- ")

		// Determine if the list element is a scalar or a vector.
		// This is necessary to decide between `- value` and `- ::\n  ...`.
		elemKind := indirect(elem, &s.err).Kind()
		if s.err != nil {
			return
		}

		isVector := elemKind == reflect.Map || elemKind == reflect.Struct || elemKind == reflect.Slice || elemKind == reflect.Array

		if isVector {
			// A vector within a list is denoted by `::` and must start on a new line.
			s.write("::\n")
			s.marshalValue(elem, indent+2)
		} else {
			// A scalar within a list is written on the same line.
			s.marshalValue(elem, indent)
		}
	}
}

// marshalString handles both single-line and multi-line strings.
func (s *state) marshalString(str string, indent int) {
	// If a string contains a newline, it must be formatted as a multi-line string.
	// We use ``` to preserve all whitespace as per the spec.
	if strings.Contains(str, "\n") {
		s.write("```\n")
		lines := strings.Split(str, "\n")
		// The last line of a multi-line string from split can be empty if the string ends with a newline.
		// We trim this to avoid an extra trailing newline inside the HUML block.
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		for _, line := range lines {
			s.write(strings.Repeat(" ", indent+2))
			s.write(line)
			s.write("\n")
		}
		s.write(strings.Repeat(" ", indent))
		s.write("```")
	} else {
		// Standard Go quoting handles all necessary escapes for a valid HUML string.
		s.write(strconv.Quote(str))
	}
}

// isStructEmpty checks if a struct has any marshallable fields.
func (s *state) isStructEmpty(v reflect.Value) bool {
	// This assumes 'v' is an indirected value of kind Struct.
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		// A field is marshallable if it's exported and not tagged with "-".
		if field.IsExported() && field.Tag.Get("huml") != "-" {
			return false
		}
	}
	return true
}

// writeKVPair writes a complete key-value pair, including indentation, the key,
// the correct indicator (':' or '::'), and the marshalled value.
func (s *state) writeKVPair(key string, val reflect.Value, indent int) {
	s.write(strings.Repeat(" ", indent))
	s.write(quoteKeyIfNeeded(key))

	// The indicator depends on whether the value is a scalar or a vector.
	iVal := indirect(val, &s.err)
	if s.err != nil {
		return
	}
	valKind := iVal.Kind()

	isVector := valKind == reflect.Map || valKind == reflect.Struct || valKind == reflect.Slice || valKind == reflect.Array

	if isVector {
		isEmpty := false
		switch valKind {
		case reflect.Map, reflect.Slice, reflect.Array:
			if iVal.Len() == 0 {
				isEmpty = true
			}
		case reflect.Struct:
			if s.isStructEmpty(iVal) {
				isEmpty = true
			}
		}

		// This is the crucial change. For multi-line (non-empty) vectors, `::` is
		// followed by a newline. For empty vectors, it's followed by a space.
		if isEmpty {
			s.write(":: ")
		} else {
			s.write("::\n")
		}
	} else {
		s.write(": ")
	}

	// The value of a key-value pair is always indented further.
	// For a multi-line vector, its content starts at the new indentation level.
	s.marshalValue(val, indent+2)
}

// A regular expression to check if a key is a "bare" key, meaning it doesn't
// require quoting. According to the spec, this includes alphanumeric characters,
// underscores, and hyphens.
var bareKeyRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// quoteKeyIfNeeded wraps a key in quotes if it contains characters that are
// not allowed in a bare key.
func quoteKeyIfNeeded(key string) string {
	if bareKeyRegex.MatchString(key) {
		return key
	}
	return strconv.Quote(key)
}

// indirect walks down a chain of pointers and interfaces to find the underlying
// concrete value. It is essential for correctly determining the kind of a value
// that might be passed by reference. If a nil pointer is found, it returns an
// invalid reflect.Value, which will be marshalled as 'null'.
func indirect(v reflect.Value, err *error) reflect.Value {
	// The loop limit is a safeguard against circular data structures, which would
	// otherwise cause an infinite loop.
	for i := 0; i < 1000; i++ {
		if !v.IsValid() {
			return v
		}
		kind := v.Kind()
		if kind != reflect.Pointer && kind != reflect.Interface {
			return v
		}
		if v.IsNil() {
			return reflect.Value{} // Return an invalid value for nil.
		}
		v = v.Elem()
	}
	// If we hit the loop limit, the structure is too deep or cyclical.
	*err = fmt.Errorf("huml: encountered a circular or excessively deep data structure")
	return reflect.Value{}
}
