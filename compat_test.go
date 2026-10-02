package tac_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
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

// The only differences v0.5 may introduce on v0.4 sources (DESIGN §5.11),
// each pinned to where it occurs. Anything else is a compatibility break.
var (
	// 1. versions in "language" — removed before comparing.
	// 2. args now carry their values (bug a): v0.4 wrote {"arg<i>": "<name>"};
	//    the names it lost the values of must be exactly the new keys
	//    (compareArgs). TestBugA_NamedArgsKeepTheirValues pins the values.
	// 3. the tail of a chained edge (bug b): extra edges, by source file.
	compatExtraEdges = map[string][]string{
		"testdata/compat/chained.tac": {"n1>n2"},
	}
	// 4. the unknown-input-type warning (bug c): added diagnostics.
	compatAddedDiags = map[string][]string{
		"examples/input_conformance.tac": {"TAC-TYPE-001@26", "TAC-TYPE-001@27", "TAC-TYPE-001@28"},
		"examples/value_types.tac":       {"TAC-TYPE-001@25"},
	}
	// 5. else targets are no longer TAC-GRAPH-003 (bug e): removed diagnostics.
	compatRemovedDiags = map[string][]string{
		"testdata/compat/else_target.tac": {"TAC-GRAPH-003@6"},
		// 3 again: with rank -> store kept, "store" is reachable.
		"testdata/compat/chained.tac": {"TAC-GRAPH-003@7"},
	}
)

type compatFlow struct {
	rest      string
	edges     []string
	edgeCount float64             // manifest.edge_count: grows with the extra chain edges
	args      map[string][]string // node id -> v0.4: the arg VALUES (names); v0.5: the arg KEYS
}

// argNames returns, per node, the sorted v0.4 arg values (old=true: v0.4 put
// the argument NAME in the value) or the sorted v0.5 arg keys.
func argNames(nodes []interface{}, old bool) map[string][]string {
	out := map[string][]string{}
	for _, n := range nodes {
		m := n.(map[string]interface{})
		id, _ := m["id"].(string)
		var names []string
		args, _ := m["args"].(map[string]interface{})
		for k, v := range args {
			if old {
				s, _ := v.(string)
				names = append(names, s)
			} else {
				names = append(names, k)
			}
		}
		sort.Strings(names)
		out[id] = names
	}
	return out
}

func normalizeCompatFlows(t *testing.T, raw json.RawMessage, old bool) []compatFlow {
	t.Helper()
	var flows []map[string]interface{}
	if err := json.Unmarshal(raw, &flows); err != nil {
		t.Fatal(err)
	}
	var out []compatFlow
	for _, f := range flows {
		delete(f, "language")
		var args map[string][]string
		if nodes, ok := f["nodes"].([]interface{}); ok {
			args = argNames(nodes, old)
			for _, n := range nodes {
				delete(n.(map[string]interface{}), "args")
			}
		}
		var edges []string
		if es, ok := f["edges"].([]interface{}); ok {
			for _, e := range es {
				b, _ := json.Marshal(e)
				m := e.(map[string]interface{})
				if len(m) == 2 { // plain from/to edge
					edges = append(edges, m["from"].(string)+">"+m["to"].(string))
				} else {
					edges = append(edges, string(b))
				}
			}
		}
		delete(f, "edges")
		var count float64
		if m, ok := f["manifest"].(map[string]interface{}); ok {
			count, _ = m["edge_count"].(float64)
			delete(m, "edge_count")
		}
		b, _ := json.Marshal(f)
		out = append(out, compatFlow{rest: string(b), edges: edges, edgeCount: count, args: args})
	}
	return out
}

func diagKeys(ds []compatDiag) map[string]int {
	m := map[string]int{}
	for _, d := range ds {
		m[d.Code+"@"+strconv.Itoa(d.Line)+"|"+d.Severity+"|"+d.Message]++
	}
	return m
}

