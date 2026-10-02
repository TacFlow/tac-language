package parser

import (
	"strings"
	"testing"

	"github.com/TacFlow/tac-language/ast"
)

func mustParse(t *testing.T, src string) *ast.Node {
	t.Helper()
	prog, err := ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return prog
}

func TestParse_LabelledEdges(t *testing.T) {
	prog := mustParse(t, `flow "g" {
  node "gate" -> skill laya.decide(task: "t", input: payload)
  gate[proceed] -> a
  gate["pedido urgente"] -> a
  gate[true] -> b
  gate[*] -> b
  gate[<10] -> c
  gate[<=10] -> c
  gate[>=50] -> c
  gate[10..50] -> c
  gate[<-5] -> c
  gate[-5..-1] -> c
  gate[low_confidence] -> d -> e
}`)
	f := ast.CollectFlows(prog)[0]
	want := []string{"proceed", "pedido urgente", "true", "*", "range:<10", "range:<=10", "range:>=50",
		"range:10..50", "range:<-5", "range:-5..-1", "low_confidence", ""}
	if len(f.Edges) != len(want) {
		t.Fatalf("edges = %d, want %d", len(f.Edges), len(want))
	}
	for i, w := range want {
		if got := ast.LabelString(ast.EdgeLabel(f.Edges[i])); got != w {
			t.Errorf("edge %d label = %q, want %q", i, got, w)
		}
	}
	if last := f.Edges[len(f.Edges)-1]; ast.EdgeSource(last) != "d" || ast.EdgeTarget(last) != "e" {
		t.Errorf("chain tail after a labelled hop = %s -> %s", ast.EdgeSource(last), ast.EdgeTarget(last))
	}
	for _, c := range f.Children {
		if c.Type == ast.NodeUnrecognized {
			t.Errorf("unexpected unrecognized form %q", c.Value)
		}
	}
}

// A malformed branch is recorded (TAC-PARSE-001), never silently dropped,
// and the rest of the flow still parses. Recovery resumes where v0.4 did:
// in `gate[proceed -> a`, v0.4 skipped `gate` and `[` and kept the edge
// `proceed -> a`.
func TestParse_MalformedBranchIsUnrecognized(t *testing.T) {
	for line, want := range map[string]string{
		"gate[] -> a":       "x>y",
		"gate[5] -> a":      "x>y",
		"gate[proceed -> a": "proceed>a,x>y",
		"gate[proceed]":     "x>y",
		"gate[<] -> a":      "x>y",
	} {
		prog := mustParse(t, "flow \"g\" {\n  "+line+"\n  x -> y\n}")
		f := ast.CollectFlows(prog)[0]
		n := 0
		for _, c := range f.Children {
			if c.Type == ast.NodeUnrecognized {
				n++
			}
		}
		var edges []string
		for _, e := range f.Edges {
			edges = append(edges, ast.EdgeSource(e)+">"+ast.EdgeTarget(e))
		}
		if n != 1 || strings.Join(edges, ",") != want {
			t.Errorf("%q: unrecognized=%d edges=%v, want 1 and %s", line, n, edges, want)
		}
	}
}

func TestParse_ScheduleAndUnrecognized(t *testing.T) {
	prog := mustParse(t, `stray tokens here
flow "s" {
  schedule "0 3 * * *" tz "Europe/Lisbon"
  schedule "@hourly"
  something odd
}`)
	if prog.Nodes[0].Type != ast.NodeUnrecognized || prog.Nodes[0].Value != "stray" {
		t.Fatalf("top-level form = %+v", prog.Nodes[0])
	}
	f := ast.CollectFlows(prog)[0]
	var kinds []ast.NodeType
	for _, c := range f.Children {
		kinds = append(kinds, c.Type)
	}
	if len(kinds) != 3 || kinds[0] != ast.NodeSchedule || kinds[1] != ast.NodeSchedule || kinds[2] != ast.NodeUnrecognized {
		t.Fatalf("children = %v", kinds)
	}
	if tz := f.Children[0].Attrs["tz"]; tz == nil || tz.Value != "Europe/Lisbon" {
		t.Errorf("tz = %+v", tz)
	}
	if f.Children[1].Attrs != nil {
		t.Errorf("schedule without tz has attrs %+v", f.Children[1].Attrs)
	}
}

