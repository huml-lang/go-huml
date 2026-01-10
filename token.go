package huml

import "fmt"

// TokenType represents the type of a lexical token in HUML.
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenError

	// Structural tokens.
	TokenNewline // End of logical line (carries indent of next line).

	// Key tokens.
	TokenKey       // Bare key: alphanumeric with - and _.
	TokenQuotedKey // Quoted key: "...".

	// Indicator tokens.
	TokenScalarInd // ':' scalar indicator.
	TokenVectorInd // '::' vector indicator.

	// Value tokens.
	TokenString // Quoted or multiline string value.
	TokenInt    // Integer value (decimal, hex, octal, binary).
	TokenFloat  // Float value (including scientific notation).
	TokenBool   // true or false.
	TokenNull   // null.
	TokenNaN    // nan.
	TokenInf    // inf, +inf, -inf.

	// Collection tokens.
	TokenEmptyList // [].
	TokenEmptyDict // {}.
	TokenListItem  // '-' list item marker.
	TokenComma     // ',' inline separator.
)

// Token represents a lexical token from HUML input.
type Token struct {
	Type        TokenType
	Value       string // Raw string value (for strings, numbers, keys).
	Line        int    // Line number (1-based).
	Column      int    // Column position (0-based).
	Indent      int    // Indentation level at start of line (in spaces).
	SpaceBefore bool   // True if whitespace preceded this token.
}

// String returns a human-readable representation of the token.
func (t Token) String() string {
	switch t.Type {
	case TokenEOF:
		return "EOF"
	case TokenError:
		return fmt.Sprintf("Error(%s)", t.Value)
	case TokenNewline:
		return fmt.Sprintf("Newline(indent=%d)", t.Indent)
	case TokenKey:
		return fmt.Sprintf("Key(%s)", t.Value)
	case TokenQuotedKey:
		return fmt.Sprintf("QuotedKey(%s)", t.Value)
	case TokenScalarInd:
		return ":"
	case TokenVectorInd:
		return "::"
	case TokenString:
		return fmt.Sprintf("String(%q)", t.Value)
	case TokenInt:
		return fmt.Sprintf("Int(%s)", t.Value)
	case TokenFloat:
		return fmt.Sprintf("Float(%s)", t.Value)
	case TokenBool:
		return fmt.Sprintf("Bool(%s)", t.Value)
	case TokenNull:
		return "Null"
	case TokenNaN:
		return "NaN"
	case TokenInf:
		return fmt.Sprintf("Inf(%s)", t.Value)
	case TokenEmptyList:
		return "[]"
	case TokenEmptyDict:
		return "{}"
	case TokenListItem:
		return "-"
	case TokenComma:
		return ","
	default:
		return fmt.Sprintf("Unknown(%d)", t.Type)
	}
}
