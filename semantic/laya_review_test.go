package semantic

import (
	"strings"
	"testing"

	"github.com/TacFlow/tac-language/parser"
)

func diagsOf(t *testing.T, src string) []Diagnostic {
	t.Helper()
	prog, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return New().Analyze(prog)
}

// hasError reports whether diags has an error with code and a message
// containing text.
func hasError(diags []Diagnostic, code, text string) bool {
	for _, d := range diags {
		if d.Severity == SeverityError && d.Code == code && strings.Contains(d.Message, text) {
			return true
		}
	}
	return false
}

// M-2: an `else` on a labelled edge sends the run from the gate to its
// target without a branch label — an unlabelled edge out of a gate
// (TAC-LAYA-016, DESIGN §5.9).
func TestLayaGate_ElseOnLabelledEdgeIs016(t *testing.T) {
	src := acaoTask + gateFlow("  gate[proceed] -> a { if: gate.confidence > 0.5, else: b }\n  gate[*] -> b\n")
	if !hasError(diagsOf(t, src), "TAC-LAYA-016", "else") {
		t.Errorf("no TAC-LAYA-016 for else on a labelled edge: %v", diagsOf(t, src))
	}
	// A condition alone is not an unlabelled edge.
	ok := acaoTask + gateFlow("  gate[proceed] -> a { if: gate.confidence > 0.5 }\n  gate[*] -> b\n")
	for _, d := range diagsOf(t, ok) {
		if d.Code == "TAC-LAYA-016" {
			t.Errorf("if without else: unexpected %v", d)
		}
	}
}

const numTask = `task "n" {
  question q: number "N?" { range: [0, 100] }
}
`

func numGate(edges string) string {
	return numTask + `flow "g" {
  node "gate" -> skill laya.decide(task: "n", input: payload)
  node "a" -> skill laya.tasks.list()
` + edges + "  gate[*] -> a\n}\n"
}

// M-3: an empty or inverted range is TAC-LAYA-002 with a message that says
// so (not "must be a range"); a bound that overflows a float64 is rejected.
func TestLayaGate_RangeMessages(t *testing.T) {
	for _, r := range []string{"50..10", "5..5", "-1..-3"} {
		d := diagsOf(t, numGate("  gate["+r+"] -> a\n"))
		if !hasError(d, "TAC-LAYA-002", "empty or inverted range") {
			t.Errorf("[%s]: want TAC-LAYA-002 \"empty or inverted range\", got %v", r, d)
		}
	}
	for _, r := range []string{"<1e999", "1e999..1e1000", "0..1e999", ">=-1e999"} {
		d := diagsOf(t, numGate("  gate["+r+"] -> a\n"))
		if !hasError(d, "TAC-LAYA-002", "not a finite number") {
			t.Errorf("[%s]: want TAC-LAYA-002 \"not a finite number\", got %v", r, d)
		}
	}
	if d := diagsOf(t, numGate("  gate[5..10] -> a\n  gate[<5] -> a\n  gate[>=10] -> a\n")); hasError(d, "TAC-LAYA-002", "") {
		t.Errorf("valid ranges: %v", d)
	}
}
