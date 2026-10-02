package semantic

import (
	"strings"
	"testing"
)

// v0.5 keeps a named argument under its own name (bug a), so an argument
// given twice now loses one value in silence: compileArgs writes both into
// one map. Twice by name, by name and in the attribute block, or a named
// `arg<i>` that collides with the i-th positional argument — each is an
// error, on known and unknown skills alike (`agent`, `subflow`, …).
func TestArgumentGivenTwiceIsAnError(t *testing.T) {
	cases := map[string]string{
		"twice by name":        `node "n" -> skill web_search(query: "q", query: "r")`,
		"name and block":       `node "n" -> skill web_search(query: "q") { query: "r" }`,
		"positional then arg0": `node "n" -> skill web_search("q", arg0: "r")`,
		"arg0 then positional": `node "n" -> skill web_search(arg0: "r", "q")`,
		"unknown skill":        `node "n" -> agent(id: "x") { id: "y" }`,
	}
	for name, node := range cases {
		diags := analyzeSrc(t, "flow \"f\" {\n  "+node+"\n}")
		found := false
		for _, d := range diags {
			if d.Severity == SeverityError && strings.Contains(d.Message, "is given more than once") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want an error \"is given more than once\", got %v", name, diags)
		}
	}
	// Distinct names, positional and named mixed: no such error.
	for _, node := range []string{
		`node "n" -> skill web_search(query: "q", count: 3)`,
		`node "n" -> skill web_search("q", count: 3)`,
		`node "n" -> skill web_search(query: "q") { count: 3 }`,
		`node "n" -> agent("x")`,
	} {
		for _, d := range analyzeSrc(t, "flow \"f\" {\n  "+node+"\n}") {
			if strings.Contains(d.Message, "more than once") {
				t.Errorf("%s: unexpected %v", node, d)
			}
		}
	}
}
