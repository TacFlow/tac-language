package lexer

import (
	"testing"
)

func TestLexer_SimpleTokens(t *testing.T) {
	tests := []struct {
		input    string
		expected []TokenType
	}{
		{"flow", []TokenType{Ident, EOF}},
		{"\"hello\"", []TokenType{String, EOF}},
		{"42", []TokenType{Number, EOF}},
		{"3.14", []TokenType{Number, EOF}},
		{"true", []TokenType{True, EOF}},
		{"false", []TokenType{False, EOF}},
		{"->", []TokenType{Arrow, EOF}},
		{"<-", []TokenType{Assign, EOF}},
		{"{", []TokenType{LBrace, EOF}},
		{"}", []TokenType{RBrace, EOF}},
		{"(", []TokenType{LParen, EOF}},
		{")", []TokenType{RParen, EOF}},
		{"[", []TokenType{LBrack, EOF}},
		{"]", []TokenType{RBrack, EOF}},
		{">", []TokenType{Greater, EOF}},
		{"<", []TokenType{Less, EOF}},
		{">=", []TokenType{GreaterEq, EOF}},
		{"<=", []TokenType{LessEq, EOF}},
		{"==", []TokenType{EqEq, EOF}},
		{"!=", []TokenType{NotEq, EOF}},
		{":", []TokenType{Colon, EOF}},
		{",", []TokenType{Comma, EOF}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := New(tt.input)
			tokens, err := l.Scan()
			if err != nil {
				t.Fatalf("Scan() error: %v", err)
			}
			if len(tokens) != len(tt.expected) {
				t.Errorf("expected %d tokens, got %d", len(tt.expected), len(tokens))
				for i, tok := range tokens {
					t.Logf("  [%d] %v", i, tok)
				}
				return
			}
			for i, exp := range tt.expected {
				if tokens[i].Type != exp {
					t.Errorf("token[%d]: expected %v, got %v", i, exp, tokens[i].Type)
				}
			}
		})
	}
}

func TestLexer_FlowKeyword(t *testing.T) {
	input := `flow "Test" {
  node "a" -> skill web_search(query: "hello")
}`
	l := New(input)
	tokens, err := l.Scan()
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if HasErrors(tokens) {
		for _, tok := range tokens {
			if tok.Type == Error {
				t.Errorf("Error token: %v", tok)
			}
		}
	}
	// Verify we got an EOF
	if tokens[len(tokens)-1].Type != EOF {
		t.Errorf("expected EOF, got %v", tokens[len(tokens)-1].Type)
	}
}

func TestLexer_ComparisonOps(t *testing.T) {
	input := "verify.confidence > 0.9"
	l := New(input)
	tokens, err := l.Scan()
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if HasErrors(tokens) {
		t.Errorf("unexpected errors: %v", tokens)
	}
	// verify.confidence = Ident
	// > = Greater
	// 0.9 = Number
	if tokens[0].Type != Ident {
		t.Errorf("token[0] expected Ident, got %v", tokens[0])
	}
	if tokens[0].Value != "verify.confidence" {
		t.Errorf("token[0] expected 'verify.confidence', got %q", tokens[0].Value)
	}
	if tokens[1].Type != Greater {
		t.Errorf("token[1] expected Greater, got %v", tokens[1])
	}
	if tokens[2].Type != Number {
		t.Errorf("token[2] expected Number, got %v", tokens[2])
	}
}

func TestLexer_Comments(t *testing.T) {
	input := "// this is a comment\nflow"
	l := New(input)
	tokens, err := l.Scan()
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	// Comment, Newline, Ident(flow), EOF
	if tokens[0].Type != Comment {
		t.Errorf("token[0] expected Comment, got %v", tokens[0])
	}
	if tokens[2].Type != Ident || tokens[2].Value != "flow" {
		t.Errorf("token[2] expected Ident(flow), got %v", tokens[2])
	}
}
