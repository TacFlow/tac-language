package tac_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tacflow1-tech/tac-language/ast"
	"github.com/tacflow1-tech/tac-language/formatter"
	"github.com/tacflow1-tech/tac-language/lexer"
	"github.com/tacflow1-tech/tac-language/manifest"
	"github.com/tacflow1-tech/tac-language/parser"
	"github.com/tacflow1-tech/tac-language/semantic"
	"github.com/tacflow1-tech/tac-language/types"
)

// ============================================================================
// Golden file tests — verify parser output matches known-good AST JSON
// ============================================================================

func TestGoldenFiles(t *testing.T) {
	goldenDir := filepath.Join("testdata")
	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatalf("cannot read testdata dir: %v", err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".golden.json") {
			continue
		}
		tacName := strings.TrimSuffix(entry.Name(), ".golden.json") + ".tac"
		tacPath := filepath.Join("examples", tacName)
		goldenPath := filepath.Join(goldenDir, entry.Name())

		t.Run(tacName, func(t *testing.T) {
			// Read the .tac source
			src, err := os.ReadFile(tacPath)
			if err != nil {
				t.Fatalf("read %s: %v", tacPath, err)
			}

			// Parse
			program, err := parser.ParseSource(string(src))
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			// Marshal to JSON (same format as golden)
			got, err := json.MarshalIndent(program, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			// Read golden
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v", goldenPath, err)
			}

			// Compare as JSON objects (not raw bytes, to allow reordering)
			var gotObj, wantObj interface{}
			if err := json.Unmarshal(got, &gotObj); err != nil {
				t.Fatalf("unmarshal got: %v", err)
			}
			if err := json.Unmarshal(want, &wantObj); err != nil {
				t.Fatalf("unmarshal golden: %v", err)
			}

			// Re-marshal both for normalized comparison
			gotNorm, _ := json.Marshal(gotObj)
			wantNorm, _ := json.Marshal(wantObj)

			if string(gotNorm) != string(wantNorm) {
				t.Errorf("AST mismatch for %s\n--- got:\n%s\n--- want:\n%s",
					tacName, string(got), string(want))
			}
		})
	}
}

// ============================================================================
// Lexer tests — verify all operators and edge cases
// ============================================================================

func TestLexerOperators(t *testing.T) {
	tests := []struct {
		input    string
		expected []lexer.TokenType
	}{
		{"->", []lexer.TokenType{lexer.Arrow, lexer.EOF}},
		{"<-", []lexer.TokenType{lexer.Assign, lexer.EOF}},
		{"==", []lexer.TokenType{lexer.EqEq, lexer.EOF}},
		{"!=", []lexer.TokenType{lexer.NotEq, lexer.EOF}},
		{">", []lexer.TokenType{lexer.Greater, lexer.EOF}},
		{"<", []lexer.TokenType{lexer.Less, lexer.EOF}},
		{">=", []lexer.TokenType{lexer.GreaterEq, lexer.EOF}},
		{"<=", []lexer.TokenType{lexer.LessEq, lexer.EOF}},
		{"x >= 5", []lexer.TokenType{lexer.Ident, lexer.GreaterEq, lexer.Number, lexer.EOF}},
		{"x != y", []lexer.TokenType{lexer.Ident, lexer.NotEq, lexer.Ident, lexer.EOF}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			tokens, err := l.Scan()
			if err != nil {
				t.Fatalf("scan error: %v", err)
			}
			if len(tokens) != len(tt.expected) {
				t.Fatalf("got %d tokens, want %d: %v", len(tokens), len(tt.expected), tokens)
			}
			for i, tok := range tokens {
				if tok.Type != tt.expected[i] {
					t.Errorf("token[%d]: got %v, want %v", i, tok.Type, tt.expected[i])
				}
			}
		})
	}
}

func TestLexerStrings(t *testing.T) {
	tests := []string{
		`"hello"`,
		`"hello\nworld"`,
		`"interpolated ${name} here"`,
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			l := lexer.New(input)
			tokens, err := l.Scan()
			if err != nil {
				t.Fatalf("scan error: %v", err)
			}
			if tokens[0].Type != lexer.String {
				t.Errorf("expected String, got %v", tokens[0].Type)
			}
		})
	}
}

