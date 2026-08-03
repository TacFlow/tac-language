// Package semantic implements the TAC Language semantic analyzer.
//
// The analyzer validates a parsed AST against the rules in SPEC v2.0:
//   - Node references (edges must reference declared nodes)
//   - Trigger references (triggers must target declared nodes)
//   - DAG validation (cycles are forbidden)
//   - Skill registry (skill names must resolve to known capabilities)
//   - Trust type dataflow checking (Secret/Untrusted/Fact/Hallucinable/Control)
//   - Dead node detection (trigger-based reachability)
//
// The analyzer supports two modes:
//   - ModeDevelopment: lenient (unknown skills are warnings)
//   - ModeProduction:  strict (unknown/unsigned/unversioned skills are errors)
package semantic

import (
	"context"
	"crypto/sha256"
	"encoding/json"
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

// Diagnostic is a single semantic finding with a structured error code
// following SPEC v0.3 §24 diagnostic categories.
type Diagnostic struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Line     int      `json:"line,omitempty"`
	Col      int      `json:"col,omitempty"`
	Flow     string   `json:"flow,omitempty"`

	// Structured metadata for machine consumption.
	SourceType        string `json:"source_type,omitempty"`
	TargetType        string `json:"target_type,omitempty"`
	SuggestedConversion string `json:"suggested_conversion,omitempty"`
}

func (d Diagnostic) String() string {
	sev := d.Severity.String()
	if d.Code != "" {
		if d.Line > 0 {
			return fmt.Sprintf("%s [%s] at line %d, col %d: %s", sev, d.Code, d.Line, d.Col, d.Message)
		}
		return fmt.Sprintf("%s [%s]: %s", sev, d.Code, d.Message)
	}
	if d.Line > 0 {
		return fmt.Sprintf("%s at line %d, col %d: %s", sev, d.Line, d.Col, d.Message)
	}
	return fmt.Sprintf("%s: %s", sev, d.Message)
}

// Diagnostic code prefixes per SPEC v0.3 §24.
const (
	DiagLex       = "TAC-LEX"
	DiagParse     = "TAC-PARSE"
	DiagSymbol    = "TAC-SYMBOL"
	DiagGraph     = "TAC-GRAPH"
	DiagSkill     = "TAC-SKILL"
	DiagType      = "TAC-TYPE"
	DiagTrust     = "TAC-TRUST"
	DiagPolicy    = "TAC-POLICY"
	DiagArtifact  = "TAC-ARTIFACT"
	DiagSchema    = "TAC-SCHEMA"
	DiagCanonical = "TAC-CANONICAL"
)

// Mode selects how strict semantic analysis is.
type Mode int

const (
	ModeDevelopment Mode = iota
	ModeProduction
)

func (m Mode) String() string {
	if m == ModeProduction {
		return "production"
	}
	return "development"
}

// SkillSpec describes a known skill and its trust contract.
type SkillSpec struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	ReturnType  types.TrustType   `json:"return_type"`
	Args        []string          `json:"args"`
	ArgTypes    map[string]types.TrustType `json:"arg_types"`
	Description string            `json:"description"`

	// Dynamic-skill metadata
	Runtime      string   `json:"runtime"`
	Entrypoint   string   `json:"entrypoint"`
	Artifact     string   `json:"artifact"`
	Digest       string   `json:"digest"`
	Signature    string   `json:"signature"`
	InputSchema  string   `json:"input_schema"`
	OutputSchema string   `json:"output_schema"`
	Capabilities []string `json:"capabilities"`

	Permissions map[string]interface{} `json:"permissions"`
	Execution   ExecutionSpec          `json:"execution"`
}

// ExecutionSpec describes runtime limits for a dynamic skill.
type ExecutionSpec struct {
	TimeoutSeconds int  `json:"timeout_seconds"`
	MemoryMB       int  `json:"memory_mb"`
	CPU            int  `json:"cpu"`
	Retries        int  `json:"retries"`
	Idempotent     bool `json:"idempotent"`
	Cancellable    bool `json:"cancellable"`
}

// IsDynamic reports whether the skill is a Python / runtime-loaded skill.
func (s SkillSpec) IsDynamic() bool {
	return s.Runtime != "" && s.Runtime != "go"
}

// Registry is a mutable skill registry.
type Registry struct {
	skills map[string]SkillSpec
}

