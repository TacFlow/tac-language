package tac_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TacFlow/tac-language/compiler"
	"github.com/TacFlow/tac-language/formatter"
	"github.com/TacFlow/tac-language/laya"
	"github.com/TacFlow/tac-language/parser"
)

// stripPos removes every "pos" key: positions legitimately move when the
// formatter re-lays the source out.
func stripPos(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		delete(t, "pos")
		for k, x := range t {
			t[k] = stripPos(x)
		}
	case []interface{}:
		for i, x := range t {
			t[i] = stripPos(x)
		}
	}
	return v
}

// compiledCanon is the program IR of src, without positions, canonical.
func compiledCanon(t *testing.T, src string) string {
	t.Helper()
	prog, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ir, err := compiler.CompileProgramIR(prog)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	raw, err := json.Marshal(ir)
	if err != nil {
		t.Fatal(err)
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(stripPos(v))
	if err != nil {
		t.Fatal(err)
	}
	c, err := laya.Canonical(b)
	if err != nil {
		t.Fatal(err)
	}
	return string(c)
}

func formatSrc(t *testing.T, src string) string {
	t.Helper()
	prog, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return formatter.Format(prog)
}

// DESIGN §5.5: fmt(parse(fmt(x))) == fmt(x) and compile(fmt(x)) ≡ compile(x)
// (JCS) for every example and every conformance case.
func TestFormatter_RoundTripExamplesAndCorpus(t *testing.T) {
	var paths []string
	for _, pattern := range []string{"examples/*.tac", "conformance/laya/*.tac"} {
		ps, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, ps...)
	}
	if len(paths) < 13 {
		t.Fatalf("found %d sources, want the 7 v0.4 examples + 6 LAYA examples at least", len(paths))
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			if dialectOnly(t, p) {
				t.Skip("dialect-only conformance case")
			}
			src := readFile(t, p)
			once := formatSrc(t, src)
			if twice := formatSrc(t, once); twice != once {
				t.Fatalf("formatter not idempotent\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
			}
			if a, b := compiledCanon(t, src), compiledCanon(t, once); a != b {
				t.Fatalf("formatting changed the compiled program\n--- source ---\n%s\n--- formatted ---\n%s", a, b)
			}
		})
	}
}

// dialectOnly reports whether p is a conformance case only the platform
// dialect compiles (expected "only": "dialect").
func dialectOnly(t *testing.T, p string) bool {
	t.Helper()
	if filepath.Dir(p) != filepath.Join("conformance", "laya") {
		return false
	}
	exp := filepath.Join("conformance", "laya", "expected", strings.TrimSuffix(filepath.Base(p), ".tac")+".json")
	var e struct {
		Only string `json:"only"`
	}
	if err := json.Unmarshal([]byte(readFile(t, exp)), &e); err != nil {
		t.Fatal(err)
	}
	return e.Only == "dialect"
}
