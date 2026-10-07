package semantic

import (
	"sort"
	"strings"
	"testing"

	"github.com/TacFlow/tac-language/laya"
	"github.com/TacFlow/tac-language/parser"
)

const acaoTask = `task "acao" {
  question acao: choice "Qual ação?" {
    proceed: "p"
    reuse: "r"
    skip: "s"
    ask: "a"
  }
}
`

func codes(t *testing.T, src string, tasks []laya.Task) []string {
	t.Helper()
	prog, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a := New()
	if tasks != nil {
		a.SetTasks(tasks)
	}
	var out []string
	for _, d := range a.Analyze(prog) {
		if d.Code != "" {
			out = append(out, d.Code)
		}
	}
	sort.Strings(out)
	return out
}

func gateFlow(edges string) string {
	return `flow "g" {
  node "gate" -> skill laya.decide(task: "acao", input: payload)
  node "a" -> skill laya.tasks.list()
  node "b" -> skill laya.tasks.list()
  node "c" -> skill laya.tasks.list()
` + edges + "}\n"
}

func TestLayaAnalyzer_Codes(t *testing.T) {
	full := "  gate[proceed] -> a\n  gate[reuse] -> a\n  gate[skip] -> b\n  gate[ask] -> c\n  gate[low_confidence] -> c\n  gate[error] -> c\n"
	cases := []struct {
		name string
		src  string
		want string // sorted codes joined by ","
	}{
		{"full coverage is clean", acaoTask + gateFlow(full), ""},
		{"star covers the rest", acaoTask + gateFlow("  gate[proceed] -> a\n  gate[*] -> b\n"), ""},
		{"no registry", gateFlow("  gate[proceed] -> a\n  gate[*] -> b\n"), "TAC-LAYA-001"},
		{"unknown task", acaoTask + strings.Replace(gateFlow("  gate[x] -> a\n  gate[*] -> b\n"), `"acao"`, `"nada"`, 1), "TAC-LAYA-001"},
		{"unknown label", acaoTask + gateFlow("  gate[proceeed] -> a\n  gate[*] -> b\n"), "TAC-LAYA-002"},
		{"missing coverage", acaoTask + gateFlow("  gate[proceed] -> a\n  gate[reuse] -> b\n"), "TAC-LAYA-003"},
		{"duplicate label", acaoTask + gateFlow("  gate[proceed] -> a\n  gate[proceed] -> b\n  gate[*] -> c\n"), "TAC-LAYA-004"},
		{"label from a non-decide node", acaoTask + gateFlow("  a[proceed] -> b\n  gate[*] -> c\n"), "TAC-LAYA-009"},
		{"unlabelled spine edge from a gate", acaoTask + gateFlow("  gate[*] -> a\n  gate -> b\n"), "TAC-LAYA-016"},
		{"reserved pseudo-label in a task", strings.Replace(acaoTask, "ask: \"a\"", "error: \"a\"", 1), "TAC-LAYA-017"},
		{"gate on text", `task "t" {
  question q: text "Explain"
}
flow "g" {
  node "gate" -> skill laya.decide(task: "t", input: payload)
  node "a" -> skill laya.tasks.list()
  gate[*] -> a
}
`, "TAC-LAYA-005"},
		{"overlapping ranges", `task "n" {
  question q: number "How many?" { range: [0, 100] }
}
flow "g" {
  node "gate" -> skill laya.decide(task: "n", input: payload)
  node "a" -> skill laya.tasks.list()
  node "b" -> skill laya.tasks.list()
  gate[<10] -> a
  gate[5..50] -> b
  gate[*] -> a
}
`, "TAC-LAYA-011"},
		{"disjoint ranges incl. negative", `task "n" {
  question q: number "Delta?" { range: [-100, 100] }
}
flow "g" {
  node "gate" -> skill laya.decide(task: "n", input: payload)
  node "a" -> skill laya.tasks.list()
  node "b" -> skill laya.tasks.list()
  gate[<-5] -> a
  gate[-5..50] -> b
  gate[>=50] -> a
  gate[*] -> b
}
`, ""},
		{"binary labels", `task "b" {
  question q: binary "Ok?"
}
flow "g" {
  node "gate" -> skill laya.decide(task: "b", input: payload)
  node "a" -> skill laya.tasks.list()
  gate[true] -> a
  gate[false] -> a
  gate[low_confidence] -> a
  gate[error] -> a
}
`, ""},
		{"requires newer", "requires \"9.0\"\n", "TAC-VER-001"},
		{"unrecognized form", "flow \"f\" {\n  node \"a\" -> skill laya.tasks.list()\n  bogus thing here\n}\n", "TAC-PARSE-001"},
		{"bad cron", "flow \"f\" {\n  schedule \"61 * * * *\"\n  node \"a\" -> skill laya.tasks.list()\n}\n", "TAC-SCHED-001"},
		{"bad tz", "flow \"f\" {\n  schedule \"0 3 * * *\" tz \"Mars/Phobos\"\n  node \"a\" -> skill laya.tasks.list()\n}\n", "TAC-SCHED-002"},
		{"schedule on an event-only flow", "flow \"f\" {\n  schedule \"0 3 * * *\"\n  on \"x\" -> a\n  node \"a\" -> skill laya.tasks.list()\n}\n", "TAC-EVT-002"},
		{"model with unknown task", "model \"m\" { tasks [\"nada\"] }\n", "TAC-LAYA-010"},
		{"splits do not sum to 1", "dataset \"d\" {\n  task \"acao\"\n  split train = 0.5\n  split test = 0.1\n}\n", "TAC-LAYA-012"},
		{"duplicate episode id", strings.Repeat("episode \"e1\" {\n  goal \"g\"\n  question q: binary \"Ok?\"\n}\n", 2), "TAC-LAYA-013"},
		{"target for an undeclared question", "episode \"e1\" {\n  goal \"g\"\n  question q: binary \"Ok?\"\n  target z = true\n}\n", "TAC-LAYA-006"},
		{"invalid target value", "episode \"e1\" {\n  goal \"g\"\n  question q: binary \"Ok?\"\n  target q = maybe\n}\n", "TAC-LAYA-007"},
		{"unknown question type", "episode \"e1\" {\n  goal \"g\"\n  question q: vibe \"Ok?\"\n}\n", "TAC-LAYA-008"},
		{"misspelt item in a task", strings.Replace(acaoTask, "  question acao:", "  quesiton x: choice \"Q\" { a: \"1\", b: \"2\" }\n  question acao:", 1), "TAC-PARSE-001"},
		{"task whose only question is misspelt", "task \"t\" {\n  quesiton x: choice \"Q\" { a: \"1\", b: \"2\" }\n}\n", "TAC-PARSE-001"},
		{"malformed option drops the question", "task \"t\" {\n  question q: choice \"Q\" { a: 3, b: \"2\" }\n}\n", "TAC-PARSE-001"},
		{"misspelt target", "episode \"e1\" {\n  goal \"g\"\n  question q: binary \"Ok?\"\n  targte q = true\n}\n", "TAC-PARSE-001"},
		{"misspelt dataset items", "dataset \"d\" {\n  task \"acao\"\n  taks \"x\"\n  splt train = 1\n}\n", "TAC-PARSE-001,TAC-PARSE-001"},
		{"number glued to text", "flow \"f\" {\n  node \"a\" -> skill laya.tasks.describe(task: \"t\", x: 3x)\n}\n", "TAC-PARSE-001"},
		{"scientific notation is a number", "episode \"e1\" {\n  goal \"g\"\n  limite 1e3\n  question q: binary \"Ok?\"\n}\n", ""},
		{"junk in a context block", "context \"c\" {\n  remember x = \"y\"\n  this is junk\n}\n", "TAC-PARSE-001"},
		{"too many reduced questions", "episode \"e1\" {\n  goal \"g\"\n" +
			strings.Repeat("  question qX: ranking \"R\" { a: \"1\", b: \"2\", c: \"3\", d: \"4\" }\n", 6) + "}\n", "TAC-LAYA-014"},
	}
	for _, c := range cases {
		src := c.src
		if c.name == "too many reduced questions" {
			for i := 0; strings.Contains(src, "qX"); i++ {
				src = strings.Replace(src, "qX", "q"+string(rune('a'+i)), 1)
			}
		}
		got := strings.Join(codes(t, src, nil), ",")
		if got != c.want {
			t.Errorf("%s: codes = %q, want %q", c.name, got, c.want)
		}
	}
}

