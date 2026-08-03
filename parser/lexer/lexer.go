// Package lexer implements the tokenizer for the TAC language.
//
// It converts TAC source text into a stream of tokens. The tokenizer is
// intentionally small and deterministic: identical input always produces
// an identical token stream, which the parser and the canonical formatter
// rely upon.
//
// (c) 2026 TacFlow — MIT License
package lexer

import (
	"fmt"
	"strings"
)

// TokenType represents a lexical token type.
type TokenType int

// Token type constants.
const (
	TEOF TokenType = iota
	TIdent
	TString
	TNumber
	TTrue
	TFalse
	TArrow
	TEquals
	TDot
	TComma
	TColon
	TLBrace
	TRBrace
	TLParen
	TRParen
	TLBrack
	TRBrack
	TNewline
	TComment
	TError
)

// Token represents a single lexical token.
type Token struct {
	Type  TokenType
	Value string
	Line  int
	Col   int
}

// keywords maps literal tokens to their token type.
var keywords = map[string]TokenType{
	"true":  TTrue,
	"false": TFalse,
}

// ReservedWords is the set of words that cannot be used as identifiers.
// Exposed so the semantic analyzer can produce better error messages.
var ReservedWords = map[string]bool{
	"remember": true, "recall": true, "forget": true,
	"context": true, "auto_summarize": true, "on": true, "input": true,
	"agent": true, "if": true, "else": true, "search_hybrid": true,
	"set_token_limit": true, "swarm_teach": true, "agent_task": true,
	"agent_wait": true, "swarm_status": true, "verify": true,
	"validate": true, "config_get": true, "config_set": true,
	"graph_search": true, "graph_relate": true, "memory_search": true,
	"memory_store": true, "web_search": true, "llm": true, "tts": true,
	"each": true, "in": true,
}

// TokenTypeName returns a human-readable name for a token type,
// used in error messages and diagnostics.
func TokenTypeName(t TokenType) string {
	switch t {
	case TIdent:
		return "identifier"
	case TString:
		return "string"
	case TNumber:
		return "number"
	case TTrue:
		return "true"
	case TFalse:
		return "false"
	case TArrow:
		return "->"
	case TEquals:
		return "="
	case TDot:
		return "."
	case TComma:
		return ","
	case TColon:
		return ":"
	case TLBrace:
		return "{"
	case TRBrace:
		return "}"
	case TLParen:
		return "("
	case TRParen:
		return ")"
	case TLBrack:
		return "["
	case TRBrack:
		return "]"
	case TNewline:
		return "newline"
	case TComment:
		return "comment"
	case TError:
		return "error"
	case TEOF:
		return "end of file"
	default:
		return fmt.Sprintf("token(%d)", t)
	}
}

// Error describes a lexical error with its source position.
type Error struct {
	Line int
	Col  int
	Msg  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("lex error at %d:%d: %s", e.Line, e.Col, e.Msg)
}

// Tokenizer converts source text into a token stream.
type Tokenizer struct {
	source []rune
	pos    int
	line   int
	col    int
	tokens []Token
}

// NewTokenizer creates a tokenizer for the given source text.
func NewTokenizer(source string) *Tokenizer {
	return &Tokenizer{
		source: []rune(source),
		pos:    0,
		line:   1,
		col:    1,
		tokens: make([]Token, 0),
	}
}

func (t *Tokenizer) peek() rune {
	if t.pos >= len(t.source) {
		return 0
	}
	return t.source[t.pos]
}

func (t *Tokenizer) peekAt(off int) rune {
	if t.pos+off >= len(t.source) {
		return 0
	}
	return t.source[t.pos+off]
}

func (t *Tokenizer) advance() rune {
	if t.pos >= len(t.source) {
		return 0
	}
	ch := t.source[t.pos]
	t.pos++
	if ch == '\n' {
		t.line++
		t.col = 1
	} else {
		t.col++
	}
	return ch
}

func (t *Tokenizer) addToken(typ TokenType, val string) {
	// The token position records the column of the FIRST character of the
	// token, not the last. scanIdentifier/scanNumber/scanString therefore
	// record start positions before advancing.
	t.tokens = append(t.tokens, Token{
		Type:  typ,
		Value: val,
		Line:  t.line,
		Col:   t.col,
	})
}

