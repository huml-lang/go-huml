package huml

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

// lexer tokenizes HUML input from an io.Reader.
type lexer struct {
	r *bufio.Reader

	line           []byte  // Current line being processed.
	lineBuf        []byte  // Reusable buffer for reading lines.
	lineNum        int     // Current line number (1-based).
	pos            int     // Position within current line.
	eof            bool    // True if EOF reached.
	err            error   // First error encountered.
	tokens         []Token // Token buffer for lookahead.
	tokPos         int     // Current position in token buffer.
	atLineStart    bool    // True if at start of line (for indent tracking).
	curIndent      int     // Indentation of current line.
	hadSpaceBefore bool    // True if space was skipped before last scanned token.
	inMultilineStr bool    // True if currently parsing multiline string content.
	strBuf         []byte  // Reusable buffer for building strings.
}

// Pre-defined keyword byte slices to avoid allocations during lexing.
var (
	kwTrue  = []byte("true")
	kwFalse = []byte("false")
	kwNull  = []byte("null")
	kwNaN   = []byte("nan")
	kwInf   = []byte("inf")
)

// newLexer creates a new lexer that reads from r.
func newLexer(r io.Reader) *lexer {
	return &lexer{
		r:           bufio.NewReader(r),
		lineNum:     0,
		atLineStart: true,
		lineBuf:     make([]byte, 0, 256),
		strBuf:      make([]byte, 0, 64),
	}
}

// next returns the next token, consuming it.
func (l *lexer) next() (Token, error) {
	if l.tokPos < len(l.tokens) {
		tk := l.tokens[l.tokPos]
		l.tokPos++
		if l.tokPos == len(l.tokens) {
			l.tokens = l.tokens[:0]
			l.tokPos = 0
		}

		return tk, nil
	}

	return l.scan()
}

// peek returns the next token without consuming it.
func (l *lexer) peek() (Token, error) {
	if l.tokPos < len(l.tokens) {
		return l.tokens[l.tokPos], nil
	}

	tok, err := l.scan()
	if err != nil {
		return Token{Type: TokenError, Value: err.Error()}, err
	}

	l.tokens = append(l.tokens, tok)
	return tok, nil
}

// scan reads the next token from input.
func (l *lexer) scan() (Token, error) {
	if l.err != nil {
		return Token{Type: TokenError, Value: l.err.Error()}, l.err
	}

	// Read a new line if needed.
	for l.line == nil || l.pos >= len(l.line) {
		if l.eof {
			return Token{Type: TokenEOF, Line: l.lineNum}, nil
		}

		if err := l.readLine(); err != nil {
			if err == io.EOF {
				l.eof = true
				return Token{Type: TokenEOF, Line: l.lineNum}, nil
			}
			l.err = err

			return Token{Type: TokenError, Value: err.Error()}, err
		}

		l.atLineStart = true
		l.curIndent = l.countIndent()
		l.pos = l.curIndent
	}

	// Skip blank lines and comment-only lines.
	for l.line != nil && l.pos < len(l.line) {
		if l.line[l.pos] == '#' {
			// Comment - validate and skip line.
			if err := l.validateComment(); err != nil {
				return Token{Type: TokenError}, err
			}

			// Read next line.
			if l.eof {
				return Token{Type: TokenEOF, Line: l.lineNum}, nil
			}
			if err := l.readLine(); err != nil {
				if err == io.EOF {
					l.eof = true
					return Token{Type: TokenEOF, Line: l.lineNum}, nil
				}
				l.err = err
				return Token{Type: TokenError}, err
			}

			l.atLineStart = true
			l.curIndent = l.countIndent()
			l.pos = l.curIndent
			continue
		}
		break
	}

	// Check if line is now empty.
	if l.line == nil || l.pos >= len(l.line) {
		if l.eof {
			return Token{Type: TokenEOF, Line: l.lineNum}, nil
		}

		return l.scan()
	}

	return l.scanToken()
}

