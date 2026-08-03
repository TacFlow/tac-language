package lexer_test

import (
	"testing"

	"github.com/tacflow1-tech/tac-language/parser/lexer"
)

func TestTokenizer_BasicTokens(t *testing.T) {
	src := `flow "test" { node "a" -> skill foo(bar: 42) }`
	tok := lexer.NewTokenizer(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedTypes := []lexer.TokenType{
		lexer.TIdent,  // flow
		lexer.TString, // "test"
		lexer.TLBrace, // {
		lexer.TIdent,  // node
		lexer.TString, // "a"
		lexer.TArrow,  // ->
		lexer.TIdent,  // skill
		lexer.TIdent,  // foo
		lexer.TLParen, // (
		lexer.TIdent,  // bar
		lexer.TColon,  // :
		lexer.TNumber, // 42
		lexer.TRParen, // )
		lexer.TRBrace, // }
		lexer.TEOF,
	}

	if len(tokens) < len(expectedTypes) {
		t.Fatalf("expected at least %d tokens, got %d", len(expectedTypes), len(tokens))
	}
	for i, exp := range expectedTypes {
		if tokens[i].Type != exp {
			t.Errorf("token %d: expected %s, got %s (%q)", i, lexer.TokenTypeName(exp), lexer.TokenTypeName(tokens[i].Type), tokens[i].Value)
		}
	}
}

func TestTokenizer_BooleansKeywords(t *testing.T) {
	src := `true false`
	tok := lexer.NewTokenizer(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokens[0].Type != lexer.TTrue || tokens[0].Value != "true" {
		t.Errorf("expected TTrue, got %v", tokens[0])
	}
	if tokens[1].Type != lexer.TFalse || tokens[1].Value != "false" {
		t.Errorf("expected TFalse, got %v", tokens[1])
	}
}

func TestTokenizer_ObjectLiteral(t *testing.T) {
	src := `{ host: "localhost", port: 8080 }`
	tok := lexer.NewTokenizer(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []lexer.TokenType{
		lexer.TLBrace, lexer.TIdent, lexer.TColon, lexer.TString,
		lexer.TComma, lexer.TIdent, lexer.TColon, lexer.TNumber,
		lexer.TRBrace, lexer.TEOF,
	}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d", len(expected), len(tokens))
	}
	for i, exp := range expected {
		if tokens[i].Type != exp {
			t.Errorf("token %d: expected %s, got %s", i, lexer.TokenTypeName(exp), lexer.TokenTypeName(tokens[i].Type))
		}
	}
}

func TestTokenizer_Arrow(t *testing.T) {
	src := `a -> b`
	tok := lexer.NewTokenizer(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokens[1].Type != lexer.TArrow {
		t.Errorf("expected TArrow, got %v", tokens[1])
	}
}

func TestTokenizer_Comments(t *testing.T) {
	src := "// this is a comment\nremember x = 1"
	tok := lexer.NewTokenizer(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Comment token is consumed; we verify the next substantive token is TIdent("remember")
	found := false
	for _, tok := range tokens {
		if tok.Type == lexer.TIdent && tok.Value == "remember" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'remember' token after comment")
	}
}

func TestTokenizer_UnterminatedString(t *testing.T) {
	src := `"hello`
	tok := lexer.NewTokenizer(src)
	tokens, err := tok.Tokenize()
	// unclosed string should produce TError
	hasErr := false
	for _, tok := range tokens {
		if tok.Type == lexer.TError {
			hasErr = true
			break
		}
	}
	if !hasErr {
		t.Error("expected error token for unterminated string")
	}
	_ = err
}

func TestTokenizer_EscapedString(t *testing.T) {
	src := `"hello\nworld\t!"`
	tok := lexer.NewTokenizer(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokens[0].Type != lexer.TString {
		t.Fatalf("expected string token")
	}
	if tokens[0].Value != "hello\nworld\t!" {
		t.Errorf("expected escaped string, got %q", tokens[0].Value)
	}
}

func TestTokenizer_MultiLineFlow(t *testing.T) {
	src := `flow "Web Q&A" {
  input question: Untrusted
  node "search" -> skill web_search(query: question)
  search -> synthesize
  on "init" -> search
}`
	tok := lexer.NewTokenizer(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify we get a valid token stream (no errors)
	for _, tok := range tokens {
		if tok.Type == lexer.TError {
			t.Errorf("unexpected error token: %v", tok)
		}
	}
}

func TestTokenizer_FloatNumber(t *testing.T) {
	src := `weight: 0.95`
	tok := lexer.NewTokenizer(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokens[2].Type != lexer.TNumber {
		t.Fatalf("expected number, got %v", tokens[2])
	}
	if tokens[2].Value != "0.95" {
		t.Errorf("expected '0.95', got %q", tokens[2].Value)
	}
}

func TestTokenizer_DotIdentifier(t *testing.T) {
	src := `llm.chat memory_search tts.speak`
	tok := lexer.NewTokenizer(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := []string{"llm.chat", "memory_search", "tts.speak"}
	for i, name := range names {
		if tokens[i].Type != lexer.TIdent || tokens[i].Value != name {
			t.Errorf("token %d: expected ident %q, got %v", i, name, tokens[i])
		}
	}
}

func TestTokenizer_InvalidCharacter(t *testing.T) {
	src := "@invalid"
	tok := lexer.NewTokenizer(src)
	tokens, _ := tok.Tokenize()
	hasErr := false
	for _, tok := range tokens {
		if tok.Type == lexer.TError {
			hasErr = true
			break
		}
	}
	if !hasErr {
		t.Error("expected error token for '@' character")
	}
}

func TestTokenizer_LineTracking(t *testing.T) {
	src := "remember x = 1\nremember y = 2"
	tok := lexer.NewTokenizer(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// First token should be line 1
	if tokens[0].Line != 1 {
		t.Errorf("first token line: expected 1, got %d", tokens[0].Line)
	}
	// Find the second "remember" — it should be on line 2
	for i, tok := range tokens {
		if tok.Type == lexer.TIdent && tok.Value == "y" {
			// The ident "y" should be on line 2
			if tokens[i].Line != 2 {
				t.Errorf("'y' token line: expected 2, got %d", tokens[i].Line)
			}
			break
		}
	}
}