// A --tasks registry makes a gate's labels checkable without a `task` in the file.
func TestLayaAnalyzer_ExternalRegistry(t *testing.T) {
	reg := []laya.Task{{Name: "acao", Questions: []laya.Question{{ID: "acao", Type: "choice", Instructions: "?",
		Options: []laya.Option{{Label: "proceed"}, {Label: "ask"}}}}}}
	got := strings.Join(codes(t, gateFlow("  gate[proceed] -> a\n  gate[nope] -> b\n  gate[*] -> c\n"), reg), ",")
	if got != "TAC-LAYA-002" {
		t.Fatalf("codes = %q, want TAC-LAYA-002", got)
	}
}

// Passing a gate's Hallucinable output to an argument that requires Fact is
// TAC-TRUST-001 (D16). Routing on it is fine.
func TestLayaAnalyzer_GateOutputIsHallucinable(t *testing.T) {
	src := acaoTask + `flow "g" {
  node "gate"  -> skill laya.decide(task: "acao", input: payload)
  node "store" -> skill memory_store(text: gate.label)
  gate[*] -> store
}
`
	if got := strings.Join(codes(t, src, nil), ","); got != "TAC-TRUST-001" {
		t.Fatalf("codes = %q, want TAC-TRUST-001", got)
	}
}

// A task with no question cannot be gated or trained on: an error, not an
// empty task (it is what remains when every question is misspelt).
func TestLayaAnalyzer_TaskWithoutQuestionIsAnError(t *testing.T) {
	prog, err := parser.ParseSource("task \"t\" {\n  quesiton x: choice \"Q\" { a: \"1\", b: \"2\" }\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	a := New()
	a.Analyze(prog)
	found := false
	for _, d := range a.Errors() {
		if strings.Contains(d.Message, `task "t" declares no question`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("want the error, got %v", a.Diagnostics())
	}
}

// A gate naming a question its task does not have says so (not "unknown task").
func TestLayaAnalyzer_UnknownQuestionMessage(t *testing.T) {
	src := acaoTask + strings.Replace(gateFlow("  gate[*] -> a\n"), `input: payload)`, `input: payload, question: "nada")`, 1)
	prog, err := parser.ParseSource(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range New().Analyze(prog) {
		if d.Code == "TAC-LAYA-001" {
			if !strings.Contains(d.Message, `has no question "nada"`) {
				t.Fatalf("message = %q", d.Message)
			}
			return
		}
	}
	t.Fatal("no TAC-LAYA-001")
}
