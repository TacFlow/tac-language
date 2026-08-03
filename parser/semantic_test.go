// semantic_test.go — TAC Semantic Analyzer Tests
//
// Covers:
//   - DAG analysis (topology, critical path, parallel sets, depth)
//   - Cycle detection (Kahn + DFS)
//   - Reference validation (undefined nodes in edges/triggers)
//   - Skill parameter validation (required params, unknown params, type checking)
//   - Trust type checking (conversion matrix, provenance)
//   - Duplicate node detection
//   - Relate/remember validation
//
// (c) 2026 TacFlow — MIT License

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Helper: parse a .tac file
// ============================================================================

func parseFile(t *testing.T, path string) *Node {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	tokenizer := NewTokenizer(string(source))
	tokens, err := tokenizer.Tokenize()
	if err != nil {
		t.Fatalf("tokenize %s: %v", path, err)
	}
	for _, tok := range tokens {
		if tok.Type == TError {
			t.Fatalf("token error in %s at line %d: %s", path, tok.Line, tok.Value)
		}
	}
	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return ast
}

// ============================================================================
// DAG topology tests
// ============================================================================

func TestDAGTopology_SimpleDAG(t *testing.T) {
	ast := parseFile(t, "../testdata/valid/simple_dag.tac")
	analyzer := NewAnalyzer(ast)
	result := analyzer.Analyze()

	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Diagnostics)
	}
	if result.Flow == nil {
		t.Fatal("expected flow analysis")
	}

	if result.Flow.NodeCount != 2 {
		t.Errorf("expected 2 nodes, got %d", result.Flow.NodeCount)
	}
	if result.Flow.EdgeCount != 1 {
		t.Errorf("expected 1 edge, got %d", result.Flow.EdgeCount)
	}
	if !result.Flow.IsDAG {
		t.Errorf("expected DAG=true, got false")
	}
	if result.Flow.Depth != 1 {
		t.Errorf("expected depth=1, got %d", result.Flow.Depth)
	}

	// Simple DAG: start -> process. Only 2 waves.
	if len(result.Flow.ParallelSets) != 2 {
		t.Errorf("expected 2 parallel sets, got %d", len(result.Flow.ParallelSets))
	}
}

func TestDAGTopology_ParallelFanout(t *testing.T) {
	ast := parseFile(t, "../testdata/valid/parallel_fanout.tac")
	analyzer := NewAnalyzer(ast)
	result := analyzer.Analyze()

	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Diagnostics)
	}
	if result.Flow == nil {
		t.Fatal("expected flow analysis")
	}

	// 4 nodes: a, b, c, merge; 3 edges: a->merge, b->merge, c->merge
	if result.Flow.NodeCount != 4 {
		t.Errorf("expected 4 nodes, got %d", result.Flow.NodeCount)
	}
	if result.Flow.EdgeCount != 3 {
		t.Errorf("expected 3 edges, got %d", result.Flow.EdgeCount)
	}

	// First parallel set should be [a, b, c] (all 3 independent)
	if len(result.Flow.ParallelSets) < 2 {
		t.Fatalf("expected at least 2 parallel sets, got %d", len(result.Flow.ParallelSets))
	}
	firstWave := strings.Join(result.Flow.ParallelSets[0], ",")
	if !strings.Contains(firstWave, "a") || !strings.Contains(firstWave, "b") || !strings.Contains(firstWave, "c") {
		t.Errorf("first parallel set should contain a,b,c, got %v", result.Flow.ParallelSets[0])
	}
	// Second wave should be [merge]
	if len(result.Flow.ParallelSets[1]) != 1 || result.Flow.ParallelSets[1][0] != "merge" {
		t.Errorf("second parallel set should be [merge], got %v", result.Flow.ParallelSets[1])
	}
}

func TestDAGTopology_RootOnly(t *testing.T) {
	ast := parseFile(t, "../testdata/valid/root_only.tac")
	analyzer := NewAnalyzer(ast)
	result := analyzer.Analyze()

	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Diagnostics)
	}
	if result.Flow.NodeCount != 1 {
		t.Errorf("expected 1 node, got %d", result.Flow.NodeCount)
	}
	if result.Flow.EdgeCount != 0 {
		t.Errorf("expected 0 edges, got %d", result.Flow.EdgeCount)
	}
	if result.Flow.Depth != 0 {
		t.Errorf("expected depth=0 for single node, got %d", result.Flow.Depth)
	}
	if len(result.Flow.ParallelSets) != 1 {
		t.Errorf("expected 1 parallel set, got %d", len(result.Flow.ParallelSets))
	}
}

