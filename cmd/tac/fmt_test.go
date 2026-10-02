package main

import (
	"bytes"
	"strings"
	"testing"
)

// I-3: `tac fmt` must never delete text it could not read. A source with a
// form the parser skipped (TAC-PARSE-001: unrecognised forms, numbers glued
// to text) is refused: the diagnostics go to stderr, nothing to stdout, and
// the exit code is 1 — a `tac fmt f.tac > f.tac` cannot lose the text.
func TestFmt_RefusesSourcesWithUnreadableForms(t *testing.T) {
	cases := map[string]int{ // source -> TAC-PARSE-001 diagnostics expected
		read(t, "../../conformance/laya/32_typos_in_declarations.tac"): 4,
		read(t, "../../conformance/laya/21_unrecognized.tac"):          1,
		"flow \"f\" {\n  node \"a\" -> skill web_search(query: \"q\", count: 3x)\n}\n": 1,
		"stray words\nflow \"f\" {\n  node \"a\" -> skill web_search(query: \"q\")\n}\n": 1,
	}
	for src, want := range cases {
		var out, errb bytes.Buffer
		code := runFmt(src, &out, &errb)
		if code != 1 {
			t.Errorf("exit code %d, want 1 for:\n%s", code, src)
		}
		if out.Len() != 0 {
			t.Errorf("stdout must be empty, got:\n%s", out.String())
		}
		if got := strings.Count(errb.String(), "[TAC-PARSE-001]"); got != want {
			t.Errorf("stderr has %d TAC-PARSE-001, want %d:\n%s", got, want, errb.String())
		}
	}
}

func TestFmt_FormatsCleanSources(t *testing.T) {
	var out, errb bytes.Buffer
	code := runFmt(read(t, "../../examples/laya_gate.tac"), &out, &errb)
	if code != 0 || out.Len() == 0 {
		t.Fatalf("clean source: exit %d, stdout %d bytes, stderr %s", code, out.Len(), errb.String())
	}
}
