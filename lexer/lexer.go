// Package lexer implements the TAC Language tokenizer.
//
// The lexer converts .tac source text into a stream of tokens
// consumed by the parser. It handles:
//   - Identifiers (including dotted, e.g. llm.chat)
//   - String literals with escape sequences
//   - Numeric literals (integers and floats)
//   - Boolean literals (true/false)
//   - Operators and delimiters (->, =, :, {, }, (, ), [, ], .)
//   - Comments (// ...)
//   - Newlines (significant in TAC for statement separation)
//   - Reserved words (remember, recall, flow, node, etc.)
package lexer

import (
	"fmt"
	"strconv"
	"strings"
)

// TokenType represents a lexical token category.
type TokenType int

//go:generate stringer -type=TokenType
const (
	EOF TokenType = iota
	Ident
	String
	Number
	True
	False
	Arrow       // ->
	Assign      // <-
	Equals      // =
	EqEq        // ==
	NotEq       // !=
	Greater     // >
	Less        // <
	GreaterEq   // >=
	LessEq      // <=
	Dot
	Comma
	Colon
	LBrace
	RBrace
	LParen
	RParen
	LBrack
	RBrack
	At     // @ (version qualifier)
	Newline
	Comment
	Error
	// v0.5 (LAYA). Appended after Error so every v0.4 token value is unchanged.
	Star   // * (wildcard branch in gate[*])
	DotDot // .. (numeric range in gate[10..50])
)

// Token represents a single lexical token with source location.
type Token struct {
	Type  TokenType
	Value string
	Line  int
	Col   int
}

// String returns a human-readable representation of the token.
func (t Token) String() string {
	switch t.Type {
	case EOF:
		return "EOF"
	case Newline:
		return "\\n"
	case Comment:
		return "//"
	case Error:
		return fmt.Sprintf("ERROR(%s)", t.Value)
	default:
		if t.Value != "" {
			return fmt.Sprintf("%q", t.Value)
		}
		return tokenTypeName(t.Type)
	}
}

func tokenTypeName(t TokenType) string {
	switch t {
	case Ident:
		return "ident"
	case String:
		return "string"
	case Number:
		return "number"
	case True:
		return "true"
	case False:
		return "false"
	case Arrow:
		return "->"
	case Assign:
		return "<-"
	case Equals:
		return "="
	case EqEq:
		return "=="
	case NotEq:
		return "!="
	case Greater:
		return ">"
	case Less:
		return "<"
	case GreaterEq:
		return ">="
	case LessEq:
		return "<="
	case Dot:
		return "."
	case Comma:
		return ","
	case Colon:
		return ":"
	case LBrace:
		return "{"
	case RBrace:
		return "}"
	case LParen:
		return "("
	case RParen:
		return ")"
	case LBrack:
		return "["
	case RBrack:
		return "]"
	case At:
		return "@"
	case Star:
		return "*"
	case DotDot:
		return ".."
	default:
		return fmt.Sprintf("token(%d)", t)
	}
}

// Keywords recognized by the lexer.
var keywords = map[string]TokenType{
	"true":  True,
	"false": False,
}

// ReservedWords are identifiers that have special meaning in TAC.
// They are tokenized as Ident but recognized by the parser.
var ReservedWords = map[string]bool{
	"remember":       true,
	"recall":         true,
	"forget":         true,
	"context":        true,
	"auto_summarize": true,
	"on":             true,
	"input":          true,
	"agent":          true,
	"if":             true,
	"else":           true,
	"search_hybrid":  true,
	"set_token_limit": true,
	"swarm_teach":    true,
	"agent_task":     true,
	"agent_wait":     true,
	"swarm_status":   true,
	"verify":         true,
	"validate":       true,
	"config_get":     true,
	"config_set":     true,
	"graph_search":   true,
	"graph_relate":   true,
	"memory_search":  true,
	"memory_store":   true,
	"web_search":     true,
	"llm":            true,
	"tts":            true,
	"each":           true,
	"in":             true,
}

// Lexer converts TAC source text into a token stream.
type Lexer struct {
	source []rune
	pos    int
	line   int
	col    int
	tokens []Token
	glued  []Glued // v0.5: numbers glued to identifier text
}

// New creates a new Lexer for the given source text.
func New(source string) *Lexer {
	return &Lexer{
		source: []rune(source),
		pos:    0,
		line:   1,
		col:    1,
		tokens: make([]Token, 0),
	}
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.source) {
		return 0
	}
	return l.source[l.pos]
}

func (l *Lexer) peekN(n int) rune {
	if l.pos+n >= len(l.source) {
		return 0
	}
	return l.source[l.pos+n]
}

func (l *Lexer) advance() rune {
	if l.pos >= len(l.source) {
		return 0
	}
	ch := l.source[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return ch
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.source) {
		ch := l.source[l.pos]
		if ch == ' ' || ch == '\t' || ch == '\r' {
			l.advance()
		} else {
			break
		}
	}
}