// ============================================================================
// Cycle detection
// ============================================================================

func TestCycleDetection(t *testing.T) {
	ast := parseFile(t, "../testdata/invalid/cycle.tac")
	analyzer := NewAnalyzer(ast)
	result := analyzer.Analyze()

	if result.Valid {
		t.Errorf("expected invalid due to cycle")
	}
	if result.Flow.IsDAG {
		t.Errorf("expected IsDAG=false")
	}
	if len(result.Flow.Cycles) == 0 {
		t.Errorf("expected at least 1 cycle detected")
	}

	// Verify one of the diagnostics mentions cycle
	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == "flow.cycle" && d.Severity == SeverityError {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a cycle diagnostic, got: %v", result.Diagnostics)
	}
}

// ============================================================================
// Reference validation
// ============================================================================

func TestUndefinedEdgeReference(t *testing.T) {
	ast := parseFile(t, "../testdata/invalid/undefined_ref.tac")
	analyzer := NewAnalyzer(ast)
	result := analyzer.Analyze()

	if result.Valid {
		t.Errorf("expected invalid due to undefined edge reference")
	}
	found := 0
	for _, d := range result.Diagnostics {
		if d.Rule == "flow.undefined-node" {
			found++
		}
	}
	if found == 0 {
		t.Errorf("expected undefined-node diagnostics, got: %v", result.Diagnostics)
	}
}

func TestBadTriggerReference(t *testing.T) {
	ast := parseFile(t, "../testdata/invalid/bad_trigger.tac")
	analyzer := NewAnalyzer(ast)
	result := analyzer.Analyze()

	if result.Valid {
		t.Errorf("expected invalid due to undefined trigger target")
	}
	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == "flow.undefined-trigger" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected undefined-trigger diagnostic, got: %v", result.Diagnostics)
	}
}

// ============================================================================
// Skill parameter validation
// ============================================================================

func TestUnknownSkill(t *testing.T) {
	ast := parseFile(t, "../testdata/invalid/unknown_skill.tac")
	analyzer := NewAnalyzer(ast)
	result := analyzer.Analyze()

	if result.Valid {
		t.Errorf("expected invalid due to unknown skill")
	}
	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == "skill.unknown" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected skill.unknown diagnostic, got: %v", result.Diagnostics)
	}
}

func TestSkillRegistryCompleteness(t *testing.T) {
	// All core skills in SPEC.md must be registered
	requiredSkills := []string{
		"memory_search", "memory_store", "web_search",
		"graph_search", "graph_relate", "llm.chat", "llm.classify",
		"tts.speak", "whisper.transcribe",
		"verify", "validate", "config_get", "config_set",
		"agent_task", "flow.run", "get_current_time",
		"set_token_limit", "auto_summarize",
	}

	for _, name := range requiredSkills {
		spec, ok := LookupSkill(name)
		if !ok {
			t.Errorf("required skill %q is not registered", name)
		}
		if spec.Name != name {
			t.Errorf("skill %q has mismatched name in spec: %q", name, spec.Name)
		}
	}
}

func TestSkillParamValidation_MissingRequired(t *testing.T) {
	// Let's test a flow that calls web_search without the required "query" param
	src := `flow "Missing Param" {
  node "s" -> skill web_search(count: 3)
}`
 	tokenizer := NewTokenizer(src)
	tokens, _ := tokenizer.Tokenize()
	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	analyzer := NewAnalyzer(ast)
	result := analyzer.Analyze()

	if result.Valid {
		t.Errorf("expected invalid due to missing required param 'query'")
	}
	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == "skill.missing-param" && strings.Contains(d.Message, "query") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected missing-param diagnostic for 'query', got: %v", result.Diagnostics)
	}
}

func TestSkillParamValidation_UnknownParam(t *testing.T) {
	src := `flow "Bad Param" {
  node "s" -> skill web_search(query: "test", laser_mode: true)
}`
	tokenizer := NewTokenizer(src)
	tokens, _ := tokenizer.Tokenize()
	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	analyzer := NewAnalyzer(ast)
	result := analyzer.Analyze()

	if result.Valid {
		t.Errorf("expected invalid due to unknown param 'laser_mode'")
	}
	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == "skill.unknown-param" && strings.Contains(d.Message, "laser_mode") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected unknown-param diagnostic, got: %v", result.Diagnostics)
	}
}