// readLine reads the next line from input, reusing the internal buffer.
func (l *lexer) readLine() error {
	// Reuse the line buffer.
	l.lineBuf = l.lineBuf[:0]

	for {
		b, err := l.r.ReadByte()
		if err != nil {
			if err == io.EOF {
				if len(l.lineBuf) == 0 {
					return io.EOF
				}
				// EOF with data - process as final line.
				l.eof = true
				break
			}
			return err
		}
		if b == '\n' {
			break
		}
		l.lineBuf = append(l.lineBuf, b)
	}

	l.lineNum++
	l.line = l.lineBuf
	l.pos = 0

	// Validate: check for trailing spaces on the line.
	// Skip this check when inside multiline strings (trailing spaces are content there).
	if !l.inMultilineStr && len(l.line) > 0 && l.line[len(l.line)-1] == ' ' {
		return l.errorf("trailing spaces are not allowed")
	}

	return nil
}

// countIndent counts leading spaces in the current line.
func (l *lexer) countIndent() int {
	indent := 0
	for indent < len(l.line) && l.line[indent] == ' ' {
		indent++
	}

	return indent
}

// validateComment validates a comment on the current line.
func (l *lexer) validateComment() error {
	if l.pos >= len(l.line) || l.line[l.pos] != '#' {
		return nil
	}

	// Check for space after #.
	if l.pos+1 < len(l.line) && l.line[l.pos+1] != ' ' && l.line[l.pos+1] != '\n' {
		return l.errorf("comment hash '#' must be followed by a space")
	}

	// Check for trailing spaces in comment.
	if len(l.line) > 0 && l.line[len(l.line)-1] == ' ' {
		return l.errorf("trailing spaces are not allowed")
	}

	return nil
}

// scanToken scans the next token from the current position.
func (l *lexer) scanToken() (Token, error) {
	startCol := l.pos

	// Skip spaces between tokens and track if we skipped any.
	l.hadSpaceBefore = false
	for l.pos < len(l.line) && l.line[l.pos] == ' ' {
		l.hadSpaceBefore = true
		l.pos++
	}

	if l.pos >= len(l.line) {
		// End of line - read next line.
		if l.eof {
			return Token{Type: TokenEOF, Line: l.lineNum}, nil
		}
		return l.scan()
	}

	startCol = l.pos
	c := l.line[l.pos]

	// Check for version directive at start of document.
	if l.lineNum == 1 && l.pos == 0 && l.peekString("%HUML") {
		return l.scanVersion()
	}

	// List item marker: "- " at start of content.
	if c == '-' && l.pos == l.curIndent {
		if l.pos+1 < len(l.line) && l.line[l.pos+1] == ' ' {
			l.pos += 2
			return Token{
				Type:   TokenListItem,
				Line:   l.lineNum,
				Column: startCol,
				Indent: l.curIndent,
			}, nil
		}
	}

	// Empty markers.
	if l.peekString("[]") {
		l.pos += 2
		return Token{
			Type:   TokenEmptyList,
			Line:   l.lineNum,
			Column: startCol,
			Indent: l.curIndent,
		}, nil
	}

	if l.peekString("{}") {
		l.pos += 2
		return Token{
			Type:   TokenEmptyDict,
			Line:   l.lineNum,
			Column: startCol,
			Indent: l.curIndent,
		}, nil
	}

	// Quoted string or key.
	if c == '"' {
		// Check for multiline string marker.
		if l.peekString(`"""`) {
			return Token{
				Type:   TokenString,
				Value:  `"""`,
				Line:   l.lineNum,
				Column: startCol,
				Indent: l.curIndent,
			}, nil
		}
		return l.scanKeyOrString()
	}

	// Bare key or keyword.
	if isAlpha(c) {
		return l.scanKeyOrKeyword()
	}

	// Indicators.
	if c == ':' {
		if l.pos+1 < len(l.line) && l.line[l.pos+1] == ':' {
			l.pos += 2
			return Token{
				Type:   TokenVectorInd,
				Line:   l.lineNum,
				Column: startCol,
				Indent: l.curIndent,
			}, nil
		}
		l.pos++
		return Token{
			Type:   TokenScalarInd,
			Line:   l.lineNum,
			Column: startCol,
			Indent: l.curIndent,
		}, nil
	}

	// Comma.
	if c == ',' {
		l.pos++
		return Token{
			Type:        TokenComma,
			Line:        l.lineNum,
			Column:      startCol,
			Indent:      l.curIndent,
			SpaceBefore: l.hadSpaceBefore,
		}, nil
	}

	// Number or special float.
	if isDigit(c) || c == '+' || c == '-' {
		return l.scanNumber()
	}

	return Token{Type: TokenError}, l.errorf("unexpected character '%c'", c)
}

