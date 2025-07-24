package huml

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAssertions(t *testing.T) {
	// Assertion function.
	f := func(name, input string, errorExpected bool) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			// Call Unmarshal directly.
			t.Helper()
			var result any
			err := Unmarshal([]byte(input), &result)
			if errorExpected && err == nil {
				t.Errorf("expected error but got none")
			}
			if !errorExpected && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Try again via the decoder.
			var result2 any
			decoder := NewDecoder(strings.NewReader(input))
			err = decoder.Decode(&result2)
			if errorExpected && err == nil {
				t.Errorf("expected error but got none")
			}
			if !errorExpected && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

		})
	}

	// Walk the ./tests/assertions directory and read each JSON file into a slice of assertion structs.
	type assertion struct {
		Name  string `json:"name"`
		Input string `json:"input"`
		Error bool   `json:"error"`
	}

	// Check if the tests/assertions directory exists
	if _, err := os.Stat("./tests/assertions"); os.IsNotExist(err) {
		t.Fatalf("tests/assertions directory not found. Please run 'git submodule update --init --recursive' to initialize test data. See README for development setup instructions.")
	}

	err := filepath.Walk("./tests/assertions", func(path string, info os.FileInfo, err error) error {
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		// Read the JSON test file.
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("error reading assertions file: %s: %v", path, err)
		}

		// Unmarshal the JSON data into an assertion struct.
		var tests []assertion
		if err := json.Unmarshal(data, &tests); err != nil {
			t.Fatalf("error unmarshalling assertions file: %s: %v", path, err)
		}

		// Run each assertion.
		for n, t := range tests {
			// +2 to account for the opening [ and the line break in the test file.
			f(fmt.Sprintf("line %d: %s", n+2, t.Name), t.Input, t.Error)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("error walking assertions directory: %v", err)
	}
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

// TestDocuments reads fully formed HUML documents from tests/documents/*.huml files,
// and compares them against their corresponding JSON files (with the same name but .json extension).
func TestDocuments(t *testing.T) {
	// Globs tests/documents/*.huml file, and for each file, read the corresponding $name.json file also.
	files, err := filepath.Glob("tests/documents/*.huml")
	if err != nil {
		t.Fatalf("failed to glob documents/huml files: %v", err)
	}

	if len(files) < 1 {
		t.Fatalf("expected at least 1 huml file in tests/documents, found %d", len(files))
	}

	for _, path := range files {
		// Read test.huml and unmarshal it.
		var resHuml map[string]any
		b, err := os.ReadFile("tests/documents/mixed.huml")
		if err != nil {
			t.Fatalf("failed to read test.huml: %v", err)
		}
		if err := Unmarshal(b, &resHuml); err != nil {
			t.Fatalf("failed to unmarshal test.huml: %v", err)
		}
		out := normalizeToJSON(resHuml)

		// Read the corresponding JSON file.
		var resJson map[string]any
		b, err = os.ReadFile(strings.TrimSuffix(path, ".huml") + ".json")
		if err != nil {
			t.Fatalf("failed to read test.json: %v", err)
		}
		if err := json.Unmarshal(b, &resJson); err != nil {
			t.Fatalf("failed to unmarshal test.json: %v", err)

		}
		// Deep-compare both.
		assert.Equal(t, out, resJson, "test.huml and test.json should be deeply equal")
	}
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

// TestDecoderMultipleDecodes tests multiple sequential decodes.
func TestDecoderMultipleDecodes(t *testing.T) {
	// Test that a single decoder can only decode once (all data is consumed).
	input := "foo: \"bar\""
	decoder := NewDecoder(strings.NewReader(input))

	var result1 any
	if err := decoder.Decode(&result1); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	v := map[string]any{"foo": "bar"}
	if !reflect.DeepEqual(result1, v) {
		t.Errorf("expected %+v, got %+v", v, result1)
	}

	// Second decode should return EOF since all data was consumed.
	var result2 any
	if err := decoder.Decode(&result2); err == nil {
		t.Error("expected error but got none")
	}
}

// TestDecoderWithDifferentReaderTypes tests with various io.Reader implementations.
func TestDecoderWithDifferentReaderTypes(t *testing.T) {
	data := "count: 42\nactive: true"
	v := map[string]any{"count": int64(42), "active": true}

	f := func(name string, reader func() any) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			t.Helper()
			decoder := NewDecoder(reader().(interface{ Read([]byte) (int, error) }))
			var result any
			if err := decoder.Decode(&result); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(result, v) {
				t.Errorf("expected %+v, got %+v", v, result)
			}
		})
	}

	f("strings.Reader", func() any { return strings.NewReader(data) })
	f("bytes.Buffer", func() any {
		var buf bytes.Buffer
		buf.WriteString(data)
		return &buf
	})
	f("bytes.Reader", func() any { return bytes.NewReader([]byte(data)) })
}

// TestDecoderErrorHandling tests error handling scenarios.
func TestDecoderErrorHandling(t *testing.T) {
	t.Run("nil dest", func(t *testing.T) {
		decoder := NewDecoder(strings.NewReader("key: \"value\""))
		err := decoder.Decode(nil)
		if err == nil {
			t.Error("expected error but got none")
		}
		if !strings.Contains(err.Error(), "nil value") {
			t.Errorf("expected error to contain 'nil value', got: %v", err)
		}
	})

	t.Run("non-pointer dest", func(t *testing.T) {
		decoder := NewDecoder(strings.NewReader("key: \"value\""))

		// Initialize with a non-nil value.
		result := make(map[string]any)
		err := decoder.Decode(result)
		if err == nil {
			t.Error("expected error but got none")
		}
		if !strings.Contains(err.Error(), "not a pointer") {
			t.Errorf("expected error to contain 'not a pointer', got: %v", err)
		}
	})

	t.Run("reader error", func(t *testing.T) {
		// Create a reader that always returns an error.
		decoder := NewDecoder(&errorReader{err: errors.New("reader error")})

		var result any
		err := decoder.Decode(&result)
		if err == nil {
			t.Error("expected error but got none")
		}
		if !strings.Contains(err.Error(), "reader error") {
			t.Errorf("expected error to contain 'reader error', got: %v", err)
		}
	})
}

// errorReader is a helper type that always returns an error when reading
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}
