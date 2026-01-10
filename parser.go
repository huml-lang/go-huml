package huml

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// streamParser parses tokens into HUML values.
type streamParser struct {
	lexer *lexer
}

// newStreamParser creates a new parser from a lexer.
func newStreamParser(l *lexer) *streamParser {
	return &streamParser{lexer: l}
}

// parse parses the entire document and returns the result.
func (p *streamParser) parse() (any, error) {
	tk, err := p.lexer.peek()
	if err != nil {
		return nil, err
	}

	if tk.Type == TokenEOF {
		return nil, fmt.Errorf("empty document is undefined")
	}

	// Root element must not be indented.
	if tk.Indent != 0 {
		return nil, fmt.Errorf("line %d: root element must not be indented", tk.Line)
	}

	// Determine root type and parse.
	rootType, err := p.inferRootType()
	if err != nil {
		return nil, err
	}

	var result any
	switch rootType {
	case typeScalar:
		result, err = p.parseRootScalar()
		if err != nil {
			return nil, err
		}
		return p.assertRootEnd(result, "root scalar value")

	case typeEmptyList:
		p.lexer.next()
		if err := p.lexer.consumeLine(); err != nil {
			return nil, err
		}

		return p.assertRootEnd([]any{}, "root list")

	case typeEmptyDict:
		p.lexer.next()
		if err := p.lexer.consumeLine(); err != nil {
			return nil, err
		}
		return p.assertRootEnd(map[string]any{}, "root dict")

	case typeMultilineList:
		return p.parseMultilineList(0)

	case typeMultilineDict:
		return p.parseMultilineDict(0)

	case typeInlineList:
		result, err = p.parseInlineList()
		if err != nil {
			return nil, err
		}
		if err := p.lexer.consumeLine(); err != nil {
			return nil, err
		}

		return p.assertRootEnd(result, "root inline list")

	case typeInlineDict:
		result, err = p.parseInlineDict()
		if err != nil {
			return nil, err
		}
		if err := p.lexer.consumeLine(); err != nil {
			return nil, err
		}

		return p.assertRootEnd(result, "root inline dict")

	default:
		return nil, fmt.Errorf("internal error: unknown root type")
	}
}

// parseRootScalar parses a scalar value at root level.
func (p *streamParser) parseRootScalar() (any, error) {
	tk, err := p.lexer.peek()
	if err != nil {
		return nil, err
	}

	// Check for multiline string.
	if tk.Type == TokenString && tk.Value == `"""` {
		p.lexer.next() // Consume the marker token.
		mlTk, err := p.lexer.scanMultilineString(0)
		if err != nil {
			return nil, err
		}

		return mlTk.Value, nil
	}

	val, err := p.parseInlineValue()
	if err != nil {
		return nil, err
	}

	if err := p.lexer.consumeLine(); err != nil {
		return nil, err
	}

	return val, nil
}

// inferRootType determines the type of the root document.
func (p *streamParser) inferRootType() (dataType, error) {
	tk, err := p.lexer.peek()
	if err != nil {
		return typeScalar, err
	}

	// Check for empty markers.
	if tk.Type == TokenEmptyList {
		return typeEmptyList, nil
	}
	if tk.Type == TokenEmptyDict {
		return typeEmptyDict, nil
	}

	// Check for list item marker.
	if tk.Type == TokenListItem {
		return typeMultilineList, nil
	}

	// Check for key (dict).
	if tk.Type == TokenKey || tk.Type == TokenQuotedKey {
		// Check what follows the key to distinguish:
		// - key:: value  -> multiline dict (even if comma on line, it's inline vector value)
		// - key: val, key: val -> inline dict at root (comma on line, : not ::)
		if p.hasVectorIndicatorAfterKey() {
			return typeMultilineDict, nil
		}

		// Look for comma on line to determine inline vs multiline.
		if p.hasCommaOnLine() {
			return typeInlineDict, nil
		}

		return typeMultilineDict, nil
	}

	// Check for inline list (values followed by comma).
	if isValueToken(tk.Type) || tk.Type == TokenString {
		if p.hasCommaOnLine() {
			return typeInlineList, nil
		}

		return typeScalar, nil
	}

	return typeScalar, nil
}