func TestLexerErrorUnterminatedString(t *testing.T) {
	l := lexer.New(`"unterminated`)
	_, err := l.Scan()
	if err == nil {
		t.Fatal("expected error for unterminated string")
	}
}

func TestLexerErrorBang(t *testing.T) {
	l := lexer.New(`!`)
	_, err := l.Scan()
	if err == nil {
		t.Fatal("expected error for lone '!'")
	}
	if !strings.Contains(err.Error(), "!=") {
		t.Errorf("error should suggest '!=', got: %v", err)
	}
}

// ============================================================================
// Parser tests — specific syntax constructs
// ============================================================================

func TestParseNodeWithSkillKeyword(t *testing.T) {
	src := `flow "test" {
  node "x" -> skill foo(query: "hello", count: 3)
}`
	program, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	flows := ast.CollectFlows(program)
	if len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}

	nodes := ast.CollectNodes(flows[0])
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	// The node should have a SkillCall child
	if len(nodes[0].Children) < 2 {
		t.Fatalf("expected at least 2 children (name + skill), got %d", len(nodes[0].Children))
	}

	call := nodes[0].Children[1]
	if call.Type != ast.NodeSkillCall {
		t.Errorf("expected SkillCall, got %s", call.Type)
	}
	if call.Value != "foo" {
		t.Errorf("expected skill name 'foo', got %q", call.Value)
	}
}

func TestParseNodeWithoutSkillKeyword(t *testing.T) {
	src := `flow "test" {
  node "x" -> foo(query: "hello")
}`
	program, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	flows := ast.CollectFlows(program)
	nodes := ast.CollectNodes(flows[0])
	call := nodes[0].Children[1]
	if call.Value != "foo" {
		t.Errorf("expected skill name 'foo', got %q", call.Value)
	}
}

func TestParseNodeDirectEdge(t *testing.T) {
	src := `flow "test" {
  node "a" -> b
  node "b" -> c
}`
	program, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	flows := ast.CollectFlows(program)
	nodes := ast.CollectNodes(flows[0])
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
}

func TestParseEdgeWithCondition(t *testing.T) {
	src := `flow "test" {
  node "a" -> skill foo()
  node "b" -> skill bar()
  a -> b { if: a.confidence > 0.8 }
}`
	program, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	flows := ast.CollectFlows(program)
	edges := ast.CollectEdges(flows[0])
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}

	// The condition is now a NodeCondition AST node: operator + children
	cond, _, hasCond := ast.EdgeCondition(edges[0])
	if !hasCond {
		t.Error("expected edge to have a condition")
	}
	if cond != "a.confidence > 0.8" {
		t.Errorf("expected condition 'a.confidence > 0.8', got %q", cond)
	}

	// Verify the condition children (LHS and RHS)
	if condNode, ok := edges[0].Attrs["if"]; ok && condNode != nil {
		if condNode.Type != ast.NodeCondition {
			t.Errorf("expected NodeCondition, got %s", condNode.Type)
		}
		if len(condNode.Children) < 2 {
			t.Errorf("expected 2 children (lhs, rhs), got %d", len(condNode.Children))
		}
	}
}

func TestParseInput(t *testing.T) {
	src := `flow "test" {
  input question: Untrusted
  node "x" -> skill foo()
}`
	program, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	flows := ast.CollectFlows(program)
	flow := flows[0]
	found := false
	for _, c := range flow.Children {
		if c.Type == ast.NodeInput {
			found = true
			if len(c.Children) < 2 || c.Children[1].Value != "Untrusted" {
				t.Errorf("input type mismatch: %+v", c.Children)
			}
		}
	}
	if !found {
		t.Error("expected Input node in flow")
	}
}

