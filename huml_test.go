package main

import (
	"embed"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	//go:embed test.huml
	efs embed.FS
)

func TestParsing(t *testing.T) {
	f := func(name, input string, errorExpected bool) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			t.Helper()
			var result any
			err := Unmarshal([]byte(input), &result)
			if errorExpected && err == nil {
				t.Errorf("expected error but got none")
			}
			if !errorExpected && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}

	f("empty_input", "", false)
	f("whitespace_only", "   \n  \n  ", true)
	f("comments_only", "# comment\n# another comment", false)
	f("version_directive_with_comment", "%HUML 1.0 # version comment", true)
	f("duplicate_key_error", "key: 1\nkey: 2", true)
	f("invalid_char_after_plus", "key: +abc", true)
	f("invalid_char_after_minus", "key: -abc", true)
	f("string_with_newline", "key: \"line1\nline2\"", true)
	f("trailing_spaces_eof", "key: value   ", true)
	f("comment_trailing_spaces", "# comment ", true)
	f("blank_line_trailing_spaces", "key: value\n ", true)
	f("space_before_comma", "key:: a: 1 , b: 2", true)

	// Basic scalar values.
	f("unquoted_string", "key: value", true)
	f("null_value", "key: null", false)
	f("bool_true", "key: true", false)
	f("bool_false", "key: false", false)
	f("integer", "key: 123", false)
	f("negative_integer", "key: -123", false)
	f("positive_integer", "key: +123", false)
	f("integer_with_underscores", "key: 1_234_567", false)
	f("float", "key: 123.456", false)
	f("negative_float", "key: -123.456", false)
	f("scientific_notation", "key: 6.022e23", false)
	f("scientific_notation_with_sign", "key: 1.5e-10", false)
	f("hex_number", "key: 0xCAFEBABE", false)
	f("octal_number", "key: 0o755", false)
	f("binary_number", "key: 0b11011001", false)
	f("nan_value", "key: nan", false)
	f("positive_infinity", "key: inf", false)
	f("negative_infinity", "key: -inf", false)
	f("positive_infinity_with_sign", "key: +inf", false)

	// Strings.
	f("quoted_string", `key: "hello world"`, false)
	f("quoted_string_with_escapes", `key: "hello \"world\""`, false)
	f("quoted_string_with_unicode", `key: "hello \u0041 world"`, false)
	f("quoted_string_with_all_escapes", `key: "test\n\t\r\b\f\\\/\""`, false)
	f("multiline_string_backticks", "key: ```\nline1\nline2\n```", false)
	f("multiline_string_triple_quotes", "key: \"\"\"\nline1\nline2\n\"\"\"", false)

	// Quoted keys
	f("quoted_key", `"quoted-key": "value"`, false)
	f("quoted_key_with_space", `"quoted key": "value"`, false)
	f("quoted_key_invalid_value", `"quoted-key": value`, true)
	f("quoted_key_special_chars", `"legacy-system.compatibility_mode": true`, false)

	// Comments with values.
	f("value_with_comment", `key: "value" # comment`, false)
	f("invalid_value_with_comment", `key: value # comment`, true)
	f("key_with_comment_after_colon", "key: # comment only", true)

	// Empty values.
	f("empty_value", "key:", true)
	f("empty_value", "key: ", true)
	f("empty_value", "key:  ", true)
	f("empty_value_with_comment", "key:# comment", true)
	f("empty_value_with_comment", "key: # comment", true)

	// Root list cases.
	f("root_empty_list_colon", "::", true)
	f("root_empty_list", ":: []", true)
	f("root_empty_dict", ":: {}", true)
	f("root_inline_list", ":: 1, 2, 3", true)
	f("root_inline_list_other_key", "1, 2, 3\nfoo: 1", true)
	f("root_multiline_list", "::\n  - \"item1\"\n  - \"item2\"", true)
	f("root_multiline_unquoted_values", "::\n  - item1\n  - item2", true)
	f("root_multiline_bad_indentation", "::\n - item1\n  - item2", true)

	f("root_list", ":: []", true)
	f("root_dict", ":: {}", true)
	f("root_dict_comment", "# test\n:: {}", true)
	f("root_scalar", "123", false)
	f("root_scalar", "", false)
	f("root_scalar", "\n\"test\"", false)
	f("root_scalar_with_extra_content", "123\nextra content", true)
	f("root_scalar_with_comment_extra", "\"test\" # comment\nextra", true)
	f("root_scalar_blank_then_extra", "true\n\nextra content", true)

	f("root_invalid", " ::", true)
	f("root_empty_list", "\n\n[]\n\n", false)
	f("root_empty_dict", "\n{}", false)
	f("root_inline_dict", "a: 1, b: 2, c: \"val\"", false)
	f("root_inline_dict_invalid", " a: 1, b: 2, c: \"val\"", true)
	f("root_inline_dict_invalid", "a: 1, b: 2\nc: 3\nd: 4", true)
	f("root_invalid_list", "   []", true)
	f("root_invalid_dict", "   {}", true)
	f("root_invalid_list", "  ::[]", true)
	f("root_invalid_dict", "  ::{}", true)
	f("root_invalid_dict_comment", "# test\n  ::{}", true)
	f("root_invalid_scalar", " 123", true)
	f("root_invalid_scalar", " true", true)
	f("root_invalid_scalar", "\n \"test\"", true)

	// dict cases.
	f("simple_dict", "dict::\n  key1: \"value1\"\n  key2: \"value2\"", false)
	f("simple_dict_unquoted_values", `dict::\n  key1: value1\n  key2: value2`, true)
	f("nested_dict", "outer::\n  inner::\n    key: 123", false)
	f("dict_invalid_char_start", "dict::\n  @invalid: value", true)
	f("dict_value_parse_error", "dict::\n  key: +", true)
	f("empty_dict", "dict:: {}", false)
	f("ambiguous_empty_vector_space", "key:: # comment", true)
	f("inline_dict", `dict:: key1: "value1", key2: "value2"`, false)
	f("inline_vector_empty_list_error", "key:: []trailing", true)
	f("inline_vector_empty_dict_error", "key:: {}trailing", true)
	f("inline_list_comma_error", "key:: 1 2", true)
	f("inline_dict_comma_error", "key:: a: 1 b: 2", true)
	f("inline_dict_missing_colon", "key:: a b", true)
	f("inline_dict_no_space_after_colon", "key:: a:1", true)
	f("inline_dict_value_parse_error", "key:: a: +", true)
	f("inline_dict_trailing_comma_error", "key:: a: 1,trailing", true)

	// List cases.
	f("simple_list", "list::\n  - \"item1\"\n  - \"item2\"", false)
	f("nested_list", "list::\n  - \"item1\"\n  - ::\n    - \"nested1\"\n    - \"nested2\"", false)
	f("empty_list", "list:: []", false)
	f("inline_list", "list:: 1, 2, 3, 4", false)
	f("mixed_inline_list", `list:: 1, "string", true, null`, false)
	f("list_no_space_after_dash", "list::\n  -item", true)
	f("list_bad_indent_item", "list::\n    - item", true)
	f("list_not_list_continuation", "list::\n  - item1\n  \"key\": \"value\"", true)

	// List with dicts.
	f("list_with_inline_dicts", `
contacts::
  - :: type: "admin", email: "admin@example.com", bool: true, empty: null
  - :: type: "support", email: "support@example.com", bool: true, empty: null`, false)
	f("list_with_multiline_dicts", `
contacts::
  - ::
      str: "admin"
      num: 1234
  - ::
      str: "admin2"
      num: 45.67`, false)

	// Complex nested structures.
	f("complex_nested_dict", "config::\n  database::\n    host: \"localhost\"\n    port: 5432\n    credentials::\n      username: \"admin\"\n      password: \"secret\"\n      enabled: true\n    features::\n      - 123\n      - \"reporting\"\n      - true\n", false)

	// Trailing spaces.
	f("trailing_spaces_after_value", "key: value ", true)
	f("trailing_spaces_after_comment", "key: value # comment ", true)
	f("trailing_spaces_on_empty_line", "key: value\n \nother: value", true)
	f("trailing_spaces_after_colon_no_value", "key: ", true)
	f("trailing_spaces_after_double_colon", "key:: ", true)
	f("trailing_spaces_after_inline_dict", "key:: foo: 1, bar: \"two\" ", true)
	f("trailing_spaces_after_inline_dict", "key:: foo: 1, bar: \"two\"   ", true)
	f("trailing_spaces_after_inline_dict", "key:: foo: 1, bar: \"two\", baz: true   ", true)
	f("trailing_spaces_after_inline_list", "key:: 1, 2, true, \"two\"  ", true)
	f("trailing_spaces_after_inline_list", "key:: 1, 2, true, \"two\" ", true)
	f("trailing_spaces_after_inline_list", "key:: 1, 2, true, \"two\"   ", true)
	f("trailing_spaces_after_inline_list", "key:: 1, 2, true, \"two\", true   ", true)

	// Spacing around colons.
	f("no_space_after_colon", "key:value", true)
	f("multiple_spaces_after_colon", "key:  value", true)
	f("no_space_after_double_colon", "key::value", true)
	f("multiple_spaces_after_double_colon", "key::  value", true)
	f("space_before_colon", "key : value", true)

	// Comma spacing.
	f("space_before_comma_in_list", "list:: 1 , 2, 3", true)
	f("no_space_after_comma_in_list", "list:: 1,2,3", true)
	f("multiple_spaces_after_comma", "list:: 1,  2, 3", true)
	f("space_before_comma_in_dict", "dict:: key1: value1 , key2: value2", true)

	// Comment formatting.
	f("comment_without_space_before", "key: value#comment", true)
	f("comment_without_space_after_hash", "key: value #comment", true)
	f("comment_line_without_space_after_hash", "#comment", true)
	f("comment_no_space_before_value", "key:\"value\"# comment", true)
	f("comment_hash_no_space", "key: \"value\"#comment", true)
	f("comment_hash_no_following_space", "key: \"value\" #comment", true)
	f("double_colon_followed_by_comment_no_space", "key::#comment", true)

	// Indentation.
	f("bad_indentation_too_much", "dict::\n    key: value", true)
	f("bad_indentation_too_little", "dict::\nkey: value", true)
	f("inconsistent_indentation", "dict::\n  key1: value1\n    key2: value2", true)
	f("list_item_bad_indentation", "list::\n  - item1\n    - item2", true)

	// Invalid syntax.
	f("missing_colon", "key value", true)
	f("invalid_character", "key: @invalid", true)
	f("unclosed_quoted_string", `key: "unclosed string`, true)
	f("invalid_escape_sequence", `key: "invalid \x escape"`, true)
	f("incomplete_escape", `key: "incomplete \"`, true)
	f("invalid_unicode_escape", `key: "invalid \uXXXX"`, true)
	f("incomplete_unicode_escape", `key: "incomplete \u12"`, true)
	f("invalid_lines", "root::\n  key: \"value\"\n:something", true)
	f("invalid_lines", "root::\n  key: \"value\"\n:bad\n  another: 123", true)
	f("invalid_lines", "root::\n  key: \"value\"\n::something else", true)
	f("invalid_lines", "root::\n  key: \"value\"\n :something else", true)
	f("invalid_lines", "root::\n  key: \"value\"\n ::something else", true)

	// Numbers.
	f("invalid_hex_number", "key: 0xGHI", true)
	f("invalid_octal_number", "key: 0o789", true)
	f("invalid_binary_number", "key: 0b123", true)

	// Multiline strings.
	f("unclosed_multiline_string", "key: ```\nline1\nline2", true)
	f("unclosed_triple_quoted_string", "key: \"\"\"\nline1\nline2", true)
	f("multiline_string_preserved_indentation", "poem: ```\n      First line\n           Second\n        Third Line\n```", false)
	f("multiline_string_stripped_indentation", "script: \"\"\"\n          First line\n          Second line\n          Third line\n\"\"\"", false)
	f("multiline_string_wrong_indent_backticks", "key::\n    a: ```\n      First line\n           Second\n        Third Line\n", true)
	f("multiline_string_wrong_indent_quotes", "key::\n    b: \"\"\"\n          First line\n          Second line\n          Third line\n", true)
	f("multiline_string_delimiter_without_newline", "key: ```content```", true)
	f("multiline_wrong_closing_indent", "key: ```\n  content\n    ```", true)
	f("multiline_invalid_after_delimiter", "key: ```\n  content\n``` extra", true)

	// List formatting.
	f("list_item_without_dash", "list::\n  item1\n  item2", true)
	f("dash_without_space", "list::\n-item1\n-item2", true)

	// Empty structures.
	f("empty_dict_again", "dict:: {}\n", false)
	f("empty_list_again", "list:: []\n", false)

	// Special values in lists.
	f("list_with_special_values", "values:: null, true, false, nan, inf, -inf", false)

	// Mixed structures.
	// {"mixed_inline_structures", "mixed:: {}, [], key: value", false})

	// Complex real-world like example.
	b, err := efs.ReadFile("test.huml")
	if err != nil {
		t.Fatalf("failed to read test.huml: %v", err)
	}

	f("complex_input", string(b), false)

	// Test the specific bad case mentioned in the issue
	badMultilineCase := `key::
    a: ` + "```" + `
      First line
           Second
        Third Line

    b: """
          First line
          Second line
          Third line`

	f("multiline_string_issue_case", badMultilineCase, true)

	// Test multiline strings in list items with wrong indentation
	badListMultilineCase := `items::
  - ` + "```" + `
      line1
      line2
`

	f("multiline_string_in_list_wrong_indent", badListMultilineCase, true)
}

