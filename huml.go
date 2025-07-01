package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
)

type parser struct {
	data string
	pos  int
	line int
}

// Unmarshal parses HUML data and stores the result in the value pointed to by v.
// If v is nil or not a pointer, it returns an error.
//
// It converts HUML data into Go values with the following mappings:
//
//   - HUML scalars (key: value) become Go primitive types:
//   - strings for quoted strings and multiline strings
//   - int64 for integers
//   - float64 for floating point numbers
//   - bool for true/false
//   - nil for null
//   - math.NaN() for nan
//   - math.Inf() for inf/+inf/-inf
//   - HUML vectors (key:: value) become:
//   - []any for lists
//   - map[string]any for dictionaries
//   - HUML documents become map[string]any or []any depending on structure
//
// To unmarshal HUML into an interface value, it stores one of:
// bool, int64, float64, string, []any, map[string]any, or nil.
//
// HUML supports inline and multiline formats for both lists and dictionaries.
// Multiline strings can use """ (with leading space trimming) or ``` (preserving spaces).
// Numbers support underscores for readability and various bases (0x, 0o, 0b).
//
// If the data contains a syntax error, a parser error is returned with line number.
func Unmarshal(data []byte, v any) error {
	p := &parser{data: string(data), line: 1}

	out, err := p.parse()
	if err != nil {
		return err
	}

	return setValue(v, out)
}

// parse parses the entire HUML document and returns the value.
func (p *parser) parse() (any, error) {
	if err := p.skipSpacesAndComments(); err != nil {
		return nil, err
	}

	if p.done() {
		return nil, nil
	}

	if p.peekString("%HUML") {
		if err := p.parseVersion(); err != nil {
			return nil, err
		}
		if err := p.skipSpacesAndComments(); err != nil {
			return nil, err
		}
	}

	if p.done() {
		return nil, nil
	}

	if p.peekString("::") {
		return p.parseRootList()
	}

	if p.getCurIndent() == 0 && p.hasKeyValuePair() {
		return p.parseDict(0)
	}

	return p.parseValue()
}

// parseVersion parses the optional HUML version line starting with "%HUML".
func (p *parser) parseVersion() error {
	p.pos += len("%HUML")
	p.skipSpaces()

	// Check line ending (handles trailing spaces and comment validation).
	if err := p.validateLineEnding(); err != nil {
		return err
	}

	p.skipToNextLine()
	return nil
}

// parseRootList parses the root list starting with "::" in a key-less document.
func (p *parser) parseRootList() (any, error) {
	p.pos += len("::")
	val, err := p.parseVector(0)
	if err != nil {
		return nil, err
	}

	if list, ok := val.([]any); ok {
		return list, nil
	}

	return p.parseMultilineList(0)
}

// hasKeyValuePair checks if the current position in the data has a key-value pair.
func (p *parser) hasKeyValuePair() bool {
	last := p.pos
	defer func() { p.pos = last }()

	if !p.isKeyStart() {
		return false
	}

	if p.data[p.pos] == '"' {
		p.pos++
		for !p.done() && p.data[p.pos] != '"' {
			if p.data[p.pos] == '\\' && p.pos+1 < len(p.data) {
				p.pos += 2
			} else {
				p.pos++
			}
		}

		if !p.done() {
			p.pos++
		}
	} else {
		for !p.done() && (isAlphaNum(p.data[p.pos]) || p.data[p.pos] == '-' || p.data[p.pos] == '_') {
			p.pos++
		}
	}

	return !p.done() && p.data[p.pos] == ':'
}

// parseDict parses a dictionary at the given indentation level.
func (p *parser) parseDict(indent int) (any, error) {
	out := make(map[string]any)
	for !p.done() {
		if err := p.skipSpacesAndComments(); err != nil {
			return nil, err
		}
		if p.done() {
			break
		}

		// Get the current indentation level.
		curIndent := p.getCurIndent()
		if indent > 0 && curIndent < indent {
			break
		}
		if curIndent != indent {
			return nil, fmt.Errorf("line %d: bad indent %d, expected %d", p.line, curIndent, indent)
		}
		if !p.isKeyStart() {
			break
		}

		// Get the key.
		key, err := p.parseKey()
		if err != nil {
			return nil, err
		}

		// Get the indicator aftrer the key, ":" or "::".
		indicator, err := p.parseIndicator()
		if err != nil {
			return nil, err
		}

		var val any
		if indicator == ":" {
			val, err = p.parseScalarValueWithIndent(curIndent)
		} else {
			val, err = p.parseVector(curIndent + 2)
		}
		if err != nil {
			return nil, err
		}
		out[key] = val
	}

	return out, nil
}

