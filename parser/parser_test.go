package parser

import (
	"testing"

	"github.com/TacFlow/tac-language/ast"
)

func TestParseSimpleFlow(t *testing.T) {
	input := `flow "Test" {
  node "a" -> skill web_search(query: "hello")
  on "init" -> a
}`
	program, err := ParseSource(input)
	if err != nil {
		t.Fatalf("ParseSource error: %v", err)
	}
	flows := ast.CollectFlows(program)
	if len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}
	flow := flows[0]
	if flow.Value != "Test" {
		t.Errorf("expected flow name 'Test', got %q", flow.Value)
	}
	nodes := ast.CollectNodes(flow)
	if len(nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(nodes))
	}
	if ast.NodeName(nodes[0]) != "a" {
		t.Errorf("expected node name 'a', got %q", ast.NodeName(nodes[0]))
	}
}

func TestParseFlowWithInput(t *testing.T) {
	input := `flow "Test" {
  input question: Untrusted
  node "search" -> skill web_search(query: question)
  on "user_message" -> search
}`
	program, err := ParseSource(input)
	if err != nil {
		t.Fatalf("ParseSource error: %v", err)
	}
	flows := ast.CollectFlows(program)
	if len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}
}

func TestParseFlowWithEdges(t *testing.T) {
	input := `flow "Test" {
  node "a" -> skill web_search(query: "x")
  node "b" -> skill memory_search(query: "x")
  node "c" -> skill llm.chat(prompt: "combine")
  a -> c
  b -> c
  on "init" -> [a, b]
}`
	program, err := ParseSource(input)
	if err != nil {
		t.Fatalf("ParseSource error: %v", err)
	}
	flow := ast.CollectFlows(program)[0]
	edges := ast.CollectEdges(flow)
	if len(edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(edges))
	}
}

func TestParseConditionalEdge(t *testing.T) {
	input := `flow "Test" {
  node "a" -> skill web_search(query: "x")
  node "b" -> skill llm.chat(prompt: "x")
  a -> b { if: a.result == "ok" }
}`
	program, err := ParseSource(input)
	if err != nil {
		t.Fatalf("ParseSource error: %v", err)
	}
	flow := ast.CollectFlows(program)[0]
	edges := ast.CollectEdges(flow)
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	cond, _, hasCond := ast.EdgeCondition(edges[0])
	if !hasCond {
		t.Error("expected conditional edge")
	}
	if cond == "" {
		t.Error("expected condition to be non-empty")
	}
}

func TestParseInlineIfElse(t *testing.T) {
	input := `flow "Test" {
  node "speak" {
    if verify.confidence > 0.8 {
      skill tts.speak(text: "hi")
    } else {
      skill tts.speak(text: "no")
    }
  }
}`
	program, err := ParseSource(input)
	if err != nil {
		t.Fatalf("ParseSource error: %v", err)
	}
	flow := ast.CollectFlows(program)[0]
	nodes := ast.CollectNodes(flow)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if ast.NodeName(nodes[0]) != "speak" {
		t.Errorf("expected node name 'speak', got %q", ast.NodeName(nodes[0]))
	}
}

func TestParseWebQA(t *testing.T) {
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
  node "speak" {
    if verify.confidence > 0.8 {
      skill tts.speak(text: synthesize.result)
    } else {
      skill tts.speak(text: "Not confident")
    }
  }
  search_web    -> synthesize
  search_memory -> synthesize
  synthesize    -> verify
  verify        -> speak
  verify        -> speak { if: verify.confidence > 0.9 }
  on "user_message" -> search_web
}`
	program, err := ParseSource(input)
	if err != nil {
		t.Fatalf("ParseSource error: %v", err)
	}
	flow := ast.CollectFlows(program)[0]
	nodes := ast.CollectNodes(flow)
	if len(nodes) != 5 {
		t.Errorf("expected 5 nodes, got %d", len(nodes))
	}
	edges := ast.CollectEdges(flow)
	if len(edges) != 5 {
		t.Errorf("expected 5 edges, got %d", len(edges))
	}
}

func TestParseRemember(t *testing.T) {
	input := `remember app_name = "Todo List" {
  type: "string"
  tags: [ "app", "config" ]
}`
	program, err := ParseSource(input)
	if err != nil {
		t.Fatalf("ParseSource error: %v", err)
	}
	if len(program.Nodes) != 1 {
		t.Fatalf("expected 1 top-level node, got %d", len(program.Nodes))
	}
	if program.Nodes[0].Type != ast.NodeRememberStmt {
		t.Errorf("expected RememberStmt, got %s", program.Nodes[0].Type)
	}
}

func TestParseContext(t *testing.T) {
	input := `context "session" {
  remember user = "Diogo"
  flow "Greeting" {
    node "greet" -> skill tts.speak(text: "Hello")
    on "init" -> greet
  }
}`
	program, err := ParseSource(input)
	if err != nil {
		t.Fatalf("ParseSource error: %v", err)
	}
	if len(program.Nodes) != 1 {
		t.Fatalf("expected 1 top-level node, got %d", len(program.Nodes))
	}
	if program.Nodes[0].Type != ast.NodeContextBlock {
		t.Errorf("expected ContextBlock, got %s", program.Nodes[0].Type)
	}
}