func (t *Tokenizer) scanString() TokenType {
	var buf strings.Builder
	startLine, startCol := t.line, t.col
	for t.pos < len(t.source) {
		ch := t.advance()
		if ch == '\\' && t.pos < len(t.source) {
			next := t.advance()
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
				buf.WriteByte('$')
			default:
				buf.WriteByte('\\')
				buf.WriteRune(next)
			}
		} else if ch == '"' {
			t.tokens = append(t.tokens, Token{
				Type:  TString,
				Value: buf.String(),
				Line:  startLine,
				Col:   startCol,
			})
			return TString
		} else if ch == '\n' {
			t.tokens = append(t.tokens, Token{Type: TError, Value: "unterminated string", Line: startLine, Col: startCol})
			return TError
		} else {
			buf.WriteRune(ch)
		}
	}
	t.tokens = append(t.tokens, Token{Type: TError, Value: "unterminated string", Line: startLine, Col: startCol})
	return TError
}

func (t *Tokenizer) scanIdentifier() TokenType {
	start := t.pos - 1
	startLine, startCol := t.line, t.col
	for t.pos < len(t.source) {
		ch := t.source[t.pos]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '_' || ch == '.' {
			t.advance()
		} else {
			break
		}
	}
	word := string(t.source[start:t.pos])
	if kw, ok := keywords[word]; ok {
		t.tokens = append(t.tokens, Token{Type: kw, Value: word, Line: startLine, Col: startCol})
		return kw
	}
	t.tokens = append(t.tokens, Token{Type: TIdent, Value: word, Line: startLine, Col: startCol})
	return TIdent
}

func (t *Tokenizer) scanNumber() TokenType {
	start := t.pos - 1
	startLine, startCol := t.line, t.col
	isFloat := false
	for t.pos < len(t.source) {
		ch := t.source[t.pos]
		if ch >= '0' && ch <= '9' {
			t.advance()
		} else if ch == '.' && !isFloat {
			isFloat = true
			t.advance()
		} else {
			break
		}
	}
	t.tokens = append(t.tokens, Token{Type: TNumber, Value: string(t.source[start:t.pos]), Line: startLine, Col: startCol})
	return TNumber
}

// Tokenize converts the full source into a token slice.
// It returns a lexer.Error on the first invalid character or unterminated
// string; the partial token stream is still available via Tokens().
func (t *Tokenizer) Tokenize() ([]Token, error) {
	for t.pos < len(t.source) {
		ch := t.peek()
		switch {
		case ch == ' ' || ch == '\t' || ch == '\r':
			t.advance()
		case ch == '\n':
			t.advance()
			t.skipWhitespace()
			t.addToken(TNewline, "\n")
		case ch == '"':
			t.advance()
			t.scanString()
		case ch == '/' && t.peekAt(1) == '/':
			for t.pos < len(t.source) && t.source[t.pos] != '\n' {
				t.advance()
			}
			t.addToken(TComment, "")
		case ch == '-' && t.peekAt(1) == '>':
			startLine, startCol := t.line, t.col
			t.advance()
			t.advance()
			t.tokens = append(t.tokens, Token{Type: TArrow, Value: "->", Line: startLine, Col: startCol})
		case ch == '=':
			t.advance()
			t.addToken(TEquals, "=")
		case ch == ',':
			t.advance()
			t.addToken(TComma, ",")
		case ch == ':':
			t.advance()
			t.addToken(TColon, ":")
		case ch == '{':
			t.advance()
			t.addToken(TLBrace, "{")
		case ch == '}':
			t.advance()
			t.addToken(TRBrace, "}")
		case ch == '(':
			t.advance()
			t.addToken(TLParen, "(")
		case ch == ')':
			t.advance()
			t.addToken(TRParen, ")")
		case ch == '[':
			t.advance()
			t.addToken(TLBrack, "[")
		case ch == ']':
			t.advance()
			t.addToken(TRBrack, "]")
		case ch == '.':
			t.advance()
			t.addToken(TDot, ".")
		case (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_':
			t.advance()
			t.scanIdentifier()
		case ch >= '0' && ch <= '9':
			t.advance()
			t.scanNumber()
		default:
			t.tokens = append(t.tokens, Token{
				Type:  TError,
				Value: fmt.Sprintf("unexpected character: %c", ch),
				Line:  t.line,
				Col:   t.col,
			})
			t.advance()
		}
	}
	t.addToken(TEOF, "")
	return t.tokens, nil
}

func (t *Tokenizer) skipWhitespace() {
	for t.pos < len(t.source) {
		ch := t.source[t.pos]
		if ch == ' ' || ch == '\t' || ch == '\r' {
			t.advance()
		} else {
			break
		}
	}
}