func TestParseTrigger(t *testing.T) {
	src := `flow "test" {
  node "x" -> skill foo()
  on "user_message" -> x
}`
	program, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	flows := ast.CollectFlows(program)
	found := false
	for _, c := range flows[0].Children {
		if c.Type == ast.NodeTrigger {
			found = true
		}
	}
	if !found {
		t.Error("expected Trigger node in flow")
	}
}

// ============================================================================
// Type system tests
// ============================================================================

func TestTrustTypes(t *testing.T) {
	// All trust types should be valid
	for _, tt := range types.ValidTrustTypes() {
		if !types.IsValidTrustType(string(tt)) {
			t.Errorf("IsValidTrustType(%s) should be true", tt)
		}
	}

	// Invalid types
	if types.IsValidTrustType("invalid") {
		t.Error("'invalid' should not be a valid trust type")
	}
}

func TestTrustTypeConversion(t *testing.T) {
	// Untrusted → Fact requires validate()
	reqs := types.RequiresConversion(types.Untrusted, types.Fact)
	if !reqs {
		t.Error("Untrusted → Fact should require conversion")
	}

	// Hallucinable → Fact requires verify()
	fn := types.ConversionFunction(types.Hallucinable, types.Fact)
	if fn != "verify" {
		t.Errorf("expected 'verify', got %q", fn)
	}

	// Secret → anything is forbidden
	_, ok := types.CanConvert(types.Secret, types.Untrusted)
	if ok {
		t.Error("Secret → Untrusted should be forbidden")
	}
}

// ============================================================================
// Semantic analysis tests
// ============================================================================

func TestSemanticValidFlow(t *testing.T) {
	src := `flow "valid" {
  node "a" -> skill web_search(query: "test")
  node "b" -> skill memory_search(query: "test")
  a -> b
}`
	program, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	a := semantic.New()
	diags := a.Analyze(program)
	if a.HasErrors() {
		for _, d := range diags {
			if d.Severity == semantic.SeverityError {
				t.Errorf("unexpected error: %s", d)
			}
		}
	}
}

func TestSemanticUndeclaredNode(t *testing.T) {
	src := `flow "bad" {
  node "a" -> skill web_search(query: "test")
  a -> b
}`
	program, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	a := semantic.New()
	diags := a.Analyze(program)
	found := false
	for _, d := range diags {
		if d.Severity == semantic.SeverityError &&
			strings.Contains(d.Message, "undeclared") {
			found = true
		}
	}
	if !found {
		t.Error("expected undeclared node error, got none")
		t.Logf("diagnostics: %+v", diags)
	}
}

func TestSemanticUnknownSkill(t *testing.T) {
	src := `flow "test" {
  node "a" -> skill nonexistent_skill()
}`
	program, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	a := semantic.New()
	diags := a.Analyze(program)
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "unknown skill") {
			found = true
		}
	}
	if !found {
		t.Error("expected unknown skill warning, got none")
	}
}

// ============================================================================
// Manifest tests
// ============================================================================

func TestManifestExtract(t *testing.T) {
	src := `flow "test" {
  input q: Untrusted
  node "a" -> skill web_search(query: q)
  node "b" -> skill memory_search(query: "x")
  a -> b
}`
	program, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	flows := ast.CollectFlows(program)
	if len(flows) == 0 {
		t.Fatal("no flows")
	}

	m := manifest.ExtractManifest(flows[0])
	if m.NodeCount != 2 {
		t.Errorf("expected 2 nodes, got %d", m.NodeCount)
	}
	if m.EdgeCount != 1 {
		t.Errorf("expected 1 edge, got %d", m.EdgeCount)
	}
	if len(m.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(m.Skills))
	}
	skills := m.UsedSkills()
	if len(skills) != 2 {
		t.Errorf("expected 2 unique skills, got %d: %v", len(skills), skills)
	}
}

// ============================================================================
// Formatter tests — round-trip: parse → format → parse should produce same AST
// ============================================================================

