package manifest

import (
	"testing"

	"github.com/tacflow1-tech/tac-language/parser"
)

func TestExtractManifest(t *testing.T) {
	input := `flow "Web Q&A" {
  input question: Untrusted
  node "search" -> skill web_search(query: question, count: 3)
  on "user_message" -> search
}`
	program, err := parser.ParseSource(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	flows := program.Nodes
	if len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}

	m := ExtractManifest(flows[0])
	if m.Name != "Web Q&A" {
		t.Errorf("expected name 'Web Q&A', got %q", m.Name)
	}
	if m.NodeCount != 1 {
		t.Errorf("expected 1 node, got %d", m.NodeCount)
	}
	if len(m.Inputs) != 1 {
		t.Errorf("expected 1 input, got %d", len(m.Inputs))
	}
	if len(m.Skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(m.Skills))
	}
	if m.Skills[0].Skill != "web_search" {
		t.Errorf("expected skill 'web_search', got %q", m.Skills[0].Skill)
	}
}

func TestUsedSkills(t *testing.T) {
	input := `flow "Test" {
  node "a" -> skill web_search(query: "x")
  node "b" -> skill memory_search(query: "x")
  node "c" -> skill llm.chat(prompt: "y")
  a -> c
  b -> c
}`
	program, _ := parser.ParseSource(input)
	m := ExtractManifest(program.Nodes[0])

	used := m.UsedSkills()
	if len(used) != 3 {
		t.Errorf("expected 3 unique skills, got %d: %v", len(used), used)
	}
	expected := map[string]bool{"web_search": true, "memory_search": true, "llm.chat": true}
	for _, s := range used {
		if !expected[s] {
			t.Errorf("unexpected skill: %q", s)
		}
	}
}