func TestParse_Declarations(t *testing.T) {
	prog := mustParse(t, `requires "0.5"
task "acao" {
  question acao: choice "Qual?" {
    proceed: "agir"
    "pedir ajuda": "perguntar"
  }
  profile { match: { any_path: ["$.sku"] } }
}
model "m" { tasks ["acao"] base "laya-multilingual" port 18101 }
episode "e1" {
  group "g"
  task "acao"
  goal "x"
  context { a: -1.5, b: [1, 2] }
  rule "r"
  tool "t"
  cache_available false
  question qtd: number "Quantos?" { range: [0, 100], bins: [10, 50] }
  target qtd = 12
  meta { synthetic: true }
}
dataset "d" {
  task "acao"
  from "db"
  include "e1", "e2"
  include "e3"
  split train = 0.9
  split test = 0.1
}`)
	types := []ast.NodeType{ast.NodeRequires, ast.NodeTaskDecl, ast.NodeModelDecl, ast.NodeEpisodeDecl, ast.NodeDatasetDecl}
	if len(prog.Nodes) != len(types) {
		t.Fatalf("nodes = %d", len(prog.Nodes))
	}
	for i, ty := range types {
		if prog.Nodes[i].Type != ty {
			t.Fatalf("node %d = %s, want %s", i, prog.Nodes[i].Type, ty)
		}
	}
	task := prog.Nodes[1]
	q := task.Children[0]
	if q.Type != ast.NodeQuestion || q.Value != "acao" || q.Attrs["type"].Value != "choice" || len(q.Children) != 2 ||
		q.Children[1].Value != "pedir ajuda" || task.Attrs["profile"] == nil {
		t.Fatalf("task = %+v / %+v", task, q)
	}
	ep := prog.Nodes[3]
	var keys []string
	for _, c := range ep.Children {
		keys = append(keys, string(c.Type)+":"+c.Value)
	}
	wantKeys := "KeyValue:group,KeyValue:task,KeyValue:goal,KeyValue:context,KeyValue:rule,KeyValue:tool," +
		"KeyValue:cache_available,Question:qtd,Target:qtd,KeyValue:meta"
	if got := join(keys); got != wantKeys {
		t.Fatalf("episode items = %s\nwant %s", got, wantKeys)
	}
	if a := ep.Children[7].Attrs["attrs"]; a == nil || len(a.MapVal["range"].ArrVal) != 2 {
		t.Fatalf("number question attrs = %+v", a)
	}
	ds := prog.Nodes[4]
	if inc := ds.Attrs["include"]; inc == nil || len(inc.ArrVal) != 3 || len(ds.Children) != 2 {
		t.Fatalf("dataset = %+v", ds)
	}
}

func join(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += ","
		}
		out += x
	}
	return out
}

// Every v0.5 keyword is still a valid v0.4 node name and edge endpoint
// inside a flow: `schedule` is a directive only when a string follows it,
// and the declaration keywords are keywords only at top level.
func TestParse_V05KeywordsStillWorkAsNodeNames(t *testing.T) {
	prog := mustParse(t, `flow "k" {
  node "schedule" -> skill foo()
  node "task" -> skill foo()
  node "model" -> skill foo()
  node "episode" -> skill foo()
  node "dataset" -> skill foo()
  node "requires" -> skill foo()
  node "question" -> skill foo()
  node "target" -> skill foo()
  node "tz" -> skill foo()
  schedule -> task
  task -> model -> episode
  episode -> dataset
  dataset -> requires
  requires -> question
  question -> target
  target -> tz
}`)
	f := ast.CollectFlows(prog)[0]
	if len(f.Edges) != 8 {
		t.Fatalf("edges = %d, want 8 (%v)", len(f.Edges), f.Edges)
	}
	for _, c := range f.Children {
		t.Errorf("unexpected flow child %s %q", c.Type, c.Value)
	}
}

// Inside a declaration an item the parser cannot read is recorded, never
// silently reinterpreted: `targte q = true` is not the state key "targte",
// a misspelt `quesiton` does not vanish, and a malformed option drops its
// whole question (recorded) — the next declaration still parses.
func TestParse_TyposInsideDeclarationsAreUnrecognized(t *testing.T) {
	prog := mustParse(t, `task "t" {
  quesiton x: choice "Q" { a: "1", b: "2" }
  question ok: choice "Q" { a: "1", b: "2" }
  question bad: choice "Q" { a: 3, b: "2" }
}
episode "e" {
  goal "g"
  targte q = true
}
dataset "d" {
  task "t"
  splt train = 1
}`)
	if len(prog.Nodes) != 3 {
		t.Fatalf("nodes = %d, want 3", len(prog.Nodes))
	}
	want := []string{"Unrecognized:quesiton,Question:ok,Unrecognized:question", "KeyValue:goal,Unrecognized:targte", "Unrecognized:splt"}
	for i, n := range prog.Nodes {
		var got []string
		for _, c := range n.Children {
			got = append(got, string(c.Type)+":"+c.Value)
		}
		if join(got) != want[i] {
			t.Errorf("%s %q children = %s, want %s", n.Type, n.Value, join(got), want[i])
		}
	}
}

// `1e3` is one number; `3x` keeps its v0.4 reading (3, then x) and is
// recorded as a GluedNumber node at the end of the program.
func TestParse_ExponentAndGluedNumber(t *testing.T) {
	prog := mustParse(t, "episode \"e\" {\n  limite 1e3\n}\nflow \"f\" {\n  node \"a\" -> skill web_search(query: \"q\", count: 3x)\n}")
	kv := prog.Nodes[0].Children[0]
	if kv.Value != "limite" || kv.Children[0].Type != ast.NodeNumberLiteral || kv.Children[0].Value != "1e3" || kv.Children[0].NumVal != 1000 {
		t.Fatalf("limite = %+v", kv.Children[0])
	}
	last := prog.Nodes[len(prog.Nodes)-1]
	if last.Type != ast.NodeGluedNumber || last.Value != "3x" {
		t.Fatalf("last node = %s %q", last.Type, last.Value)
	}
	call := ast.CollectFlows(prog)[0].Nodes[0].Children[1]
	if len(call.Args) != 3 { // query, count: 3, x — exactly what v0.4 parsed
		t.Fatalf("args = %d, want 3 (v0.4 reading kept)", len(call.Args))
	}
}