// NewRegistry returns an empty skill registry.
func NewRegistry() *Registry {
	return &Registry{skills: make(map[string]SkillSpec)}
}

// Register adds or replaces a skill spec.
func (r *Registry) Register(spec SkillSpec) {
	r.skills[spec.Name] = spec
}

// Lookup resolves a skill by name.
func (r *Registry) Lookup(name string) (SkillSpec, bool) {
	spec, ok := r.skills[name]
	return spec, ok
}

// LookupVersioned resolves a skill by name and version.
func (r *Registry) LookupVersioned(name, version string) (SkillSpec, bool) {
	spec, ok := r.skills[name]
	if !ok {
		return SkillSpec{}, false
	}
	if version != "" && spec.Version != "" && spec.Version != version {
		return SkillSpec{}, false
	}
	return spec, true
}

// Names returns sorted registered skill names.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Resolve implements SkillRegistry and wraps LookupVersioned.
func (r *Registry) Resolve(_ context.Context, id, version string) (SkillSpec, error) {
	spec, ok := r.LookupVersioned(id, version)
	if !ok {
		if version != "" {
			return SkillSpec{}, fmt.Errorf("skill %q@%q not found", id, version)
		}
		return SkillSpec{}, fmt.Errorf("skill %q not found", id)
	}
	return spec, nil
}

// SnapshotDigest returns a cryptographic hash of the registry contents.
func (r *Registry) SnapshotDigest(_ context.Context) (string, error) {
	names := r.Names()
	type entry struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Digest  string `json:"digest"`
		Runtime string `json:"runtime,omitempty"`
	}
	entries := make([]entry, 0, len(names))
	for _, name := range names {
		s := r.skills[name]
		entries = append(entries, entry{
			Name:    s.Name,
			Version: s.Version,
			Digest:  s.Digest,
			Runtime: s.Runtime,
		})
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("snapshot marshal: %w", err)
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", h), nil
}

