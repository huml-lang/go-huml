package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// parser holds the state of the parsing process.
type parser struct {
	data []byte // The input HUML data.
	pos  int    // The current position in the data.
	line int    // The current line number, for error reporting.
}

// Unmarshal parses HUML data and stores the result in the value pointed to by v.
// It is the main entry point for the HUML parser.
func Unmarshal(data []byte, v any) error {
	// A completely empty document is a valid empty dict.
	if len(data) == 0 {
		return setValue(v, map[string]any{})
	}

	p := &parser{data: data, line: 1}

	// Parse the document. The result can be any valid HUML type.
	out, err := p.parseDocument()
	if err != nil {
		return err
	}

	// The parsed Go value is assigned to the user-provided variable.
	return setValue(v, out)
}

// errorf creates a new error with the current line number.
// This is a helper to consistently format error messages.
func (p *parser) errorf(format string, args ...any) error {
	return fmt.Errorf("line %d: "+format, append([]any{p.line}, args...)...)
}

// parseDocument is the top-level parsing function for a HUML document.
func (p *parser) parseDocument() (any, error) {
	// A document can begin with a version declaration.
	if p.peekString("%HUML") {
		p.advance(len("%HUML"))
		// The rest of the version line is ignored, but must be well-formed.
		if err := p.consumeLine(); err != nil {
			return nil, err
		}
	}

	// Skip any blank lines or comments at the start of the content.
	if err := p.skipBlankLines(); err != nil {
		return nil, err
	}

	// If the document is empty or only contains comments, it's an empty dict.
	if p.done() {
		return map[string]any{}, nil
	}

	// Root element must not be indented
	if p.getCurIndent() != 0 {
		return nil, p.errorf("root element must not be indented")
	}

	// Root indicators : and :: are not permitted at document root
	if p.peekString("::") {
		return nil, p.errorf("'::' indicator not allowed at document root")
	}
	if p.peekString(":") && !p.hasKeyValuePair() {
		return nil, p.errorf("':' indicator not allowed at document root")
	}

	// A document can be a dict, inline list, multiline list, or scalar value.
	switch {
	case p.hasKeyValuePair() && p.hasInlineDictAtRoot():
		// The document is an inline dict.
		val, err := p.parseInlineDict()
		if err != nil {
			return nil, err
		}

		// Ensure no other content follows the inline dict.
		if err := p.skipBlankLines(); err != nil {
			return nil, err
		}
		if !p.done() {
			return nil, p.errorf("unexpected content after root inline dict")
		}

		return val, nil
	case p.hasKeyValuePair():
		// The document is a standard multi-line dict.
		return p.parseDict(0)
	case p.peekString("[]"):
		// Empty list at root
		p.advance(2)
		if err := p.consumeLine(); err != nil {
			return nil, err
		}
		if err := p.skipBlankLines(); err != nil {
			return nil, err
		}
		if !p.done() {
			return nil, p.errorf("unexpected content after root list")
		}
		return []any{}, nil
	case p.peekString("{}"):
		// Empty dict at root
		p.advance(2)
		if err := p.consumeLine(); err != nil {
			return nil, err
		}
		if err := p.skipBlankLines(); err != nil {
			return nil, err
		}
		if !p.done() {
			return nil, p.errorf("unexpected content after root dict")
		}
		return map[string]any{}, nil
	case p.peekChar(p.pos) == '-':
		// Multiline list at root
		return p.parseMultilineList(0)
	case p.hasInlineListAtRoot():
		// Inline list at root (comma-separated values)
		val, err := p.parseInlineList()
		if err != nil {
			return nil, err
		}

		// Ensure no other content follows the inline list.
		if err := p.skipBlankLines(); err != nil {
			return nil, err
		}
		if !p.done() {
			return nil, p.errorf("unexpected content after root inline list")
		}

		return val, nil
	default:
		// The document is a single scalar value.
		val, err := p.parseValue(0)
		if err != nil {
			return nil, err
		}

		// Ensure no other content follows the scalar.
		if err := p.consumeLine(); err != nil {
			return nil, err
		}
		if err := p.skipBlankLines(); err != nil {
			return nil, err
		}
		if !p.done() {
			return nil, p.errorf("unexpected content after root scalar value")
		}

		return val, nil
	}
}