func TestValues(t *testing.T) {
	f := func(name, input string, expectedVal any) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			t.Helper()
			var result any
			if err := Unmarshal([]byte(input), &result); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(result, expectedVal) {
				t.Errorf("expected %+v, got %+v", expectedVal, result)
			}
		})
	}

	f("null_value", "key: null", map[string]any{"key": nil})
	f("boolean_values", "t: true\nf: false", map[string]any{"t": true, "f": false})
	f("integer", "num: 42", map[string]any{"num": int64(42)})
	f("float", "num: 3.14", map[string]any{"num": 3.14})
	f("string", `str: "hello"`, map[string]any{"str": "hello"})
	f("empty_list", "list:: []", map[string]any{"list": []any{}})
	f("empty_dict", "dict:: {}", map[string]any{"dict": map[string]any{}})
	f("inline_list", "list:: 1, 2, 3", map[string]any{"list": []any{int64(1), int64(2), int64(3)}})
	f("root_list", "1, 2, 3, 5.6, +4, -2", []any{int64(1), int64(2), int64(3), float64(5.6), int64(4), int64(-2)})

	// Test special numeric values
	t.Run("special_numbers", func(t *testing.T) {
		var result any
		err := Unmarshal([]byte("nan_val: nan\ninf_val: inf\nneginf_val: -inf"), &result)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		m := result.(map[string]any)
		if !math.IsNaN(m["nan_val"].(float64)) {
			t.Error("Expected NaN")
		}
		if !math.IsInf(m["inf_val"].(float64), 1) {
			t.Error("Expected +Inf")
		}
		if !math.IsInf(m["neginf_val"].(float64), -1) {
			t.Error("Expected -Inf")
		}
	})
}

