// Package huml provides functionality for parsing, encoding and decoding
// HUML (Human-Oriented Markup Language) documents.
package huml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
)

// dataType represents the type of a HUML document structure.
type dataType int

const (
	typeScalar dataType = iota
	typeEmptyDict
	typeInlineDict
	typeMultilineDict
	typeEmptyList
	typeInlineList
	typeMultilineList
)

// Decoder reads and decodes HUML values from an input stream.
type Decoder struct {
	parser *streamParser
}

// NewDecoder returns a new decoder that reads from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		parser: newStreamParser(newLexer(r)),
	}
}

// Decode reads the HUML document from the input stream and stores the result in the pointer v.
func (dec *Decoder) Decode(v any) error {
	out, err := dec.parser.parse()
	if err != nil {
		return err
	}

	return setValue(v, out)
}

// Unmarshal parses HUML data and stores the result in the value pointed to by v.
// If v is nil or not a pointer, it returns an error.
//
// It converts HUML data into values with the following mappings:
//   - scalars (key: value) become primitive types:
//   - strings for quoted strings and multiline strings
//   - int64 for integers
//   - float64 for floating point numbers
//   - bool for true/false
//   - nil for null
//   - math.NaN() for nan
//   - math.Inf() for inf/+inf/-inf
//   - HUML vectors (key:: value) become []any for lists and map[string]any for dicts.
//   - HUML documents can become any of the above types, including nil.
//
// If the data contains a syntax error, a parser error is returned with line number.
func Unmarshal(data []byte, v any) error {
	if len(data) == 0 {
		return errors.New("empty document is undefined")
	}

	dec := NewDecoder(bytes.NewReader(data))
	return dec.Decode(v)
}

// setValue sets the destination value from the parsed source value.
func setValue(dst, src any) error {
	if dst == nil {
		return errors.New("cannot unmarshal into a nil value")
	}

	val := reflect.ValueOf(dst)
	if val.Kind() != reflect.Ptr {
		return errors.New("destination is not a pointer")
	}
	if val.IsNil() {
		return errors.New("destination pointer is nil")
	}

	d := val.Elem()
	return setValueReflect(d, src)
}

// setValueReflect recursively sets values to dst from src using reflection.
func setValueReflect(dst reflect.Value, src any) error {
	if src == nil {
		dst.Set(reflect.Zero(dst.Type()))
		return nil
	}

	s := reflect.ValueOf(src)

	// If the destination is an interface, set it directly.
	if dst.Kind() == reflect.Interface {
		if s.IsValid() {
			dst.Set(s)
		} else {
			dst.Set(reflect.Zero(dst.Type()))
		}
		return nil
	}

	// Assign directly if types are compatible.
	if s.IsValid() && s.Type().AssignableTo(dst.Type()) {
		dst.Set(s)
		return nil
	}

	// Handle type conversions.
	switch dst.Kind() {
	case reflect.Struct:
		return setStruct(dst, src)
	case reflect.Slice:
		return setSlice(dst, src)
	case reflect.Map:
		return setMap(dst, src)
	case reflect.Ptr:
		return setPtr(dst, src)
	case reflect.String:
		return setString(dst, src)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return setInt(dst, src)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return setUint(dst, src)
	case reflect.Float32, reflect.Float64:
		return setFloat(dst, src)
	case reflect.Bool:
		return setBool(dst, src)
	default:
		return fmt.Errorf("cannot unmarshal %T into %s", src, dst.Type())
	}
}

// setStruct unmarshals a map into a struct.
func setStruct(dst reflect.Value, src any) error {
	srcMap, ok := src.(map[string]any)
	if !ok {
		return fmt.Errorf("cannot unmarshal %T into struct", src)
	}

	structType := dst.Type()
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldValue := dst.Field(i)

		// Skip unexported fields.
		if !fieldValue.CanSet() {
			continue
		}

		// Get the field name for mapping.
		fieldName := getFieldName(field)
		if fieldName == "-" {
			continue
		}

		// Look for the value in the source map.
		if srcValue, exists := srcMap[fieldName]; exists {
			if err := setValueReflect(fieldValue, srcValue); err != nil {
				return fmt.Errorf("error setting field %s: %w", field.Name, err)
			}
		}
	}

	return nil
}

// getFieldName returns the field name to use for mapping, checking for struct tags.
func getFieldName(field reflect.StructField) string {
	name, _ := parseStructTag(field.Tag)
	if name == "" {
		return field.Name
	}
	return name
}

// setSlice unmarshals an array into a slice.
func setSlice(dst reflect.Value, src any) error {
	srcSlice, ok := src.([]any)
	if !ok {
		return fmt.Errorf("cannot unmarshal %T into slice", src)
	}

	sliceType := dst.Type()
	newSlice := reflect.MakeSlice(sliceType, len(srcSlice), len(srcSlice))

	for i, srcElem := range srcSlice {
		elemValue := newSlice.Index(i)
		if err := setValueReflect(elemValue, srcElem); err != nil {
			return fmt.Errorf("error setting slice element %d: %w", i, err)
		}
	}

	dst.Set(newSlice)
	return nil
}