// parseKey parses and returns the string key.
func (p *parser) parseKey() (string, error) {
	if p.data[p.pos] == '"' {
		return p.parseString()
	}

	start := p.pos
	for !p.done() && (isAlphaNum(p.data[p.pos]) || p.data[p.pos] == '-' || p.data[p.pos] == '_') {
		p.pos++
	}

	if p.pos == start {
		return "", fmt.Errorf("line %d: expected key", p.line)
	}

	return p.data[start:p.pos], nil
}

// parseIndicator parses the indicator after a key, whic is either ":" or "::".
func (p *parser) parseIndicator() (string, error) {
	if p.done() || p.data[p.pos] != ':' {
		return "", fmt.Errorf("line %d: expected ':'", p.line)
	}

	p.pos++

	if !p.done() && p.data[p.pos] == ':' {
		p.pos++
		return "::", nil
	}

	return ":", nil
}

func (p *parser) parseScalarValueWithIndent(indent int) (any, error) {
	// After single :, must have exactly one space before value
	if p.done() || p.data[p.pos] != ' ' {
		return nil, fmt.Errorf("line %d: expected single space after ':'", p.line)
	}
	p.pos++

	// Check for multiple spaces after :
	if !p.done() && p.data[p.pos] == ' ' && (p.pos+1 >= len(p.data) || p.data[p.pos+1] != '#') {
		return nil, fmt.Errorf("line %d: expected single space after ':'", p.line)
	}

	// After the required space, there must be a value, not newline or EOF.
	if p.done() || p.data[p.pos] == '\n' {
		return nil, fmt.Errorf("line %d: expected value after ':'", p.line)
	}

	// On reaching a comment after the space, there must be a preceding value.
	if p.data[p.pos] == '#' {
		return nil, fmt.Errorf("line %d: expected value after ':'", p.line)
	}

	// Check if this is a multiline string before parsing value.
	isMultilineStr := p.peekString(`"""`) || p.peekString("```")

	val, err := p.parseValue()
	if err != nil {
		return nil, err
	}

	// No need to check line endings for multiline strings as they handle their own line consumption.
	if !isMultilineStr {
		if err := p.validateLineEnding(); err != nil {
			return nil, err
		}
	}

	if _, isStr := val.(string); !isStr {
		p.skipToNextLine()
	}

	return val, nil
}

// parseVector handles the parsing of vectors after the double colon (::) indicator.
// It can be either an inline list/dict or a multiline list/dict.
func (p *parser) parseVector(indent int) (any, error) {
	// After ::, check what follows.
	if !p.done() && p.data[p.pos] == '#' {
		return nil, fmt.Errorf("line %d: :: must be followed by a space before comment", p.line)
	}

	// Check for spaces after ::.
	spacePos := p.pos
	p.skipSpaces()

	if p.done() || p.data[p.pos] == '\n' || p.data[p.pos] == '#' {
		// Check if we had trailing spaces or validate comment
		if p.done() || p.data[p.pos] == '\n' {
			if p.pos > spacePos {
				return nil, fmt.Errorf("line %d: trailing spaces not allowed", p.line)
			}
		} else if p.data[p.pos] == '#' {
			if err := p.validateComment(); err != nil {
				return nil, err
			}
		}
		p.skipToNextLine()
		if err := p.skipSpacesAndComments(); err != nil {
			return nil, err
		}
		if p.done() {
			return nil, fmt.Errorf("line %d: ambiguous empty vector after '::'. Use [] or {}.", p.line-1)
		}

		curIndent := p.getCurIndent()
		if curIndent < indent {
			if p.data[p.pos] == '-' || p.isKeyStart() {
				return nil, fmt.Errorf("line %d: bad indent %d, expected %d", p.line, curIndent, indent)
			}

			return nil, fmt.Errorf("line %d: ambiguous empty vector after '::'. Use [] or {}.", p.line-1)
		}

		if p.data[p.pos] == '-' {
			return p.parseMultilineList(curIndent)
		}

		return p.parseDict(curIndent)
	}

	// For inline values after ::, must have exactly one space
	if p.pos > spacePos && p.pos-spacePos != 1 {
		return nil, fmt.Errorf("line %d: expected single space after '::'", p.line)
	}

	return p.parseInlineVector()
}

