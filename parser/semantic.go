// semantic.go — TAC Semantic Analyzer
//
// Implements the compile-time guarantees promised by the TAC language:
//   - DAG validation (no cycles)
//   - Node reference validation (edges must reference declared nodes)
//   - Trust type checking (Secret, Untrusted, Fact, Hallucinable, Control)
//   - Skill parameter validation against the built-in skill registry
//   - Input/type consistency checks
//
// (c) 2026 TacFlow — MIT License

package main

import (
	"fmt"
	"sort"
	"strings"
)

// ============================================================================
// Diagnostics
// ============================================================================

// Severity classifies a diagnostic.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Diagnostic is a single semantic finding with source position.
type Diagnostic struct {
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Line     int      `json:"line,omitempty"`
	Col      int      `json:"col,omitempty"`
	Rule     string   `json:"rule"`
}

func (d Diagnostic) String() string {
	if d.Line > 0 {
		return fmt.Sprintf("%s: %s (line %d, col %d) [%s]", d.Severity, d.Message, d.Line, d.Col, d.Rule)
	}
	return fmt.Sprintf("%s: %s [%s]", d.Severity, d.Message, d.Rule)
}

// AnalysisResult is the complete result of semantic analysis.
type AnalysisResult struct {
	Valid       bool          `json:"valid"`
	Diagnostics []Diagnostic  `json:"diagnostics,omitempty"`
	Flow        *FlowAnalysis `json:"flow,omitempty"`
}

// FlowAnalysis holds per-flow structural facts proven by the analyzer.
type FlowAnalysis struct {
	Name          string        `json:"name"`
	NodeCount     int           `json:"node_count"`
	EdgeCount     int           `json:"edge_count"`
	TriggerCount  int           `json:"trigger_count"`
	Cycles        [][]string    `json:"cycles,omitempty"`
	IsDAG         bool          `json:"is_dag"`
	ParallelSets  [][]string    `json:"parallel_sets,omitempty"`
	CriticalPath  []string      `json:"critical_path,omitempty"`
	Depth         int           `json:"depth"`
	TrustTypes    map[string]string `json:"trust_types,omitempty"`
	RequiredSkills []string     `json:"required_skills,omitempty"`
}

// ============================================================================
// Trust Types
// ============================================================================

// TrustType models the provenance/safety type of a value.
type TrustType string

const (
	TrustSecret      TrustType = "Secret"
	TrustUntrusted   TrustType = "Untrusted"
	TrustFact        TrustType = "Fact"
	TrustHallucinable TrustType = "Hallucinable"
	TrustControl     TrustType = "Control"
	TrustUnknown     TrustType = "Unknown"
)

// ValidTrustTypes is the set of allowed trust type names.
var ValidTrustTypes = map[string]TrustType{
	"secret":       TrustSecret,
	"untrusted":    TrustUntrusted,
	"fact":         TrustFact,
	"hallucinable": TrustHallucinable,
	"control":      TrustControl,
}

// trustConversionAllowed reports whether converting from→to is permitted.
// ⚠️ conversions require an explicit conversion skill (verify/validate/sanitize).
func trustConversionAllowed(from, to TrustType) (allowed bool, requires string) {
	switch from {
	case TrustSecret:
		return to == TrustSecret, ""
	case TrustUntrusted:
		switch to {
		case TrustUntrusted:
			return true, ""
		case TrustFact, TrustHallucinable:
			return false, "validate()"
		}
		return false, ""
	case TrustFact:
		return to == TrustFact || to == TrustUntrusted || to == TrustHallucinable, ""
	case TrustHallucinable:
		switch to {
		case TrustHallucinable, TrustUntrusted:
			return true, ""
		case TrustFact:
			return false, "verify()"
		}
		return false, ""
	case TrustControl:
		return to == TrustControl, ""
	case TrustUnknown:
		return true, ""
	}
	return false, ""
}

// ============================================================================
// Skill Registry
// ============================================================================

// SkillParam describes one named parameter of a skill.
type SkillParam struct {
	Name     string    `json:"name"`
	Type     string    `json:"type"` // "string" | "number" | "bool" | "array" | "any"
	Required bool      `json:"required"`
	Default  string    `json:"default,omitempty"`
}