func (l *Lexer) emit(typ TokenType, val string) {
	l.tokens = append(l.tokens, Token{
		Type:  typ,
		Value: val,
		Line:  l.line,
		Col:   l.col,
	})
}

func (l *Lexer) scanString() error {
	var buf strings.Builder
	startLine := l.line
	for l.pos < len(l.source) {
		ch := l.advance()
		if ch == '\\' && l.pos < len(l.source) {
			next := l.advance()
			switch next {
			case 'n':
				buf.WriteByte('\n')
			case 't':
				buf.WriteByte('\t')
			case 'r':
				buf.WriteByte('\r')
			case '\\':
				buf.WriteByte('\\')
			case '"':
				buf.WriteByte('"')
			case '$':
				// String interpolation: ${...}
				buf.WriteString("${")
				braceDepth := 0
				for l.pos < len(l.source) {
					c := l.advance()
					if c == '{' {
						braceDepth++
					} else if c == '}' {
						if braceDepth == 0 {
							buf.WriteByte('}')
							break
						}
						braceDepth--
					} else if c == '\n' {
						return fmt.Errorf("unterminated string interpolation at line %d", l.line)
					}
					buf.WriteRune(c)
				}
			default:
				buf.WriteByte('\\')
				buf.WriteRune(next)
			}
		} else if ch == '"' {
			l.emit(String, buf.String())
			return nil
		} else if ch == '\n' {
			return fmt.Errorf("unterminated string starting at line %d", startLine)
		} else {
			buf.WriteRune(ch)
		}
	}
	return fmt.Errorf("unterminated string starting at line %d", startLine)
}

func (l *Lexer) scanIdentifier() {
	start := l.pos - 1
	for l.pos < len(l.source) {
		ch := l.source[l.pos]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '_' || ch == '.' {
			l.advance()
		} else {
			break
		}
	}
	word := string(l.source[start:l.pos])
	if kw, ok := keywords[word]; ok {
		l.emit(kw, word)
		return
	}
	l.emit(Ident, word)
}

func (l *Lexer) scanNumber() {
	start := l.pos - 1
	isFloat := false
	for l.pos < len(l.source) {
		ch := l.source[l.pos]
		if ch >= '0' && ch <= '9' {
			l.advance()
		} else if ch == '.' && !isFloat && l.pos+1 < len(l.source) &&
			l.source[l.pos+1] >= '0' && l.source[l.pos+1] <= '9' {
			isFloat = true
			l.advance()
		} else {
			break
		}
	}
	// v0.5: an exponent (`1e3`, `2.5E-4`, `1e+21`) is part of the number —
	// valid JSON number syntax. v0.4 read `1e3` as 1 followed by `e3`.
	if l.pos < len(l.source) && (l.source[l.pos] == 'e' || l.source[l.pos] == 'E') {
		j := l.pos + 1
		if j < len(l.source) && (l.source[j] == '+' || l.source[j] == '-') {
			j++
		}
		if j < len(l.source) && isDigit(l.source[j]) {
			for l.pos < j {
				l.advance()
			}
			for l.pos < len(l.source) && isDigit(l.source[l.pos]) {
				l.advance()
			}
		}
	}
	lit := string(l.source[start:l.pos])
	l.emit(Number, lit)
	// Any other text glued to the number (`3x`, `2.5kg`, `1_000`) still lexes
	// exactly as in v0.4 — the number, then the rest — but is recorded, so
	// the analyzer can warn (TAC-PARSE-001) instead of staying silent.
	if l.pos < len(l.source) && isIdentStart(l.source[l.pos]) {
		end := l.pos
		for end < len(l.source) && (isIdentStart(l.source[end]) || isDigit(l.source[end]) || l.source[end] == '.') {
			end++
		}
		// Col: where the number starts (a number never spans lines and is
		// ASCII), not the column after it that tokens carry.
		l.glued = append(l.glued, Glued{Line: l.line, Col: l.col - len(lit), Number: lit, Rest: string(l.source[l.pos:end])})
	}
}

// Glued is a number immediately followed by identifier text (`3x`): v0.4
// semantics are kept (two tokens), and the parser reports it. Line and Col
// are where the number starts.
type Glued struct {
	Line, Col int
	Number    string
	Rest      string
}

// Glued returns the numbers Scan found glued to identifier text.
func (l *Lexer) Glued() []Glued { return l.glued }