// ensure Registry satisfies SkillRegistry.
var _ SkillRegistry = (*Registry)(nil)
// builtinSkills is the standard library of TAC skills (SPEC §7.1).
var builtinSkills = map[string]SkillSpec{
	"memory_search": {
		Name: "memory_search", Version: "1.0", ReturnType: types.Fact,
		Args:        []string{"query", "scope", "top_k"},
		ArgTypes:    map[string]types.TrustType{},
		Description: "Search BM25+Vector+Graph memory",
	},
	"memory_store": {
		Name: "memory_store", Version: "1.0", ReturnType: types.Fact,
		Args:        []string{"text", "tags", "shared"},
		ArgTypes:    map[string]types.TrustType{"text": types.Fact},
		Description: "Store in persistent memory",
	},
	"web_search": {
		Name: "web_search", Version: "1.0", ReturnType: types.Untrusted,
		Args:        []string{"query", "count"},
		ArgTypes:    map[string]types.TrustType{},
		Description: "Search the web (Brave)",
	},
	"llm.chat": {
		Name: "llm.chat", Version: "1.0", ReturnType: types.Hallucinable,
		Args:        []string{"prompt", "context", "model"},
		ArgTypes:    map[string]types.TrustType{},
		Description: "Call LLM with prompt",
	},
	"llm.classify": {
		Name: "llm.classify", Version: "1.0", ReturnType: types.Hallucinable,
		Args:        []string{"text"},
		ArgTypes:    map[string]types.TrustType{},
		Description: "Classify input text",
	},
	"tts.speak": {
		Name: "tts.speak", Version: "1.0", ReturnType: types.Control,
		Args:        []string{"text", "voice"},
		ArgTypes:    map[string]types.TrustType{},
		Description: "Text-to-speech output",
	},
	"whisper.transcribe": {
		Name: "whisper.transcribe", Version: "1.0", ReturnType: types.Untrusted,
		Args:        []string{"audio"},
		ArgTypes:    map[string]types.TrustType{},
		Description: "Speech-to-text input",
	},
	"vision.analyze": {
		Name: "vision.analyze", Version: "1.0", ReturnType: types.Hallucinable,
		Args:        []string{"image"},
		ArgTypes:    map[string]types.TrustType{},
		Description: "Analyze an image",
	},
	"vision.generate": {
		Name: "vision.generate", Version: "1.0", ReturnType: types.Hallucinable,
		Args:        []string{"prompt"},
		ArgTypes:    map[string]types.TrustType{},
		Description: "Generate an image",
	},
	"agent_task": {
		Name: "agent_task", Version: "1.0", ReturnType: types.Control,
		Args:        []string{"agent", "payload", "priority"},
		ArgTypes:    map[string]types.TrustType{},
		Description: "Delegate to swarm agent",
	},
	"agent_wait": {
		Name: "agent_wait", Version: "1.0", ReturnType: types.Control,
		Args:        []string{"task_id", "timeout"},
		ArgTypes:    map[string]types.TrustType{},
		Description: "Await agent task result",
	},
	"flow.run": {
		Name: "flow.run", Version: "1.0", ReturnType: types.Control,
		Args:        []string{"flow", "params"},
		ArgTypes:    map[string]types.TrustType{},
		Description: "Execute a sub-flow",
	},
	"graph_search": {
		Name: "graph_search", Version: "1.0", ReturnType: types.Fact,
		Args:        []string{"query", "depth"},
		ArgTypes:    map[string]types.TrustType{},
		Description: "Traverse the knowledge graph",
	},
	"graph_relate": {
		Name: "graph_relate", Version: "1.0", ReturnType: types.Control,
		Args:        []string{"source", "target", "type"},
		ArgTypes:    map[string]types.TrustType{"source": types.Fact, "target": types.Fact},
		Description: "Create graph edge",
	},
	"validate": {
		Name: "validate", Version: "1.0", ReturnType: types.Fact,
		Args:        []string{"value"},
		ArgTypes:    map[string]types.TrustType{"value": types.Untrusted},
		Description: "Sanitize an Untrusted value",
	},
	"verify": {
		Name: "verify", Version: "1.0", ReturnType: types.Fact,
		Args:        []string{"source"},
		ArgTypes:    map[string]types.TrustType{"source": types.Hallucinable},
		Description: "Fact-check a Hallucinable value",
	},
	"authorize": {
		Name: "authorize", Version: "1.0", ReturnType: types.Fact,
		Args:        []string{"source", "policy"},
		ArgTypes:    map[string]types.TrustType{"source": types.Control},
		Description: "Authorize a Control or Secret value for Fact use",
	},
	"config_get": {
		Name: "config_get", Version: "1.0", ReturnType: types.Fact,
		Args:        []string{"key"},
		ArgTypes:    map[string]types.TrustType{},
		Description: "Read configuration",
	},
	"config_set": {
		Name: "config_set", Version: "1.0", ReturnType: types.Control,
		Args:        []string{"key", "value"},
		ArgTypes:    map[string]types.TrustType{},
		Description: "Write configuration",
	},
	"get_current_time": {
		Name: "get_current_time", Version: "1.0", ReturnType: types.Fact,
		Args:        nil,
		ArgTypes:    map[string]types.TrustType{},
		Description: "Get current date and time",
	},
	"swarm_status": {
		Name: "swarm_status", Version: "1.0", ReturnType: types.Control,
		Args:        nil,
		ArgTypes:    map[string]types.TrustType{},
		Description: "Check swarm health",
	},
	"swarm_check_my_status": {
		Name: "swarm_check_my_status", Version: "1.0", ReturnType: types.Control,
		Args:        nil,
		ArgTypes:    map[string]types.TrustType{},
		Description: "Check own reputation",
	},
	"swarm_teach": {
		Name: "swarm_teach", Version: "1.0", ReturnType: types.Control,
		Args:        []string{"name", "content"},
		ArgTypes:    map[string]types.TrustType{"content": types.Fact},
		Description: "Share knowledge across the swarm",
	},
	"search_hybrid": {
		Name: "search_hybrid", Version: "1.0", ReturnType: types.Fact,
		Args:        []string{"query", "bm25_weight", "vector_weight", "graph_weight", "top_k"},
		ArgTypes:    map[string]types.TrustType{},
		Description: "Hybrid search across all memory layers",
	},
	"set_token_limit": {
		Name: "set_token_limit", Version: "1.0", ReturnType: types.Control,
		Args:        []string{"max", "scope"},
		ArgTypes:    map[string]types.TrustType{},
		Description: "Set per-session or per-flow token budget",
	},
}

// BuiltinSkills is the standard library of TAC skills (SPEC §7.1), exposed as
// a map for backwards compatibility.
var BuiltinSkills = builtinSkills

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

