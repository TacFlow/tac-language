package semantic

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/TacFlow/tac-language/ast"
	"github.com/TacFlow/tac-language/laya"
)

// LAYA diagnostics (DESIGN §5.9 of the LAYA mission).
const (
	DiagLaya  = "TAC-LAYA"
	DiagVer   = "TAC-VER"
	DiagEvt   = "TAC-EVT"
	DiagSched = "TAC-SCHED"
)

// SupportedLanguage is the newest language level this analyzer understands.
// A `requires` above it is TAC-VER-001 (warning).
const SupportedLanguage = "0.5"

// SetTasks gives the analyzer an external task registry (`tac --tasks`, or
// the platform's /v1/laya/tasks). Tasks declared in the file are added to
// it; a file task wins over a registry task with the same name. Without a
// registry and without `task` declarations, gate labels are not checked
// (TAC-LAYA-001).
func (a *Analyzer) SetTasks(tasks []laya.Task) {
	if a.tasks == nil {
		a.tasks = map[string]laya.Task{}
	}
	for _, t := range tasks {
		a.tasks[t.Name] = t
	}
	a.registry_ = true
}

// versionLess compares dotted numeric versions ("0.5" < "0.10").
func versionLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x < y
		}
	}
	return false
}

func (a *Analyzer) issue(is laya.Issue, line, col int, prefix string) {
	msg := prefix + is.Msg
	if is.Path != "" {
		msg = prefix + is.Path + ": " + is.Msg
	}
	if is.Warning {
		a.warningf(is.Code, line, col, "%s", msg)
	} else {
		a.errorf(is.Code, line, col, "%s", msg)
	}
}

// analyzeDecls checks the program-level v0.5 declarations and registers the
// file's tasks. Called before the flows are validated.
func (a *Analyzer) analyzeDecls(program *ast.Node) {
	if a.tasks == nil {
		a.tasks = map[string]laya.Task{}
	}
	for _, n := range program.Nodes {
		if n.Type == ast.NodeTaskDecl {
			t := laya.TaskFromAST(n)
			a.tasks[t.Name] = t
			a.registry_ = true
			if len(t.Questions) == 0 {
				a.errorf("", n.Pos.Line, n.Pos.Col, "task %q declares no question", t.Name)
			}
			for i, q := range t.Questions {
				p := fmt.Sprintf("task %q questions[%d]", t.Name, i)
				for _, is := range laya.ValidateQuestion(q, p) {
					a.issue(is, n.Pos.Line, n.Pos.Col, "")
				}
				for _, is := range laya.ReservedLabelIssues(q, p) {
					a.issue(is, n.Pos.Line, n.Pos.Col, "")
				}
			}
		}
	}
	seenEpisode := map[string]bool{}
	for _, n := range program.Nodes {
		switch n.Type {
		case ast.NodeTaskDecl, ast.NodeModelDecl, ast.NodeEpisodeDecl, ast.NodeDatasetDecl:
			// An item the parser could not read inside a declaration is never
			// dropped in silence (a misspelt `quesiton`, `targte`, `splt`…).
			for _, c := range n.Children {
				if c.Type == ast.NodeUnrecognized {
					a.warningf(DiagParse+"-001", c.Pos.Line, c.Pos.Col,
						"%s %q: unrecognized item starting at %s; it was ignored", declKeyword(n.Type), n.Value, c.Value)
				}
			}
		}
		switch n.Type {
		case ast.NodeGluedNumber:
			a.warningf(DiagParse+"-001", n.Pos.Line, n.Pos.Col,
				"%q: a number glued to text is read as the number followed by the rest, as in v0.4; separate them with a space or drop the suffix", n.Value)
		case ast.NodeUnrecognized:
			a.warningf(DiagParse+"-001", n.Pos.Line, n.Pos.Col,
				"unrecognized form starting at %s; it was ignored", n.Value)
		case ast.NodeRequires:
			if versionLess(SupportedLanguage, n.Value) {
				a.warningf(DiagVer+"-001", n.Pos.Line, n.Pos.Col,
					"requires %q is newer than this compiler (language %s); constructs it does not know are ignored",
					n.Value, SupportedLanguage)
			}
		case ast.NodeModelDecl:
			m := laya.ModelFromAST(n)
			for _, t := range m.Tasks {
				if _, ok := a.tasks[t]; !ok {
					a.warningf(DiagLaya+"-010", n.Pos.Line, n.Pos.Col,
						"model %q lists unknown task %q", m.Name, t)
				}
			}
		case ast.NodeDatasetDecl:
			d := laya.DatasetFromAST(n)
			if d.Task == "" {
				a.errorf("", n.Pos.Line, n.Pos.Col, "dataset %q: task is required", d.Name)
			}
			if len(d.Splits) > 0 {
				sum := 0.0
				for name, v := range d.Splits {
					f, err := strconv.ParseFloat(string(v), 64)
					if err != nil || f < 0 {
						a.errorf("", n.Pos.Line, n.Pos.Col, "dataset %q: split %q must be a non-negative number", d.Name, name)
					}
					switch name {
					case "train", "validation", "calibration", "test":
					default:
						a.errorf("", n.Pos.Line, n.Pos.Col,
							"dataset %q: unknown split %q (train, validation, calibration, test)", d.Name, name)
					}
					sum += f
				}
				if math.Abs(sum-1) > 1e-9 {
					a.errorf(DiagLaya+"-012", n.Pos.Line, n.Pos.Col,
						"dataset %q: splits sum to %g, not 1", d.Name, sum)
				}
			}
		case ast.NodeEpisodeDecl:
			if seenEpisode[n.Value] {
				a.warningf(DiagLaya+"-013", n.Pos.Line, n.Pos.Col,
					"episode id %q is declared more than once; the last one wins", n.Value)
			}
			seenEpisode[n.Value] = true
			e := laya.EpisodeFromAST(n)
			for _, is := range laya.ValidateEpisode(e) {
				a.issue(is, n.Pos.Line, n.Pos.Col, fmt.Sprintf("episode %q ", e.ID))
			}
		}
	}
}