// scanVersion scans the %HUML version directive.
func (l *lexer) scanVersion() (Token, error) {
	l.pos += len("%HUML")

	// Skip optional space and version.
	if l.pos < len(l.line) && l.line[l.pos] == ' ' {
		l.pos++
		// Skip version string.
		for l.pos < len(l.line) && l.line[l.pos] != ' ' && l.line[l.pos] != '#' {
			l.pos++
		}
	}

	// Validate rest of line.
	if err := l.validateRemaining(); err != nil {
		return Token{Type: TokenError}, err
	}

	// Move to next line.
	l.line = nil
	return l.scan()
}

// validateRemaining checks for trailing content/spaces and consumes the line.
func (l *lexer) validateRemaining() error {
	// Skip spaces.
	spaceStart := l.pos
	for l.pos < len(l.line) && l.line[l.pos] == ' ' {
		l.pos++
	}

	if l.pos >= len(l.line) {
		// End of line - check for trailing spaces.
		if l.pos > spaceStart {
			return l.errorf("trailing spaces are not allowed")
		}
		return nil
	}

	if l.line[l.pos] == '#' {
		// Comment - validate it.
		return l.validateComment()
	}

	return l.errorf("unexpected content at end of line")
}

// scanKeyOrString scans a quoted string, determining if it's a key or value.
func (l *lexer) scanKeyOrString() (Token, error) {
	startCol := l.pos
	str, err := l.scanQuotedString()
	if err != nil {
		return Token{Type: TokenError}, err
	}

	// Skip spaces after the string.
	for l.pos < len(l.line) && l.line[l.pos] == ' ' {
		l.pos++
	}

	// Check if followed by ':' (it's a key).
	if l.pos < len(l.line) && l.line[l.pos] == ':' {
		return Token{
			Type:   TokenQuotedKey,
			Value:  str,
			Line:   l.lineNum,
			Column: startCol,
			Indent: l.curIndent,
		}, nil
	}

	// It's a string value.
	return Token{
		Type:   TokenString,
		Value:  str,
		Line:   l.lineNum,
		Column: startCol,
		Indent: l.curIndent,
	}, nil
}

// scanQuotedString scans a double-quoted string with escapes.
func (l *lexer) scanQuotedString() (string, error) {
	l.pos++ // Consume opening quote.

	// Fast path: check if string has no escapes.
	start := l.pos
	hasEscape := false
	for i := l.pos; i < len(l.line); i++ {
		if l.line[i] == '"' {
			if !hasEscape {
				// No escapes - return substring directly.
				l.pos = i + 1
				return string(l.line[start:i]), nil
			}
			break
		}
		if l.line[i] == '\\' {
			hasEscape = true
			i++ // Skip next char.
		}
	}

	// Slow path: has escapes, use buffer.
	l.strBuf = l.strBuf[:0]
	for l.pos < len(l.line) {
		c := l.line[l.pos]
		if c == '"' {
			l.pos++
			return string(l.strBuf), nil
		}
		if c == '\\' {
			l.pos++
			if l.pos >= len(l.line) {
				return "", l.errorf("incomplete escape sequence")
			}

			switch esc := l.line[l.pos]; esc {
			case '"', '\\', '/':
				l.strBuf = append(l.strBuf, esc)
			case 'b':
				l.strBuf = append(l.strBuf, '\b')
			case 'f':
				l.strBuf = append(l.strBuf, '\f')
			case 'n':
				l.strBuf = append(l.strBuf, '\n')
			case 'r':
				l.strBuf = append(l.strBuf, '\r')
			case 't':
				l.strBuf = append(l.strBuf, '\t')
			case 'v':
				l.strBuf = append(l.strBuf, '\v')
			default:
				return "", l.errorf("invalid escape character '\\%c'", esc)
			}
		} else {
			l.strBuf = append(l.strBuf, c)
		}
		l.pos++
	}

	return "", l.errorf("unclosed string")
}

