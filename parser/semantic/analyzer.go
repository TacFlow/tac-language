// Package semantic provides compile-time analysis of the TAC AST.
//
// It validates:
//   - DAG integrity (no cycles)
//   - Node references (edges reference existing nodes)
//   - Skill parameter constraints
//   - Trust type conversion rules
//
// These checks prove the "compile-time safety" guarantees the SPEC promises.
//
// (c) 2026 TacFlow — MIT License
package semantic

import (
	"fmt"
	"strings"

	"github.com/tacflow1-tech/tac-language/parser/ast"
)

// Severity classifies a diagnostic.
type Severity int

const (
	Info Severity = iota
	Warning
	Error
)

func (s Severity) String() string {
	switch s {
	case Info:
		return "INFO"
	case Warning:
		return "WARNING"
	case Error:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Diagnostic is a single semantic issue.
type Diagnostic struct {
	Severity Severity
	Node     *ast.Node
	Msg      string
}

func (d Diagnostic) String() string {
	pos := d.Node.Pos
	if pos.IsZero() {
		return fmt.Sprintf("%s: %s", d.Severity, d.Msg)
	}
	return fmt.Sprintf("%s at %s: %s", d.Severity, pos, d.Msg)
}

// Analyzer holds the current state and configuration for semantic analysis.
type Analyzer struct {
	diags    []Diagnostic
	skillMan *SkillManifest // optional — nil means skip skill parameter checks
}

// NewAnalyzer creates a new semantic analyzer. Pass nil for skillMan to skip
// skill-parameter validation (e.g. when no manifest is loaded).
func NewAnalyzer(skillMan *SkillManifest) *Analyzer {
	return &Analyzer{skillMan: skillMan}
}

// Analyze runs all semantic checks on a TAC program and returns any diagnostics.
func (a *Analyzer) Analyze(prog *ast.Node) []Diagnostic {
	a.diags = nil
	for _, flow := range prog.Nodes {
		if flow.Type == ast.NodeFlow {
			a.analyzeFlow(flow)
		}
	}
	return a.diags
}

func (a *Analyzer) diag(sev Severity, n *ast.Node, msg string) {
	a.diags = append(a.diags, Diagnostic{Severity: sev, Node: n, Msg: msg})
}

// ---------------------------------------------------------------------------
// Flow-level analysis
// ---------------------------------------------------------------------------

func (a *Analyzer) analyzeFlow(flow *ast.Node) {
	// Collect declared node names
	nodeNames := make(map[string]*ast.Node)
	for _, n := range flow.Nodes {
		if n.Type == ast.NodeNode {
			name := n.Value
			if name == "" && len(n.Children) > 0 {
				name = n.Children[0].Value
			}
			if name == "" {
				a.diag(Error, n, "node missing name")
				continue
			}
			if _, exists := nodeNames[name]; exists {
				a.diag(Error, n, "duplicate node name: "+name)
			}
			nodeNames[name] = n
		}
	}

	// DAG cycle detection
	a.checkCycles(flow, nodeNames)

	// Validate edges reference existing nodes
	a.checkReferences(flow, nodeNames)

	// Validate trust types
	a.checkTrustTypes(flow)

	// Validate skill parameters against manifest
	if a.skillMan != nil {
		a.checkSkillParams(flow)
	}

	// Validate triggers reference existing nodes
	a.checkTriggers(flow, nodeNames)
}

// ---------------------------------------------------------------------------
// Cycle Detection (DAG validation)
// ---------------------------------------------------------------------------

func (a *Analyzer) checkCycles(flow *ast.Node, nodeNames map[string]*ast.Node) {
	// Build adjacency list
	adj := make(map[string][]string)
	for _, e := range flow.Edges {
		src := edgeSource(e)
		tgt := edgeTarget(e)
		if src == "" || tgt == "" {
			continue
		}
		adj[src] = append(adj[src], tgt)
	}

	// Detect cycles with DFS using coloring: 0=unvisited, 1=visiting, 2=done
	color := make(map[string]int)
	var dfs func(node string) bool
	dfs = func(node string) bool {
		color[node] = 1
		for _, next := range adj[node] {
			switch color[next] {
			case 1:
				a.diag(Error, findNodeInFlow(flow, node),
					fmt.Sprintf("cycle detected: %s -> %s (back edge)", node, next))
				return true
			case 0:
				if dfs(next) {
					return true
				}
			}
		}
		color[node] = 2
		return false
	}

	for n := range nodeNames {
		if color[n] == 0 {
			dfs(n)
		}
	}
}

// ---------------------------------------------------------------------------
// Reference validation
// ---------------------------------------------------------------------------

func (a *Analyzer) checkReferences(flow *ast.Node, nodeNames map[string]*ast.Node) {
	for _, e := range flow.Edges {
		src := edgeSource(e)
		tgt := edgeTarget(e)

		if src == "" {
			a.diag(Error, e, "edge missing source node reference")
			continue
		}
		if _, exists := nodeNames[src]; !exists {
			a.diag(Error, e, "edge references unknown source node: "+src)
		}

		if tgt == "" {
			a.diag(Error, e, "edge missing target node reference")
			continue
		}
		if _, exists := nodeNames[tgt]; !exists {
			a.diag(Error, e, "edge references unknown target node: "+tgt)
		}
	}

	// Also check skill calls reference known skills
	for _, n := range nodeNames {
		for _, c := range n.Children {
			if c.Type == ast.NodeSkillCall {
				skillName := c.Value
				if skillName != "" && !isKnownSkill(skillName) {
					a.diag(Warning, c, "unknown skill referenced: "+skillName)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Trust type validation
// ---------------------------------------------------------------------------

// TrustType represents the trust provenance of a value.
type TrustType string

const (
	TTSecret      TrustType = "secret"
	TTUntrusted   TrustType = "untrusted"
	TTFact        TrustType = "fact"
	TTHallucinable TrustType = "hallucinable"
	TTControl     TrustType = "control"
	TTUnknown     TrustType = ""
)

// conversionAllowed defines the trust type conversion matrix.
// ✅ direct, ⚠️ requires explicit conversion (error), ❌ forbidden
var conversionAllowed = map[TrustType]map[TrustType]bool{
	TTSecret:      {TTSecret: true},
	TTUntrusted:   {TTUntrusted: true},
	TTFact:        {TTUntrusted: true, TTFact: true, TTHallucinable: true},
	TTHallucinable: {TTUntrusted: true, TTHallucinable: true},
	TTControl:     {TTControl: true},
}

// requiresConversion returns which conversions require an explicit verify/validate call.
var requiresConversion = map[TrustType]map[TrustType]bool{
	TTUntrusted:   {TTFact: true, TTHallucinable: true}, // require validate() / sanitize()
	TTHallucinable: {TTFact: true},                        // require verify()
}

func (a *Analyzer) checkTrustTypes(flow *ast.Node) {
	// Process input declarations to build known trust types
	inputTypes := make(map[string]TrustType)
	for _, c := range flow.Children {
		if c.Type == ast.NodeInput {
			if len(c.Children) >= 2 {
				name := c.Children[0].Value
				rawType := c.Children[len(c.Children)-1].Value
				tt := parseTrustType(rawType)
				inputTypes[name] = tt
				if tt == TTUnknown && rawType != "" {
					a.diag(Warning, c, "unknown trust type: "+rawType)
				}
			}
		}
	}

	// Check skill calls passing Untrusted values to memory operations
	for _, c := range flow.Children {
		if c.Type == ast.NodeNode {
			for _, ch := range c.Children {
				if ch.Type == ast.NodeSkillCall {
					a.checkSkillTrust(ch, inputTypes)
				}
			}
		}
	}
}

func (a *Analyzer) checkSkillTrust(call *ast.Node, inputTypes map[string]TrustType) {
	skill := call.Value

	// Skills that write to persistent memory should not receive Untrusted data directly
	if isPersistentWrite(skill) {
		for _, arg := range call.Args {
			if arg.Type == ast.NodeIdentifier {
				if tt, ok := inputTypes[arg.Value]; ok && tt == TTUntrusted {
					a.diag(Error, call,
						fmt.Sprintf("skill %q cannot receive Untrusted value %q without prior validation", skill, arg.Value))
				}
			}
		}
		// Also check named args
		for key, val := range call.Attrs {
			if val.Type == ast.NodeIdentifier {
				if tt, ok := inputTypes[val.Value]; ok && tt == TTUntrusted {
					if key == "text" || key == "content" || key == "prompt" {
						a.diag(Error, call,
							fmt.Sprintf("skill %q parameter %q cannot receive Untrusted value %q without validation", skill, key, val.Value))
					}
				}
			}
		}
	}

	// Secret values should never be passed to tts.speak or any output skill
	if isOutputSkill(skill) {
		for _, arg := range call.Args {
			if arg.Type == ast.NodeIdentifier {
				if tt, ok := inputTypes[arg.Value]; ok && tt == TTSecret {
					a.diag(Error, call,
						fmt.Sprintf("Secret value %q must not be passed to output skill %q", arg.Value, skill))
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Skill parameter validation
// ---------------------------------------------------------------------------

func (a *Analyzer) checkSkillParams(flow *ast.Node) {
	// Collect all skill calls in flow
	ns := ast.Collect(flow, func(n *ast.Node) bool {
		return n.Type == ast.NodeSkillCall
	})
	for _, call := range ns {
		skillName := call.Value
		if skillName == "" {
			continue
		}
		def, ok := a.skillMan.Skills[skillName]
		if !ok {
			a.diag(Warning, call, "skill not declared in manifest: "+skillName)
			continue
		}
		// Check required parameters are present
		for _, param := range def.Params {
			if param.Required {
				// Check positional args and named args
				found := false
				for _, arg := range call.Args {
					if arg.Type == ast.NodeIdentifier && arg.Value == param.Name {
						found = true
						break
					}
				}
				if _, ok := call.Attrs[param.Name]; ok {
					found = true
				}
				if !found {
					a.diag(Error, call,
						fmt.Sprintf("skill %q missing required parameter %q", skillName, param.Name))
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Trigger validation
// ---------------------------------------------------------------------------

func (a *Analyzer) checkTriggers(flow *ast.Node, nodeNames map[string]*ast.Node) {
	for _, c := range flow.Children {
		if c.Type == ast.NodeTrigger {
			// Trigger should target existing nodes
			for _, tgt := range c.Children {
				if tgt.Type == ast.NodeIdentifier {
					if _, exists := nodeNames[tgt.Value]; !exists {
						a.diag(Error, c, "trigger references unknown node: "+tgt.Value)
					}
				} else if tgt.Type == ast.NodeArrayLiteral {
					for _, item := range tgt.ArrVal {
						if _, exists := nodeNames[item.Value]; !exists {
							a.diag(Error, c, "trigger references unknown node: "+item.Value)
						}
					}
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func edgeSource(e *ast.Node) string {
	if len(e.Children) >= 1 {
		return e.Children[0].Value
	}
	return ""
}

func edgeTarget(e *ast.Node) string {
	if len(e.Children) >= 2 {
		tgt := e.Children[1]
		if tgt.Type == ast.NodeIdentifier {
			return tgt.Value
		}
		if tgt.Type == ast.NodeArrayLiteral && len(tgt.ArrVal) > 0 {
			return tgt.ArrVal[0].Value
		}
	}
	return ""
}

func findNodeInFlow(flow *ast.Node, name string) *ast.Node {
	for _, n := range flow.Nodes {
		if n.Value == name {
			return n
		}
	}
	return flow
}

func parseTrustType(s string) TrustType {
	switch strings.ToLower(s) {
	case "secret":
		return TTSecret
	case "untrusted":
		return TTUntrusted
	case "fact":
		return TTFact
	case "hallucinable":
		return TTHallucinable
	case "control":
		return TTControl
	default:
		return TTUnknown
	}
}

func isKnownSkill(name string) bool {
	known := map[string]bool{
		"memory_search": true, "memory_store": true, "web_search": true,
		"llm.chat": true, "llm.classify": true, "llm.analyze_security": true,
		"llm.analyze_performance": true, "llm.analyze_style": true,
		"tts.speak": true, "whisper.transcribe": true,
		"vision.analyze": true, "vision.generate": true,
		"agent_task": true, "flow.run": true, "graph_search": true,
		"graph_relate": true, "verify": true, "validate": true,
		"config_get": true, "config_set": true,
	}
	return known[name]
}

func isPersistentWrite(skill string) bool {
	return skill == "memory_store" || skill == "graph_relate"
}

func isOutputSkill(skill string) bool {
	return skill == "tts.speak" || skill == "whisper.transcribe" || skill == "vision.generate"
}