func isIdentStart(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' }

// Scan converts the source into a slice of tokens.
// It returns an error if tokenization fails (e.g. unterminated string).
func (l *Lexer) Scan() ([]Token, error) {
	for l.pos < len(l.source) {
		ch := l.peek()
		switch {
		case ch == ' ' || ch == '\t' || ch == '\r':
			l.advance()
		case ch == '\n':
			l.advance()
			l.skipWhitespace()
			l.emit(Newline, "\n")
		case ch == '"':
			l.advance()
			if err := l.scanString(); err != nil {
				l.emit(Error, err.Error())
				return l.tokens, err
			}
		case ch == '/' && l.pos+1 < len(l.source) && l.source[l.pos+1] == '/':
			// Comment — consume until end of line
			for l.pos < len(l.source) && l.source[l.pos] != '\n' {
				l.advance()
			}
			l.emit(Comment, "")
		case ch == '-' && l.pos+1 < len(l.source) && l.source[l.pos+1] == '>':
			l.advance()
			l.advance()
			l.emit(Arrow, "->")
		case ch == '-' && l.pos+1 < len(l.source) && isDigit(l.source[l.pos+1]) && !l.prevIsOperand():
			// v0.5: a negative number literal. Only where a value can start —
			// TAC has no subtraction, so after an operand '-' stays an error.
			l.advance()
			l.scanNumber()
		case ch == '<' && l.pos+1 < len(l.source) && l.source[l.pos+1] == '-':
			l.advance()
			l.advance()
			l.emit(Assign, "<-")
		case ch == '=' && l.pos+1 < len(l.source) && l.source[l.pos+1] == '=':
			l.advance()
			l.advance()
			l.emit(EqEq, "==")
		case ch == '!':
			if l.pos+1 < len(l.source) && l.source[l.pos+1] == '=' {
				l.advance()
				l.advance()
				l.emit(NotEq, "!=")
			} else {
				l.emit(Error, fmt.Sprintf("unexpected character: %c (0x%02X)", ch, ch))
				l.advance()
				return l.tokens, fmt.Errorf("unexpected character %q at line %d, col %d (did you mean '!='?)", ch, l.line, l.col-1)
			}
		case ch == '>' && l.pos+1 < len(l.source) && l.source[l.pos+1] == '=':
			l.advance()
			l.advance()
			l.emit(GreaterEq, ">=")
		case ch == '>':
			l.advance()
			l.emit(Greater, ">")
		case ch == '<' && l.pos+1 < len(l.source) && l.source[l.pos+1] == '=':
			l.advance()
			l.advance()
			l.emit(LessEq, "<=")
		case ch == '<':
			l.advance()
			l.emit(Less, "<")
		case ch == '=':
			l.advance()
			l.emit(Equals, "=")
		case ch == ',':
			l.advance()
			l.emit(Comma, ",")
		case ch == ':':
			l.advance()
			l.emit(Colon, ":")
		case ch == '{':
			l.advance()
			l.emit(LBrace, "{")
		case ch == '}':
			l.advance()
			l.emit(RBrace, "}")
		case ch == '(':
			l.advance()
			l.emit(LParen, "(")
		case ch == ')':
			l.advance()
			l.emit(RParen, ")")
		case ch == '[':
			l.advance()
			l.emit(LBrack, "[")
		case ch == ']':
			l.advance()
			l.emit(RBrack, "]")
		case ch == '@':
			l.advance()
			l.emit(At, "@")
		case ch == '.' && l.pos+1 < len(l.source) && l.source[l.pos+1] == '.':
			l.advance()
			l.advance()
			l.emit(DotDot, "..")
		case ch == '.':
			l.advance()
			l.emit(Dot, ".")
		case ch == '*':
			l.advance()
			l.emit(Star, "*")
		case (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_':
			l.advance()
			l.scanIdentifier()
		case ch >= '0' && ch <= '9':
			l.advance()
			l.scanNumber()
		default:
			l.emit(Error, fmt.Sprintf("unexpected character: %c (0x%02X)", ch, ch))
			l.advance()
			return l.tokens, fmt.Errorf("unexpected character %q at line %d, col %d", ch, l.line, l.col-1)
		}
	}
	l.emit(EOF, "")
	return l.tokens, nil
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

// prevIsOperand reports whether the last significant token ends a value, in
// which case a following '-' cannot start a negative number.
func (l *Lexer) prevIsOperand() bool {
	for i := len(l.tokens) - 1; i >= 0; i-- {
		switch l.tokens[i].Type {
		case Comment:
			continue
		case Ident, Number, String, True, False, RParen, RBrack, RBrace:
			return true
		default:
			return false
		}
	}
	return false
}

// HasErrors checks if any token in the stream is an error.
func HasErrors(tokens []Token) bool {
	for _, tok := range tokens {
		if tok.Type == Error {
			return true
		}
	}
	return false
}

// TokenValue converts a token value to its Go representation.
func TokenValue(tok Token) (interface{}, error) {
	switch tok.Type {
	case String:
		return tok.Value, nil
	case Number:
		if strings.ContainsAny(tok.Value, ".eE") {
			return strconv.ParseFloat(tok.Value, 64)
		}
		return strconv.Atoi(tok.Value)
	case True:
		return true, nil
	case False:
		return false, nil
	case Ident:
		return tok.Value, nil
	default:
		return tok.Value, nil
	}
}