// scanKeyOrKeyword scans a bare identifier.
func (l *lexer) scanKeyOrKeyword() (Token, error) {
	startCol := l.pos
	start := l.pos

	for l.pos < len(l.line) && (isAlphaNum(l.line[l.pos]) || l.line[l.pos] == '_' || l.line[l.pos] == '-') {
		l.pos++
	}

	wb := l.line[start:l.pos]

	// Skip spaces after word.
	for l.pos < len(l.line) && l.line[l.pos] == ' ' {
		l.pos++
	}

	// If followed by ':', it's a key.
	if l.pos < len(l.line) && l.line[l.pos] == ':' {
		return Token{
			Type:   TokenKey,
			Value:  string(wb),
			Line:   l.lineNum,
			Column: startCol,
			Indent: l.curIndent,
		}, nil
	}

	// Check for keywords using pre-defined byte slices (no allocation).
	var (
		tkType TokenType
		tkVal  string
	)

	switch {
	case bytes.Equal(wb, kwTrue):
		tkType, tkVal = TokenBool, "true"
	case bytes.Equal(wb, kwFalse):
		tkType, tkVal = TokenBool, "false"
	case bytes.Equal(wb, kwNull):
		tkType, tkVal = TokenNull, "null"
	case bytes.Equal(wb, kwNaN):
		tkType, tkVal = TokenNaN, "nan"
	case bytes.Equal(wb, kwInf):
		tkType, tkVal = TokenInf, "+"
	default:
		return Token{Type: TokenError}, l.errorf("unquoted string '%s' is not allowed", string(wb))
	}

	return Token{
		Type:   tkType,
		Value:  tkVal,
		Line:   l.lineNum,
		Column: startCol,
		Indent: l.curIndent,
	}, nil
}

// scanNumber scans a numeric literal.
func (l *lexer) scanNumber() (Token, error) {
	startCol := l.pos
	start := l.pos

	// Handle signs.
	if l.line[l.pos] == '+' || l.line[l.pos] == '-' {
		sign := l.line[l.pos]
		l.pos++

		// Check for +-inf.
		if l.peekString("inf") {
			l.pos += 3
			signStr := "+"
			if sign == '-' {
				signStr = "-"
			}
			return Token{
				Type:   TokenInf,
				Value:  signStr,
				Line:   l.lineNum,
				Column: startCol,
				Indent: l.curIndent,
			}, nil
		}

		if l.pos >= len(l.line) || !isDigit(l.line[l.pos]) {
			return Token{Type: TokenError}, l.errorf("invalid char after '%c'", sign)
		}
	}

	// Check for base prefixes.
	if l.line[l.pos] == '0' && l.pos+1 < len(l.line) {
		switch l.line[l.pos+1] {
		case 'x', 'X':
			return l.scanBaseNumber(start, startCol, isHex)
		case 'o', 'O':
			return l.scanBaseNumber(start, startCol, isOctal)
		case 'b', 'B':
			return l.scanBaseNumber(start, startCol, isBinary)
		}
	}

	// Decimal number.
	isFloat := false
	for l.pos < len(l.line) {
		c := l.line[l.pos]
		if isDigit(c) || c == '_' {
			l.pos++
		} else if c == '.' {
			isFloat = true
			l.pos++
		} else if c == 'e' || c == 'E' {
			isFloat = true
			l.pos++
			if l.pos < len(l.line) && (l.line[l.pos] == '+' || l.line[l.pos] == '-') {
				l.pos++
			}
		} else {
			break
		}
	}

	numStr := string(l.line[start:l.pos])
	if isFloat {
		return Token{
			Type:   TokenFloat,
			Value:  numStr,
			Line:   l.lineNum,
			Column: startCol,
			Indent: l.curIndent,
		}, nil
	}

	return Token{
		Type:   TokenInt,
		Value:  numStr,
		Line:   l.lineNum,
		Column: startCol,
		Indent: l.curIndent,
	}, nil
}

// scanBaseNumber scans a number with a base prefix (0x, 0o, 0b).
func (l *lexer) scanBaseNumber(start, startCol int, isValidDigit func(byte) bool) (Token, error) {
	l.pos += 2
	numStart := l.pos

	for l.pos < len(l.line) && (isValidDigit(l.line[l.pos]) || l.line[l.pos] == '_') {
		l.pos++
	}

	if l.pos == numStart {
		return Token{Type: TokenError}, l.errorf("invalid number literal, requires digits after prefix")
	}

	return Token{
		Type:   TokenInt,
		Value:  string(l.line[start:l.pos]),
		Line:   l.lineNum,
		Column: startCol,
		Indent: l.curIndent,
	}, nil
}

