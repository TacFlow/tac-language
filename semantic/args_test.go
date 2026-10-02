package semantic_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/TacFlow/tac-language/compiler"
	"github.com/TacFlow/tac-language/parser"
	"github.com/TacFlow/tac-language/semantic"
)

// v0.5 keeps a named argument under its own name (bug a), so an argument
// given twice keeps only one value: compileArgs writes both into one map,
// and the last one in the source wins (the attribute block comes after the
// parentheses). Twice by name, by name and in the attribute block, or a
// named `arg<i>` that collides with the i-th positional argument — each
// warns, on known and unknown skills alike (`agent`, `subflow`, …).
//
// A warning, not an error: v0.4 compiled such sources (`s("q", arg0: "r")`)
// and v0.5 must still compile them, with the same IR as before the warning.
func TestArgumentGivenTwiceWarnsAndCompiles(t *testing.T) {
	cases := []struct {
		name, node, key, want string
	}{
		{"twice by name", `node "n" -> skill web_search(query: "q", query: "r")`, "query", "r"},
		{"name and block", `node "n" -> skill web_search(query: "q") { query: "r" }`, "query", "r"},
		{"positional then arg0", `node "n" -> skill web_search("q", arg0: "r")`, "arg0", "r"},
		{"arg0 then positional", `node "n" -> skill web_search(arg0: "r", "q")`, "arg0", "q"},
		{"unknown skill", `node "n" -> agent(id: "x") { id: "y" }`, "id", "y"},
	}
	for _, c := range cases {
		src := "flow \"f\" {\n  " + c.node + "\n}"
		prog, err := parser.ParseSource(src)
		if err != nil {
			t.Fatalf("%s: parse: %v", c.name, err)
		}
		flows, diags, err := compiler.CompileAndValidate(prog)
		found := false
		for _, d := range diags {
			if !strings.Contains(d.Message, "is given more than once") {
				continue
			}
			found = true
			if d.Severity != semantic.SeverityWarning {
				t.Errorf("%s: want a warning, got severity %v: %v", c.name, d.Severity, d)
			}
		}
		if !found {
			t.Errorf("%s: want a warning \"is given more than once\", got %v", c.name, diags)
		}
		if err != nil {
			t.Errorf("%s: want it to compile (v0.4 did), got %v (%v)", c.name, err, diags)
			continue
		}
		if len(flows) != 1 || len(flows[0].Nodes) != 1 {
			t.Fatalf("%s: want one flow with one node, got %+v", c.name, flows)
		}
		args, _ := json.Marshal(flows[0].Nodes[0].Args)
		var got map[string]interface{}
		if err := json.Unmarshal(args, &got); err != nil {
			t.Fatalf("%s: args %s: %v", c.name, args, err)
		}
		if len(got) != 1 || got[c.key] != c.want {
			t.Errorf("%s: want args {%q: %q} (the last value wins), got %s", c.name, c.key, c.want, args)
		}
	}
	// Distinct names, positional and named mixed: no such diagnostic.
	for _, node := range []string{
		`node "n" -> skill web_search(query: "q", count: 3)`,
		`node "n" -> skill web_search("q", count: 3)`,
		`node "n" -> skill web_search(query: "q") { count: 3 }`,
		`node "n" -> agent("x")`,
	} {
		prog, err := parser.ParseSource("flow \"f\" {\n  " + node + "\n}")
		if err != nil {
			t.Fatalf("%s: parse: %v", node, err)
		}
		for _, d := range semantic.New().Analyze(prog) {
			if strings.Contains(d.Message, "more than once") {
				t.Errorf("%s: unexpected %v", node, d)
			}
		}
	}
}