// BuiltinRegistry returns a Registry preloaded with the standard library.
func BuiltinRegistry() *Registry {
	r := NewRegistry()
	for name, spec := range BuiltinSkills {
		spec.Name = name
		r.Register(spec)
	}
	return r
}

// Analyzer validates a TAC program.
type Analyzer struct {
	diagnostics []Diagnostic
	mode        Mode
	registry    *Registry
	// Track declared inputs across the program
	inputs map[string]types.TrustType
	// Track declared agents
	agents map[string]bool
}

// New creates a new Analyzer in development mode.
func New() *Analyzer {
	return NewWithMode(ModeDevelopment)
}

// NewWithMode creates an Analyzer with the given strictness mode.
func NewWithMode(mode Mode) *Analyzer {
	return &Analyzer{
		mode:     mode,
		registry: BuiltinRegistry(),
		inputs:   make(map[string]types.TrustType),
		agents:   make(map[string]bool),
	}
}

// NewWithRegistry creates an Analyzer with a custom skill registry.
func NewWithRegistry(registry *Registry, mode Mode) *Analyzer {
	if registry == nil {
		registry = BuiltinRegistry()
	}
	return &Analyzer{
		mode:     mode,
		registry: registry,
		inputs:   make(map[string]types.TrustType),
		agents:   make(map[string]bool),
	}
}

// Mode returns the analyzer strictness mode.
func (a *Analyzer) Mode() Mode { return a.mode }

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

func (a *Analyzer) errorf(code string, line, col int, format string, args ...interface{}) {
	a.diagnostics = append(a.diagnostics, Diagnostic{
		Code:     code,
		Severity: SeverityError,
		Message:  fmt.Sprintf(format, args...),
		Line:     line,
		Col:      col,
	})
}

func (a *Analyzer) warningf(code string, line, col int, format string, args ...interface{}) {
	a.diagnostics = append(a.diagnostics, Diagnostic{
		Code:     code,
		Severity: SeverityWarning,
		Message:  fmt.Sprintf(format, args...),
		Line:     line,
		Col:      col,
	})
}

// trustDiag emits a diagnostic with structured Trust Type metadata.
func (a *Analyzer) trustDiag(line, col int, srcType, tgtType types.TrustType, format string, args ...interface{}) {
	suggested := types.ConversionFunction(srcType, tgtType)
	a.diagnostics = append(a.diagnostics, Diagnostic{
		Code:                DiagTrust + "-001",
		Severity:            SeverityError,
		Message:             fmt.Sprintf(format, args...),
		Line:                line,
		Col:                 col,
		SourceType:          string(srcType),
		TargetType:          string(tgtType),
		SuggestedConversion: suggested,
	})
}

// Analyze validates a complete Program AST.
func (a *Analyzer) Analyze(program *ast.Node) []Diagnostic {
	if program == nil {
		a.errorf("", 0, 0, "nil program AST")
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
		a.errorf("", n.Pos.Line, n.Pos.Col, "remember requires a name and a value")
		return
	}
	// Validate type attribute if present
	if attrs := n.Attrs; attrs != nil {
		if typeAttr, ok := attrs["type"]; ok && typeAttr != nil {
			if !types.IsValidTrustType(typeAttr.Value) {
				a.warningf("", typeAttr.Pos.Line, typeAttr.Pos.Col,
					"unknown trust type %q in remember (valid: %s)",
					typeAttr.Value, strings.Join(trustTypeNames(), ", "))
			}
		}
	}
}

