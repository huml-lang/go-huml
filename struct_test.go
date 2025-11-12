package huml

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Doc struct {
	FooFinal struct {
		FooFinalTest struct {
			BarEverything []any `huml:"bar_everything" json:"bar_everything"`
		} `huml:"foo_final_test" json:"foo_final_test"`
	} `huml:"foo_final" json:"foo_final"`

	FooOne struct {
		BarString string `huml:"bar_string" json:"bar_string"`
		BazInt    int    `huml:"baz_int" json:"baz_int"`
		CorgeBool bool   `huml:"corge_bool" json:"corge_bool"`
		FooFloats struct {
			BarSimple         float64 `huml:"bar_simple" json:"bar_simple"`
			BazNegative       float64 `huml:"baz_negative" json:"baz_negative"`
			CorgeZero         float64 `huml:"corge_zero" json:"corge_zero"`
			GarplyLargeExp    float64 `huml:"garply_large_exp" json:"garply_large_exp"`
			GraultPrecision   float64 `huml:"grault_precision" json:"grault_precision"`
			QuuxScientificNeg float64 `huml:"quux_scientific_neg" json:"quux_scientific_neg"`
			QuxScientific     float64 `huml:"qux_scientific" json:"qux_scientific"`
		} `huml:"foo_floats" json:"foo_floats"`
		FooIntegers struct {
			BarPositive    int64  `huml:"bar_positive" json:"bar_positive"`
			BazNegative    int64  `huml:"baz_negative" json:"baz_negative"`
			CorgeHex       uint32 `huml:"corge_hex" json:"corge_hex"`
			GarplyBinary   int    `huml:"garply_binary" json:"garply_binary"`
			GraultOctal    int    `huml:"grault_octal" json:"grault_octal"`
			QuuxUnderscore int    `huml:"quux_underscore" json:"quux_underscore"`
			QuxZero        int    `huml:"qux_zero" json:"qux_zero"`
			WaldoLarge     int64  `huml:"waldo_large" json:"waldo_large"`
		} `huml:"foo_integers" json:"foo_integers"`
		FooString  string `huml:"foo_string" json:"foo_string"`
		FooStrings struct {
			BarEmpty       string `huml:"bar_empty" json:"bar_empty"`
			BazSpaces      string `huml:"baz_spaces" json:"baz_spaces"`
			CorgeUnicode   string `huml:"corge_unicode" json:"corge_unicode"`
			GarplyLong     string `huml:"garply_long" json:"garply_long"`
			GraultNewlines string `huml:"grault_newlines" json:"grault_newlines"`
			QuuxPath       string `huml:"quux_path" json:"quux_path"`
			QuxEscaped     string `huml:"qux_escaped" json:"qux_escaped"`
		} `huml:"foo_strings" json:"foo_strings"`
		GraultNull any     `huml:"grault_null" json:"grault_null"`
		QuuxBool   bool    `huml:"quux_bool" json:"quux_bool"`
		QuxFloat   float64 `huml:"qux_float" json:"qux_float"`
	} `huml:"foo_one" json:"foo_one"`

	FooTwo struct {
		BarComments     string         `huml:"bar_comments" json:"bar_comments"`
		BarEmptyDict    map[string]any `huml:"bar_empty_dict" json:"bar_empty_dict"`
		BarInlineList   []string       `huml:"bar_inline_list" json:"bar_inline_list"`
		BarMixedList    []any          `huml:"bar_mixed_list" json:"bar_mixed_list"`
		BazComments     string         `huml:"baz_comments" json:"baz_comments"`
		BazEmptySpaced  []any          `huml:"baz_empty_spaced" json:"baz_empty_spaced"`
		BazInlineList   []any          `huml:"baz_inline_list" json:"baz_inline_list"`
		CorgeInlineDict struct {
			Nested string `huml:"nested" json:"nested"`
			Simple string `huml:"simple" json:"simple"`
		} `huml:"corge_inline_dict" json:"corge_inline_dict"`
		FooComments    string `huml:"foo_comments" json:"foo_comments"`
		FooComplexList []struct {
			BarType    string         `huml:"bar_type" json:"bar_type"`
			BazValue   int            `huml:"baz_value,omitempty" json:"baz_value,omitempty"`
			BazEmpty   map[string]any `huml:"baz_empty" json:"baz_empty"`
			Bar        any            `huml:"bar,omitempty" json:"bar,omitempty"`
			Foo        any            `huml:"foo,omitempty" json:"foo,omitempty"`
			InlineDict any            `huml:"inline_dict,omitempty" json:"inline_dict,omitempty"`
			QuxFlag    bool           `huml:"qux_flag,omitempty" json:"qux_flag,omitempty"`
			QuxNull    any            `huml:"qux_null,omitempty" json:"qux_null,omitempty"`
			QuxNested  struct {
				CorgeList []string `huml:"corge_list" json:"corge_list"`
				QuuxInner string   `huml:"quux_inner" json:"quux_inner"`
			} `huml:"qux_nested,omitempty" json:"qux_nested,omitempty"`
		} `huml:"foo_complex_list" json:"foo_complex_list"`
		FooDict struct {
			BarKey    string `huml:"bar_key" json:"bar_key"`
			BazKey    int    `huml:"baz_key" json:"baz_key"`
			QuxNested struct {
				CorgeSub   bool `huml:"corge_sub" json:"corge_sub"`
				GraultDeep struct {
					GarplyDeeper string `huml:"garply_deeper" json:"garply_deeper"`
					WaldoNumbers []int  `huml:"waldo_numbers" json:"waldo_numbers"`
				} `huml:"grault_deep" json:"grault_deep"`
				QuuxSub string `huml:"quux_sub" json:"quux_sub"`
			} `huml:"qux_nested" json:"qux_nested"`
		} `huml:"foo_dict" json:"foo_dict"`
		FooEmptyList   []any          `huml:"foo_empty_list" json:"foo_empty_list"`
		FooInlineList  []int          `huml:"foo_inline_list" json:"foo_inline_list"`
		FooList        []any          `huml:"foo_list" json:"foo_list"`
		FooSpecialKeys map[string]any `huml:"foo_special_keys" json:"foo_special_keys"`
		QuuxComments   string         `huml:"quux_comments" json:"quux_comments"`
		QuuxInlineDict struct {
			Baz int    `huml:"baz" json:"baz"`
			Foo string `huml:"foo" json:"foo"`
			Qux bool   `huml:"qux" json:"qux"`
		} `huml:"quux_inline_dict" json:"quux_inline_dict"`
		QuxComments    string         `huml:"qux_comments" json:"qux_comments"`
		QuxEmptySpaced map[string]any `huml:"qux_empty_spaced" json:"qux_empty_spaced"`
		QuxInlineList  []any          `huml:"qux_inline_list" json:"qux_inline_list"`
	} `huml:"foo_two" json:"foo_two"`

	FooThree struct {
		BarLargeInline struct {
			A int    `huml:"a" json:"a"`
			B int    `huml:"b" json:"b"`
			C int    `huml:"c" json:"c"`
			D int    `huml:"d" json:"d"`
			E int    `huml:"e" json:"e"`
			F string `huml:"f" json:"f"`
			G bool   `huml:"g" json:"g"`
			H any    `huml:"h" json:"h"`
		} `huml:"bar_large_inline" json:"bar_large_inline"`
		BarMultilineStripped string `huml:"bar_multiline_stripped" json:"bar_multiline_stripped"`
		BazMultilineEdge     string `huml:"baz_multiline_edge" json:"baz_multiline_edge"`
		FooBooleans          struct {
			BarTrue     bool `huml:"bar_true" json:"bar_true"`
			BazFalse    bool `huml:"baz_false" json:"baz_false"`
			CorgeTrue   bool `huml:"corge_True" json:"corge_True"`
			GraultFalse bool `huml:"grault_False" json:"grault_False"`
			QuuxFalse   bool `huml:"quux_FALSE" json:"quux_FALSE"`
			QuxTrue     bool `huml:"qux_TRUE" json:"qux_TRUE"`
		} `huml:"foo_booleans" json:"foo_booleans"`
		FooComplexNesting struct {
			BarLevel1 struct {
				BazLevel2 struct {
					QuxLevel3 struct {
						QuuxLevel4 struct {
							CorgeDeepValue   string `huml:"corge_deep_value" json:"corge_deep_value"`
							GarplyDeepInline struct {
								Deep string `huml:"deep" json:"deep"`
								Dict bool   `huml:"dict" json:"dict"`
							} `huml:"garply_deep_inline" json:"garply_deep_inline"`
							GraultDeepList []any `huml:"grault_deep_list" json:"grault_deep_list"`
						} `huml:"quux_level4" json:"quux_level4"`
					} `huml:"qux_level3" json:"qux_level3"`
				} `huml:"baz_level2" json:"baz_level2"`
			} `huml:"bar_level1" json:"bar_level1"`
		} `huml:"foo_complex_nesting" json:"foo_complex_nesting"`
		FooEdgeCases      map[string]string `huml:"foo_edge_cases" json:"foo_edge_cases"`
		FooLargeInline    []any             `huml:"foo_large_inline" json:"foo_large_inline"`
		FooMixedStructure struct {
			BarInlineInMulti struct {
				Quick string `huml:"quick" json:"quick"`
			} `huml:"bar_inline_in_multi" json:"bar_inline_in_multi"`
			BazMultiList []any `huml:"baz_multi_list" json:"baz_multi_list"`
		} `huml:"foo_mixed_structure" json:"foo_mixed_structure"`
		FooMultilinePreserved string `huml:"foo_multiline_preserved" json:"foo_multiline_preserved"`
		FooNulls              struct {
			BarNull any `huml:"bar_null" json:"bar_null"`
			BazNULL any `huml:"baz_NULL" json:"baz_NULL"`
			QuxNull any `huml:"qux_Null" json:"qux_Null"`
		} `huml:"foo_nulls" json:"foo_nulls"`
	} `huml:"foo_three" json:"foo_three"`
}

