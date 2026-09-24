package huml

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const maxDepth = 1000

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
		result, err = p.parseMultilineList(0)
		if err != nil {
			return nil, err
		}
		return p.assertRootEnd(result, "root list")

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
	if tk.Type == tokenMultilineString {
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
		hasComma, err := p.hasCommaOnLine()
		if err != nil {
			return typeScalar, err
		}
		if hasComma {
			return typeInlineDict, nil
		}

		return typeMultilineDict, nil
	}

	// Check for inline list (values followed by comma).
	if isValueToken(tk.Type) || tk.Type == TokenString {
		hasComma, err := p.hasCommaOnLine()
		if err != nil {
			return typeScalar, err
		}
		if hasComma {
			return typeInlineList, nil
		}

		return typeScalar, nil
	}

	return typeScalar, nil
}

// hasVectorIndicatorAfterKey checks if the first key on the line is followed by ::.
func (p *streamParser) hasVectorIndicatorAfterKey() bool {
	return p.lexer.peekString("::")
}

// hasCommaOnLine looks for separator tokens without consuming parser input.
func (p *streamParser) hasCommaOnLine() (bool, error) {
	look := *p.lexer
	look.tokens = nil
	look.tokPos = 0
	look.strBuf = nil
	for !look.atEndOfLine() {
		tk, err := look.scanToken()
		if err != nil {
			return false, err
		}
		switch tk.Type {
		case TokenComma:
			return true, nil
		case tokenMultilineString:
			return false, nil
		}
	}
	return false, nil
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
			return nil, fmt.Errorf("line %d: expected list item", tk.Line)
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
	if tk.Type == tokenMultilineString {
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
	if indent/2 >= maxDepth {
		return nil, p.lexer.errorf("maximum nesting depth exceeded")
	}
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
	if tk.Type == tokenMultilineString {
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
		v, err := p.parseIntValue(tok.Value)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", tok.Line, err)
		}
		return v, nil

	case TokenFloat:
		v, err := p.parseFloatValue(tok.Value)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", tok.Line, err)
		}
		return v, nil

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
	s = strings.ReplaceAll(s, "_", "")
	sign := ""
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		sign, s = s[:1], s[1:]
	}
	base := 10
	if len(s) >= 2 {
		switch s[:2] {
		case "0x":
			base = 16
		case "0o":
			base = 8
		case "0b":
			base = 2
		}
		if base != 10 {
			s = s[2:]
		}
	}
	return strconv.ParseInt(sign+s, base, 64)
}

// parseFloatValue parses a float value from string, skipping underscores.
func (p *streamParser) parseFloatValue(s string) (float64, error) {
	if strings.Contains(s, "_") {
		s = strings.ReplaceAll(s, "_", "")
	}
	return strconv.ParseFloat(s, 64)
}