// parseInlineVector parses an inline vector or dictionary.
func (p *parser) parseInlineVector() (any, error) {
	// [] is a special case indicating an empty list.
	if p.peekString("[]") {
		p.pos += 2
		// Check line ending after []
		if err := p.validateLineEnding(); err != nil {
			return nil, err
		}
		p.skipToNextLine()
		return []any{}, nil
	}

	// // {} is a special case indicating an empty dict.
	if p.peekString("{}") {
		p.pos += 2
		// Check line ending after {}
		if err := p.validateLineEnding(); err != nil {
			return nil, err
		}
		p.skipToNextLine()
		return make(map[string]any), nil
	}

	// Peek ahead to determine if dict or list.
	var (
		last   = p.pos
		isDict = false
	)
	for i := p.pos; i < len(p.data) && p.data[i] != '\n' && p.data[i] != '#'; i++ {
		// It's a dict.
		if p.data[i] == ':' && (i+1 >= len(p.data) || p.data[i+1] != ':') {
			isDict = true
			break
		}

		// It's a list.
		if p.data[i] == ',' {
			break
		}
	}
	p.pos = last

	if isDict {
		return p.parseInlineDict()
	}

	return p.parseInlineList()
}

// parseInlineList parses an inline list where items are separated by commas.
// Eg: list:: 1, "two", 3, true
func (p *parser) parseInlineList() (any, error) {
	var out []any
	for !p.done() && p.data[p.pos] != '\n' && p.data[p.pos] != '#' {
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		out = append(out, val)

		if err := p.parseInlineComma(); err != nil {
			return nil, err
		}
		if p.done() || p.data[p.pos] == '\n' || p.data[p.pos] == '#' || p.data[p.pos] == ',' {
			break
		}
	}

	return out, nil
}

// parseInlineDict parses an inline dictionary where key value pairs are separated by commas.
// Eg: dict:: key1: "value", key2: 123, key3: true
func (p *parser) parseInlineDict() (any, error) {
	res := make(map[string]any)
	for !p.done() && p.data[p.pos] != '\n' && p.data[p.pos] != '#' {
		key, err := p.parseKey()
		if err != nil {
			return nil, err
		}

		if p.done() || p.data[p.pos] != ':' {
			return nil, fmt.Errorf("line %d: expected ':'", p.line)
		}
		p.advance(1)
		p.skipSpaces()

		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		res[key] = val

		if err := p.parseInlineComma(); err != nil {
			return nil, err
		}

		if p.done() || p.data[p.pos] == '\n' || p.data[p.pos] == '#' || p.data[p.pos] == ',' {
			break
		}
	}

	return res, nil
}

// parseInlineComma checks for a comma in an inline list or dictionary.
// Commas cannot have preceding spaces, and must be followed by exactly one space.
// Eg: 1, "two", three
func (p *parser) parseInlineComma() error {
	start := p.pos
	p.skipSpaces()

	if !p.done() && p.data[p.pos] == ',' {
		if p.pos > start {
			return fmt.Errorf("line %d: no spaces allowed before comma", p.line)
		}
		p.advance(1)

		// After the comma, there must be exactly one space.
		if p.done() || p.data[p.pos] != ' ' {
			return fmt.Errorf("line %d: expected single space after comma", p.line)
		}

		p.advance(1)
		if !p.done() && p.data[p.pos] == ' ' {
			return fmt.Errorf("line %d: expected single space after comma", p.line)
		}
	}

	return nil
}

// parseMultilineList parses a list where every item starts with a dash (-) and continues through multiple lines.
func (p *parser) parseMultilineList(indent int) (any, error) {
	var out []any
	for !p.done() {
		if err := p.skipSpacesAndComments(); err != nil {
			return nil, err
		}
		if p.done() {
			break
		}

		currIndent := p.getCurIndent()
		if currIndent < indent {
			break
		}
		if currIndent > indent {
			return nil, fmt.Errorf("line %d: bad indent %d, expected %d", p.line, currIndent, indent)
		}

		if p.data[p.pos] != '-' {
			break
		}

		p.advance(1)
		p.skipSpaces()

		var (
			val any
			err error
		)
		if p.peekString("::") {
			p.pos += 2
			val, err = p.parseVector(currIndent + 2)
		} else {
			val, err = p.parseValue()
			if err == nil {
				if err := p.validateLineEnding(); err != nil {
					return nil, err
				}

				p.skipToNextLine()
			}
		}

		if err != nil {
			return nil, err
		}

		out = append(out, val)
	}

	return out, nil
}