func TestStruct(t *testing.T) {
	// Scan HUML to struct.
	var resHuml Doc
	b, err := os.ReadFile("tests/documents/mixed.huml")
	if err != nil {
		t.Fatalf("failed to read tests/documents/mixed.huml: %v", err)
	}
	if err := Unmarshal(b, &resHuml); err != nil {
		t.Fatalf("failed to unmarshal tests/documents/mixed.huml: %v", err)
	}

	// Convert it to JSON and back to struct so that the int/float
	// conversions are handled correctly.
	jsonConverted, err := json.Marshal(resHuml)
	if err != nil {
		t.Fatalf("failed to marshal to JSON: %v", err)
	}

	// Convert JSON back to struct.
	var resJsonConverted Doc
	if err := json.Unmarshal(jsonConverted, &resJsonConverted); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	// Read JSON file.
	var resJson Doc
	b, err = os.ReadFile("tests/documents/mixed.json")
	if err != nil {
		t.Fatalf("failed to read tests/documents/mixed.json: %v", err)
	}
	if err := json.Unmarshal(b, &resJson); err != nil {
		t.Fatalf("failed to unmarshal tests/documents/mixed.json: %v", err)
	}

	// Deep-compare both.
	assert.Equal(t, resJson, resJsonConverted, "tests/documents/mixed.huml and tests/documents/mixed.json should be deeply equal")
}