// parseDict parses a multi-line dict at a given indentation level.
func (p *parser) parseDict(indent int) (any, error) {
	out := map[string]any{}
	for {
		if err := p.skipBlankLines(); err != nil {
			return nil, err
		}
		if p.done() {
			break
		}

		// Check if de-dented, which marks the end of the current dict.
		curIndent := p.getCurIndent()
		if curIndent < indent {
			break
		}

		// Enforce strict indentation.
		if curIndent != indent {
			return nil, p.errorf("bad indent %d, expected %d", curIndent, indent)
		}

		if !p.isKeyStart() {
			return nil, p.errorf("invalid character '%c', expected key", p.data[p.pos])
		}

		// Parse the key-value pair.
		key, err := p.parseKey()
		if err != nil {
			return nil, err
		}

		if _, exists := out[key]; exists {
			return nil, p.errorf("duplicate key '%s' in dict", key)
		}

		// The indicator determines if the value is a scalar (:) or vector (::).
		indicator, err := p.parseIndicator()
		if err != nil {
			return nil, err
		}

		var val any
		if indicator == ":" {
			// A scalar value is on the same line as its key.
			if err := p.expectSpace("after ':'"); err != nil {
				return nil, err
			}

			// Determine if a value is multi-line *before* parsing it,
			// because the multi-line parser consumes its own newlines.
			isMultiline := p.peekString("```") || p.peekString(`"""`)

			val, err = p.parseValue(curIndent)
			if err != nil {
				return nil, err
			}

			// If the parsed value was not a multi-line string, consume
			// the rest of the line, which may include a comment.
			if !isMultiline {
				if err := p.consumeLine(); err != nil {
					return nil, err
				}
			}
		} else {
			// A vector value starts on the next line or is inline.
			val, err = p.parseVector(curIndent + 2)
			if err != nil {
				return nil, err
			}
		}
		if err != nil {
			return nil, err
		}

		out[key] = val
	}

	return out, nil
}

// parseMultilineList parses a multi-line list at a given indentation level.
func (p *parser) parseMultilineList(indent int) (any, error) {
	var out []any
	for {
		if err := p.skipBlankLines(); err != nil {
			return nil, err
		}
		if p.done() {
			break
		}

		curIndent := p.getCurIndent()
		if curIndent < indent {
			break
		}
		if curIndent != indent {
			return nil, p.errorf("bad indent %d, expected %d", curIndent, indent)
		}
		if p.data[p.pos] != '-' {
			break // No longer in a list.
		}

		p.advance(1)
		p.expectSpace("after '-'")

		var (
			val any
			err error
		)
		// A list item can be a nested vector.
		if p.peekString("::") {
			p.advance(2)
			val, err = p.parseVector(curIndent + 2)
		} else {
			// Or it can be a simple scalar value.
			val, err = p.parseValue(curIndent)
			if err == nil {
				err = p.consumeLine()
			}
		}
		if err != nil {
			return nil, err
		}

		out = append(out, val)
	}

	return out, nil
}

// parseVector parses a vector (list or dict) after a '::' indicator.
func (p *parser) parseVector(indent int) (any, error) {
	// After `::`, distinguish between an inline vector and a multi-line vector.
	// A multi-line vector is indicated by a newline, or a comment followed by a newline.
	// An inline vector is indicated by a single space followed by a value.
	startPos := p.pos
	p.skipSpaces()

	// Check for indicators of a multi-line vector (a comment or the end of the line).
	if p.done() || p.data[p.pos] == '\n' || p.data[p.pos] == '#' {
		// This is a multi-line vector. Rewind to let consumeLine handle validation.
		p.pos = startPos
		if err := p.consumeLine(); err != nil {
			return nil, err
		}

		// Now, parse the block that starts on the next line.
		if err := p.skipBlankLines(); err != nil {
			return nil, err
		}
		if p.done() {
			return nil, p.errorf("ambiguous empty vector after '::'. Use [] or {}.")
		}

		curIndent := p.getCurIndent()
		if curIndent < indent {
			return nil, p.errorf("ambiguous empty vector after '::'. Use [] or {}.")
		}

		// The first character on the next line determines the type.
		if p.data[p.pos] == '-' {
			return p.parseMultilineList(curIndent)
		}

		return p.parseDict(curIndent)
	}

	// If it's not a multi-line vector, it must be an inline one.
	// For an inline vector, there must be exactly one space after '::'.
	p.pos = startPos
	if err := p.expectSpace("after '::'"); err != nil {
		return nil, err
	}

	return p.parseInlineVector()
}

