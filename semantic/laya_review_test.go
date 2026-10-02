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