func TestSkillParamValidation_ValidFull(t *testing.T) {
	src := `flow "Valid Params" {
  node "s" -> skill web_search(query: "test query", count: 3)
}`
	tokenizer := NewTokenizer(src)
	tokens, _ := tokenizer.Tokenize()
	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	analyzer := NewAnalyzer(ast)
	result := analyzer.Analyze()

	if !result.Valid {
		t.Errorf("expected valid, got: %v", result.Diagnostics)
	}
}

func TestDuplicateNode(t *testing.T) {
	ast := parseFile(t, "../testdata/invalid/duplicate_node.tac")
	analyzer := NewAnalyzer(ast)
	result := analyzer.Analyze()

	if result.Valid {
		t.Errorf("expected invalid due to duplicate node")
	}
	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == "flow.duplicate-node" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected duplicate-node diagnostic, got: %v", result.Diagnostics)
	}
}

// ============================================================================
// Trust type tests
// ============================================================================

func TestTrustTypeConversionMatrix(t *testing.T) {
	tests := []struct {
		from, to       TrustType
		allowed        bool
		requires       string
	}{
		{TrustSecret, TrustSecret, true, ""},
		{TrustSecret, TrustUntrusted, false, ""},
		{TrustSecret, TrustFact, false, ""},
		{TrustUntrusted, TrustFact, false, "validate()"},
		{TrustUntrusted, TrustUntrusted, true, ""},
		{TrustUntrusted, TrustHallucinable, false, "sanitize()"},
		{TrustFact, TrustFact, true, ""},
		{TrustFact, TrustUntrusted, true, ""},
		{TrustFact, TrustHallucinable, true, ""},
		{TrustHallucinable, TrustFact, false, "verify()"},
		{TrustHallucinable, TrustHallucinable, true, ""},
		{TrustHallucinable, TrustUntrusted, true, ""},
		{TrustControl, TrustControl, true, ""},
		{TrustControl, TrustUntrusted, false, ""},
		{TrustUnknown, TrustUnknown, true, ""},
	}

	for _, tc := range tests {
		allowed, requires := trustConversionAllowed(tc.from, tc.to)
		if allowed != tc.allowed {
			t.Errorf("%s -> %s: expected allowed=%v, got %v", tc.from, tc.to, tc.allowed, allowed)
		}
		if requires != tc.requires {
			t.Errorf("%s -> %s: expected requires=%q, got %q", tc.from, tc.to, tc.requires, requires)
		}
	}
}

func TestTrustTypeAllRegistered(t *testing.T) {
	for _, name := range []string{"secret", "untrusted", "fact", "hallucinable", "control"} {
		if _, ok := ValidTrustTypes[name]; !ok {
			t.Errorf("trust type %q should be registered", name)
		}
	}
}

func TestTrustTypeSkillReturns(t *testing.T) {
	// Verify specific skill return types
	tests := []struct {
		skill   string
		returns TrustType
	}{
		{"memory_search", TrustFact},
		{"web_search", TrustUntrusted},
		{"llm.chat", TrustHallucinable},
		{"llm.classify", TrustHallucinable},
		{"verify", TrustFact},
		{"validate", TrustFact},
		{"config_get", TrustSecret},
		{"get_current_time", TrustFact},
		{"agent_task", TrustUntrusted},
		{"whisper.transcribe", TrustUntrusted},
	}

	for _, tc := range tests {
		spec, ok := LookupSkill(tc.skill)
		if !ok {
			t.Errorf("skill %q not registered", tc.skill)
			continue
		}
		if spec.Returns != tc.returns {
			t.Errorf("skill %q returns %s, expected %s", tc.skill, spec.Returns, tc.returns)
		}
	}
}

// ============================================================================
// Relate and Remember validation
// ============================================================================

func TestRelateRequiresTypeAttribute(t *testing.T) {
	src := `relate "X" -> "Y" {}`
	tokenizer := NewTokenizer(src)
	tokens, _ := tokenizer.Tokenize()
	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	analyzer := NewAnalyzer(ast)
	result := analyzer.Analyze()
	if result.Valid {
		t.Errorf("expected invalid: relate without type attribute")
	}
}

