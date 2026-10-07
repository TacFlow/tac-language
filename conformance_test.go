package tac_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/TacFlow/tac-language/compiler"
	"github.com/TacFlow/tac-language/laya"
	"github.com/TacFlow/tac-language/parser"
	"github.com/TacFlow/tac-language/semantic"
)

// The shared LAYA conformance corpus (DESIGN §5.10, §10): conformance/laya.
// The platform dialect runs the same cases against its own compiler.

type conformanceExpected struct {
	Only          string          `json:"only"`
	OKUpstream    bool            `json:"ok_upstream"`
	OKDialect     bool            `json:"ok_dialect"`
	CodesUpstream []string        `json:"codes_upstream"`
	CodesDialect  []string        `json:"codes_dialect"`
	Shape         json.RawMessage `json:"shape"`
}

type shapeNode struct {
	Name  string                 `json:"name"`
	Skill string                 `json:"skill"`
	Args  map[string]interface{} `json:"args"`
}

type shapeEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}

type shapeTrigger struct {
	Event   string   `json:"event"`
	Targets []string `json:"targets"`
}

type shapeFlow struct {
	Name      string                `json:"name"`
	Nodes     []shapeNode           `json:"nodes"`
	Edges     []shapeEdge           `json:"edges"`
	Triggers  []shapeTrigger        `json:"triggers"`
	Schedules []compiler.ScheduleIR `json:"schedules"`
}

type shape struct {
	Flows    []shapeFlow    `json:"flows"`
	Tasks    []laya.Task    `json:"tasks"`
	Models   []laya.Model   `json:"models"`
	Episodes []laya.Episode `json:"episodes"`
	Datasets []laya.Dataset `json:"datasets"`
}

// upstreamShape reduces the program IR to the common shape.
func upstreamShape(ir *compiler.ProgramIR) shape {
	s := shape{Flows: []shapeFlow{}, Tasks: ir.Tasks, Models: ir.Models, Episodes: ir.Episodes, Datasets: ir.Datasets}
	for _, f := range ir.Flows {
		names := map[string]string{}
		sf := shapeFlow{Name: f.Name, Nodes: []shapeNode{}, Edges: []shapeEdge{}, Triggers: []shapeTrigger{}, Schedules: f.Schedules}
		if sf.Schedules == nil {
			sf.Schedules = []compiler.ScheduleIR{}
		}
		for _, n := range f.Nodes {
			names[n.ID] = n.Name
			args := n.Args
			if args == nil {
				args = map[string]interface{}{}
			}
			sf.Nodes = append(sf.Nodes, shapeNode{n.Name, n.Skill, args})
		}
		for _, e := range f.Edges {
			label := e.Label
			if r := e.Range; r != nil {
				if r.Op != "" {
					label = "range:" + r.Op + string(r.Value)
				} else {
					label = "range:" + string(r.Min) + ".." + string(r.Max)
				}
			}
			sf.Edges = append(sf.Edges, shapeEdge{names[e.From], names[e.To], label})
		}
		for _, tr := range f.Triggers {
			st := shapeTrigger{Event: tr.Event, Targets: []string{}}
			for _, id := range tr.Targets {
				st.Targets = append(st.Targets, names[id])
			}
			sf.Triggers = append(sf.Triggers, st)
		}
		s.Flows = append(s.Flows, sf)
	}
	return s
}

func canonJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := laya.CanonicalValue(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestConformance_LayaCorpus(t *testing.T) {
	paths, err := filepath.Glob("conformance/laya/*.tac")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < 33 {
		t.Fatalf("corpus has %d cases, want at least 33", len(paths))
	}
	for _, p := range paths {
		name := strings.TrimSuffix(filepath.Base(p), ".tac")
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("conformance/laya/expected", name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var want conformanceExpected
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatal(err)
			}
			if want.Only == "dialect" {
				t.Skip("dialect-only case")
			}
			prog, err := parser.ParseSource(readFile(t, p))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			a := semantic.New()
			codes := []string{}
			for _, d := range a.Analyze(prog) {
				if d.Code != "" {
					codes = append(codes, d.Code)
				}
			}
			sort.Strings(codes)
			if strings.Join(codes, ",") != strings.Join(want.CodesUpstream, ",") {
				t.Errorf("codes = %v, want %v", codes, want.CodesUpstream)
			}
			if ok := !a.HasErrors(); ok != want.OKUpstream {
				t.Fatalf("ok = %v, want %v (%v)", ok, want.OKUpstream, a.Errors())
			}
			if !want.OKUpstream {
				return
			}
			ir, err := compiler.CompileProgramIR(prog)
			if err != nil {
				t.Fatal(err)
			}
			var wantShape interface{}
			if err := json.Unmarshal(want.Shape, &wantShape); err != nil || wantShape == nil {
				t.Fatalf("expected shape missing: %v", err)
			}
			if got, exp := canonJSON(t, upstreamShape(ir)), canonJSON(t, wantShape); got != exp {
				t.Fatalf("shape differs:\n got  %s\n want %s", got, exp)
			}
		})
	}
}

// TestConformance_WriteUpstream (TAC_WRITE_CONFORMANCE=1) fills the upstream
// half of every expected file — ok_upstream, codes_upstream and, when the
// upstream compile succeeds, shape — keeping the dialect half as written.
// The result is reviewed by hand before it is committed.
func TestConformance_WriteUpstream(t *testing.T) {
	if os.Getenv("TAC_WRITE_CONFORMANCE") != "1" {
		t.Skip("set TAC_WRITE_CONFORMANCE=1 to (re)write the upstream half of conformance/laya/expected")
	}
	paths, _ := filepath.Glob("conformance/laya/*.tac")
	for _, p := range paths {
		name := strings.TrimSuffix(filepath.Base(p), ".tac")
		ep := filepath.Join("conformance/laya/expected", name+".json")
		var e conformanceExpected
		if raw, err := os.ReadFile(ep); err == nil {
			if err := json.Unmarshal(raw, &e); err != nil {
				t.Fatal(err)
			}
		}
		if e.CodesDialect == nil {
			e.CodesDialect = []string{}
		}
		e.CodesUpstream = []string{}
		e.OKUpstream = false
		if e.Only != "dialect" {
			prog, err := parser.ParseSource(readFile(t, p))
			if err != nil {
				t.Fatal(err)
			}
			a := semantic.New()
			for _, d := range a.Analyze(prog) {
				if d.Code != "" {
					e.CodesUpstream = append(e.CodesUpstream, d.Code)
				}
			}
			sort.Strings(e.CodesUpstream)
			e.OKUpstream = !a.HasErrors()
			if e.OKUpstream {
				ir, err := compiler.CompileProgramIR(prog)
				if err != nil {
					t.Fatal(err)
				}
				b, _ := json.Marshal(upstreamShape(ir))
				e.Shape = b
			}
		}
		if len(e.Shape) == 0 {
			e.Shape = json.RawMessage("null")
		}
		out, err := json.MarshalIndent(e, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ep, append(out, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