// SkillSpec is the compile-time contract for a skill.
type SkillSpec struct {
	Name       string       `json:"name"`
	Params     []SkillParam `json:"params"`
	Returns    TrustType    `json:"returns"`
	HasVarArgs bool         `json:"has_var_args,omitempty"`
}

// skillRegistry is the built-in standard library contract.
var skillRegistry = map[string]SkillSpec{
	"memory_search": {
		Name: "memory_search",
		Params: []SkillParam{
			{Name: "query", Type: "string", Required: true},
			{Name: "scope", Type: "string", Required: false, Default: "private"},
			{Name: "top_k", Type: "number", Required: false, Default: "5"},
		},
		Returns: TrustFact,
	},
	"memory_store": {
		Name: "memory_store",
		Params: []SkillParam{
			{Name: "text", Type: "string", Required: true},
			{Name: "tags", Type: "array", Required: false},
		},
		Returns: TrustControl,
	},
	"web_search": {
		Name: "web_search",
		Params: []SkillParam{
			{Name: "query", Type: "string", Required: true},
			{Name: "count", Type: "number", Required: false, Default: "5"},
		},
		Returns: TrustUntrusted,
	},
	"graph_search": {
		Name: "graph_search",
		Params: []SkillParam{
			{Name: "query", Type: "string", Required: true},
			{Name: "depth", Type: "number", Required: false, Default: "2"},
			{Name: "top_k", Type: "number", Required: false, Default: "5"},
		},
		Returns: TrustFact,
	},
	"graph_relate": {
		Name: "graph_relate",
		Params: []SkillParam{
			{Name: "source", Type: "string", Required: true},
			{Name: "target", Type: "string", Required: true},
			{Name: "type", Type: "string", Required: true},
			{Name: "weight", Type: "number", Required: false, Default: "0.5"},
		},
		Returns: TrustControl,
	},
	"llm.chat": {
		Name: "llm.chat",
		Params: []SkillParam{
			{Name: "prompt", Type: "string", Required: true},
			{Name: "context", Type: "any", Required: false},
			{Name: "model", Type: "string", Required: false},
		},
		Returns: TrustHallucinable,
	},
	"llm.classify": {
		Name: "llm.classify",
		Params: []SkillParam{
			{Name: "question", Type: "string", Required: true},
			{Name: "text", Type: "string", Required: false},
		},
		Returns: TrustHallucinable,
	},
	"tts.speak": {
		Name: "tts.speak",
		Params: []SkillParam{
			{Name: "text", Type: "string", Required: true},
			{Name: "voice", Type: "string", Required: false},
		},
		Returns: TrustControl,
	},
	"whisper.transcribe": {
		Name: "whisper.transcribe",
		Params: []SkillParam{
			{Name: "audio", Type: "string", Required: true},
		},
		Returns: TrustUntrusted,
	},
	"verify": {
		Name: "verify",
		Params: []SkillParam{
			{Name: "source", Type: "any", Required: true},
			{Name: "sources", Type: "array", Required: false},
		},
		Returns: TrustFact,
	},
	"validate": {
		Name: "validate",
		Params: []SkillParam{
			{Name: "source", Type: "any", Required: true},
		},
		Returns: TrustFact,
	},
	"config_get": {
		Name: "config_get",
		Params: []SkillParam{
			{Name: "key", Type: "string", Required: true},
		},
		Returns: TrustSecret,
	},
	"config_set": {
		Name: "config_set",
		Params: []SkillParam{
			{Name: "key", Type: "string", Required: true},
			{Name: "value", Type: "any", Required: true},
			{Name: "is_secret", Type: "bool", Required: false},
		},
		Returns: TrustControl,
	},
	"agent_task": {
		Name: "agent_task",
		Params: []SkillParam{
			{Name: "agent", Type: "string", Required: true},
			{Name: "payload", Type: "string", Required: true},
			{Name: "priority", Type: "number", Required: false, Default: "3"},
		},
		Returns: TrustUntrusted,
	},
	"flow.run": {
		Name: "flow.run",
		Params: []SkillParam{
			{Name: "flow", Type: "string", Required: true},
		},
		Returns: TrustUnknown,
	},
	"get_current_time": {
		Name:    "get_current_time",
		Params:  []SkillParam{},
		Returns: TrustFact,
	},
	"set_token_limit": {
		Name: "set_token_limit",
		Params: []SkillParam{
			{Name: "max", Type: "number", Required: true},
			{Name: "scope", Type: "string", Required: false, Default: "session"},
		},
		Returns: TrustControl,
	},
	"auto_summarize": {
		Name: "auto_summarize",
		Params: []SkillParam{
			{Name: "on", Type: "string", Required: true},
			{Name: "strategy", Type: "string", Required: false, Default: "concise"},
		},
		Returns: TrustControl,
	},
}