// hasVectorIndicatorAfterKey checks if the first key on the line is followed by ::.
func (p *streamParser) hasVectorIndicatorAfterKey() bool {
	origPos := p.lexer.pos

	// Scan past the key.
	for p.lexer.pos < len(p.lexer.line) && p.lexer.line[p.lexer.pos] != ':' {
		p.lexer.pos++
	}

	// Check for ::
	result := false
	if p.lexer.pos+1 < len(p.lexer.line) && p.lexer.line[p.lexer.pos] == ':' && p.lexer.line[p.lexer.pos+1] == ':' {
		result = true
	}

	p.lexer.pos = origPos
	return result
}

// hasCommaOnLine checks if there's a comma on the current line.
func (p *streamParser) hasCommaOnLine() bool {
	// Scan through the current line looking for comma (read-only, no state changes).
	for i := p.lexer.pos; i < len(p.lexer.line); i++ {
		if p.lexer.line[i] == ',' {
			return true
		}
	}
	return false
}

// isValueToken returns true if the token type represents a value.
func isValueToken(t TokenType) bool {
	switch t {
	case TokenString, TokenInt, TokenFloat, TokenBool, TokenNull, TokenNaN, TokenInf:
		return true
	}
	return false
}

// assertRootEnd ensures no content follows a completed root element.
func (p *streamParser) assertRootEnd(val any, description string) (any, error) {
	tk, err := p.lexer.peek()
	if err != nil {
		return nil, err
	}
	if tk.Type != TokenEOF {
		return nil, fmt.Errorf("line %d: unexpected content after %s", tk.Line, description)
	}
	return val, nil
}

// parseMultilineDict parses a multi-line dict at a given indentation level.
func (p *streamParser) parseMultilineDict(indent int) (any, error) {
	out := make(map[string]any, 8) // Pre-allocate for common case.

	for {
		tk, err := p.lexer.peek()
		if err != nil {
			return nil, err
		}

		// End conditions.
		if tk.Type == TokenEOF {
			break
		}
		if tk.Indent < indent {
			break
		}

		// Validate indentation.
		if tk.Indent != indent {
			return nil, fmt.Errorf("line %d: bad indent %d, expected %d", tk.Line, tk.Indent, indent)
		}

		// Expect a key.
		if tk.Type != TokenKey && tk.Type != TokenQuotedKey {
			return nil, fmt.Errorf("line %d: invalid character, expected key", tk.Line)
		}

		// Consume key.
		keyTk, _ := p.lexer.next()
		key := keyTk.Value

		if _, exists := out[key]; exists {
			return nil, fmt.Errorf("line %d: duplicate key '%s' in dict", keyTk.Line, key)
		}

		// Expect indicator.
		indTk, err := p.lexer.next()
		if err != nil {
			return nil, err
		}

		var val any
		switch indTk.Type {
		case TokenScalarInd:
			// Check for required space after :.
			if err := p.lexer.skipRequiredSpace("after ':'"); err != nil {
				return nil, err
			}

			// Parse scalar value.
			val, err = p.parseScalarValue(indent)
			if err != nil {
				return nil, err
			}
		case TokenVectorInd:
			// Vector value.
			val, err = p.parseVector(indent + 2)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("line %d: expected ':' or '::' after key", indTk.Line)
		}

		out[key] = val
	}

	return out, nil
}

// parseMultilineList parses a multi-line list at a given indentation level.
func (p *streamParser) parseMultilineList(indent int) (any, error) {
	out := make([]any, 0, 8) // Pre-allocate for common case.

	for {
		tk, err := p.lexer.peek()
		if err != nil {
			return nil, err
		}

		// End conditions.
		if tk.Type == TokenEOF {
			break
		}
		if tk.Indent < indent {
			break
		}

		// Validate indentation.
		if tk.Indent != indent {
			return nil, fmt.Errorf("line %d: bad indent %d, expected %d", tk.Line, tk.Indent, indent)
		}

		// Expect list item marker.
		if tk.Type != TokenListItem {
			break
		}

		// Consume list item marker.
		p.lexer.next()

		// Check for nested vector.
		nextTk, err := p.lexer.peek()
		if err != nil {
			return nil, err
		}

		var val any
		if nextTk.Type == TokenVectorInd {
			p.lexer.next() // Consume ::
			// After "- ::", content is at indent + 2 (one level deeper than list item).
			val, err = p.parseVector(indent + 2)
		} else {
			val, err = p.parseListItemValue(indent)
		}
		if err != nil {
			return nil, err
		}

		out = append(out, val)
	}

	return out, nil
}