// TestStructTags tests the struct tag functionality including renaming, omitempty, and skipping.
func TestStructTags(t *testing.T) {
	t.Run("field_renaming", func(t *testing.T) {
		type TestStruct struct {
			FieldName    string `huml:"custom_name"`
			AnotherField int    `huml:"another_custom"`
		}

		data := TestStruct{
			FieldName:    "value1",
			AnotherField: 42,
		}

		marshalled, err := Marshal(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		humlStr := string(marshalled)
		// Check that custom names are used
		assert.Contains(t, humlStr, "custom_name")
		assert.Contains(t, humlStr, "another_custom")
		// Check that original names are NOT used
		assert.NotContains(t, humlStr, "FieldName")
		assert.NotContains(t, humlStr, "AnotherField")
	})

	t.Run("omitempty_with_zero_values", func(t *testing.T) {
		type TestStruct struct {
			IncludedString string `huml:"included_string"`
			OmittedString  string `huml:"omitted_string,omitempty"`
			OmittedInt     int    `huml:"omitted_int,omitempty"`
			OmittedBool    bool   `huml:"omitted_bool,omitempty"`
			IncludedInt    int    `huml:"included_int"`
			IncludedBool   bool   `huml:"included_bool"`
		}

		data := TestStruct{
			IncludedString: "present",
			OmittedString:  "",    // empty - should be omitted
			OmittedInt:     0,     // zero - should be omitted
			OmittedBool:    false, // false - should be omitted
			IncludedInt:    0,     // zero but no omitempty - should be included
			IncludedBool:   false, // false but no omitempty - should be included
		}

		marshalled, err := Marshal(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		humlStr := string(marshalled)
		// Check that included fields are present
		assert.Contains(t, humlStr, "included_string")
		assert.Contains(t, humlStr, "included_int")
		assert.Contains(t, humlStr, "included_bool")
		// Check that omitted fields are NOT present
		assert.NotContains(t, humlStr, "omitted_string")
		assert.NotContains(t, humlStr, "omitted_int")
		assert.NotContains(t, humlStr, "omitted_bool")
	})

	t.Run("omitempty_with_non_zero_values", func(t *testing.T) {
		type TestStruct struct {
			IncludedString string `huml:"included_string,omitempty"`
			IncludedInt    int    `huml:"included_int,omitempty"`
			IncludedBool   bool   `huml:"included_bool,omitempty"`
		}

		data := TestStruct{
			IncludedString: "present",
			IncludedInt:    42,
			IncludedBool:   true,
		}

		marshalled, err := Marshal(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		humlStr := string(marshalled)
		// All fields have non-zero values, so they should all be included
		assert.Contains(t, humlStr, "included_string")
		assert.Contains(t, humlStr, "included_int")
		assert.Contains(t, humlStr, "included_bool")
	})

	t.Run("skip_field_with_dash", func(t *testing.T) {
		type TestStruct struct {
			IncludedField string `huml:"included"`
			SkippedField  string `huml:"-"`
			AnotherField  int    `huml:"another"`
		}

		data := TestStruct{
			IncludedField: "value1",
			SkippedField:  "should not appear",
			AnotherField:  42,
		}

		marshalled, err := Marshal(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		humlStr := string(marshalled)
		// Check that included fields are present
		assert.Contains(t, humlStr, "included")
		assert.Contains(t, humlStr, "another")
		// Check that skipped field is NOT present
		assert.NotContains(t, humlStr, "SkippedField")
		assert.NotContains(t, humlStr, "should not appear")
	})

	t.Run("omitempty_with_slices_and_maps", func(t *testing.T) {
		type TestStruct struct {
			EmptySlice    []string          `huml:"empty_slice,omitempty"`
			NonEmptySlice []string          `huml:"non_empty_slice,omitempty"`
			EmptyMap      map[string]string `huml:"empty_map,omitempty"`
			NonEmptyMap   map[string]string `huml:"non_empty_map,omitempty"`
		}

		data := TestStruct{
			EmptySlice:    []string{},
			NonEmptySlice: []string{"item1", "item2"},
			EmptyMap:      map[string]string{},
			NonEmptyMap:   map[string]string{"key": "value"},
		}

		marshalled, err := Marshal(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		humlStr := string(marshalled)
		// Empty collections should be omitted (check for exact field names as keys)
		// Use word boundary to avoid matching substrings like "non_empty_slice"
		assert.NotRegexp(t, `(^|\n)\s*empty_slice\s*::`, humlStr)
		assert.NotRegexp(t, `(^|\n)\s*empty_map\s*::`, humlStr)
		// Non-empty collections should be included
		assert.Contains(t, humlStr, "non_empty_slice")
		assert.Contains(t, humlStr, "non_empty_map")
	})

	t.Run("omitempty_with_pointers", func(t *testing.T) {
		type TestStruct struct {
			NilPtr       *string `huml:"nil_ptr,omitempty"`
			NonNilPtr    *string `huml:"non_nil_ptr,omitempty"`
			NilPtrNoOmit *string `huml:"nil_ptr_no_omit"`
		}

		strValue := "value"
		data := TestStruct{
			NilPtr:       nil,
			NonNilPtr:    &strValue,
			NilPtrNoOmit: nil,
		}

		marshalled, err := Marshal(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		humlStr := string(marshalled)
		// Nil pointer with omitempty should be omitted (check for exact field name as key)
		assert.NotRegexp(t, `(^|\n)\s*nil_ptr\s*:`, humlStr)
		// Non-nil pointer should be included
		assert.Contains(t, humlStr, "non_nil_ptr")
		// Nil pointer without omitempty should be included as null
		assert.Contains(t, humlStr, "nil_ptr_no_omit")
		assert.Contains(t, humlStr, "null")
	})

	t.Run("omitempty_with_nested_structs", func(t *testing.T) {
		type Nested struct {
			Value string `huml:"value"`
		}
		type TestStruct struct {
			EmptyNested    Nested `huml:"empty_nested,omitempty"`
			NonEmptyNested Nested `huml:"non_empty_nested,omitempty"`
		}

		data := TestStruct{
			EmptyNested:    Nested{Value: ""},
			NonEmptyNested: Nested{Value: "present"},
		}

		marshalled, err := Marshal(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		humlStr := string(marshalled)
		// Empty nested struct should be omitted (check for exact field name as key)
		assert.NotRegexp(t, `(^|\n)\s*empty_nested\s*::`, humlStr)
		// Non-empty nested struct should be included
		assert.Contains(t, humlStr, "non_empty_nested")
		assert.Contains(t, humlStr, "present")
	})

	t.Run("decode_with_renamed_fields", func(t *testing.T) {
		type TestStruct struct {
			FieldName    string `huml:"custom_name"`
			AnotherField int    `huml:"another_custom"`
		}

		humlData := `custom_name: "value1"
another_custom: 42`

		var result TestStruct
		err := Unmarshal([]byte(humlData), &result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assert.Equal(t, "value1", result.FieldName)
		assert.Equal(t, 42, result.AnotherField)
	})

	t.Run("decode_with_skipped_fields", func(t *testing.T) {
		type TestStruct struct {
			IncludedField string `huml:"included"`
			SkippedField  string `huml:"-"`
		}

		// HUML data doesn't have the skipped field, which is fine
		humlData := `included: "value"`

		var result TestStruct
		err := Unmarshal([]byte(humlData), &result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assert.Equal(t, "value", result.IncludedField)
		assert.Equal(t, "", result.SkippedField) // Should remain zero value
	})

	t.Run("round_trip_with_tags", func(t *testing.T) {
		type TestStruct struct {
			RenamedField string `huml:"renamed"`
			OmitEmpty    int    `huml:"omit_empty,omitempty"`
			Skipped      string `huml:"-"`
			NormalField  string `huml:"normal"`
		}

		original := TestStruct{
			RenamedField: "value1",
			OmitEmpty:    0, // Will be omitted
			Skipped:      "should not appear",
			NormalField:  "value2",
		}

		// Marshal
		marshalled, err := Marshal(original)
		if err != nil {
			t.Fatalf("unexpected error marshalling: %v", err)
		}

		// Unmarshal back
		var result TestStruct
		err = Unmarshal(marshalled, &result)
		if err != nil {
			t.Fatalf("unexpected error unmarshalling: %v", err)
		}

		// Verify round trip
		assert.Equal(t, original.RenamedField, result.RenamedField)
		assert.Equal(t, original.NormalField, result.NormalField)
		assert.Equal(t, 0, result.OmitEmpty) // Should remain zero since it was omitted
		assert.Equal(t, "", result.Skipped)  // Should remain zero since it was skipped
	})
}