// parseInlineVector parses an inline vector, which can be a list, dict, or empty marker.
func (p *parser) parseInlineVector() (any, error) {
	// Special markers for empty list and dict.
	if p.peekString("[]") {
		p.advance(2)
		if err := p.consumeLine(); err != nil {
			return nil, err
		}
		return []any{}, nil
	}

	if p.peekString("{}") {
		p.advance(2)
		if err := p.consumeLine(); err != nil {
			return nil, err
		}
		return map[string]any{}, nil
	}

	// To distinguish between an inline list and dict, check for a 'key:' pattern.
	if p.hasInlineDict() {
		return p.parseInlineDict()
	}

	return p.parseInlineList()
}

// parseInlineList parses a comma-separated list of scalar values.
func (p *parser) parseInlineList() (any, error) {
	var out []any
	isFirst := true
	for !p.done() && p.data[p.pos] != '\n' && p.data[p.pos] != '#' {
		if !isFirst {
			if err := p.expectComma(); err != nil {
				return nil, err
			}
		}
		isFirst = false

		val, err := p.parseValue(0)
		if err != nil {
			return nil, err
		}

		out = append(out, val)

		// Only skip spaces if there might be a comma following.
		if !p.done() && p.data[p.pos] == ' ' {
			nextPos := p.pos + 1
			for nextPos < len(p.data) && p.data[nextPos] == ' ' {
				nextPos++
			}
			if nextPos < len(p.data) && p.data[nextPos] == ',' {
				p.skipSpaces()
			} else {
				// Don't consume spaces if they're trailing spaces at end of line.
				break
			}
		}
	}

	if err := p.consumeLine(); err != nil {
		return nil, err
	}

	return out, nil
}

// parseInlineDict parses a comma-separated dict of scalar key-value pairs.
func (p *parser) parseInlineDict() (any, error) {
	res := map[string]any{}
	isFirst := true
	for !p.done() && p.data[p.pos] != '\n' && p.data[p.pos] != '#' {
		if !isFirst {
			if err := p.expectComma(); err != nil {
				return nil, err
			}
		}
		isFirst = false

		key, err := p.parseKey()
		if err != nil {
			return nil, err
		}
		if p.done() || p.data[p.pos] != ':' {
			return nil, p.errorf("expected ':' in inline dict")
		}

		p.advance(1)
		if err := p.expectSpace("in inline dict"); err != nil {
			return nil, err
		}

		val, err := p.parseValue(0)
		if err != nil {
			return nil, err
		}

		res[key] = val

		// Only skip spaces if there might be a comma following
		if !p.done() && p.data[p.pos] == ' ' {
			nextPos := p.pos + 1
			for nextPos < len(p.data) && p.data[nextPos] == ' ' {
				nextPos++
			}
			if nextPos < len(p.data) && p.data[nextPos] == ',' {
				p.skipSpaces()
			} else {
				// Don't consume spaces if they're trailing spaces at end of line
				break
			}
		}
	}

	if err := p.consumeLine(); err != nil {
		return nil, err
	}

	return res, nil
}

// parseKey parses a dict key, which can be a bare string or quoted.
func (p *parser) parseKey() (string, error) {
	p.skipSpaces()
	if p.peekChar(p.pos) == '"' {
		return p.parseString()
	}

	start := p.pos
	for !p.done() && (isAlphaNum(p.data[p.pos]) || p.data[p.pos] == '-' || p.data[p.pos] == '_') {
		p.pos++
	}
	if p.pos == start {
		return "", p.errorf("expected a key")
	}

	return string(p.data[start:p.pos]), nil
}