func TestFormatterRoundTrip(t *testing.T) {
	tests := []string{
		`flow "test" {
  input q: Untrusted
  node "a" -> skill web_search(query: q)
}`,
		`flow "edges" {
  node "a" -> skill foo()
  node "b" -> skill bar()
  a -> b { if: a.confidence > 0.5 }
}`,
	}

	for i, src := range tests {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			// Parse original
			prog1, err := parser.ParseSource(src)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			// Format
			formatted := formatter.Format(prog1)

			// Parse formatted
			prog2, err := parser.ParseSource(formatted)
			if err != nil {
				t.Fatalf("re-parse formatted: %v\n--- formatted:\n%s", err, formatted)
			}

			// Compare ASTs ignoring position metadata (positions differ after format)
			equal := astNodesEqual(prog1, prog2)
			if !equal {
				j1, _ := json.MarshalIndent(prog1, "", "  ")
				j2, _ := json.MarshalIndent(prog2, "", "  ")
				t.Errorf("round-trip AST mismatch\n--- original JSON:\n%s\n--- formatted JSON:\n%s\n--- formatted text:\n%s",
					string(j1), string(j2), formatted)
			}
		})
	}
}

// astNodesEqual compares two AST nodes structurally, ignoring positions.
func astNodesEqual(a, b *ast.Node) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Type != b.Type || a.Value != b.Value || a.NumVal != b.NumVal || a.BoolVal != b.BoolVal {
		return false
	}
	if len(a.Children) != len(b.Children) ||
		len(a.Nodes) != len(b.Nodes) ||
		len(a.Edges) != len(b.Edges) ||
		len(a.Args) != len(b.Args) ||
		len(a.Attrs) != len(b.Attrs) ||
		len(a.MapVal) != len(b.MapVal) ||
		len(a.ArrVal) != len(b.ArrVal) {
		return false
	}
	for i := range a.Children {
		if !astNodesEqual(a.Children[i], b.Children[i]) {
			return false
		}
	}
	for i := range a.Nodes {
		if !astNodesEqual(a.Nodes[i], b.Nodes[i]) {
			return false
		}
	}
	for i := range a.Edges {
		if !astNodesEqual(a.Edges[i], b.Edges[i]) {
			return false
		}
	}
	for i := range a.Args {
		if !astNodesEqual(a.Args[i], b.Args[i]) {
			return false
		}
	}
	for k := range a.Attrs {
		if !astNodesEqual(a.Attrs[k], b.Attrs[k]) {
			return false
		}
	}
	for k := range a.MapVal {
		if !astNodesEqual(a.MapVal[k], b.MapVal[k]) {
			return false
		}
	}
	for i := range a.ArrVal {
		if !astNodesEqual(a.ArrVal[i], b.ArrVal[i]) {
			return false
		}
	}
	return true
}

// ============================================================================
// Fuzz test — feed random bytes to the parser, ensure no panic
// ============================================================================

func FuzzParser(f *testing.F) {
	// Seed corpus with valid TAC snippets
	seeds := []string{
		`flow "x" { node "a" -> skill foo() }`,
		`remember x = "value"`,
		`recall x`,
		`forget x`,
		`relate a -> b { type: "edge" }`,
		`flow "f" { input q: Untrusted }`,
		`node "a" -> skill web_search(query: "test", count: 3)`,
		`a -> b { if: x > 5 }`,
		`on "event" { cron: "* * * * *" } -> handler`,
		`context "ctx" { remember x = "y" }`,
		`auto_summarize(max: 100, scope: "session")`,
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Lexer should never panic
		l := lexer.New(input)
		tokens, err := l.Scan()
		if err != nil {
			// Lexer errors are expected for invalid input
			return
		}

		// Parser should never panic
		p := parser.New(tokens)
		program, err := p.Parse()
		if err != nil {
			// Parse errors are expected for invalid input
			return
		}

		// If we got a valid AST, it should survive:
		// 1. Walk
		ast.Walk(program, func(n *ast.Node, depth int) bool {
			if n.Type == "" {
				t.Error("node has empty type")
				return false
			}
			return true
		})

		// 2. Formatter
		_ = formatter.Format(program)

		// 3. Semantic analysis (may produce warnings but should not panic)
		a := semantic.New()
		_ = a.Analyze(program)
	})
}