// TestSetValueErrors tests error conditions in setValue function
func TestSetValue(t *testing.T) {
	f := func(name string, dst any, val any, errExpected bool, expectedVal any) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			t.Helper()
			err := setValue(dst, val)
			if errExpected {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if dst != nil && !errExpected {
					switch v := dst.(type) {
					case *any:
						if *v != expectedVal {
							t.Errorf("expected %v, got %v", expectedVal, *v)
						}
					}
				}
			}
		})
	}

	var s *string
	f("nil_destination", nil, "test", true, nil)
	f("non_pointer_destination", "", "test", true, nil)
	f("nil_pointer_destination", s, "test", true, nil)
	f("incompatible_types", new(int), "string_value", true, nil)
	f("interface_assignment", new(any), "test", false, "test")
	f("interface_nil_assignment", new(any), nil, false, nil)
}

func FuzzParsing(f *testing.F) {
	inputs := []string{
		"",
		"   \n  \n  ",
		"# comment\n# another comment",
		"%HUML 1.0 # version comment",
		"key: value",
		"key: null",
		"key: true",
		"key: false",
		"key: 123",
		"key: -123",
		"key: +123",
		"key: 1_234_567",
		"key: 123.456",
		"key: -123.456",
		"key: 6.022e23",
		"key: 1.5e-10",
		"key: 0xCAFEBABE",
		"key: 0o755",
		"key: 0b11011001",
		"key: nan",
		"key: inf",
		"key: -inf",
		"key: +inf",
		"key: \"hello world\"",
		"key: \"hello \"world\"",
		"key: \"hello \u0041 world\"",
		"key: \"test\n\t\r\b\f\\|",
		"key: ```\nline1\nline2\n```",
		"key: \"\"\"\nline1\nline2\n\"\"\"",
		"\"quoted-key\": \"value\"",
		"\"quoted key\": \"value\"",
		"\"quoted-key\": value",
		"\"legacy-system.compatibility_mode\": true",
		"key: \"value\" # comment",
		"key: value # comment",
		"key: # comment only",
		"key:",
		"key: ",
		"key:  ",
		"key:# comment",
		"key: # comment",
		"::",
		":: []",
		":: {}",
		":: 1, 2, 3",
		"::\n  - \"item1\"\n  - \"item2\"",
		"::\n  - item1\n  - item2",
		"::\n - item1\n  - item2",
		"dict::\n  key1: \"value1\"\n  key2: \"value2\"",
		"dict::\n  key1: value1\n  key2: value2",
		"outer::\n  inner::\n    key: 123",
		"dict:: {}",
		"dict:: key1: \"value1\", key2: \"value2\"",
		"list::\n  - \"item1\"\n  - \"item2\"",
		"list::\n  - \"item1\"\n  - ::\n    - \"nested1\"\n    - \"nested2\"",
		"list:: []",
		"list:: 1, 2, 3, 4",
		"list:: 1, \"string\", true, null",
		"config::\n  database::\n    host: \"localhost\"\n    port: 5432\n    credentials::\n      username: \"admin\"\n      password: \"secret\"\n      enabled: true\n    features::\n      - 123\n      - \"reporting\"\n      - true\n",
		"key: value ",
		"key: value # comment ",
		"key: value\n \nother: value",
		"key: ",
		"key:: ",
		"key:value",
		"key:  value",
		"key::value",
		"key::  value",
		"key : value",
		"list:: 1 , 2, 3",
		"list:: 1,2,3",
		"list:: 1,  2, 3",
		"dict:: key1: value1 , key2: value2",
		"key: value#comment",
		"key: value #comment",
		"#comment",
		"key::#comment",
		"dict::\n    key: value",
		"dict::\nkey: value",
		"dict::\n  key1: value1\n    key2: value2",
		"list::\n  - item1\n    - item2",
		"key value",
		"key: @invalid",
		"key: \"unclosed string",
		"key: \"incomplete \"",
		"key: \"invalid \u1234\"",
		"key: \"incomplete \u1234\"",
		"key: 0xGHI",
		"key: 0o789",
		"key: 0b123",
		"key: ```\nline1\nline2",
		"key: \"\"\"\nline1\nline2",
		"poem: ```\n      First line\n           Second\n        Third Line\n```",
		"script: \"\"\"\n          First line\n          Second line\n          Third line\n\"\"\"",
		"key::\n    a: ```\n      First line\n           Second\n        Third Line\n",
		"key::\n    b: \"\"\"\n          First line\n          Second line\n          Third line\n",
		"list::\n  item1\n  item2",
		"list::\n-item1\n-item2",
		"dict:: {}\n",
		"list:: []\n",
		"values:: null, true, false, nan, inf, -inf",
	}

	for _, seed := range inputs {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		var result any
		_ = Unmarshal([]byte(input), &result)
	})
}