// setMap unmarshals a src map into a dest map.
func setMap(dst reflect.Value, src any) error {
	srcMap, ok := src.(map[string]any)
	if !ok {
		return fmt.Errorf("cannot unmarshal %T into map", src)
	}

	mapType := dst.Type()
	keyType := mapType.Key()
	valueType := mapType.Elem()

	// Only support string keys for now (like JSON).
	if keyType.Kind() != reflect.String {
		return fmt.Errorf("maps with non-string keys are not supported")
	}

	newMap := reflect.MakeMap(mapType)
	for key, srcValue := range srcMap {
		keyValue := reflect.ValueOf(key)
		valueValue := reflect.New(valueType).Elem()

		if err := setValueReflect(valueValue, srcValue); err != nil {
			return fmt.Errorf("error setting map value for key %s: %w", key, err)
		}

		newMap.SetMapIndex(keyValue, valueValue)
	}

	dst.Set(newMap)
	return nil
}

// setPtr unmarshals into a pointer.
func setPtr(dst reflect.Value, src any) error {
	if src == nil {
		dst.Set(reflect.Zero(dst.Type()))
		return nil
	}

	elemType := dst.Type().Elem()
	newPtr := reflect.New(elemType)

	if err := setValueReflect(newPtr.Elem(), src); err != nil {
		return err
	}

	dst.Set(newPtr)
	return nil
}

// setString converts various types to string.
func setString(dst reflect.Value, src any) error {
	switch v := src.(type) {
	case string:
		dst.SetString(v)
		return nil
	default:
		return fmt.Errorf("cannot unmarshal %T into string", src)
	}
}

// setInt converts various numeric types to int.
func setInt(dst reflect.Value, src any) error {
	switch v := src.(type) {
	case int64:
		if dst.OverflowInt(v) {
			return fmt.Errorf("value %d overflows %s", v, dst.Type())
		}
		dst.SetInt(v)
		return nil
	case float64:
		// Convert float to int if it's a whole number.
		if v != math.Trunc(v) {
			return fmt.Errorf("cannot unmarshal float %g into integer type", v)
		}
		intVal := int64(v)
		if dst.OverflowInt(intVal) {
			return fmt.Errorf("value %g overflows %s", v, dst.Type())
		}
		dst.SetInt(intVal)
		return nil
	default:
		return fmt.Errorf("cannot unmarshal %T into integer", src)
	}
}

// setUint converts various numeric types to uint.
func setUint(dst reflect.Value, src any) error {
	switch v := src.(type) {
	case int64:
		if v < 0 {
			return fmt.Errorf("cannot unmarshal negative value %d into unsigned integer", v)
		}
		uintVal := uint64(v)
		if dst.OverflowUint(uintVal) {
			return fmt.Errorf("value %d overflows %s", v, dst.Type())
		}
		dst.SetUint(uintVal)
		return nil
	case float64:
		if v < 0 {
			return fmt.Errorf("cannot unmarshal negative value %g into unsigned integer", v)
		}
		if v != math.Trunc(v) {
			return fmt.Errorf("cannot unmarshal float %g into integer type", v)
		}
		uintVal := uint64(v)
		if dst.OverflowUint(uintVal) {
			return fmt.Errorf("value %g overflows %s", v, dst.Type())
		}
		dst.SetUint(uintVal)
		return nil
	default:
		return fmt.Errorf("cannot unmarshal %T into unsigned integer", src)
	}
}

// setFloat converts various numeric types to float.
func setFloat(dst reflect.Value, src any) error {
	switch v := src.(type) {
	case int64:
		floatVal := float64(v)
		if dst.OverflowFloat(floatVal) {
			return fmt.Errorf("value %d overflows %s", v, dst.Type())
		}
		dst.SetFloat(floatVal)
		return nil
	case float64:
		if dst.OverflowFloat(v) {
			return fmt.Errorf("value %g overflows %s", v, dst.Type())
		}
		dst.SetFloat(v)
		return nil
	default:
		return fmt.Errorf("cannot unmarshal %T into float", src)
	}
}

// setBool converts various types to bool.
func setBool(dst reflect.Value, src any) error {
	switch v := src.(type) {
	case bool:
		dst.SetBool(v)
		return nil
	default:
		return fmt.Errorf("cannot unmarshal %T into bool", src)
	}
}

// Helper functions for character classification.
func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isAlphaNum(c byte) bool {
	return isAlpha(c) || isDigit(c)
}

func isHex(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isOctal(c byte) bool {
	return c >= '0' && c <= '7'
}

func isBinary(c byte) bool {
	return c == '0' || c == '1'
}

func isSpaceBytes(b []byte) bool {
	for i := 0; i < len(b); i++ {
		if b[i] != ' ' {
			return false
		}
	}
	return true
}