func declKeyword(t ast.NodeType) string {
	switch t {
	case ast.NodeTaskDecl:
		return "task"
	case ast.NodeModelDecl:
		return "model"
	case ast.NodeEpisodeDecl:
		return "episode"
	}
	return "dataset"
}

// stringArg returns the string literal of a named argument of a skill call.
func stringArg(call *ast.Node, name string) (string, bool) {
	for _, arg := range call.Args {
		if arg.Type == ast.NodeNamedArg && arg.Value == name && len(arg.Children) > 0 &&
			arg.Children[0].Type == ast.NodeStringLiteral {
			return arg.Children[0].Value, true
		}
	}
	return "", false
}

// validateLayaFlow runs the v0.5 checks of one flow: unrecognised forms,
// schedules, event-only entries and gates (DESIGN §5.4).
func (a *Analyzer) validateLayaFlow(flow *ast.Node, flowName string, declared map[string]*ast.Node) {
	hasSchedule, hasTrigger := false, false
	triggerTargets := map[string]bool{}
	for _, c := range flow.Children {
		switch c.Type {
		case ast.NodeUnrecognized:
			a.warningf(DiagParse+"-001", c.Pos.Line, c.Pos.Col,
				"flow %q: unrecognized form starting at %s; it was ignored", flowName, c.Value)
		case ast.NodeSchedule:
			hasSchedule = true
			if len(c.Children) > 0 {
				if err := ValidateCron(c.Children[0].Value); err != nil {
					a.errorf(DiagSched+"-001", c.Pos.Line, c.Pos.Col,
						"flow %q: schedule %q: the scheduler does not accept this cron: %v", flowName, c.Children[0].Value, err)
				}
			}
			if tz := c.Attrs["tz"]; tz != nil {
				if err := ValidateTZ(tz.Value); err != nil {
					a.errorf(DiagSched+"-002", c.Pos.Line, c.Pos.Col, "flow %q: schedule tz: %v", flowName, err)
				}
			}
		case ast.NodeTrigger:
			hasTrigger = true
			if len(c.Children) > 1 {
				last := c.Children[len(c.Children)-1]
				if last.Type == ast.NodeIdentifier {
					triggerTargets[last.Value] = true
				}
				for _, it := range last.ArrVal {
					triggerTargets[it.Value] = true
				}
			}
		}
	}
	// TAC-EVT-002: a schedule fires runs that no event pattern matched, so
	// in a flow whose every root is an `on` target it does nothing.
	if hasSchedule && hasTrigger {
		incoming := map[string]bool{}
		for _, e := range flow.Edges {
			incoming[ast.EdgeTarget(e)] = true
		}
		manualRoot := false
		for name := range declared {
			if !incoming[name] && !triggerTargets[name] {
				manualRoot = true
			}
		}
		if !manualRoot {
			a.warningf(DiagEvt+"-002", flow.Pos.Line, flow.Pos.Col,
				"flow %q: every entry is an `on` event, so its schedule starts runs that do nothing", flowName)
		}
	}

	// Gates: group the labelled edges by source.
	labelled := map[string][]branchEdge{}
	plain := map[string][]*ast.Node{}
	var order []string
	for _, e := range flow.Edges {
		src := ast.EdgeSource(e)
		if _, ok := declared[src]; !ok {
			continue
		}
		if _, seen := labelled[src]; !seen {
			if _, seen2 := plain[src]; !seen2 {
				order = append(order, src)
			}
		}
		if b := ast.EdgeLabel(e); b != nil {
			labelled[src] = append(labelled[src], branchEdge{e, ast.LabelString(b)})
		} else {
			plain[src] = append(plain[src], e)
		}
	}
	for _, src := range order {
		outs := labelled[src]
		if len(outs) == 0 {
			continue
		}
		call := findSkillCall(declared[src])
		if call == nil || call.Value != "laya.decide" {
			for _, o := range outs {
				a.errorf(DiagLaya+"-009", o.edge.Pos.Line, o.edge.Pos.Col,
					"flow %q: labelled edge %s[%s] from node %q, which is not a laya.decide gate", flowName, src, o.label, src)
			}
			continue
		}
		a.validateGate(flowName, src, call, outs, plain[src])
	}
}