// parseValue parses the value from the current position in the data.
func (p *parser) parseValue() (any, error) {
	p.skipSpaces()
	if p.done() {
		return nil, nil
	}

	switch c := p.data[p.pos]; {
	case c == '"':
		if p.peekString(`"""`) {
			return p.parseMultilineString(0)
		}
		return p.parseString()

	case c == '`' && p.peekString("```"):
		return p.parseMultilineString(0)

	case c == 't' && p.peekString("true"):
		p.pos += 4
		return true, nil

	case c == 'f' && p.peekString("false"):
		p.pos += 5
		return false, nil

	case c == 'n' && p.peekString("null"):
		p.pos += 4
		return nil, nil

	case c == 'n' && p.peekString("nan"):
		p.pos += 3
		return math.NaN(), nil

	case (c == '+' || c == '-'):
		// Is the next char a digit?
		if isDigit(p.peekChar(p.pos + 1)) {
			return p.parseNumber()
		}

		// Are the next chars "inf"?
		p.pos += 1
		if p.peekString("inf") {
			sign := 1
			if c == '-' {
				sign = -1
			}
			p.pos += 3
			return math.Inf(sign), nil
		}

	case c == 'i' && p.peekString("inf"):
		p.pos += 3
		return math.Inf(1), nil

	case isDigit(c):
		return p.parseNumber()

	default:
		return nil, fmt.Errorf("line %d: invalid char '%c'", p.line, c)
	}
	return nil, fmt.Errorf("line %d: invalid char '%c'", p.line, p.data[p.pos])
}

// parseString parses a string from the current position in the data.
// Strings are quoted with double quotes and can contain escaped characters.
func (p *parser) parseString() (string, error) {
	p.advance(1)
	var b strings.Builder

	for !p.done() {
		switch c := p.data[p.pos]; {
		case c == '"':
			p.advance(1)
			return b.String(), nil
		case c == '\\':
			p.advance(1)
			if p.done() {
				return "", fmt.Errorf("line %d: incomplete escape", p.line)
			}
			switch e := p.data[p.pos]; e {
			case '"', '\\', '/':
				b.WriteByte(e)
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
				if p.pos+4 >= len(p.data) {
					return "", fmt.Errorf("line %d: bad unicode escape", p.line)
				}
				hex := p.data[p.pos+1 : p.pos+5]
				code, err := strconv.ParseUint(hex, 16, 32)
				if err != nil {
					return "", fmt.Errorf("line %d: invalid unicode: %s", p.line, hex)
				}
				b.WriteRune(rune(code))
				p.pos += 4
			default:
				return "", fmt.Errorf("line %d: invalid escape '\\%c'", p.line, e)
			}
			p.advance(1)
		default:
			b.WriteByte(c)
			p.advance(1)
		}
	}
	return "", fmt.Errorf("line %d: unclosed string", p.line)
}

// parseMultilineString parses a multi-line string starting with either `"""` or ```.
// """ does not preserve indentation, while ``` preserves indentation of lines.
func (p *parser) parseMultilineString(keyIndent int) (string, error) {
	delim := p.data[p.pos : p.pos+3]
	isPreserve := delim == "```"

	// Advance past the indicator.
	p.pos += 3

	// Validate line ending after delimiter.
	if err := p.validateLineEnding(); err != nil {
		return "", err
	}
	p.skipToNextLine()

	reqIndent := keyIndent + 2
	var lines []string
	var minIndent = -1

	for !p.done() {
		start := p.pos
		for !p.done() && p.data[p.pos] != '\n' {
			p.pos++
		}

		line := p.data[start:p.pos]

		if !p.done() {
			p.advance(1)
		}

		if trim := strings.TrimSpace(line); trim == delim {
			break
		}

		if isPreserve {
			if len(line) >= reqIndent && isSpaceString(line[:reqIndent]) {
				line = line[reqIndent:]
			}
		} else if strings.TrimSpace(line) != "" {
			indent := 0
			for _, r := range line {
				if r != ' ' {
					break
				}
				indent++
			}

			if minIndent == -1 || indent < minIndent {
				minIndent = indent
			}
		}
		lines = append(lines, line)
	}

	if !isPreserve && minIndent > 0 {
		for i, line := range lines {
			if len(line) >= minIndent && strings.TrimSpace(line) != "" {
				lines[i] = line[minIndent:]
			}
		}
	}
	return strings.Join(lines, "\n"), nil
}

