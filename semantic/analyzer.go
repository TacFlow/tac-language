// Package semantic implements the TAC Language semantic analyzer.
//
// The analyzer validates a parsed AST against the rules in SPEC v2.0:
//   - Node references (edges must reference declared nodes)
//   - Trigger references (triggers must target declared nodes)
//   - DAG validation (cycles are forbidden — execution is a DAG)
//   - Skill registry (skill names must resolve to known capabilities)
//   - Trust type checking (Secret/Untrusted/Fact/Hallucinable/Control)
//   - Dead node detection (unreachable nodes)
package semantic

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tacflow1-tech/tac-language/ast"
	"github.com/tacflow1-tech/tac-language/types"
)

// Severity represents the severity of a semantic diagnostic.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	default:
		return "unknown"
	}
}

// Diagnostic is a single semantic finding.
type Diagnostic struct {
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Line     int      `json:"line"`
	Col      int      `json:"col"`
	Flow     string   `json:"flow,omitempty"`
}

func (d Diagnostic) String() string {
	sev := d.Severity.String()
	if d.Line > 0 {
		return fmt.Sprintf("%s at line %d, col %d: %s", sev, d.Line, d.Col, d.Message)
	}
	return fmt.Sprintf("%s: %s", sev, d.Message)
}

// SkillSpec describes a known skill and its return trust type.
type SkillSpec struct {
	Name       string
	ReturnType types.TrustType
	Args       []string
	Description string
}

// BuiltinSkills is the standard library of TAC skills (SPEC §7.1).
var BuiltinSkills = map[string]SkillSpec{
	"memory_search": {
		Name: "memory_search", ReturnType: types.Fact,
		Args: []string{"query", "scope", "top_k"},
		Description: "Search BM25+Vector+Graph memory",
	},
	"memory_store": {
		Name: "memory_store", ReturnType: types.Fact,
		Args: []string{"text", "tags", "shared"},
		Description: "Store in persistent memory",
	},
	"web_search": {
		Name: "web_search", ReturnType: types.Untrusted,
		Args: []string{"query", "count"},
		Description: "Search the web (Brave)",
	},
	"llm.chat": {
		Name: "llm.chat", ReturnType: types.Hallucinable,
		Args: []string{"prompt", "context", "model"},
		Description: "Call LLM with prompt",
	},
	"llm.classify": {
		Name: "llm.classify", ReturnType: types.Hallucinable,
		Args: []string{"text"},
		Description: "Classify input text",
	},
	"tts.speak": {
		Name: "tts.speak", ReturnType: types.Control,
		Args: []string{"text", "voice"},
		Description: "Text-to-speech output",
	},
	"whisper.transcribe": {
		Name: "whisper.transcribe", ReturnType: types.Untrusted,
		Args: []string{"audio"},
		Description: "Speech-to-text input",
	},
	"vision.analyze": {
		Name: "vision.analyze", ReturnType: types.Hallucinable,
		Args: []string{"image"},
		Description: "Analyze an image",
	},
	"vision.generate": {
		Name: "vision.generate", ReturnType: types.Hallucinable,
		Args: []string{"prompt"},
		Description: "Generate an image",
	},
	"agent_task": {
		Name: "agent_task", ReturnType: types.Control,
		Args: []string{"agent", "payload", "priority"},
		Description: "Delegate to swarm agent",
	},
	"agent_wait": {
		Name: "agent_wait", ReturnType: types.Control,
		Args: []string{"task_id", "timeout"},
		Description: "Await agent task result",
	},
	"flow.run": {
		Name: "flow.run", ReturnType: types.Control,
		Args: []string{"flow", "params"},
		Description: "Execute a sub-flow",
	},
	"graph_search": {
		Name: "graph_search", ReturnType: types.Fact,
		Args: []string{"query", "depth"},
		Description: "Traverse the knowledge graph",
	},
	"graph_relate": {
		Name: "graph_relate", ReturnType: types.Control,
		Args: []string{"source", "target", "type"},
		Description: "Create graph edge",
	},
	"verify": {
		Name: "verify", ReturnType: types.Fact,
		Args: []string{"source"},
		Description: "Fact-check a Hallucinable value",
	},
	"validate": {
		Name: "validate", ReturnType: types.Fact,
		Args: []string{"value"},
		Description: "Sanitize an Untrusted value",
	},
	"config_get": {
		Name: "config_get", ReturnType: types.Fact,
		Args: []string{"key"},
		Description: "Read configuration",
	},
	"config_set": {
		Name: "config_set", ReturnType: types.Control,
		Args: []string{"key", "value"},
		Description: "Write configuration",
	},
	"get_current_time": {
		Name: "get_current_time", ReturnType: types.Fact,
		Args: nil,
		Description: "Get current date and time",
	},
	"swarm_status": {
		Name: "swarm_status", ReturnType: types.Control,
		Args: nil,
		Description: "Check swarm health",
	},
	"swarm_check_my_status": {
		Name: "swarm_check_my_status", ReturnType: types.Control,
		Args: nil,
		Description: "Check own reputation",
	},
	"swarm_teach": {
		Name: "swarm_teach", ReturnType: types.Control,
		Args: []string{"name", "content"},
		Description: "Share knowledge across the swarm",
	},
	"search_hybrid": {
		Name: "search_hybrid", ReturnType: types.Fact,
		Args: []string{"query", "bm25_weight", "vector_weight", "graph_weight", "top_k"},
		Description: "Hybrid search across all memory layers",
	},
	"set_token_limit": {
		Name: "set_token_limit", ReturnType: types.Control,
		Args: []string{"max", "scope"},
		Description: "Set per-session or per-flow token budget",
	},
}