// branchEdge is one labelled edge out of a gate and its canonical label.
type branchEdge struct {
	edge  *ast.Node
	label string
}

func (a *Analyzer) validateGate(flowName, src string, call *ast.Node, outs []branchEdge, plain []*ast.Node) {
	for _, e := range plain {
		a.errorf(DiagLaya+"-016", e.Pos.Line, e.Pos.Col,
			"flow %q: edge %s -> %s leaves gate %q without a label; every edge out of a gate is a branch",
			flowName, src, ast.EdgeTarget(e), src)
	}
	// `gate[x] -> a { if: …, else: b }`: when the condition fails the run
	// goes from the gate to b with no branch label — an unlabelled edge out
	// of a gate, like the plain edges above.
	for _, o := range outs {
		if _, fb, _ := ast.EdgeCondition(o.edge); fb != "" {
			a.errorf(DiagLaya+"-016", o.edge.Pos.Line, o.edge.Pos.Col,
				"flow %q: edge %s[%s] -> %s has else: %s, which leaves gate %q without a label; give that path its own branch",
				flowName, src, o.label, ast.EdgeTarget(o.edge), fb, src)
		}
	}
	seen := map[string]bool{}
	labels := map[string]bool{}
	for _, o := range outs {
		if seen[o.label] {
			a.errorf(DiagLaya+"-004", o.edge.Pos.Line, o.edge.Pos.Col,
				"flow %q: gate %q has two edges labelled [%s]", flowName, src, o.label)
		}
		seen[o.label] = true
		labels[o.label] = true
	}

	// Resolve the question the gate answers.
	pos := call.Pos
	taskName, hasTask := stringArg(call, "task")
	task, known := a.tasks[taskName]
	var q laya.Question
	missingQuestion := ""
	if hasTask && known && len(task.Questions) > 0 {
		q = task.Questions[0]
		if qid, ok := stringArg(call, "question"); ok {
			found := false
			for _, x := range task.Questions {
				if x.ID == qid {
					q, found = x, true
				}
			}
			if !found {
				known, missingQuestion = false, qid
			}
		}
	} else {
		known = false
	}
	if !known {
		if missingQuestion != "" {
			a.warningf(DiagLaya+"-001", pos.Line, pos.Col,
				"flow %q: gate %q: task %q has no question %q; labels are not checked", flowName, src, taskName, missingQuestion)
		} else if !a.registry_ {
			a.warningf(DiagLaya+"-001", pos.Line, pos.Col,
				"flow %q: gate %q: no task registry (declare `task %q` or pass --tasks); labels are not checked", flowName, src, taskName)
		} else {
			a.warningf(DiagLaya+"-001", pos.Line, pos.Col,
				"flow %q: gate %q: unknown task %q; labels are not checked", flowName, src, taskName)
		}
	} else if !laya.Gateable(q.Type) {
		a.errorf(DiagLaya+"-005", pos.Line, pos.Col,
			"flow %q: gate %q branches on question %q of type %s, which has no labels", flowName, src, q.ID, q.Type)
		known = false
	} else if !laya.KnownType(q.Type) {
		known = false // TAC-LAYA-008 is reported on the task
	}

	if known {
		valid := map[string]bool{}
		for _, l := range laya.TaskLabels(q) {
			valid[l] = true
		}
		var ranges []laya.Interval
		for _, o := range outs {
			if laya.IsPseudoLabel(o.label) {
				continue
			}
			if q.Type == laya.TypeNumber {
				iv, err := laya.CheckRangeLabel(o.label)
				if err != nil {
					a.errorf(DiagLaya+"-002", o.edge.Pos.Line, o.edge.Pos.Col,
						"flow %q: gate %q: question %q is a number; branch [%s]: %v",
						flowName, src, q.ID, strings.TrimPrefix(o.label, "range:"), err)
					continue
				}
				for _, prev := range ranges {
					if laya.Overlap(prev, iv) {
						a.errorf(DiagLaya+"-011", o.edge.Pos.Line, o.edge.Pos.Col,
							"flow %q: gate %q: branches [%s] and [%s] overlap",
							flowName, src, strings.TrimPrefix(prev.Label, "range:"), strings.TrimPrefix(iv.Label, "range:"))
					}
				}
				ranges = append(ranges, iv)
				continue
			}
			if !valid[o.label] {
				a.errorf(DiagLaya+"-002", o.edge.Pos.Line, o.edge.Pos.Col,
					"flow %q: gate %q: task %q has no label %q (labels: %s)",
					flowName, src, taskName, o.label, strings.Join(laya.TaskLabels(q), ", "))
			}
		}
	}

	if !labels[laya.PseudoLabels[2]] { // no [*]
		var missing []string
		if known && q.Type != laya.TypeNumber {
			for _, l := range laya.TaskLabels(q) {
				if !labels[l] {
					missing = append(missing, l)
				}
			}
		}
		for _, p := range []string{"low_confidence", "error"} {
			if !labels[p] {
				missing = append(missing, p)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			a.warningf(DiagLaya+"-003", pos.Line, pos.Col,
				"flow %q: gate %q does not cover [%s] and has no [*]; a run taking an uncovered branch fails",
				flowName, src, strings.Join(missing, "], ["))
		}
	}
}

// checkNumberLiterals reports every number literal that overflows a float64
// (`1e999`): the IR keeps the literal (json.Number), which is valid JSON
// that no consumer can decode. A literal that underflows to 0 (`1e-400`) is
// finite and accepted. Reported in source order.
func (a *Analyzer) checkNumberLiterals(program *ast.Node) {
	var bad []*ast.Node
	ast.Walk(program, func(n *ast.Node, _ int) bool {
		if n.Type == ast.NodeNumberLiteral {
			if f, err := strconv.ParseFloat(n.Value, 64); err != nil && (math.IsInf(f, 0) || math.IsNaN(f)) {
				bad = append(bad, n)
			}
		}
		return true
	})
	sort.SliceStable(bad, func(i, j int) bool {
		if bad[i].Pos.Line != bad[j].Pos.Line {
			return bad[i].Pos.Line < bad[j].Pos.Line
		}
		return bad[i].Pos.Col < bad[j].Pos.Col
	})
	for _, n := range bad {
		a.errorf("", n.Pos.Line, n.Pos.Col, "number %s is not a finite number (it overflows a 64-bit float)", n.Value)
	}
}