func (a *Analyzer) validateRecall(n *ast.Node) {
	if len(n.Children) < 1 {
		a.errorf("", n.Pos.Line, n.Pos.Col, "recall requires a name")
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
			a.errorf("", node.Pos.Line, node.Pos.Col, "flow %q: node without a name", flowName)
			continue
		}
		if _, dup := declared[name]; dup {
			a.errorf("", node.Pos.Line, node.Pos.Col, "flow %q: duplicate node %q", flowName, name)
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
			a.errorf("", edge.Pos.Line, edge.Pos.Col,
				"flow %q: edge references undeclared source node %q", flowName, src)
			continue
		}
		if _, ok := declared[tgt]; !ok {
			a.errorf("", edge.Pos.Line, edge.Pos.Col,
				"flow %q: edge references undeclared target node %q", flowName, tgt)
			continue
		}

		adjacency[src] = append(adjacency[src], tgt)
		inDegree[tgt]++

		// Validate conditional edges
		if cond, fallback, hasCond := ast.EdgeCondition(edge); hasCond {
			if cond == "" {
				a.warningf("", edge.Pos.Line, edge.Pos.Col,
					"flow %q: edge %s -> %s has empty if condition", flowName, src, tgt)
			}
			if fallback != "" {
				if _, ok := declared[fallback]; !ok {
					a.errorf("", edge.Pos.Line, edge.Pos.Col,
						"flow %q: else target %q is not a declared node", flowName, fallback)
				}
			}
		}
	}

	// --- Cycle detection (Kahn algorithm) ---
	cycle := detectCycle(declared, adjacency, inDegree)
	if cycle != "" {
		a.errorf(DiagGraph+"-001", flow.Pos.Line, flow.Pos.Col,
			"flow %q: cycle detected in dependency graph: %s", flowName, cycle)
	}

	// --- Dead node detection (trigger-based reachability) ---
	// Reachability starts from trigger targets and entry points, not
	// from every node with indegree zero. In a disconnected DAG each
	// component has a zero-indegree node, so indegree alone is not a
	// sound proxy for reachability.
	reachable := a.computeReachable(flow, declared, adjacency)
	for name := range declared {
		if !reachable[name] {
			a.report(DiagGraph+"-003", declared[name].Pos.Line, declared[name].Pos.Col,
				"flow %q: node %q is unreachable (not started by any trigger or entry)",
				flowName, name)
		}
	}

	// --- Validate node internals ---
	for _, node := range flow.Nodes {
		a.validateNodeDef(node, flowName, declared)
	}

	// --- Trust type dataflow checking ---
	a.checkTrustDataflow(flow, flowName, declared)

	// --- Validate triggers ---
	a.validateTriggers(flow, flowName, declared)

	// --- Validate input references ---
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

// validateSkillCall validates a skill invocation against the registry.
// In development mode unknown skills are warnings; in production they are errors.
// Dynamic skills require digest + signature + schemas in production.
func (a *Analyzer) validateSkillCall(call *ast.Node, flowName, nodeName string) {
	skillName := call.Value
	version := call.Version
	spec, ok := a.registry.LookupVersioned(skillName, version)
	if !ok {
		if version != "" {
			a.report(DiagSkill+"-001", call.Pos.Line, call.Pos.Col,
				"flow %q node %q: unknown skill %q@%q (not in standard library)",
				flowName, nodeName, skillName, version)
		} else {
			a.report(DiagSkill+"-001", call.Pos.Line, call.Pos.Col,
				"flow %q node %q: unknown skill %q (not in standard library)",
				flowName, nodeName, skillName)
		}
		return
	}

	// Production-mode strictness for dynamic skills
	if a.mode == ModeProduction && spec.IsDynamic() {
		if spec.Digest == "" {
			a.errorf(DiagArtifact+"-001", call.Pos.Line, call.Pos.Col,
				"flow %q node %q: dynamic skill %q is missing artifact digest (required in production)",
				flowName, nodeName, skillName)
		}
		if spec.Signature == "" {
			a.errorf(DiagArtifact+"-002", call.Pos.Line, call.Pos.Col,
				"flow %q node %q: dynamic skill %q is unsigned (required in production)",
				flowName, nodeName, skillName)
		}
		if spec.InputSchema == "" {
			a.errorf(DiagSchema+"-001", call.Pos.Line, call.Pos.Col,
				"flow %q node %q: dynamic skill %q is missing input schema (required in production)",
				flowName, nodeName, skillName)
		}
		if spec.OutputSchema == "" {
			a.errorf(DiagSchema+"-002", call.Pos.Line, call.Pos.Col,
				"flow %q node %q: dynamic skill %q is missing output schema (required in production)",
				flowName, nodeName, skillName)
		}
	}

	// Validate known arguments
	if call.Attrs != nil {
		for argName := range call.Attrs {
			if !contains(spec.Args, argName) {
				a.warningf("", call.Pos.Line, call.Pos.Col,
					"flow %q node %q: skill %q does not declare argument %q (known: %s)",
					flowName, nodeName, skillName, argName, strings.Join(spec.Args, ", "))
			}
		}
	}
	// Also validate named args in parens (NodeNamedArg children)
	for _, arg := range call.Args {
		if arg != nil && arg.Type == ast.NodeNamedArg {
			if !contains(spec.Args, arg.Value) {
				a.warningf("", arg.Pos.Line, arg.Pos.Col,
					"flow %q node %q: skill %q does not declare argument %q (known: %s)",
					flowName, nodeName, skillName, arg.Value, strings.Join(spec.Args, ", "))
			}
		}
	}
}