// parseNumber parses a number from the current position in the data.
func (p *parser) parseNumber() (any, error) {
	start := p.pos
	if p.data[p.pos] == '+' || p.data[p.pos] == '-' {
		p.advance(1)
	}

	if p.pos < len(p.data) && p.data[p.pos] == '0' && p.pos+2 < len(p.data) {
		switch p.data[p.pos+1] {
		case 'x', 'X':
			return p.parseBase(start, 16, "0x")

		case 'o', 'O':
			return p.parseBase(start, 8, "0o")

		case 'b', 'B':
			return p.parseBase(start, 2, "0b")
		}
	}

	hasDecimal, hasExponent := false, false
loop:
	for !p.done() {
		switch c := p.data[p.pos]; {
		case isDigit(c) || c == '_':
		case c == '.' && !hasDecimal && !hasExponent:
			hasDecimal = true
		case (c == 'e' || c == 'E') && !hasExponent:
			hasExponent = true
			if p.pos+1 < len(p.data) && (p.data[p.pos+1] == '+' || p.data[p.pos+1] == '-') {
				p.advance(1)
			}

		default:
			break loop
		}

		p.advance(1)
	}

	// Strip optional underscores from the number string (eg: 123_456).
	numStr := stripNumUnderscores(p.data[start:p.pos])
	if hasDecimal || hasExponent {
		return strconv.ParseFloat(numStr, 64)
	}

	return strconv.ParseInt(numStr, 10, 64)
}

// parseBase parses a number in a specific base (2, 8, or 16) with an optional prefix.
func (p *parser) parseBase(start int, base int, prefix string) (int64, error) {
	p.pos += len(prefix)
	for !p.done() {
		c := p.data[p.pos]
		if (base == 16 && !isHex(c)) || (base == 8 && !(c >= '0' && c <= '7')) || (base == 2 && !(c == '0' || c == '1')) {
			break
		}
		p.advance(1)
	}

	// Strip optional underscores from the number string (eg: 123_456).
	numStr := stripNumUnderscores(p.data[start+len(prefix) : p.pos])
	return strconv.ParseInt(numStr, base, 64)
}

// validateLineEnding checks for trailing spaces or comments at the end of a line.
func (p *parser) validateLineEnding() error {
	// Skip spaces and check for trailing spaces or the presence of a comment.
	spaceStart := p.pos
	p.skipSpaces()

	// Line ends in trailing spaces.
	if p.done() || p.data[p.pos] == '\n' {
		if p.pos > spaceStart {
			return fmt.Errorf("line %d: trailing spaces not allowed", p.line)
		}
		return nil
	}

	// It's a comment.
	if p.data[p.pos] == '#' {
		// # should always be preceded by a space.
		if p.pos == spaceStart {
			return fmt.Errorf("line %d: comment must be preceded by a space", p.line)
		}

		return p.validateComment()
	}

	return fmt.Errorf("line %d: unexpected content after value", p.line)
}

// validateComment checks for valid comment syntax and trailing spaces.
func (p *parser) validateComment() error {
	p.pos++

	// # Should immediately be followed by a space or newline.
	if !p.done() && p.data[p.pos] != ' ' && p.data[p.pos] != '\n' {
		return fmt.Errorf("line %d: comment must have space after #", p.line)
	}

	// Check for trailing spaces.
	for !p.done() && p.data[p.pos] != '\n' {
		p.pos++
	}
	if p.pos > 0 && p.data[p.pos-1] == ' ' {
		return fmt.Errorf("line %d: trailing spaces not allowed", p.line)
	}

	return nil
}

// skipSpaces skips over spaces in the stream.
func (p *parser) skipSpaces() {
	for !p.done() && p.data[p.pos] == ' ' {
		p.advance(1)
	}
}

