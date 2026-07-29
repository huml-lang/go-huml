package huml

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// roundTrip marshals v, decodes it back and asserts the value survives. The
// library's own decoder is the oracle: the encoder must never emit bytes its
// decoder rejects, and the value must be preserved exactly.
func roundTrip(t *testing.T, name string, v any) {
	t.Helper()
	b, err := Marshal(v)
	if !assert.NoError(t, err, "%s: marshal", name) {
		return
	}
	var out any
	if !assert.NoError(t, Unmarshal(b, &out), "%s: decoder rejected encoder output\n%s", name, b) {
		return
	}
	assert.Equal(t, fmt.Sprint(v), fmt.Sprint(out), "%s: value changed on round-trip\n%s", name, b)
}

// Every byte must survive a round-trip as a single-line string value. C0
// controls, DEL, ESC and bell previously produced \x/\u/\a escapes the decoder
// could not read.
func TestEncodeStringAllBytesValue(t *testing.T) {
	for i := 0; i < 256; i++ {
		s := "a" + string([]byte{byte(i)}) + "b"
		roundTrip(t, fmt.Sprintf("value-byte-0x%02x", i), map[string]any{"k": s})
	}
}

// Quoted keys go through the same escaping path as values (the sibling site) and
// must round-trip for every byte too. A newline in a key uses the \n escape,
// since a key is never a multi-line block.
func TestEncodeStringAllBytesKey(t *testing.T) {
	for i := 0; i < 256; i++ {
		k := "a" + string([]byte{byte(i)}) + "b"
		roundTrip(t, fmt.Sprintf("key-byte-0x%02x", i), map[string]any{k: "v"})
	}
}

// Encoded output must not contain the escapes HUML forbids (\x, \u, \U, \a).
func TestEncodeStringNoForbiddenEscapes(t *testing.T) {
	b, err := Marshal(map[string]any{"k": "\x00\a\b\x1b\x7f\xff"})
	assert.NoError(t, err)
	for _, bad := range []string{`\x`, `\u`, `\U`, `\a`} {
		assert.NotContains(t, string(b), bad, "emitted forbidden escape %s\n%s", bad, b)
	}
}

// A normal string must not gain spurious escapes (over-escaping guard).
func TestEncodeStringNoOverEscape(t *testing.T) {
	assert.Equal(t, `"hello world / a.b-c"`, humlQuote("hello world / a.b-c"))
	assert.Equal(t, `"tab\ttab \"q\" \\slash"`, humlQuote("tab\ttab \"q\" \\slash"))
}

// Multi-line strings must preserve trailing and interior newlines. The encoder
// used to drop a genuine trailing newline (data loss).
func TestEncodeStringMultilineEdges(t *testing.T) {
	cases := map[string]string{
		"trailing-newline":    "a\n",
		"two-trailing":        "a\n\n",
		"leading-newline":     "\na",
		"only-newline":        "\n",
		"blank-middle":        "a\n\nb",
		"interior-indent":     "a\n  b\n",
		"crlf":                "a\r\nb",
		"no-trailing-newline": "a\nb",
	}
	for name, s := range cases {
		roundTrip(t, name, map[string]any{"k": s})
	}
}

// A multi-line string at the document root must not panic (indent 0 underflowed
// strings.Repeat) and must round-trip.
func TestEncodeStringRootMultiline(t *testing.T) {
	for name, v := range map[string]any{
		"root-multiline":        "line1\nline2",
		"root-trailing-newline": "a\nb\n",
		"root-control":          "a\x1bb",
		"root-plain":            "hello",
	} {
		roundTrip(t, name, v)
	}
}

// Agreeing controls: values the encoder already handled correctly must stay green,
// proving the harness is discriminating rather than failing everything.
func TestEncodeStringControls(t *testing.T) {
	roundTrip(t, "plain", map[string]any{"k": "hello"})
	roundTrip(t, "unicode", map[string]any{"k": "Hello 🌏 café"})
	roundTrip(t, "float-exp", map[string]any{"k": 1e20})
	roundTrip(t, "small-float", map[string]any{"k": 1e-7})
	roundTrip(t, "multiline-no-trailer", map[string]any{"k": "line1\nline2"})
}