func TestCompat_V04SourcesCompileUnchanged(t *testing.T) {
	b, err := os.ReadFile(compatBaselinePath)
	if err != nil {
		t.Fatal(err)
	}
	var baseline map[string]compatEntry
	if err := json.Unmarshal(b, &baseline); err != nil {
		t.Fatal(err)
	}
	srcs := compatSources(t)
	if len(baseline) != 10 {
		t.Fatalf("baseline has %d sources, want 10", len(baseline))
	}
	for name, old := range baseline {
		src, ok := srcs[name]
		if !ok {
			t.Errorf("%s: in the baseline but no longer present", name)
			continue
		}
		cur := compileForCompat(t, src)

		of, nf := normalizeCompatFlows(t, old.Flows, true), normalizeCompatFlows(t, cur.Flows, false)
		if len(of) != len(nf) {
			t.Errorf("%s: %d flows, was %d", name, len(nf), len(of))
			continue
		}
		for i := range of {
			if of[i].rest != nf[i].rest {
				t.Errorf("%s flow %d changed:\n was %s\n now %s", name, i, of[i].rest, nf[i].rest)
			}
			want := append(append([]string{}, of[i].edges...), compatExtraEdges[name]...)
			if strings.Join(nf[i].edges, ",") != strings.Join(want, ",") {
				t.Errorf("%s flow %d edges = %v, want %v", name, i, nf[i].edges, want)
			}
			if !reflect.DeepEqual(of[i].args, nf[i].args) {
				t.Errorf("%s flow %d: argument names %v, v0.4 had %v", name, i, nf[i].args, of[i].args)
			}
			if nf[i].edgeCount != of[i].edgeCount+float64(len(compatExtraEdges[name])) {
				t.Errorf("%s flow %d manifest.edge_count = %v, was %v", name, i, nf[i].edgeCount, of[i].edgeCount)
			}
		}

		oldKeys, curKeys := diagKeys(old.Diags), diagKeys(cur.Diags)
		for _, r := range compatRemovedDiags[name] {
			for k := range oldKeys {
				if strings.HasPrefix(k, r+"|") {
					delete(oldKeys, k)
				}
			}
		}
		for _, a := range compatAddedDiags[name] {
			found := false
			for k := range curKeys {
				if strings.HasPrefix(k, a+"|warning|") {
					delete(curKeys, k)
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: expected new warning %s", name, a)
			}
		}
		if !reflect.DeepEqual(oldKeys, curKeys) {
			t.Errorf("%s: diagnostics changed:\n was %v\n now %v", name, oldKeys, curKeys)
		}
	}
}

// testdata/compat/recovery/v0.4.0.json was written ONCE by the v0.4.0
// compiler (main@e185d55, the compileForCompat of this file) for the
// sources next to it: v0.4 sources with forms the parser cannot read. v0.4
// skipped such a form one token at a time and resumed at the next item, so
// `step: a -> b` kept the edge. v0.5 reports the form (TAC-PARSE-001) and
// must resume at exactly the same place: the IR is identical, and the only
// added diagnostics are TAC-PARSE-001 warnings.
const compatRecoveryBaselinePath = "testdata/compat/recovery/v0.4.0.json"

func TestCompat_V04ErrorRecoveryUnchanged(t *testing.T) {
	b, err := os.ReadFile(compatRecoveryBaselinePath)
	if err != nil {
		t.Fatal(err)
	}
	var baseline map[string]compatEntry
	if err := json.Unmarshal(b, &baseline); err != nil {
		t.Fatal(err)
	}
	if len(baseline) != 8 {
		t.Fatalf("recovery baseline has %d sources, want 8", len(baseline))
	}
	for name, old := range baseline {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		cur := compileForCompat(t, string(src))
		of, nf := normalizeCompatFlows(t, old.Flows, true), normalizeCompatFlows(t, cur.Flows, false)
		if len(of) != len(nf) {
			t.Errorf("%s: %d flows, v0.4 had %d", name, len(nf), len(of))
			continue
		}
		for i := range of {
			if of[i].rest != nf[i].rest {
				t.Errorf("%s flow %d changed:\n was %s\n now %s", name, i, of[i].rest, nf[i].rest)
			}
			if strings.Join(nf[i].edges, ",") != strings.Join(of[i].edges, ",") {
				t.Errorf("%s flow %d edges = %v, v0.4 had %v", name, i, nf[i].edges, of[i].edges)
			}
			if !reflect.DeepEqual(of[i].args, nf[i].args) {
				t.Errorf("%s flow %d: argument names %v, v0.4 had %v", name, i, nf[i].args, of[i].args)
			}
			if nf[i].edgeCount != of[i].edgeCount {
				t.Errorf("%s flow %d manifest.edge_count = %v, was %v", name, i, nf[i].edgeCount, of[i].edgeCount)
			}
		}
		var rest []compatDiag
		parse001 := 0
		for _, d := range cur.Diags {
			if d.Code == "TAC-PARSE-001" && d.Severity == "warning" {
				parse001++
				continue
			}
			rest = append(rest, d)
		}
		if parse001 == 0 {
			t.Errorf("%s: the unreadable forms are not reported (TAC-PARSE-001)", name)
		}
		if !reflect.DeepEqual(diagKeys(old.Diags), diagKeys(rest)) {
			t.Errorf("%s: diagnostics changed:\n was %v\n now %v", name, diagKeys(old.Diags), diagKeys(rest))
		}
	}
}