// parseIndicator parses the ':' or '::' after a key.
func (p *parser) parseIndicator() (string, error) {
	if p.done() || p.data[p.pos] != ':' {
		return "", p.errorf("expected ':' or '::' after key")
	}

	p.advance(1)
	if !p.done() && p.data[p.pos] == ':' {
		p.advance(1)
		return "::", nil
	}

	return ":", nil
}

// parseValue parses any scalar value (string, number, bool, null).
func (p *parser) parseValue(keyIndent int) (any, error) {
	if p.done() {
		return nil, p.errorf("unexpected end of input, expected a value")
	}

	switch c := p.data[p.pos]; {
	case c == '"':
		if p.peekString(`"""`) {
			return p.parseMultilineString(keyIndent, false)
		}
		return p.parseString()
	case c == '`' && p.peekString("```"):
		return p.parseMultilineString(keyIndent, true)
	case c == 't' && p.peekString("true"):
		p.advance(4)
		return true, nil
	case c == 'f' && p.peekString("false"):
		p.advance(5)
		return false, nil
	case c == 'n' && p.peekString("null"):
		p.advance(4)
		return nil, nil
	case c == 'n' && p.peekString("nan"):
		p.advance(3)
		return math.NaN(), nil
	case c == 'i' && p.peekString("inf"):
		p.advance(3)
		return math.Inf(1), nil
	case c == '+':
		p.advance(1)
		if p.peekString("inf") {
			p.advance(3)
			return math.Inf(1), nil
		}

		if isDigit(p.peekChar(p.pos)) {
			// parseNumber will handle the sign.
			p.pos--
			return p.parseNumber()
		}
		return nil, p.errorf("invalid character after '+'")
	case c == '-':
		p.advance(1)
		if p.peekString("inf") {
			p.advance(3)
			return math.Inf(-1), nil
		}
		if isDigit(p.peekChar(p.pos)) {
			// parseNumber will handle the sign.
			p.pos--
			return p.parseNumber()
		}
		return nil, p.errorf("invalid character after '-'")
	case isDigit(c):
		return p.parseNumber()
	default:
		return nil, p.errorf("unexpected character '%c' when parsing value", c)
	}
}

// parseString parses a standard double-quoted string with escapes.
func (p *parser) parseString() (string, error) {
	// Consume the quote.
	p.advance(1)

	var b strings.Builder
	for !p.done() {
		c := p.data[p.pos]
		if c == '"' {
			p.advance(1) // Consume trailing quote.
			return b.String(), nil
		}
		if c == '\n' {
			return "", p.errorf("newlines not allowed in single-line strings")
		}
		if c == '\\' {
			p.advance(1) // Consume '\'.
			if p.done() {
				return "", p.errorf("incomplete escape sequence")
			}
			switch esc := p.data[p.pos]; esc {
			case '"', '\\', '/':
				b.WriteByte(esc)
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'u':
				// Handle a 4-hex-digit Unicode escape sequence.
				if p.pos+4 >= len(p.data) {
					return "", p.errorf("incomplete unicode escape sequence \\u")
				}
				hex := p.data[p.pos+1 : p.pos+5]
				code, err := strconv.ParseUint(string(hex), 16, 32)
				if err != nil {
					return "", p.errorf("invalid unicode escape sequence \\u%s", string(hex))
				}
				b.WriteRune(rune(code))

				// Consume the 4 hex digits.
				p.advance(4)
			default:
				return "", p.errorf("invalid escape character '\\%c'", esc)
			}
		} else {
			b.WriteByte(c)
		}

		// Consume the character or the final character of the escape code.
		p.advance(1)
	}

	return "", p.errorf("unclosed string")
}

