package tac_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/TacFlow/tac-language/compiler"
	"github.com/TacFlow/tac-language/parser"
	"github.com/TacFlow/tac-language/semantic"
)

// The v0.4.0 compatibility baseline (DESIGN §5.11 of the LAYA mission).
//
// testdata/compat/v0.4.0.json was written ONCE, by the v0.4.0 compiler, before
// any v0.5 change landed. It must never be regenerated: it is the evidence of
// what v0.4.0 did. TestCompat_V04SourcesCompileUnchanged (added later) compares
// the current compiler against it and accepts only the listed differences.

const compatBaselinePath = "testdata/compat/v0.4.0.json"

type compatDiag struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Message  string `json:"message"`
}

type compatEntry struct {
	Flows json.RawMessage `json:"flows"`
	Diags []compatDiag    `json:"diags"`
}

// compatSources returns every v0.4 source the compatibility check covers:
// examples/*.tac that existed in v0.4.0 plus testdata/compat/*.tac.
func compatSources(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, pattern := range []string{"examples/*.tac", "testdata/compat/*.tac"} {
		paths, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range paths {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			out[filepath.ToSlash(p)] = string(b)
		}
	}
	return out
}

func compileForCompat(t *testing.T, src string) compatEntry {
	t.Helper()
	program, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := semantic.New().Analyze(program)
	flows, err := compiler.CompileProgram(program)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	raw, err := json.Marshal(flows)
	if err != nil {
		t.Fatal(err)
	}
	out := compatEntry{Flows: raw, Diags: []compatDiag{}}
	for _, d := range diags {
		out.Diags = append(out.Diags, compatDiag{d.Code, d.Severity.String(), d.Line, d.Col, d.Message})
	}
	sort.Slice(out.Diags, func(i, j int) bool {
		a, b := out.Diags[i], out.Diags[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		return a.Message < b.Message
	})
	return out
}

// TestCompat_WriteBaseline writes the frozen baseline. It only runs with
// TAC_WRITE_COMPAT_BASELINE=1 and refuses to overwrite an existing file.
func TestCompat_WriteBaseline(t *testing.T) {
	if os.Getenv("TAC_WRITE_COMPAT_BASELINE") != "1" {
		t.Skip("set TAC_WRITE_COMPAT_BASELINE=1 to write the frozen v0.4.0 baseline")
	}
	if _, err := os.Stat(compatBaselinePath); err == nil {
		t.Fatalf("%s already exists: the v0.4.0 baseline is frozen and must not be regenerated", compatBaselinePath)
	}
	all := map[string]compatEntry{}
	for name, src := range compatSources(t) {
		all[name] = compileForCompat(t, src)
	}
	b, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(compatBaselinePath, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
