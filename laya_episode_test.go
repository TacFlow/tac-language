package tac_test

import (
	"os"
	"testing"

	"github.com/TacFlow/tac-language/compiler"
	"github.com/TacFlow/tac-language/laya"
	"github.com/TacFlow/tac-language/parser"
)

// The golden of the LAYA mission (DESIGN §5.5, §11): the reference episode
// written in TAC compiles to JSON structurally equal (JCS) to the reference
// JSON, testdata/laya_episode.reference.json (a verbatim copy of the
// client-app's docs/laya/schema/reference-episode.json).
func TestLayaEpisode_ReferenceGolden(t *testing.T) {
	src, err := os.ReadFile("examples/laya_episode.tac")
	if err != nil {
		t.Fatal(err)
	}
	prog, err := parser.ParseSource(string(src))
	if err != nil {
		t.Fatal(err)
	}
	ir, err := compiler.CompileProgramIR(prog)
	if err != nil {
		t.Fatal(err)
	}
	if len(ir.Episodes) != 1 {
		t.Fatalf("episodes = %d, want 1", len(ir.Episodes))
	}
	got, err := laya.CanonicalValue(ir.Episodes[0])
	if err != nil {
		t.Fatal(err)
	}
	ref, err := os.ReadFile("testdata/laya_episode.reference.json")
	if err != nil {
		t.Fatal(err)
	}
	want, err := laya.Canonical(ref)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("episode differs from the reference:\n got  %s\n want %s", got, want)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