// TestDeepEqual reads test.huml and test.json, unmarshals them, and compares the results.
func TestDeepEqual(t *testing.T) {
	// Read test.huml and unmarshal it.
	var resHuml map[string]any
	b, err := os.ReadFile("test.huml")
	if err != nil {
		t.Fatalf("failed to read test.huml: %v", err)
	}
	if err := Unmarshal(b, &resHuml); err != nil {
		t.Fatalf("failed to unmarshal test.huml: %v", err)
	}
	out := normalizeToJSON(resHuml)

	// Read test.json and unmarshal it.
	var resJson map[string]any
	b, err = os.ReadFile("test.json")
	if err != nil {
		t.Fatalf("failed to read test.json: %v", err)
	}
	if err := json.Unmarshal(b, &resJson); err != nil {
		t.Fatalf("failed to unmarshal test.json: %v", err)
	}

	// Deep-compare both.
	assert.Equal(t, out, resJson, "test.huml and test.json should be deeply equal")
}

func BenchmarkParseHUML(b *testing.B) {
	f, err := os.ReadFile("test.huml")
	if err != nil {
		b.Fatalf("failed to read test.huml: %v", err)
	}
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		var result any
		if err := Unmarshal(f, &result); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkParseJSON(b *testing.B) {
	f, err := os.ReadFile("test.json")
	if err != nil {
		b.Fatalf("failed to read test.json: %v", err)
	}

	b.ReportAllocs()

	for b.Loop() {
		var result any
		if err := json.Unmarshal(f, &result); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// json lib uses float64 for all numbers. Convert all numbers to the same type
// in the HUML-parsed structure to make a deep-comparison with the JSON structure possible.
func normalizeToJSON(data any) any {
	switch v := data.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, val := range v {
			result[key] = normalizeToJSON(val)
		}
		return result

	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			result[i] = normalizeToJSON(val)
		}
		return result

	case int64:
		return float64(v)

	default:
		return v
	}
}