// parseMultilineString parses ```` (preserve space) or `"""` (strip space) strings.
func (p *parser) parseMultilineString(keyIndent int, preserveSpaces bool) (string, error) {
	delim := string(p.data[p.pos : p.pos+3])
	p.advance(3)

	// Delimiter must be followed by a newline or valid comment.
	if err := p.consumeLine(); err != nil {
		return "", err
	}

	var builder strings.Builder
	for !p.done() {
		lineStartPos := p.pos
		lineIndent := 0
		for !p.done() && p.data[p.pos] == ' ' {
			lineIndent++
			p.pos++
		}

		// Check for the closing delimiter.
		if p.peekString(delim) {
			if lineIndent != keyIndent {
				return "", p.errorf("multiline closing delimiter must be at same indentation as the key (%d spaces)", keyIndent)
			}
			// Consume delimiter.
			p.advance(3)

			// After the closing delimiter, there might be a comment or a newline.
			if err := p.consumeLine(); err != nil {
				return "", p.errorf("invalid content after multiline string closing delimiter")
			}

			// Trim the final newline added by the loop.
			return strings.TrimSuffix(builder.String(), "\n"), nil
		}

		// Rewind to the start of the line to process its content.
		p.pos = lineStartPos
		lineContent := p.consumeLineContent()

		if preserveSpaces {
			// Strip the required 2-space indent relative to the key.
			requiredIndent := keyIndent + 2
			if len(lineContent) >= requiredIndent && isSpaceString(lineContent[:requiredIndent]) {
				builder.WriteString(lineContent[requiredIndent:])
			} else {
				builder.WriteString(lineContent)
			}
		} else {
			// Strip all leading and trailing whitespace from the line.
			builder.WriteString(strings.TrimSpace(lineContent))
		}

		builder.WriteByte('\n')
	}

	return "", p.errorf("unclosed multiline string")
}

// parseNumber parses any numeric format (integer, float, hex, octal, binary).
func (p *parser) parseNumber() (any, error) {
	start := p.pos
	if p.peekChar(p.pos) == '+' || p.peekChar(p.pos) == '-' {
		p.advance(1)
	}

	if p.peekString("0x") {
		return p.parseBase(start, 16, "0x")
	}
	if p.peekString("0o") {
		return p.parseBase(start, 8, "0o")
	}
	if p.peekString("0b") {
		return p.parseBase(start, 2, "0b")
	}

	isFloat := false
loop:
	for !p.done() {
		c := p.data[p.pos]
		switch {
		case isDigit(c) || c == '_':
			p.advance(1)
		case c == '.':
			isFloat = true
			p.advance(1)
		case (c == 'e' || c == 'E'):
			isFloat = true
			p.advance(1)
			if p.peekChar(p.pos) == '+' || p.peekChar(p.pos) == '-' {
				p.advance(1)
			}
		default:
			break loop
		}
	}

	// Replace any underscores in the number string.
	numStr := strings.ReplaceAll(string(p.data[start:p.pos]), "_", "")
	if isFloat {
		return strconv.ParseFloat(numStr, 64)
	}

	return strconv.ParseInt(numStr, 10, 64)
}

// parseBase parses a number in a non-decimal base.
func (p *parser) parseBase(start, base int, prefix string) (int64, error) {
	p.advance(len(prefix))
	numStart := p.pos
	for !p.done() {
		c := p.data[p.pos]
		valid := false
		switch base {
		case 16:
			valid = isHex(c)
		case 8:
			valid = c >= '0' && c <= '7'
		case 2:
			valid = c == '0' || c == '1'
		}
		if !valid {
			break
		}
		p.advance(1)
	}
	if p.pos == numStart {
		return 0, p.errorf("invalid number literal, requires digits after prefix")
	}

	sign := ""
	if p.data[start] == '+' || p.data[start] == '-' {
		sign = string(p.data[start])
	}

	numStr := strings.ReplaceAll(string(p.data[numStart:p.pos]), "_", "")
	val, err := strconv.ParseInt(numStr, base, 64)
	if err != nil {
		return 0, p.errorf("invalid number: %v", err)
	}
	if sign == "-" {
		return -val, nil
	}

	return val, nil
}

// skipBlankLines consumes empty lines and comment-only lines, validating them.
func (p *parser) skipBlankLines() error {
	for !p.done() {
		lineStart := p.pos
		p.skipSpaces()
		if p.done() {
			// We're at end of input after consuming spaces.
			// This is only valid if there were no spaces to consume (empty input).
			if p.pos > lineStart {
				return p.errorf("trailing spaces are not allowed")
			}
			return nil
		}

		if p.data[p.pos] != '\n' && p.data[p.pos] != '#' {
			// Found non-blank content, stop.
			return nil
		}

		// Check for trailing spaces on blank lines
		if p.data[p.pos] == '\n' && p.pos > lineStart {
			return p.errorf("trailing spaces are not allowed")
		}

		// Reset position and consume the blank or comment-only line.
		p.pos = lineStart
		if err := p.consumeLine(); err != nil {
			return err
		}
	}

	return nil
}

