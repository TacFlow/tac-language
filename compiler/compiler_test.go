package compiler

import (
	"encoding/json"
	"testing"

	"github.com/TacFlow/tac-language/parser"
)

func compileOne(t *testing.T, src string) *FlowJSON {
	t.Helper()
	prog, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	flows, err := CompileProgram(prog)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("want 1 flow, got %d", len(flows))
	}
	return flows[0]
}

func argsJSON(t *testing.T, n FlowNode) string {
	t.Helper()
	b, err := json.Marshal(n.Args)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Bug a: compileArgs turned every named argument into {"arg0": "<name>"} and
// lost its value. Named arguments keep their name and value; identifiers are
// references ({"ref": ...}); numbers keep their literal.
func TestBugA_NamedArgsKeepTheirValues(t *testing.T) {
	fj := compileOne(t, `flow "f" {
  input q: Untrusted
  node "a" -> skill web_search(query: q, count: 3)
  node "b" -> skill memory_store(text: "done", tags: ["x", a.result], shared: true)
  node "c" -> skill llm.chat(prompt: "p", context: { k: 0.50, src: b.result })
  node "d" -> skill foo("pos", 2)
}`)
	want := map[string]string{
		"a": `{"count":3,"query":{"ref":"q"}}`,
		"b": `{"shared":true,"tags":["x",{"ref":"a.result"}],"text":"done"}`,
		"c": `{"context":{"k":0.50,"src":{"ref":"b.result"}},"prompt":"p"}`,
		"d": `{"arg0":"pos","arg1":2}`,
	}
	for _, n := range fj.Nodes {
		if got := argsJSON(t, n); got != want[n.Name] {
			t.Errorf("node %s args = %s, want %s", n.Name, got, want[n.Name])
		}
	}
}