// report emits a diagnostic whose severity depends on the analyzer mode.
func (a *Analyzer) report(code string, line, col int, format string, args ...interface{}) {
	if a.mode == ModeProduction {
		a.errorf(code, line, col, format, args...)
	} else {
		a.warningf(code, line, col, format, args...)
	}
}

// checkTrustDataflow performs forward dataflow analysis over the flow DAG and
// verifies every value crossing a skill boundary satisfies the skill's
// trust-type requirements (SPEC §5.3).
//
// Example rejection:
//   input prompt: Untrusted
//   node "persist" -> skill memory_store(text: prompt)
//   // ERROR: prompt is Untrusted, memory_store.text requires Fact
//
// The flow must add an explicit conversion:
//   node "validate" -> skill validate(value: prompt)
//   node "persist"  -> skill memory_store(text: validate.result)
func (a *Analyzer) checkTrustDataflow(flow *ast.Node, flowName string, declared map[string]*ast.Node) {
	// Node output types: node name -> trust type produced by its skill.
	nodeTypes := make(map[string]types.TrustType)
	// skillOf maps node name -> its skill call node.
	skillOf := make(map[string]*ast.Node)

	for name, node := range declared {
		call := findSkillCall(node)
		if call == nil {
			continue
		}
		skillOf[name] = call
		spec, ok := a.registry.Lookup(call.Value)
		if !ok {
			continue // unknown skill — cannot infer output type
		}
		nodeTypes[name] = spec.ReturnType
	}

	// Input types declared inside the flow.
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

	// For every skill call, check each named arg against the required type.
	for nodeName, call := range skillOf {
		spec, ok := a.registry.Lookup(call.Value)
		if !ok {
			continue
		}
		required := spec.ArgTypes
		if len(required) == 0 {
			continue
		}
		for _, arg := range call.Args {
			if arg == nil || arg.Type != ast.NodeNamedArg {
				continue
			}
			reqType, needs := required[arg.Value]
			if !needs {
				continue
			}
			if len(arg.Children) == 0 {
				continue
			}
			val := arg.Children[0]
			a.checkValueTrust(flowName, nodeName, arg.Value, val, reqType, nodeTypes, flowInputs)
		}
		// Also check block attrs (named args after the call body).
		for argName, val := range call.Attrs {
			reqType, needs := required[argName]
			if !needs {
				continue
			}
			a.checkValueTrust(flowName, nodeName, argName, val, reqType, nodeTypes, flowInputs)
		}
	}
}

// checkValueTrust recurses into a value tree (identifier, array, object) and
// verifies each node-output or input reference against the required trust type.
// Literal values (strings, numbers, bools) are safe by construction.
func (a *Analyzer) checkValueTrust(flowName, nodeName, argName string, val *ast.Node, reqType types.TrustType, nodeTypes map[string]types.TrustType, flowInputs map[string]types.TrustType) {
	if val == nil {
		return
	}
	switch val.Type {
	case ast.NodeIdentifier:
		srcType, ok := resolveReferenceType(val.Value, nodeTypes, flowInputs)
		if !ok {
			return // not a typed reference; skip
		}
		a.checkConversion(flowName, nodeName, argName, srcType, reqType, val.Pos.Line, val.Pos.Col)
	case ast.NodeArrayLiteral:
		for _, item := range val.ArrVal {
			a.checkValueTrust(flowName, nodeName, argName, item, reqType, nodeTypes, flowInputs)
		}
	case ast.NodeObjectLiteral:
		for _, item := range val.MapVal {
			a.checkValueTrust(flowName, nodeName, argName, item, reqType, nodeTypes, flowInputs)
		}
	}
}

// resolveReferenceType maps an identifier like "search_web.result" or "question"
// to a trust type from node outputs or flow inputs.
func resolveReferenceType(id string, nodeTypes map[string]types.TrustType, flowInputs map[string]types.TrustType) (types.TrustType, bool) {
	base := id
	if idx := strings.Index(id, "."); idx > 0 {
		base = id[:idx]
	}
	if tt, ok := nodeTypes[base]; ok {
		return tt, true
	}
	if tt, ok := flowInputs[base]; ok {
		return tt, true
	}
	return "", false
}