// parseListItemValue parses a value after "- ".
func (p *streamParser) parseListItemValue(indent int) (any, error) {
	tk, err := p.lexer.peek()
	if err != nil {
		return nil, err
	}

	// Check for multiline string.
	if tk.Type == TokenString && tk.Value == `"""` {
		p.lexer.next()
		mlTk, err := p.lexer.scanMultilineString(indent)
		if err != nil {
			return nil, err
		}
		return mlTk.Value, nil
	}

	val, err := p.parseInlineValue()
	if err != nil {
		return nil, err
	}

	if err := p.lexer.consumeLine(); err != nil {
		return nil, err
	}

	return val, nil
}

// parseVector parses a vector after the :: indicator.
func (p *streamParser) parseVector(indent int) (any, error) {
	// Check if inline (space follows) or multiline (newline/comment follows).
	if p.lexer.atEndOfLine() {
		// Multiline vector.
		if err := p.lexer.consumeLine(); err != nil {
			return nil, err
		}

		tk, err := p.lexer.peek()
		if err != nil {
			return nil, err
		}

		if tk.Type == TokenEOF || tk.Indent < indent {
			return nil, fmt.Errorf("line %d: ambiguous empty vector after '::'. Use [] or {}.", tk.Line)
		}

		if tk.Type == TokenListItem {
			return p.parseMultilineList(indent)
		}

		return p.parseMultilineDict(indent)
	}

	// Inline vector - skip required space.
	if err := p.lexer.skipRequiredSpace("after '::'"); err != nil {
		return nil, err
	}

	return p.parseInlineVectorValue()
}

// parseInlineVectorValue parses an inline vector ([], {}, or comma-separated values).
func (p *streamParser) parseInlineVectorValue() (any, error) {
	tk, err := p.lexer.peek()
	if err != nil {
		return nil, err
	}

	var val any

	switch tk.Type {
	case TokenEmptyList:
		p.lexer.next()
		val = []any{}
	case TokenEmptyDict:
		p.lexer.next()
		val = map[string]any{}
	case TokenKey, TokenQuotedKey:
		val, err = p.parseInlineDict()
	default:
		val, err = p.parseInlineList()
	}

	if err != nil {
		return nil, err
	}
	if err := p.lexer.consumeLine(); err != nil {
		return nil, err
	}
	return val, nil
}

// parseInlineDict parses an inline dict (key: val, key: val).
func (p *streamParser) parseInlineDict() (map[string]any, error) {
	out := make(map[string]any, 4) // Pre-allocate for common case.
	isFirst := true

	for {
		// Check for end of inline content.
		if p.lexer.atEndOfLine() {
			break
		}

		tk, err := p.lexer.peek()
		if err != nil {
			return nil, err
		}

		if tk.Type == TokenEOF {
			break
		}

		// Handle comma separator.
		if !isFirst {
			if tk.Type != TokenComma {
				break
			}
			// Check for space before comma.
			if tk.SpaceBefore {
				return nil, p.lexer.errorf("no spaces allowed before comma")
			}
			p.lexer.next() // Consume comma.

			// Skip required space after comma.
			if err := p.lexer.skipRequiredSpace("after comma"); err != nil {
				return nil, err
			}

			tk, err = p.lexer.peek()
			if err != nil {
				return nil, err
			}
		}
		isFirst = false

		// Expect key.
		if tk.Type != TokenKey && tk.Type != TokenQuotedKey {
			return nil, fmt.Errorf("line %d: expected key in inline dict", tk.Line)
		}

		keyTk, _ := p.lexer.next()
		key := keyTk.Value

		if _, exists := out[key]; exists {
			return nil, fmt.Errorf("line %d: duplicate key '%s' in dict", keyTk.Line, key)
		}

		// Expect scalar indicator.
		indTk, err := p.lexer.next()
		if err != nil {
			return nil, err
		}
		if indTk.Type != TokenScalarInd {
			return nil, fmt.Errorf("line %d: expected ':' in inline dict", indTk.Line)
		}

		// Skip required space.
		if err := p.lexer.skipRequiredSpace("in inline dict"); err != nil {
			return nil, err
		}

		// Parse value.
		val, err := p.parseInlineValue()
		if err != nil {
			return nil, err
		}

		out[key] = val
	}

	return out, nil
}