func TestRelateValid(t *testing.T) {
	ast := parseFile(t, "../testdata/valid/relate_ops.tac")
	analyzer := NewAnalyzer(ast)
	result := analyzer.Analyze()
	if !result.Valid {
		t.Errorf("expected valid relate ops, got: %v", result.Diagnostics)
	}
}

func TestRememberValid(t *testing.T) {
	ast := parseFile(t, "../testdata/valid/memory_ops.tac")
	analyzer := NewAnalyzer(ast)
	result := analyzer.Analyze()
	if !result.Valid {
		t.Errorf("expected valid memory ops, got: %v", result.Diagnostics)
	}
}

// ============================================================================
// Web Q&A (example) full validation
// ============================================================================

func TestWebQA_CompleteValidation(t *testing.T) {
	ast := parseFile(t, "../examples/web_qa.tac")
	analyzer := NewAnalyzer(ast)
	result := analyzer.Analyze()

	if !result.Valid {
		t.Errorf("web_qa.tac should be valid, got: %v", result.Diagnostics)
	}
	if result.Flow == nil {
		t.Fatal("expected flow analysis")
	}
	if result.Flow.NodeCount != 6 {
		t.Errorf("expected 6 nodes, got %d", result.Flow.NodeCount)
	}
	if !result.Flow.IsDAG {
		t.Errorf("web_qa.tac should be a DAG")
	}

	// Verify parallel phase: search_web, search_memory, search_graph should be in same wave
	if len(result.Flow.ParallelSets) < 1 {
		t.Fatalf("expected parallel sets")
	}
	firstWave := strings.Join(result.Flow.ParallelSets[0], ",")
	if !strings.Contains(firstWave, "search_web") ||
		!strings.Contains(firstWave, "search_memory") ||
		!strings.Contains(firstWave, "search_graph") {
		t.Errorf("first wave should contain all 3 searches, got %v", result.Flow.ParallelSets[0])
	}
}

func TestMultiAgentReview_CompleteValidation(t *testing.T) {
	ast := parseFile(t, "../examples/multi_agent_review.tac")
	analyzer := NewAnalyzer(ast)
	result := analyzer.Analyze()

	if !result.Valid {
		t.Errorf("multi_agent_review.tac should be valid, got: %v", result.Diagnostics)
	}
	if result.Flow.NodeCount != 4 {
		t.Errorf("expected 4 nodes, got %d", result.Flow.NodeCount)
	}
}

func TestGraphBuilder_CompleteValidation(t *testing.T) {
	ast := parseFile(t, "../examples/graph_builder.tac")
	analyzer := NewAnalyzer(ast)
	result := analyzer.Analyze()

	if !result.Valid {
		t.Errorf("graph_builder.tac should be valid, got: %v", result.Diagnostics)
	}
	if result.Flow.NodeCount != 4 {
		t.Errorf("expected 4 nodes, got %d", result.Flow.NodeCount)
	}
}

// ============================================================================
// Invalid file batch tests
// ============================================================================

func TestAllInvalidFiles(t *testing.T) {
	invalidDir := "../testdata/invalid"
	entries, err := os.ReadDir(invalidDir)
	if err != nil {
		t.Fatalf("cannot read invalid dir: %v", err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".tac") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			ast := parseFile(t, filepath.Join(invalidDir, entry.Name()))
			analyzer := NewAnalyzer(ast)
			result := analyzer.Analyze()

			// All invalid files should have errors
			if result.Valid && entry.Name() != "syntax_error.tac" {
				t.Errorf("%s should be invalid, but passed semantic analysis", entry.Name())
			}

			// Every invalid file should produce at least one diagnostic
			if len(result.Diagnostics) == 0 && entry.Name() != "syntax_error.tac" {
				t.Errorf("%s should produce at least one diagnostic", entry.Name())
			}
		})
	}
}

// ============================================================================
// Valid file batch tests
// ============================================================================

func TestAllValidFiles(t *testing.T) {
	validDir := "../testdata/valid"
	entries, err := os.ReadDir(validDir)
	if err != nil {
		t.Fatalf("cannot read valid dir: %v", err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".tac") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			ast := parseFile(t, filepath.Join(validDir, entry.Name()))
			analyzer := NewAnalyzer(ast)
			result := analyzer.Analyze()

			if !result.Valid {
				t.Errorf("%s should be valid, got errors: %v", entry.Name(), result.Diagnostics)
			}
		})
	}
}