// consumeLine validates and consumes the rest of a line.
// It ensures there are no trailing spaces and that comments are well-formed.
func (p *parser) consumeLine() error {
	contentStartPos := p.pos
	p.skipSpaces()

	if p.done() || p.data[p.pos] == '\n' {
		if p.pos > contentStartPos {
			return p.errorf("trailing spaces are not allowed")
		}
	} else if p.data[p.pos] == '#' {
		if p.pos == contentStartPos && p.getCurIndent() != p.pos-p.lineStart() {
			return p.errorf("a value must be separated from an inline comment by a space")
		}

		// Consume '#'.
		p.pos++
		if !p.done() && p.data[p.pos] != ' ' && p.data[p.pos] != '\n' {
			return p.errorf("comment hash '#' must be followed by a space")
		}
	} else {
		return p.errorf("unexpected content at end of line")
	}

	commentEndPos := p.pos
	for !p.done() && p.data[p.pos] != '\n' {
		p.pos++
	}

	if p.pos > 0 && p.data[p.pos-1] == ' ' {
		// Check the character before the trailing space.
		if p.pos-1 > commentEndPos {
			return p.errorf("trailing spaces are not allowed")
		}
	}

	if !p.done() && p.data[p.pos] == '\n' {
		p.pos++
		p.line++
	}

	return nil
}

// consumeLineContent reads the rest of a line without validation, used for multiline strings.
func (p *parser) consumeLineContent() string {
	start := p.pos
	for !p.done() && p.data[p.pos] != '\n' {
		p.pos++
	}

	content := p.data[start:p.pos]
	if !p.done() && p.data[p.pos] == '\n' {
		p.pos++
		p.line++
	}

	return string(content)
}

// expectSpace consumes exactly one space and returns an error if not found.
func (p *parser) expectSpace(context string) error {
	if p.done() || p.data[p.pos] != ' ' {
		return p.errorf("expected single space %s", context)
	}

	p.advance(1)
	if !p.done() && p.data[p.pos] == ' ' {
		return p.errorf("expected single space %s, found multiple", context)
	}

	return nil
}

// expectComma consumes a comma, ensuring correct spacing.
func (p *parser) expectComma() error {
	p.skipSpaces()
	if p.done() || p.data[p.pos] != ',' {
		return p.errorf("expected a comma in inline collection")
	}

	if p.pos > 0 && p.data[p.pos-1] == ' ' {
		return p.errorf("no spaces allowed before comma")
	}
	p.advance(1)

	return p.expectSpace("after comma")
}

// getCurIndent calculates the indentation of the current line.
func (p *parser) getCurIndent() int {
	var (
		lineStart = p.lineStart()
		indent    = 0
	)
	for lineStart+indent < len(p.data) && p.data[lineStart+indent] == ' ' {
		indent++
	}

	return indent
}

// lineStart returns the starting position of the current line.
func (p *parser) lineStart() int {
	start := p.pos
	if start > 0 && start <= len(p.data) && p.data[start-1] == '\n' {
		return start
	}

	for start > 0 && p.data[start-1] != '\n' {
		start--
	}

	return start
}

// hasKeyValuePair checks if the current line looks like a `key: value` pair.
func (p *parser) hasKeyValuePair() bool {
	// A simple lookahead to distinguish a dict from a scalar at the root.
	last := p.pos
	defer func() { p.pos = last }()

	if _, err := p.parseKey(); err != nil {
		return false
	}

	return !p.done() && p.data[p.pos] == ':'
}

// hasInlineDict peeks ahead to see if an inline collection is a dict.
func (p *parser) hasInlineDict() bool {
	// A simple lookahead to differentiate `key: val, ...` from `val1, val2, ...`
	pos := p.pos
	for pos < len(p.data) && p.data[pos] != '\n' && p.data[pos] != '#' {
		if p.data[pos] == ':' {
			if pos+1 < len(p.data) && p.data[pos+1] != ':' {
				// Found a scalar indicator ':'.
				return true
			}
		}
		pos++
	}

	return false
}