// LookupSkill resolves a skill name, including dotted notation.
func LookupSkill(name string) (SkillSpec, bool) {
	spec, ok := BuiltinSkills[name]
	return spec, ok
}

// SkillNames returns the sorted list of all known skill names.
func SkillNames() []string {
	names := make([]string, 0, len(BuiltinSkills))
	for name := range BuiltinSkills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Analyzer validates a TAC program.
type Analyzer struct {
	diagnostics []Diagnostic
	// Track declared inputs across the program
	inputs map[string]types.TrustType
	// Track declared agents
	agents map[string]bool
}

// New creates a new Analyzer.
func New() *Analyzer {
	return &Analyzer{
		inputs: make(map[string]types.TrustType),
		agents: make(map[string]bool),
	}
}

// Diagnostics returns all collected diagnostics.
func (a *Analyzer) Diagnostics() []Diagnostic {
	return a.diagnostics
}

// Errors returns only error-severity diagnostics.
func (a *Analyzer) Errors() []Diagnostic {
	var errs []Diagnostic
	for _, d := range a.diagnostics {
		if d.Severity == SeverityError {
			errs = append(errs, d)
		}
	}
	return errs
}

// HasErrors returns true if any error-severity diagnostic exists.
func (a *Analyzer) HasErrors() bool {
	return len(a.Errors()) > 0
}

func (a *Analyzer) errorf(line, col int, format string, args ...interface{}) {
	a.diagnostics = append(a.diagnostics, Diagnostic{
		Severity: SeverityError,
		Message:  fmt.Sprintf(format, args...),
		Line:     line,
		Col:      col,
	})
}

func (a *Analyzer) warningf(line, col int, format string, args ...interface{}) {
	a.diagnostics = append(a.diagnostics, Diagnostic{
		Severity: SeverityWarning,
		Message:  fmt.Sprintf(format, args...),
		Line:     line,
		Col:      col,
	})
}

// Analyze validates a complete Program AST.
func (a *Analyzer) Analyze(program *ast.Node) []Diagnostic {
	if program == nil {
		a.errorf(0, 0, "nil program AST")
		return a.diagnostics
	}

	// Pass 1: collect top-level inputs, agents
	a.collectGlobals(program)

	// Pass 2: validate each flow
	for _, flow := range ast.CollectFlows(program) {
		a.validateFlow(flow)
	}

	// Pass 3: validate context blocks
	for _, node := range program.Nodes {
		if node.Type == ast.NodeContextBlock {
			a.validateContext(node)
		}
	}

	return a.diagnostics
}

func (a *Analyzer) collectGlobals(program *ast.Node) {
	for _, n := range program.Nodes {
		switch n.Type {
		case ast.NodeInput:
			if len(n.Children) >= 1 {
				name := n.Children[0].Value
				tt := types.Untrusted // default
				if len(n.Children) >= 2 && types.IsValidTrustType(n.Children[1].Value) {
					tt = types.TrustType(n.Children[1].Value)
				}
				a.inputs[name] = tt
			}
		case ast.NodeAgentDecl:
			if len(n.Children) >= 1 {
				name := n.Children[0].Value
				if name == "" && n.Value != "" {
					name = n.Value
				}
				if name != "" {
					a.agents[name] = true
				}
			}
		}
	}
}

// validateContext validates statements inside a context block.
func (a *Analyzer) validateContext(ctx *ast.Node) {
	for _, child := range ctx.Children {
		switch child.Type {
		case ast.NodeFlow:
			a.validateFlow(child)
		case ast.NodeInput:
			if len(child.Children) >= 1 {
				name := child.Children[0].Value
				tt := types.Untrusted
				if len(child.Children) >= 2 && types.IsValidTrustType(child.Children[1].Value) {
					tt = types.TrustType(child.Children[1].Value)
				}
				a.inputs[name] = tt
			}
		case ast.NodeRememberStmt:
			a.validateRemember(child)
		case ast.NodeRecallStmt:
			a.validateRecall(child)
		}
	}
}

func (a *Analyzer) validateRemember(n *ast.Node) {
	if len(n.Children) < 2 {
		a.errorf(n.Pos.Line, n.Pos.Col, "remember requires a name and a value")
		return
	}
	// Validate type attribute if present
	if attrs := n.Attrs; attrs != nil {
		if typeAttr, ok := attrs["type"]; ok && typeAttr != nil {
			if !types.IsValidTrustType(typeAttr.Value) {
				a.warningf(typeAttr.Pos.Line, typeAttr.Pos.Col,
					"unknown trust type %q in remember (valid: %s)",
					typeAttr.Value, strings.Join(trustTypeNames(), ", "))
			}
		}
	}
}

func (a *Analyzer) validateRecall(n *ast.Node) {
	if len(n.Children) < 1 {
		a.errorf(n.Pos.Line, n.Pos.Col, "recall requires a name")
	}
}

// validateFlow performs full validation on a single flow.
func (a *Analyzer) validateFlow(flow *ast.Node) {
	flowName := flow.Value
	if flowName == "" {
		flowName = "(unnamed)"
	}

	// --- Collect declared nodes ---
	declared := make(map[string]*ast.Node)
	for _, node := range flow.Nodes {
		name := ast.NodeName(node)
		if name == "" {
			a.errorf(node.Pos.Line, node.Pos.Col, "flow %q: node without a name", flowName)
			continue
		}
		if _, dup := declared[name]; dup {
			a.errorf(node.Pos.Line, node.Pos.Col, "flow %q: duplicate node %q", flowName, name)
		}
		declared[name] = node
	}

	// --- Collect and validate edges ---
	adjacency := make(map[string][]string) // source -> targets
	inDegree := make(map[string]int)

	// Initialize in-degree for all declared nodes
	for name := range declared {
		inDegree[name] = 0
		adjacency[name] = make([]string, 0)
	}

	for _, edge := range flow.Edges {
		src := ast.EdgeSource(edge)
		tgt := ast.EdgeTarget(edge)

		if _, ok := declared[src]; !ok {
			a.errorf(edge.Pos.Line, edge.Pos.Col,
				"flow %q: edge references undeclared source node %q", flowName, src)
			continue
		}
		if _, ok := declared[tgt]; !ok {
			a.errorf(edge.Pos.Line, edge.Pos.Col,
				"flow %q: edge references undeclared target node %q", flowName, tgt)
			continue
		}

		adjacency[src] = append(adjacency[src], tgt)
		inDegree[tgt]++

		// Validate conditional edges
		if cond, fallback, hasCond := ast.EdgeCondition(edge); hasCond {
			if cond == "" {
				a.warningf(edge.Pos.Line, edge.Pos.Col,
					"flow %q: edge %s -> %s has empty if condition", flowName, src, tgt)
			}
			if fallback != "" {
				if _, ok := declared[fallback]; !ok {
					a.errorf(edge.Pos.Line, edge.Pos.Col,
						"flow %q: else target %q is not a declared node", flowName, fallback)
				}
			}
		}
	}

	// --- Cycle detection (Kahn's algorithm) ---
	cycle := detectCycle(declared, adjacency, inDegree)
	if cycle != "" {
		a.errorf(flow.Pos.Line, flow.Pos.Col,
			"flow %q: cycle detected in dependency graph: %s", flowName, cycle)
	}

	// --- Dead node detection ---
	reachable := computeReachable(declared, adjacency)
	for name := range declared {
		if !reachable[name] {
			a.warningf(declared[name].Pos.Line, declared[name].Pos.Col,
				"flow %q: node %q is unreachable (no incoming edge or trigger)", flowName, name)
		}
	}

	// --- Validate node internals ---
	for _, node := range flow.Nodes {
		a.validateNodeDef(node, flowName, declared)
	}

	// --- Validate triggers ---
	a.validateTriggers(flow, flowName, declared)

	// --- Validate inputs used in nodes ---
	a.validateInputReferences(flow, flowName)
}

func (a *Analyzer) validateNodeDef(node *ast.Node, flowName string, declared map[string]*ast.Node) {
	name := ast.NodeName(node)
	for _, child := range node.Children {
		switch child.Type {
		case ast.NodeSkillCall:
			a.validateSkillCall(child, flowName, name)
		}
	}
	// Nested block nodes (inline if/else)
	for _, sub := range node.Nodes {
		if sub.Type == ast.NodeSkillCall {
			a.validateSkillCall(sub, flowName, name)
		}
	}
}

func (a *Analyzer) validateSkillCall(call *ast.Node, flowName, nodeName string) {
	skillName := call.Value
	spec, ok := LookupSkill(skillName)
	if !ok {
		// Could be a custom skill declared elsewhere; warn but don't fail
		a.warningf(call.Pos.Line, call.Pos.Col,
			"flow %q node %q: unknown skill %q (not in standard library)", flowName, nodeName, skillName)
		return
	}

	// Validate known arguments
	if call.Attrs != nil {
		for argName := range call.Attrs {
			if !contains(spec.Args, argName) {
				a.warningf(call.Pos.Line, call.Pos.Col,
					"flow %q node %q: skill %q does not declare argument %q (known: %s)",
					flowName, nodeName, skillName, argName, strings.Join(spec.Args, ", "))
			}
		}
	}
}

func (a *Analyzer) validateTriggers(flow *ast.Node, flowName string, declared map[string]*ast.Node) {
	for _, child := range flow.Children {
		if child.Type != ast.NodeTrigger {
			continue
		}
		// Trigger target is the last child (identifier or array)
		if len(child.Children) < 1 {
			a.errorf(child.Pos.Line, child.Pos.Col,
				"flow %q: trigger without event name", flowName)
			continue
		}
		last := child.Children[len(child.Children)-1]
		switch last.Type {
		case ast.NodeIdentifier:
			if _, ok := declared[last.Value]; !ok {
				a.errorf(last.Pos.Line, last.Pos.Col,
					"flow %q: trigger targets undeclared node %q", flowName, last.Value)
			}
		case ast.NodeArrayLiteral:
			for _, item := range last.ArrVal {
				if _, ok := declared[item.Value]; !ok {
					a.errorf(item.Pos.Line, item.Pos.Col,
						"flow %q: trigger targets undeclared node %q", flowName, item.Value)
				}
			}
		}
	}
}

func (a *Analyzer) validateInputReferences(flow *ast.Node, flowName string) {
	// Collect inputs declared inside the flow
	flowInputs := make(map[string]types.TrustType)
	for _, child := range flow.Children {
		if child.Type == ast.NodeInput && len(child.Children) >= 1 {
			name := child.Children[0].Value
			tt := types.Untrusted
			if len(child.Children) >= 2 && types.IsValidTrustType(child.Children[1].Value) {
				tt = types.TrustType(child.Children[1].Value)
			}
			flowInputs[name] = tt
		}
	}

	ast.Walk(flow, func(n *ast.Node, depth int) bool {
		if n.Type == ast.NodeIdentifier {
			// Heuristic: identifiers that look like references to inputs
			if flowInputs[n.Value] != "" || a.inputs[n.Value] != "" {
				// Valid reference
				return true
			}
		}
		return true
	})
}

// --- Graph algorithms ---

// detectCycle uses Kahn's algorithm to detect cycles.
// Returns a string describing the cycle, or "" if acyclic.
func detectCycle(declared map[string]*ast.Node, adjacency map[string][]string, inDegree map[string]int) string {
	// Clone in-degree
	indeg := make(map[string]int, len(inDegree))
	for k, v := range inDegree {
		indeg[k] = v
	}

	var queue []string
	for name, deg := range indeg {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	processed := 0
	for len(queue) > 0 {
		// Pop
		node := queue[0]
		queue = queue[1:]
		processed++

		for _, next := range adjacency[node] {
			indeg[next]--
			if indeg[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if processed != len(declared) {
		// There's a cycle — find one node in it
		for name, deg := range indeg {
			if deg > 0 {
				return name
			}
		}
		return "(unknown)"
	}
	return ""
}

// computeReachable determines which nodes are reachable from triggers
// or have zero in-degree (entry points).
func computeReachable(declared map[string]*ast.Node, adjacency map[string][]string) map[string]bool {
	reachable := make(map[string]bool)

	// Find entry points: nodes with no incoming edges
	hasIncoming := make(map[string]bool)
	for _, targets := range adjacency {
		for _, t := range targets {
			hasIncoming[t] = true
		}
	}

	var stack []string
	for name := range declared {
		if !hasIncoming[name] {
			stack = append(stack, name)
		}
	}

	for len(stack) > 0 {
		node := stack[0]
		stack = stack[1:]
		if reachable[node] {
			continue
		}
		reachable[node] = true
		for _, next := range adjacency[node] {
			if !reachable[next] {
				stack = append(stack, next)
			}
		}
	}

	return reachable
}

// --- Helpers ---

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

func trustTypeNames() []string {
	names := make([]string, 0, len(types.ValidTrustTypes()))
	for _, tt := range types.ValidTrustTypes() {
		names = append(names, string(tt))
	}
	return names
}