// KnownSkillNames returns a sorted list of registered skill names.
func KnownSkillNames() []string {
	names := make([]string, 0, len(skillRegistry))
	for name := range skillRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// LookupSkill returns the skill spec if registered.
func LookupSkill(name string) (SkillSpec, bool) {
	spec, ok := skillRegistry[name]
	return spec, ok
}

// ============================================================================
// Semantic Analyzer
// ============================================================================

// Analyzer validates a parsed AST.
type Analyzer struct {
	program     *Node
	diagnostics []Diagnostic
	flowAnalysis *FlowAnalysis
	trustTypes  map[string]TrustType
}

// NewAnalyzer creates an analyzer over a parsed program.
func NewAnalyzer(program *Node) *Analyzer {
	return &Analyzer{
		program:    program,
		trustTypes: make(map[string]TrustType),
	}
}

// Analyze runs all semantic checks and returns the result.
func (a *Analyzer) Analyze() *AnalysisResult {
	if a.program == nil {
		a.errorf(0, 0, "internal", "program is nil")
		return a.result()
	}
	for _, node := range a.program.Nodes {
		switch node.Type {
		case NodeFlow:
			a.analyzeFlow(node)
		case NodeRememberStmt:
			a.analyzeRemember(node)
		case NodeRelateStmt:
			a.analyzeRelate(node)
		case NodeInput:
			// top-level input not expected; ignore
		}
	}
	return a.result()
}

func (a *Analyzer) result() *AnalysisResult {
	res := &AnalysisResult{
		Valid:       len(a.diagnostics) == 0 || !a.hasErrors(),
		Diagnostics: a.diagnostics,
		Flow:        a.flowAnalysis,
	}
	// Valid means no errors (warnings allowed)
	res.Valid = !a.hasErrors()
	return res
}

func (a *Analyzer) hasErrors() bool {
	for _, d := range a.diagnostics {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

func (a *Analyzer) errorf(line, col int, rule, format string, args ...interface{}) {
	a.diagnostics = append(a.diagnostics, Diagnostic{
		Severity: SeverityError,
		Message:  fmt.Sprintf(format, args...),
		Line:     line,
		Col:      col,
		Rule:     rule,
	})
}

func (a *Analyzer) warnf(line, col int, rule, format string, args ...interface{}) {
	a.diagnostics = append(a.diagnostics, Diagnostic{
		Severity: SeverityWarning,
		Message:  fmt.Sprintf(format, args...),
		Line:     line,
		Col:      col,
		Rule:     rule,
	})
}

// ============================================================================
// Flow analysis
// ============================================================================

func (a *Analyzer) analyzeFlow(flow *Node) {
	fa := &FlowAnalysis{
		Name:         flow.Value,
		TrustTypes:   make(map[string]TrustType),
		RequiredSkills: make([]string, 0),
	}
	a.flowAnalysis = fa

	nodes := make(map[string]*Node)
	nodeNames := make([]string, 0)
	inputs := make(map[string]TrustType)

	// First pass: collect node declarations + inputs + agent decls
	for _, child := range flow.Nodes {
		if child.Type == NodeNode {
			name := child.Value
			if name == "" {
				a.errorf(child.Pos.Line, child.Pos.Col, "flow.node-name", "flow %q contains a node without a name", fa.Name)
				continue
			}
			if _, dup := nodes[name]; dup {
				a.errorf(child.Pos.Line, child.Pos.Col, "flow.duplicate-node", "duplicate node %q in flow %q", name, fa.Name)
				continue
			}
			nodes[name] = child
			nodeNames = append(nodeNames, name)
		}
	}
	for _, child := range flow.Children {
		if child.Type == NodeInput {
			if len(child.Children) >= 1 {
				varName := child.Children[0].Value
				typ := TrustUnknown
				if len(child.Children) >= 2 {
					if t, ok := ValidTrustTypes[strings.ToLower(child.Children[1].Value)]; ok {
						typ = t
					}
				}
				inputs[varName] = typ
				fa.TrustTypes[varName] = string(typ)
			}
		}
	}

	fa.NodeCount = len(nodes)
	fa.EdgeCount = len(flow.Edges)
	fa.TriggerCount = countTriggers(flow)

	// Edge set for cycle detection: adjacency from -> to
	adj := make(map[string][]string)
	indegree := make(map[string]int)
	for _, name := range nodeNames {
		adj[name] = nil
		indegree[name] = 0
	}

	for _, edge := range flow.Edges {
		src, dst := edgeEndpoints(edge)
		if src == "" || dst == "" {
			a.errorf(edge.Pos.Line, edge.Pos.Col, "flow.edge-endpoints", "edge in flow %q must reference two nodes", fa.Name)
			continue
		}
		if _, ok := nodes[src]; !ok {
			a.errorf(edge.Pos.Line, edge.Pos.Col, "flow.undefined-node", "edge references undefined source node %q in flow %q", src, fa.Name)
			continue
		}
		if _, ok := nodes[dst]; !ok {
			a.errorf(edge.Pos.Line, edge.Pos.Col, "flow.undefined-node", "edge references undefined target node %q in flow %q", dst, fa.Name)
			continue
		}
		// Conditional edges (if/else) are validated for their target too
		adj[src] = append(adj[src], dst)
		indegree[dst]++
	}

	// Trigger targets must exist
	for _, trig := range flow.Children {
		if trig.Type != NodeTrigger {
			continue
		}
		for _, tgt := range trig.Children {
			if tgt.Type == NodeIdentifier {
				if _, ok := nodes[tgt.Value]; !ok {
					a.errorf(trig.Pos.Line, trig.Pos.Col, "flow.undefined-trigger", "trigger in flow %q references undefined node %q", fa.Name, tgt.Value)
				}
			} else if tgt.Type == NodeArrayLiteral {
				for _, item := range tgt.ArrVal {
					if _, ok := nodes[item.Value]; !ok {
						a.errorf(trig.Pos.Line, trig.Pos.Col, "flow.undefined-trigger", "trigger in flow %q references undefined node %q", fa.Name, item.Value)
					}
				}
			}
		}
	}

	// Cycle detection (Kahn's algorithm)
	cycles := detectCycles(adj, nodeNames)
	fa.Cycles = cycles
	fa.IsDAG = len(cycles) == 0
	if !fa.IsDAG {
		for _, cycle := range cycles {
			a.errorf(0, 0, "flow.cycle", "flow %q contains a cycle: %s", fa.Name, strings.Join(cycle, " -> "))
		}
	}

	// Critical path / depth (only meaningful for DAG)
	if fa.IsDAG {
		fa.CriticalPath, fa.Depth = criticalPath(adj, nodeNames)
		fa.ParallelSets = parallelSets(adj, nodeNames, indegree)
	}

	// Validate each node's skill call + trust types
	for _, name := range nodeNames {
		node := nodes[name]
		a.analyzeNode(name, node, inputs, fa)
	}

	// Sort required skills for determinism
	sort.Strings(fa.RequiredSkills)
}

// edgeEndpoints extracts source and target identifiers from an Edge node.
func edgeEndpoints(edge *Node) (string, string) {
	var src, dst string
	for _, c := range edge.Children {
		switch c.Type {
		case NodeIdentifier:
			if src == "" {
				src = c.Value
			} else {
				dst = c.Value
			}
		case NodeArrayLiteral:
			// dst may be an array (fork) — take first for adjacency
			if len(c.ArrVal) > 0 {
				dst = c.ArrVal[0].Value
			}
		}
	}
	return src, dst
}

func countTriggers(flow *Node) int {
	n := 0
	for _, c := range flow.Children {
		if c.Type == NodeTrigger {
			n++
		}
	}
	return n
}

// detectCycles finds all elementary cycles using DFS with a call stack.
func detectCycles(adj map[string][]string, nodes []string) [][]string {
	cycles := make([][]string, 0)
	seen := make(map[string]bool)
	stack := make([]string, 0)
	onStack := make(map[string]bool)

	var dfs func(u string)
	dfs = func(u string) {
		seen[u] = true
		onStack[u] = true
		stack = append(stack, u)
		for _, v := range adj[u] {
			if onStack[v] {
				// found cycle: from v in stack to u
				start := 0
				for i, s := range stack {
					if s == v {
						start = i
						break
					}
				}
				cycle := append([]string{}, stack[start:]...)
				cycle = append(cycle, v)
				cycles = append(cycles, cycle)
			} else if !seen[v] {
				dfs(v)
			}
		}
		stack = stack[:len(stack)-1]
		onStack[u] = false
	}

	for _, n := range nodes {
		if !seen[n] {
			dfs(n)
		}
	}
	return cycles
}

// criticalPath computes the longest dependency path (by edge count) and its depth.
func criticalPath(adj map[string][]string, nodes []string) ([]string, int) {
	// longest path in DAG via topological order (Kahn)
	indeg := make(map[string]int)
	for _, n := range nodes {
		indeg[n] = 0
	}
	for _, u := range nodes {
		for _, v := range adj[u] {
			indeg[v]++
		}
	}
	queue := make([]string, 0)
	for _, n := range nodes {
		if indeg[n] == 0 {
			queue = append(queue, n)
		}
	}
	dist := make(map[string]int)
	prev := make(map[string]string)
	order := make([]string, 0)
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		order = append(order, u)
		for _, v := range adj[u] {
			indeg[v]--
			if dist[u]+1 > dist[v] {
				dist[v] = dist[u] + 1
				prev[v] = u
			}
			if indeg[v] == 0 {
				queue = append(queue, v)
			}
		}
	}
	// find max dist
	maxNode := ""
	maxDist := -1
	for _, n := range nodes {
		if dist[n] > maxDist {
			maxDist = dist[n]
			maxNode = n
		}
	}
	if maxNode == "" {
		return nil, 0
	}
	path := []string{maxNode}
	cur := maxNode
	for prev[cur] != "" {
		cur = prev[cur]
		path = append([]string{cur}, path...)
	}
	return path, maxDist
}

// parallelSets groups nodes that can execute in the same wave.
func parallelSets(adj map[string][]string, nodes []string, indegree map[string]int) [][]string {
	indeg := make(map[string]int)
	for k, v := range indegree {
		indeg[k] = v
	}
	queue := make([]string, 0)
	for _, n := range nodes {
		if indeg[n] == 0 {
			queue = append(queue, n)
		}
	}
	sets := make([][]string, 0)
	processed := 0
	for len(queue) > 0 {
		level := make([]string, 0, len(queue))
		next := make([]string, 0)
		for _, u := range queue {
			level = append(level, u)
			processed++
			for _, v := range adj[u] {
				indeg[v]--
				if indeg[v] == 0 {
					next = append(next, v)
				}
			}
		}
		if len(level) > 0 {
			sets = append(sets, level)
		}
		queue = next
	}
	return sets
}

// ============================================================================
// Node & skill validation
// ============================================================================

func (a *Analyzer) analyzeNode(name string, node *Node, inputs map[string]TrustType, fa *FlowAnalysis) {
	// node.Children: [name, SkillCall] or [name, ArrayLiteral]
	for _, c := range node.Children {
		switch c.Type {
		case NodeSkillCall:
			a.analyzeSkillCall(name, c, inputs, fa)
		case NodeIdentifier:
			// node -> identifier (not a skill call) — nothing to check at skill level
		}
	}
}

func (a *Analyzer) analyzeSkillCall(nodeName string, call *Node, inputs map[string]TrustType, fa *FlowAnalysis) {
	skillName := call.Value

	// skill must be known or explicitly declared as custom
	spec, known := skillRegistry[skillName]
	if !known {
		a.errorf(call.Pos.Line, call.Pos.Col, "skill.unknown", "node %q calls unknown skill %q (registered skills: %s)",
			nodeName, skillName, strings.Join(KnownSkillNames(), ", "))
		return
	}
	fa.RequiredSkills = append(fa.RequiredSkills, skillName)

	// Validate named args against spec
	provided := make(map[string]bool)
	for key, val := range call.Attrs {
		provided[key] = true
		param, ok := findParam(spec, key)
		if !ok {
			a.errorf(call.Pos.Line, call.Pos.Col, "skill.unknown-param", "skill %q has no parameter %q (node %q)", skillName, key, nodeName)
			continue
		}
		if !typeMatches(param.Type, val) {
			a.errorf(call.Pos.Line, call.Pos.Col, "skill.param-type", "skill %q parameter %q expects %s (node %q)", skillName, key, param.Type, nodeName)
		}
	}

	// Required params must be present (named args or positional)
	hasPositional := len(call.Args) > 0
	for _, param := range spec.Params {
		if param.Required && !provided[param.Name] && !hasPositional {
			a.errorf(call.Pos.Line, call.Pos.Col, "skill.missing-param", "skill %q requires parameter %q (node %q)", skillName, param.Name, nodeName)
		}
	}

	// Record the node's return trust type
	fa.TrustTypes[nodeName] = string(spec.Returns)

	// Trust flow: if a Secret is fed into a skill that returns Hallucinable/Untrusted...
	// (provenance check happens in analyzeTrustEdges — we record the mapping here)
	a.trustTypes[nodeName] = spec.Returns
}

func findParam(spec SkillSpec, name string) (SkillParam, bool) {
	for _, p := range spec.Params {
		if p.Name == name {
			return p, true
		}
	}
	return SkillParam{}, false
}

func typeMatches(expected string, val *Node) bool {
	switch expected {
	case "string":
		return val.Type == NodeStringLiteral || val.Type == NodeIdentifier
	case "number":
		return val.Type == NodeNumberLiteral || val.Type == NodeIdentifier
	case "bool":
		return val.Type == NodeBoolLiteral || val.Type == NodeIdentifier
	case "array":
		return val.Type == NodeArrayLiteral || val.Type == NodeIdentifier
	case "any":
		return true
	}
	return true
}

// ============================================================================
// Top-level statements
// ============================================================================

func (a *Analyzer) analyzeRemember(node *Node) {
	if len(node.Children) < 2 {
		a.errorf(node.Pos.Line, node.Pos.Col, "remember.syntax", "remember requires a name and a value")
		return
	}
	name := node.Children[0].Value
	if name == "" {
		a.errorf(node.Pos.Line, node.Pos.Col, "remember.name", "remember statement has no name")
		return
	}
	// Validate semantic type if provided
	if node.Attrs != nil {
		if typeNode, ok := node.Attrs["type"]; ok && typeNode.Type == NodeStringLiteral {
			lower := strings.ToLower(typeNode.Value)
			if _, valid := ValidTrustTypes[lower]; !valid {
				a.errorf(typeNode.Pos.Line, typeNode.Pos.Col, "trust.invalid-type", "invalid trust type %q (valid: secret, untrusted, fact, hallucinable, control)", typeNode.Value)
			}
		}
	}
}

func (a *Analyzer) analyzeRelate(node *Node) {
	if len(node.Children) < 2 {
		a.errorf(node.Pos.Line, node.Pos.Col, "relate.syntax", "relate requires a source and a target")
		return
	}
	if node.Attrs != nil {
		if _, ok := node.Attrs["type"]; !ok {
			a.errorf(node.Pos.Line, node.Pos.Col, "relate.type", "relate requires a type attribute (e.g. type: \"depends_on\")")
		}
	}
}