// hasInlineListAtRoot checks if the document starts with an inline list (comma-separated values).
func (p *parser) hasInlineListAtRoot() bool {
	// Simple lookahead to detect comma-separated values at root (not a key-value pair)
	pos := p.pos
	for pos < len(p.data) && p.data[pos] != '\n' && p.data[pos] != '#' {
		if p.data[pos] == ',' {
			return true
		}
		if p.data[pos] == ':' {
			// This would be a key-value pair, not an inline list
			return false
		}
		pos++
	}
	return false
}

// hasInlineDictAtRoot checks if the document starts with an inline dict (key-value pairs with commas).
func (p *parser) hasInlineDictAtRoot() bool {
	// At root level, check if we have key: value, key: value pattern on same line
	// But make sure we don't have :: at the beginning (which would be a keyed vector)
	pos := p.pos
	foundColon := false
	foundComma := false
	foundDoubleColon := false

	// Check the current line for inline dict pattern
	for pos < len(p.data) && p.data[pos] != '\n' && p.data[pos] != '#' {
		if p.data[pos] == ':' {
			if pos+1 < len(p.data) && p.data[pos+1] == ':' {
				foundDoubleColon = true
			} else {
				foundColon = true
			}
		}
		if p.data[pos] == ',' {
			foundComma = true
		}
		pos++
	}

	// Only consider it an inline dict if we have both : and , on the same line
	// but NOT :: (which would be a keyed vector)
	if !(foundColon && foundComma && !foundDoubleColon) {
		return false
	}

	// For a true inline dict at root, there should be no subsequent content
	// (except comments and blank lines) after this line

	// Skip to end of current line
	for pos < len(p.data) && p.data[pos] != '\n' {
		pos++
	}
	if pos < len(p.data) && p.data[pos] == '\n' {
		pos++ // Skip the newline
	}

	// Check if there's any content that would make this a multi-line dict
	for pos < len(p.data) {
		// Skip spaces at start of line
		for pos < len(p.data) && p.data[pos] == ' ' {
			pos++
		}

		if pos >= len(p.data) {
			break // End of input
		}

		if p.data[pos] == '\n' {
			// Blank line, continue
			pos++
			continue
		}

		if p.data[pos] == '#' {
			// Comment line, skip to end of line
			for pos < len(p.data) && p.data[pos] != '\n' {
				pos++
			}
			if pos < len(p.data) && p.data[pos] == '\n' {
				pos++
			}
			continue
		}

		// Found non-blank, non-comment content
		// This means it's a multi-line dict, not an inline dict at root
		return false
	}

	return true
}

func (p *parser) isKeyStart() bool {
	return !p.done() && (p.data[p.pos] == '"' || isAlpha(p.data[p.pos]))
}

func (p *parser) done() bool {
	return p.pos >= len(p.data)
}

func (p *parser) advance(n int) {
	p.pos += n
}

func (p *parser) skipSpaces() {
	for !p.done() && p.data[p.pos] == ' ' {
		p.advance(1)
	}
}

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

	var (
		d = val.Elem()
		s = reflect.ValueOf(src)
	)

	// If the destination is an interface, set it directly.
	if d.Kind() == reflect.Interface {
		if s.IsValid() {
			d.Set(s)
		} else {
			d.Set(reflect.Zero(d.Type()))
		}
		return nil
	}

	if s.IsValid() && s.Type().AssignableTo(d.Type()) {
		d.Set(s)
		return nil
	}

	return fmt.Errorf("cannot assign %T to %s", src, d.Type())
}

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

func isSpaceString(s string) bool {
	return strings.TrimSpace(s) == ""
}

func (p *parser) peekString(s string) bool {
	if p.pos+len(s) > len(p.data) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if p.data[p.pos+i] != s[i] {
			return false
		}
	}
	return true
}

func (p *parser) peekChar(pos int) byte {
	if pos >= len(p.data) || pos < 0 {
		return 0
	}
	return p.data[pos]
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: huml <filename>")
		os.Exit(1)
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	var result any
	if err := Unmarshal(raw, &result); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Error marshalling to JSON: %v", err)
	}

	fmt.Println(string(b))
}
