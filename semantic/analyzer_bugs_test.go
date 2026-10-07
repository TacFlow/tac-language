package semantic

import (
	"testing"

	"github.com/TacFlow/tac-language/parser"
)

func analyzeSrc(t *testing.T, src string) []Diagnostic {
	t.Helper()
	prog, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return New().Analyze(prog)
}

func countCode(diags []Diagnostic, code string) int {
	n := 0
	for _, d := range diags {
		if d.Code == code {
			n++
		}
	}
	return n
}

// Bug c: SPEC §5.2 rule 3 — an unrecognised input type is unconstrained WITH
// a warning, never an error. v0.4.0 said nothing.
func TestBugC_UnknownInputTypeWarns(t *testing.T) {
	diags := analyzeSrc(t, `flow "f" {
  input a: string
  input b: Untrusted
  input c
  input d: Frobnicate
  input e: int
  node "n" -> skill validate(value: b)
}`)
	if got := countCode(diags, "TAC-TYPE-001"); got != 2 {
		t.Fatalf("TAC-TYPE-001 count = %d, want 2 (d, e): %v", got, diags)
	}
	for _, d := range diags {
		if d.Code == "TAC-TYPE-001" && d.Severity != SeverityWarning {
			t.Errorf("TAC-TYPE-001 must be a warning: %v", d)
		}
	}
}

// Bug e: the target of an `else:` fallback was reported unreachable.
func TestBugE_ElseTargetIsReachable(t *testing.T) {
	diags := analyzeSrc(t, `flow "f" {
  node "check"    -> skill web_search(query: "s")
  node "proceed"  -> skill memory_search(query: "ok")
  node "fallback" -> skill memory_search(query: "degraded")
  check -> proceed { if: check.confidence > 0.5, else: fallback }
  on "tick" -> check
}`)
	if got := countCode(diags, "TAC-GRAPH-003"); got != 0 {
		t.Fatalf("TAC-GRAPH-003 = %d, want 0: %v", got, diags)
	}
}