// skipSpacesAndComments skips over spaces and comments in the stream.
func (p *parser) skipSpacesAndComments() error {
	for !p.done() {
		switch p.data[p.pos] {
		case ' ':
			// Check if this space extends to end of line (trailing space)
			pos := p.pos
			for pos < len(p.data) && p.data[pos] == ' ' {
				pos++
			}

			// Disallow trailing spaces.
			if pos >= len(p.data) || p.data[pos] == '\n' {
				return fmt.Errorf("line %d: trailing spaces not allowed", p.line)
			}

			p.advance(1)

		case '\n':
			p.advance(1)
			p.line++

		case '#':
			// Check if this is a comment-only line (starts at the beginning or after spaces).
			isLineComment := true
			pos := p.pos - 1
			for pos >= 0 && p.data[pos] != '\n' {
				if p.data[pos] != ' ' {
					isLineComment = false
					break
				}

				pos--
			}

			// If it's a comment only line, stricly validate spaces still.
			if isLineComment {
				p.pos++ // Skip the # char.
				if !p.done() && p.data[p.pos] != ' ' && p.data[p.pos] != '\n' {
					return fmt.Errorf("line %d: comment must have space after #", p.line)
				}

				// Check for trailing spaces in the comment.
				commentStart := p.pos
				for !p.done() && p.data[p.pos] != '\n' {
					p.pos++
				}
				if p.pos > commentStart && p.data[p.pos-1] == ' ' {
					return fmt.Errorf("line %d: trailing spaces not allowed", p.line)
				}

				// Skip newline.
				if !p.done() {
					p.pos++
					p.line++
				}
			} else {
				// Inline comment, just skip to the end of the line.
				p.skipToNextLine()
			}
		default:
			return nil
		}
	}
	return nil
}

func (p *parser) skipToNextLine() {
	for !p.done() && p.data[p.pos] != '\n' {
		p.advance(1)
	}

	if !p.done() && p.data[p.pos] == '\n' {
		p.advance(1)
		p.line++
	}
}

func (p *parser) getCurIndent() int {
	save := p.pos
	for save > 0 && p.data[save-1] != '\n' {
		save--
	}

	indent := 0
	for save+indent < len(p.data) && p.data[save+indent] == ' ' {
		indent++
	}

	return indent
}

func (p *parser) isKeyStart() bool {
	return !p.done() && (p.data[p.pos] == '"' || isAlpha(p.data[p.pos]) || p.data[p.pos] == '_')
}

func (p *parser) done() bool {
	return p.pos >= len(p.data)
}

func (p *parser) advance(n int) {
	p.pos += n
}

func (p *parser) peekString(s string) bool {
	end := p.pos + len(s)
	return end <= len(p.data) && p.data[p.pos:end] == s
}

func (p *parser) peekChar(pos int) byte {
	if len(p.data) < pos {
		return 0
	}

	return p.data[pos]
}

func setValue(dst, src any) error {
	switch d := dst.(type) {
	case *any:
		*d = src
	case *map[string]any:
		if m, ok := src.(map[string]any); ok {
			*d = m
			return nil
		}
		return errors.New("cannot assign non-map to map")

	case *[]any:
		if s, ok := src.([]any); ok {
			*d = s
			return nil
		}
		return errors.New("cannot assign non-slice to slice")

	case *string:
		if s, ok := src.(string); ok {
			*d = s
			return nil
		}
		return errors.New("cannot assign non-string to string")

	case *int:
		if n, ok := src.(int64); ok {
			*d = int(n)
			return nil
		}
		return errors.New("cannot assign non-int to int")

	case *int64:
		if n, ok := src.(int64); ok {
			*d = n
			return nil
		}
		return errors.New("cannot assign non-int64 to int64")

	case *float64:
		switch s := src.(type) {
		case float64:
			*d = s
		case int64:
			*d = float64(s)
		default:
			return errors.New("cannot assign non-number to float64")
		}
		return nil

	case *bool:
		if b, ok := src.(bool); ok {
			*d = b
			return nil
		}
		return errors.New("cannot assign non-bool to bool")
	default:
		return fmt.Errorf("unsupported type: %T", dst)
	}

	return nil
}

// Helper functions
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
	for _, r := range s {
		if r != ' ' {
			return false
		}
	}
	return true
}

func stripNumUnderscores(s string) string {
	return strings.ReplaceAll(s, "_", "")
}

// Example usage and test
func main() {
	humlData, err := os.ReadFile("test.huml")
	if err != nil {
		panic(err)
	}

	var result any
	if err := Unmarshal([]byte(humlData), &result); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	b, err := json.Marshal(result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(string(b))
}