// checkConversion emits an error when a source trust type cannot satisfy a
// required target type without an explicit conversion step.
func (a *Analyzer) checkConversion(flowName, nodeName, argName string, src, dst types.TrustType, line, col int) {
	rule, allowed := types.CanConvert(src, dst)
	if !allowed {
		a.trustDiag(line, col, src, dst,
			"flow %q node %q: argument %q has trust type %s, but skill requires %s; %s → %s is forbidden",
			flowName, nodeName, argName, src, dst, src, dst)
		return
	}
	if rule == types.ConvertExplicit {
		a.trustDiag(line, col, src, dst,
			"flow %q node %q: argument %q has trust type %s, but skill requires %s; %s → %s requires explicit %s()",
			flowName, nodeName, argName, src, dst, src, dst, types.ConversionFunction(src, dst))
	}
}

// findSkillCall returns the first SkillCall node inside a node definition.
func findSkillCall(node *ast.Node) *ast.Node {
	if node == nil {
		return nil
	}
	for _, child := range node.Children {
		if child.Type == ast.NodeSkillCall {
			return child
		}
	}
	for _, sub := range node.Nodes {
		if sub.Type == ast.NodeSkillCall {
			return sub
		}
	}
	return nil
}

func (a *Analyzer) validateTriggers(flow *ast.Node, flowName string, declared map[string]*ast.Node) {
	for _, child := range flow.Children {
		if child.Type != ast.NodeTrigger {
			continue
		}
		if len(child.Children) < 1 {
			a.errorf("", child.Pos.Line, child.Pos.Col,
				"flow %q: trigger without event name", flowName)
			continue
		}
		last := child.Children[len(child.Children)-1]
		switch last.Type {
		case ast.NodeIdentifier:
			if _, ok := declared[last.Value]; !ok {
				a.errorf("", last.Pos.Line, last.Pos.Col,
					"flow %q: trigger targets undeclared node %q", flowName, last.Value)
			}
		case ast.NodeArrayLiteral:
			for _, item := range last.ArrVal {
				if _, ok := declared[item.Value]; !ok {
					a.errorf("", item.Pos.Line, item.Pos.Col,
						"flow %q: trigger targets undeclared node %q", flowName, item.Value)
				}
			}
		}
	}
}

func (a *Analyzer) validateInputReferences(flow *ast.Node, flowName string) {
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
			if flowInputs[n.Value] != "" || a.inputs[n.Value] != "" {
				return true
			}
		}
		return true
	})
}

// --- Graph algorithms ---

// detectCycle uses Kahn algorithm to detect cycles.
func detectCycle(declared map[string]*ast.Node, adjacency map[string][]string, inDegree map[string]int) string {
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
		for name, deg := range indeg {
			if deg > 0 {
				return name
			}
		}
		return "(unknown)"
	}
	return ""
}

// computeReachable determines which nodes are reachable from trigger targets
// and entry points in the flow, NOT from all zero-indegree nodes.
//
// When triggers exist, only trigger-targeted nodes (and their downstream
// dependencies) are considered reachable. When no triggers are declared,
// we fall back to all indegree-zero nodes (external invocation via flow.run).
func (a *Analyzer) computeReachable(flow *ast.Node, declared map[string]*ast.Node, adjacency map[string][]string) map[string]bool {
	reachable := make(map[string]bool)

	// Collect trigger targets
	triggerTargets := make(map[string]bool)
	for _, child := range flow.Children {
		if child.Type != ast.NodeTrigger || len(child.Children) < 1 {
			continue
		}
		last := child.Children[len(child.Children)-1]
		switch last.Type {
		case ast.NodeIdentifier:
			triggerTargets[last.Value] = true
		case ast.NodeArrayLiteral:
			for _, item := range last.ArrVal {
				triggerTargets[item.Value] = true
			}
		}
	}

	// Roots for BFS
	var stack []string

	if len(triggerTargets) > 0 {
		// Only trigger-targeted nodes are roots
		for name := range declared {
			if triggerTargets[name] {
				stack = append(stack, name)
			}
		}
	} else {
		// No triggers: fall back to indegree-zero nodes
		hasIncoming := make(map[string]bool)
		for _, targets := range adjacency {
			for _, t := range targets {
				hasIncoming[t] = true
			}
		}
		for name := range declared {
			if !hasIncoming[name] {
				stack = append(stack, name)
			}
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
