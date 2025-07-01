package main

import (
	"embed"
	"math"
	"reflect"
	"testing"
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
	f("root_empty_list", ":: []", false)
	f("root_empty_dict", ":: {}", false)
	f("root_inline_list", ":: 1, 2, 3", false)
	f("root_multiline_list", "::\n  - \"item1\"\n  - \"item2\"", false)
	f("root_multiline_unquoted_values", "::\n  - item1\n  - item2", true)
	f("root_multiline_bad_indentation", "::\n - item1\n  - item2", true)

	// dict cases.
	f("simple_dict", "dict::\n  key1: \"value1\"\n  key2: \"value2\"", false)
	f("simple_dict_unquoted_values", `dict::\n  key1: value1\n  key2: value2`, true)
	f("nested_dict", "outer::\n  inner::\n    key: 123", false)
	f("empty_dict", "dict:: {}", false)
	f("inline_dict", `dict:: key1: "value1", key2: "value2"`, false)

	// List cases.
	f("simple_list", "list::\n  - \"item1\"\n  - \"item2\"", false)
	f("nested_list", "list::\n  - \"item1\"\n  - ::\n    - \"nested1\"\n    - \"nested2\"", false)
	f("empty_list", "list:: []", false)
	f("inline_list", "list:: 1, 2, 3, 4", false)
	f("mixed_inline_list", `list:: 1, "string", true, null`, false)

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
	// {"trailing_spaces_after_colon_no_value", "key: ", true)
	f("trailing_spaces_after_double_colon", "key:: ", true)

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

	// Numbers.
	f("invalid_hex_number", "key: 0xGHI", true)
	f("invalid_octal_number", "key: 0o789", true)
	f("invalid_binary_number", "key: 0b123", true)

	// Multiline strings.
	// {"unclosed_multiline_string", "key: ```\nline1\nline2", true)
	// {"unclosed_triple_quoted_string", "key: \"\"\"\nline1\nline2", true)
	f("multiline_string_preserved_indentation", "poem: ```\n      First line\n           Second\n        Third Line\n```", false)
	f("multiline_string_stripped_indentation", "script: \"\"\"\n          First line\n          Second line\n          Third line\n\"\"\"", false)

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
	f("root_list", ":: 1, 2, 3, 5.6, +4, -2", []any{int64(1), int64(2), int64(3), float64(5.6), int64(4), int(-2)})

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
