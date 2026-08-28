package semantic

import (
	"testing"

	"github.com/TacFlow/tac-language/parser"
)

func TestAnalyze_ValidFlow(t *testing.T) {
	input := `flow "Test" {
  node "a" -> skill web_search(query: "x")
  node "b" -> skill llm.chat(prompt: "y")
  a -> b
  on "init" -> a
}`
	program, err := parser.ParseSource(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	analyzer := New()
	diags := analyzer.Analyze(program)
	if analyzer.HasErrors() {
		for _, d := range diags {
			if d.Severity == SeverityError {
				t.Errorf("unexpected error: %s", d)
			}
		}
	}
}

func TestAnalyze_UndeclaredNode(t *testing.T) {
	input := `flow "Test" {
  node "a" -> skill web_search(query: "x")
  a -> b
  on "init" -> a
}`
	program, err := parser.ParseSource(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	analyzer := New()
	diags := analyzer.Analyze(program)
	if !analyzer.HasErrors() {
		t.Error("expected errors for undeclared node 'b'")
	} else {
		// Verify the error is about undeclared node
		found := false
		for _, d := range analyzer.Errors() {
			if d.Message != "" {
				found = true
				t.Logf("Found expected error: %s", d)
			}
		}
		if !found {
			t.Error("expected error about undeclared node")
		}
	}
	_ = diags
}

func TestAnalyze_Cycle(t *testing.T) {
	input := `flow "Test" {
  node "a" -> skill web_search(query: "x")
  node "b" -> skill web_search(query: "y")
  a -> b
  b -> a
}`
	program, err := parser.ParseSource(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	analyzer := New()
	diags := analyzer.Analyze(program)
	if !analyzer.HasErrors() {
		t.Error("expected cycle detection error")
	} else {
		t.Logf("Cycle detected: %v", analyzer.Errors())
	}
	_ = diags
}

func TestAnalyze_WebQA(t *testing.T) {
	input := `
flow "Web Q&A" {
  input question: Untrusted
  node "search_web"    -> skill web_search(query: question, count: 3)
  node "search_memory" -> skill memory_search(query: question, scope: "shared")
  node "synthesize" -> skill llm.chat(
    prompt: "Use sources",
    context: [search_web.result, search_memory.result]
  )
  node "verify" -> skill verify(source: synthesize.result)
  node "speak" -> skill tts.speak(text: synthesize.result)
  search_web    -> synthesize
  search_memory -> synthesize
  synthesize    -> verify
  verify        -> speak
  on "user_message" -> search_web
}`
	program, err := parser.ParseSource(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	analyzer := New()
	diags := analyzer.Analyze(program)
	if analyzer.HasErrors() {
		for _, d := range analyzer.Errors() {
			t.Errorf("unexpected error: %s", d)
		}
	}
	_ = diags
}

func TestSkillRegistry(t *testing.T) {
	// Verify built-in skills are registered
	skills := []string{
		"web_search", "memory_search", "llm.chat", "llm.classify",
		"tts.speak", "verify", "validate", "agent_task",
	}
	for _, name := range skills {
		spec, ok := LookupSkill(name)
		if !ok {
			t.Errorf("skill %q not found in registry", name)
		} else {
			t.Logf("Skill %q: %s (returns %s)", name, spec.Description, spec.ReturnType)
		}
	}
}