// parseInlineList parses an inline list (val, val, val).
func (p *streamParser) parseInlineList() ([]any, error) {
	out := make([]any, 0, 8) // Pre-allocate for common case.
	isFirst := true

	for {
		// Check for end of inline content.
		if p.lexer.atEndOfLine() {
			break
		}

		tk, err := p.lexer.peek()
		if err != nil {
			return nil, err
		}

		if tk.Type == TokenEOF {
			break
		}

		// Handle comma separator.
		if !isFirst {
			if tk.Type != TokenComma {
				break
			}
			// Check for space before comma.
			if tk.SpaceBefore {
				return nil, p.lexer.errorf("no spaces allowed before comma")
			}
			p.lexer.next() // Consume comma.

			// Skip required space after comma.
			if err := p.lexer.skipRequiredSpace("after comma"); err != nil {
				return nil, err
			}
		}
		isFirst = false

		// Parse value.
		val, err := p.parseInlineValue()
		if err != nil {
			return nil, err
		}

		out = append(out, val)
	}

	return out, nil
}

// parseInlineValue parses a single value in an inline context.
func (p *streamParser) parseInlineValue() (any, error) {
	tk, err := p.lexer.next()
	if err != nil {
		return nil, err
	}

	return p.tokenToValue(tk)
}

// parseScalarValue parses a scalar value (handles multiline strings).
func (p *streamParser) parseScalarValue(keyIndent int) (any, error) {
	tk, err := p.lexer.peek()
	if err != nil {
		return nil, err
	}

	// Check for multiline string.
	if tk.Type == TokenString && tk.Value == `"""` {
		p.lexer.next() // Consume the marker.
		mlTk, err := p.lexer.scanMultilineString(keyIndent)
		if err != nil {
			return nil, err
		}
		return mlTk.Value, nil
	}

	val, err := p.parseInlineValue()
	if err != nil {
		return nil, err
	}

	if err := p.lexer.consumeLine(); err != nil {
		return nil, err
	}

	return val, nil
}

// tokenToValue converts a token to its Go value.
func (p *streamParser) tokenToValue(tok Token) (any, error) {
	switch tok.Type {
	case TokenString:
		return tok.Value, nil

	case TokenInt:
		return p.parseIntValue(tok.Value)

	case TokenFloat:
		return p.parseFloatValue(tok.Value)

	case TokenBool:
		return tok.Value == "true", nil

	case TokenNull:
		return nil, nil

	case TokenNaN:
		return math.NaN(), nil

	case TokenInf:
		if tok.Value == "-" {
			return math.Inf(-1), nil
		}
		return math.Inf(1), nil

	case TokenEOF:
		return nil, fmt.Errorf("unexpected end of input, expected a value")

	case TokenError:
		return nil, fmt.Errorf("%s", tok.Value)

	default:
		return nil, fmt.Errorf("line %d: unexpected token %s when parsing value", tok.Line, tok.String())
	}
}

// parseIntValue parses an integer value from string.
func (p *streamParser) parseIntValue(s string) (int64, error) {
	// Handle sign.
	sign := int64(1)
	idx := 0
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		if s[0] == '-' {
			sign = -1
		}
		idx = 1
	}

	// Handle base prefixes.
	base := 10
	if len(s)-idx > 2 {
		prefix := s[idx : idx+2]
		switch prefix {
		case "0x", "0X":
			base = 16
			idx += 2
		case "0o", "0O":
			base = 8
			idx += 2
		case "0b", "0B":
			base = 2
			idx += 2
		}
	}

	// Parse digits, skipping underscores inline.
	var val int64
	for i := idx; i < len(s); i++ {
		c := s[i]
		if c == '_' {
			continue
		}

		var digit int64
		switch {
		case c >= '0' && c <= '9':
			digit = int64(c - '0')
		case c >= 'a' && c <= 'f':
			digit = int64(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			digit = int64(c - 'A' + 10)
		default:
			return 0, fmt.Errorf("invalid digit '%c'", c)
		}

		if digit >= int64(base) {
			return 0, fmt.Errorf("invalid digit '%c' for base %d", c, base)
		}

		val = val*int64(base) + digit
	}

	return sign * val, nil
}

// parseFloatValue parses a float value from string, skipping underscores.
func (p *streamParser) parseFloatValue(s string) (float64, error) {
	if strings.Contains(s, "_") {
		s = strings.ReplaceAll(s, "_", "")
	}
	return strconv.ParseFloat(s, 64)
}
