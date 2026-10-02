package tac_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/TacFlow/tac-language/compiler"
	"github.com/TacFlow/tac-language/parser"
	"github.com/TacFlow/tac-language/semantic"
)

// compileNumbers parses, analyses and compiles src the way `tac compile
// --json` does, and decodes the result into plain Go values.
func compileNumbers(t *testing.T, src string) (map[string]interface{}, []semantic.Diagnostic) {
	t.Helper()
	program, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a := semantic.New()
	diags := a.Analyze(program)
	if a.HasErrors() {
		return nil, diags
	}
	ir, err := compiler.CompileProgramIR(program)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	b, err := json.Marshal(ir)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// `tac compile` without --json marshals the flows alone.
	flows, _ := compiler.CompileProgram(program)
	if _, err := json.Marshal(flows); err != nil {
		t.Fatalf("marshal flows: %v", err)
	}
	var out map[string]interface{}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out, diags
}

func dig(v interface{}, path ...interface{}) interface{} {
	for _, p := range path {
		switch k := p.(type) {
		case string:
			v = v.(map[string]interface{})[k]
		case int:
			v = v.([]interface{})[k]
		}
	}
	return v
}

// I-1: the lexer accepts leading zeros (`007`, `00.5`) that JSON forbids. v0.4
// compiled `007` to 7; the json.Number IR must be valid JSON, so the literal
// is canonicalised (leading zeros dropped) and every other spelling is kept.
func TestNumberLiterals_LeadingZerosAreValidJSON(t *testing.T) {
	src := `task "t" {
  question q: number "N?" { range: [000, 0100] }
}
model "m" { tasks ["t"] port 0080 }
episode "e" {
  group "g"
  meta { nonce: 00012 }
  question q: number "N?" { range: [0, 0100] }
  target q = 007
}
dataset "d" {
  task "t"
  split train = 01.0
}
flow "f" {
  node "g" -> skill laya.decide(task: "t", input: payload)
  node "a" -> skill web_search(query: "q", n: 007, m: -007, x: 00.5, keep: 1.50, e: 1e3, z: 0)
  g[010..20] -> a
  g[<-05] -> a
  g[*] -> a
}`
	ir, diags := compileNumbers(t, src)
	if ir == nil {
		t.Fatalf("errors: %v", diags)
	}
	checks := []struct {
		path []interface{}
		want string
	}{
		{[]interface{}{"flows", 0, "nodes", 1, "args", "n"}, "7"},
		{[]interface{}{"flows", 0, "nodes", 1, "args", "m"}, "-7"},
		{[]interface{}{"flows", 0, "nodes", 1, "args", "x"}, "0.5"},
		{[]interface{}{"flows", 0, "nodes", 1, "args", "keep"}, "1.50"},
		{[]interface{}{"flows", 0, "nodes", 1, "args", "e"}, "1e3"},
		{[]interface{}{"flows", 0, "nodes", 1, "args", "z"}, "0"},
		{[]interface{}{"flows", 0, "edges", 0, "range", "min"}, "10"},
		{[]interface{}{"flows", 0, "edges", 0, "range", "max"}, "20"},
		{[]interface{}{"flows", 0, "edges", 1, "range", "value"}, "-5"},
		{[]interface{}{"tasks", 0, "questions", 0, "range", 0}, "0"},
		{[]interface{}{"tasks", 0, "questions", 0, "range", 1}, "100"},
		{[]interface{}{"models", 0, "port"}, "80"},
		{[]interface{}{"episodes", 0, "meta", "nonce"}, "12"},
		{[]interface{}{"episodes", 0, "targets", "q"}, "7"},
		{[]interface{}{"datasets", 0, "splits", "train"}, "1.0"},
	}
	for _, c := range checks {
		got := dig(ir, c.path...)
		if n, ok := got.(json.Number); !ok || string(n) != c.want {
			t.Errorf("%v = %#v, want %s", c.path, got, c.want)
		}
	}
}

// I-1: a literal that does not fit a float64 compiles to JSON no consumer
// can read; the analyzer rejects it. Underflow to 0 is finite and accepted.
func TestNumberLiterals_NonFiniteIsAnError(t *testing.T) {
	for _, lit := range []string{"1e999999999", "1e309", "-1e400"} {
		srcs := []string{
			`flow "f" { node "a" -> skill web_search(query: "q", n: ` + lit + `) }`,
			"episode \"e\" {\n group \"g\"\n meta { nonce: " + lit + " }\n question q: binary \"B?\"\n}",
			`flow "f" { node "g" -> skill laya.decide(task: "t", input: payload)
node "a" -> skill laya.tasks.list()
g[<` + lit + `] -> a
g[*] -> a }`,
		}
		for _, src := range srcs {
			ir, diags := compileNumbers(t, src)
			if ir != nil {
				t.Errorf("%s: compiled; want an error for the non-finite literal", src)
				continue
			}
			found := false
			for _, d := range diags {
				if d.Severity == semantic.SeverityError && strings.Contains(d.Message, "finite") {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: no non-finite error in %v", src, diags)
			}
		}
	}
	ir, diags := compileNumbers(t, `flow "f" { node "a" -> skill web_search(query: "q", n: 1e-400) }`)
	if ir == nil {
		t.Fatalf("1e-400 underflows to 0, which is finite: %v", diags)
	}
}