// scanMultilineString scans a multiline string starting with """.
// Per the v0.2.0 spec, the content block must be indented by one level (2 spaces)
// relative to the key. These initial 2 spaces on each line are stripped.
// All other preceding and trailing spaces are preserved as content.
func (l *lexer) scanMultilineString(keyIndent int) (Token, error) {
	startLine := l.lineNum
	startCol := l.pos
	l.pos += 3 // Consume """

	// Rest of line after """ must be empty or comment.
	if err := l.validateRemaining(); err != nil {
		return Token{Type: TokenError}, err
	}

	// Reuse strBuf as content buffer.
	l.strBuf = l.strBuf[:0]
	// Always strip keyIndent + 2 spaces.
	reqIndent := keyIndent + 2

	// Set flag to allow trailing spaces in content lines.
	l.inMultilineStr = true
	defer func() { l.inMultilineStr = false }()

	for {
		if err := l.readLine(); err != nil {
			if err == io.EOF {
				l.eof = true
				return Token{Type: TokenError}, fmt.Errorf(
					"line %d: unclosed multiline string",
					startLine,
				)
			}

			return Token{Type: TokenError}, err
		}

		lineIndent := l.countIndent()
		l.pos = lineIndent

		if l.peekString(`"""`) {
			if lineIndent != keyIndent {
				return Token{Type: TokenError}, l.errorf(
					"multiline closing delimiter must be at same indentation as the key (%d spaces)",
					keyIndent,
				)
			}
			l.pos += 3

			if err := l.validateRemaining(); err != nil {
				return Token{Type: TokenError}, l.errorf(
					"invalid content after multiline string closing delimiter",
				)
			}

			l.line = nil
			l.atLineStart = true

			// Trim trailing newline.
			result := l.strBuf
			if len(result) > 0 && result[len(result)-1] == '\n' {
				result = result[:len(result)-1]
			}
			return Token{
				Type:   TokenString,
				Value:  string(result),
				Line:   startLine,
				Column: startCol,
				Indent: keyIndent,
			}, nil
		}

		// Strip the required indentation (keyIndent + 2 spaces).
		lineContent := l.line
		if len(lineContent) >= reqIndent && isSpaceBytes(lineContent[:reqIndent]) {
			lineContent = lineContent[reqIndent:]
		}
		l.strBuf = append(l.strBuf, lineContent...)
		l.strBuf = append(l.strBuf, '\n')
	}
}

// consumeLine validates rest of line and moves to next line.
func (l *lexer) consumeLine() error {
	// Skip spaces.
	spaceStart := l.pos
	for l.pos < len(l.line) && l.line[l.pos] == ' ' {
		l.pos++
	}

	if l.pos >= len(l.line) {
		if l.pos > spaceStart {
			return l.errorf("trailing spaces are not allowed")
		}
		l.line = nil
		return nil
	}

	if l.line[l.pos] == '#' {
		if err := l.validateComment(); err != nil {
			return err
		}
		l.line = nil
		return nil
	}

	return l.errorf("unexpected content at end of line")
}

// peekString checks if the given string is at the current position.
func (l *lexer) peekString(s string) bool {
	if l.pos+len(s) > len(l.line) {
		return false
	}

	for i := 0; i < len(s); i++ {
		if l.line[l.pos+i] != s[i] {
			return false
		}
	}

	return true
}

// errorf creates an error with line number.
func (l *lexer) errorf(format string, args ...any) error {
	return fmt.Errorf("line %d: "+format, append([]any{l.lineNum}, args...)...)
}

// currentIndent returns the indentation of the current line.
func (l *lexer) currentIndent() int {
	return l.curIndent
}

// atEndOfLine returns true if at end of logical content on line.
func (l *lexer) atEndOfLine() bool {
	// Skip spaces.
	pos := l.pos
	for pos < len(l.line) && l.line[pos] == ' ' {
		pos++
	}
	return pos >= len(l.line) || l.line[pos] == '#'
}

// skipRequiredSpace consumes exactly one required space.
func (l *lexer) skipRequiredSpace(context string) error {
	if l.pos >= len(l.line) || l.line[l.pos] != ' ' {
		return l.errorf("expected single space %s", context)
	}
	l.pos++
	if l.pos < len(l.line) && l.line[l.pos] == ' ' {
		return l.errorf("expected single space %s, found multiple", context)
	}
	return nil
}
