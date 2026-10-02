package formatter

import (
	"strings"
	"testing"

	"github.com/TacFlow/tac-language/parser"
)

func fmtSrc(t *testing.T, src string) string {
	t.Helper()
	prog, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return Format(prog)
}

func TestFormat_KeyStr(t *testing.T) {
	cases := map[string]string{"sku": "sku", "a.b": "a.b", "x_1": "x_1",
		"sku id": `"sku id"`, "1a": `"1a"`, "true": `"true"`, "": `""`, "lote/validade": `"lote/validade"`}
	for in, want := range cases {
		if got := keyStr(in); got != want {
			t.Errorf("keyStr(%q) = %s, want %s", in, got, want)
		}
	}
}

// Keys that are not identifiers stay quoted, so the output parses again.
func TestFormat_QuotedObjectKeysSurvive(t *testing.T) {
	src := `episode "e" {
  goal "g"
  context { "sku id": "x", qtd: 3, "lote/validade": "2026-12" }
  question q: binary "Ok?"
}
`
	once := fmtSrc(t, src)
	if !strings.Contains(once, `context {"lote/validade": "2026-12", qtd: 3, "sku id": "x"}`) {
		t.Fatalf("formatted:\n%s", once)
	}
	if twice := fmtSrc(t, once); twice != once {
		t.Fatalf("not idempotent:\n%s\n---\n%s", once, twice)
	}
}

// Canonical forms of every v0.5 construct.
func TestFormat_LayaCanonicalForms(t *testing.T) {
	src := `requires "0.5"
task "t" { question q: number "N" {range: [-10, 10], bins: [0]}
profile { match: { any_path: ["$.a"] } } }
model "m" { port 18101 base "b" tasks ["t"] }
dataset "d" { split test = 0.5
task "t" include "e1", "e2" from "db" split train = 0.5 }
flow "f" {
  schedule "0 3 * * *" tz "Europe/Lisbon"
  node "g" -> skill laya.decide(task: "t", input: payload)
  node "a" -> skill laya.tasks.list()
  g[<-5] -> a
  g[-5..5] -> a
  g[>=5] -> a
  g["texto livre"] -> a
  g[*] -> a
}
`
	want := `requires "0.5"

task "t" {
    question q: number "N" {bins: [0], range: [-10, 10]}
    profile {match: {any_path: ["$.a"]}}
}

model "m" {
    tasks ["t"]
    base "b"
    port 18101
}

dataset "d" {
    task "t"
    from "db"
    include "e1", "e2"
    split test = 0.5
    split train = 0.5
}

flow "f" {
    schedule "0 3 * * *" tz "Europe/Lisbon"
    node "g" -> skill laya.decide(task: "t", input: payload)
    node "a" -> skill laya.tasks.list()
    g[<-5] -> a
    g[-5..5] -> a
    g[>=5] -> a
    g["texto livre"] -> a
    g[*] -> a
}

`
	if got := fmtSrc(t, src); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

// An object that does not fit in 100 columns goes one key per line.
func TestFormat_LongObjectBreaks(t *testing.T) {
	src := `episode "e" {
  context { a_long_key_number_one: "a fairly long value one", a_long_key_number_two: "a fairly long value two" }
  question q: binary "Ok?"
}
`
	got := fmtSrc(t, src)
	if !strings.Contains(got, "    context {\n        a_long_key_number_one: \"a fairly long value one\"\n        a_long_key_number_two: \"a fairly long value two\"\n    }\n") {
		t.Fatalf("got:\n%s", got)
	}
}
