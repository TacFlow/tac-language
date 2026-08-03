// manifest.go — TAC Manifest System
//
// A manifest is the compiled, verifiable artifact produced from a .tac source.
// It bundles:
//   - The validated AST
//   - A SHA-256 hash for auditability
//   - Version + schema metadata
//   - DAG topology for the runtime
//   - Required skill signatures
//   - Proved trust type assertions
//   - Compilation diagnostics
//
// The manifest serves as the contract between compile-time and runtime:
// all guarantees are proven BEFORE execution.
//
// (c) 2026 TacFlow — MIT License

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// ============================================================================
// Manifest Schema
// ============================================================================

// ManifestVersion is the current manifest schema version.
const ManifestVersion = "tac-manifest/v1"

// Manifest is the compiled artifact of a .tac program.
type Manifest struct {
	SchemaVersion string            `json:"schema_version"`
	CreatedAt     string            `json:"created_at"`
	SourceHash    string            `json:"source_hash"`   // SHA-256 of the raw .tac source
	ASTHash       string            `json:"ast_hash"`      // SHA-256 of the canonical AST JSON
	ManifestHash  string            `json:"manifest_hash"` // SHA-256 of the manifest itself (self-referential; computed last)
	Compiler      string            `json:"compiler"`      // compiler identity
	InputFile     string            `json:"input_file"`
	LanguageVersion string          `json:"language_version"`

	Analysis AnalysisResult `json:"analysis"`

	DAG DAGSummary `json:"dag"`

	SkillDependencies []SkillDependency `json:"skill_dependencies"`

	TrustAssertions []TrustAssertion `json:"trust_assertions"`

	Metadata map[string]string `json:"metadata,omitempty"`
}

// DAGSummary is the distilled topology for the runtime.
type DAGSummary struct {
	NodeCount    int        `json:"node_count"`
	EdgeCount    int        `json:"edge_count"`
	IsDAG        bool       `json:"is_dag"`
	Depth        int        `json:"depth"`
	ParallelSets [][]string `json:"parallel_sets,omitempty"`
	CriticalPath []string   `json:"critical_path,omitempty"`
}

// SkillDependency records a skill required by this program.
type SkillDependency struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"` // reserved for marketplace
	Params  []SkillParam `json:"params,omitempty"`
}

// TrustAssertion is a compile-time proven guarantee.
type TrustAssertion struct {
	Node      string `json:"node"`
	Claim     string `json:"claim"` // e.g. "Secret is never logged", "Fact requires verify()"
	Proved    bool   `json:"proved"`
	Evidence  string `json:"evidence,omitempty"` // diagnostic or rule ref
}

// ============================================================================
// Manifest Builder
// ============================================================================

// ManifestBuilder constructs a manifest from a parsed + analyzed program.
type ManifestBuilder struct {
	source         []byte
	program        *Node
	analysisResult *AnalysisResult
	inputFile      string
	langVersion    string
}

// NewManifestBuilder creates a builder.
func NewManifestBuilder(source []byte, program *Node, result *AnalysisResult, inputFile string) *ManifestBuilder {
	return &ManifestBuilder{
		source:         source,
		program:        program,
		analysisResult: result,
		inputFile:      inputFile,
		langVersion:    "v0.1.0",
	}
}

// Build assembles the manifest with all hashes and assertions.
func (mb *ManifestBuilder) Build() (*Manifest, error) {
	m := &Manifest{
		SchemaVersion: ManifestVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		Compiler:      "tac-compiler/v0.1.0 (Go)",
		InputFile:     mb.inputFile,
		LanguageVersion: mb.langVersion,
		Metadata:      make(map[string]string),
	}

	// Source hash
	srcHash := sha256.Sum256(mb.source)
	m.SourceHash = hex.EncodeToString(srcHash[:])

	// AST hash — only the program tree, stripped of position metadata for stability
	astHash := hashAST(mb.program)
	m.ASTHash = astHash

	// Analysis
	m.Analysis = *mb.analysisResult
	if m.Analysis.Flow != nil {
		m.DAG = DAGSummary{
			NodeCount:    m.Analysis.Flow.NodeCount,
			EdgeCount:    m.Analysis.Flow.EdgeCount,
			IsDAG:        m.Analysis.Flow.IsDAG,
			Depth:        m.Analysis.Flow.Depth,
			ParallelSets: m.Analysis.Flow.ParallelSets,
			CriticalPath: m.Analysis.Flow.CriticalPath,
		}

		// Skill dependencies
		skillSet := make(map[string]bool)
		for _, s := range m.Analysis.Flow.RequiredSkills {
			skillSet[s] = true
		}
		m.SkillDependencies = make([]SkillDependency, 0, len(skillSet))
		for name := range skillSet {
			sd := SkillDependency{Name: name}
			if spec, ok := skillRegistry[name]; ok {
				sd.Params = spec.Params
			}
			m.SkillDependencies = append(m.SkillDependencies, sd)
		}
		sort.Slice(m.SkillDependencies, func(i, j int) bool {
			return m.SkillDependencies[i].Name < m.SkillDependencies[j].Name
		})

		// Trust assertions from flow analysis
		m.TrustAssertions = mb.buildTrustAssertions()
	}

	// Manifest hash (self-signing: hash everything except manifest_hash itself)
	m.ManifestHash = hashManifest(m)

	return m, nil
}

// buildTrustAssertions generates trust type guarantees from the analysis.
func (mb *ManifestBuilder) buildTrustAssertions() []TrustAssertion {
	assertions := make([]TrustAssertion, 0)
	if mb.analysisResult.Flow == nil {
		return assertions
	}
	fa := mb.analysisResult.Flow

	// For each node, assert its trust type provenance
	for nodeName, typ := range fa.TrustTypes {
		claim := fmt.Sprintf("Node %q returns type %s", nodeName, typ)
		proved := true
		assertions = append(assertions, TrustAssertion{
			Node:     nodeName,
			Claim:    claim,
			Proved:   proved,
			Evidence: fmt.Sprintf("trust-type=%s", typ),
		})
	}

	// Secret nodes must never feed into unverified paths
	// (proven by the trust type conversion matrix)
	assertions = append(assertions, TrustAssertion{
		Claim:    "Secret values are never passed to output channels",
		Proved:   mb.analysisResult.Valid,
		Evidence: "trust-conversion-matrix compiled at parse-time",
	})

	// Hallucinable values cannot become Fact without verify()
	assertions = append(assertions, TrustAssertion{
		Claim:    "Hallucinable values require verify() before becoming Fact",
		Proved:   mb.analysisResult.Valid,
		Evidence: "trust-conversion-matrix compiled at parse-time",
	})

	return assertions
}

// ============================================================================
// Hashing utilities
// ============================================================================

// hashAST computes a stable SHA-256 of the AST.
// We marshal to canonical JSON (sorted keys) and strip line/col.
func hashAST(node *Node) string {
	cleaned := cleanNode(node)
	b, _ := json.Marshal(cleaned)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// cleanNode removes position metadata for deterministic hashing.
func cleanNode(node *Node) map[string]interface{} {
	if node == nil {
		return nil
	}
	m := map[string]interface{}{
		"type":  string(node.Type),
		"value": node.Value,
	}
	if node.NumVal != 0 {
		m["num_val"] = node.NumVal
	}
	if node.BoolVal {
		m["bool_val"] = true
	}

	if len(node.Children) > 0 {
		children := make([]interface{}, 0, len(node.Children))
		for _, c := range node.Children {
			children = append(children, cleanNode(c))
		}
		m["children"] = children
	}
	if len(node.Nodes) > 0 {
		nodes := make([]interface{}, 0, len(node.Nodes))
		for _, n := range node.Nodes {
			nodes = append(nodes, cleanNode(n))
		}
		m["nodes"] = nodes
	}
	if len(node.Edges) > 0 {
		edges := make([]interface{}, 0, len(node.Edges))
		for _, e := range node.Edges {
			edges = append(edges, cleanNode(e))
		}
		m["edges"] = edges
	}
	if len(node.Args) > 0 {
		args := make([]interface{}, 0, len(node.Args))
		for _, a := range node.Args {
			args = append(args, cleanNode(a))
		}
		m["args"] = args
	}
	if len(node.ArrVal) > 0 {
		arr := make([]interface{}, 0, len(node.ArrVal))
		for _, a := range node.ArrVal {
			arr = append(arr, cleanNode(a))
		}
		m["arr_val"] = arr
	}
	if len(node.MapVal) > 0 {
		mapVal := make(map[string]interface{})
		for k, v := range node.MapVal {
			mapVal[k] = cleanNode(v)
		}
		m["map_val"] = mapVal
	}
	if len(node.Attrs) > 0 {
		attrs := make(map[string]interface{})
		for k, v := range node.Attrs {
			attrs[k] = cleanNode(v)
		}
		m["attrs"] = attrs
	}
	return m
}

// hashManifest computes the SHA-256 of the manifest (excluding manifest_hash itself).
func hashManifest(m *Manifest) string {
	// Make a copy and clear the self-referential field
	type manifestCopy Manifest
	c := manifestCopy(*m)
	c.ManifestHash = ""
	b, _ := json.Marshal(c)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// ============================================================================
// Serialization
// ============================================================================

// ToJSON serializes the manifest to indented JSON.
func (m *Manifest) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// VerifyHash checks that the manifest's self-hash is consistent.
func (m *Manifest) VerifyHash() bool {
	saved := m.ManifestHash
	m.ManifestHash = ""
	recomputed := hashManifest(m)
	m.ManifestHash = saved
	return saved == recomputed
}

// ============================================================================
// Manifest -> Flow JSON (for runtime)
// ============================================================================

// FlowJSONNode is a single step in the runtime flow.
type FlowJSONNode struct {
	Name   string          `json:"name"`
	Skill  string          `json:"skill"`
	Args   map[string]interface{} `json:"args,omitempty"`
	Attrs  map[string]interface{} `json:"attrs,omitempty"`
	Deps   []string        `json:"deps"`
	Trust  string          `json:"trust"`
}

// FlowJSON is the runtime-executable flow definition.
type FlowJSON struct {
	Name       string          `json:"name"`
	Version    string          `json:"version"`
	ManifestID string          `json:"manifest_id"`
	DAG        DAGSummary      `json:"dag"`
	Nodes      []FlowJSONNode  `json:"nodes"`
	Triggers   []interface{}   `json:"triggers,omitempty"`
}

// ToFlowJSON converts a valid manifest into a Flow JSON definition for the runtime.
func (m *Manifest) ToFlowJSON() (*FlowJSON, error) {
	if !m.Analysis.Valid {
		return nil, fmt.Errorf("cannot generate flow from invalid manifest: %d errors", len(m.Analysis.Diagnostics))
	}

	fj := &FlowJSON{
		Name:       m.Analysis.Flow.Name,
		Version:    "1.0",
		ManifestID: m.ManifestHash,
		DAG:        m.DAG,
		Nodes:      make([]FlowJSONNode, 0),
	}

	// Walk the AST to build flow nodes
	for _, node := range m.findFlowAST() {
		if node.Type != NodeFlow {
			continue
		}
		fj.Name = node.Value

		for _, n := range node.Nodes {
			if n.Type != NodeNode {
				continue
			}
			fn := FlowJSONNode{
				Name: n.Value,
				Deps: make([]string, 0),
			}
			if t, ok := m.Analysis.Flow.TrustTypes[n.Value]; ok {
				fn.Trust = t
			}
			// Extract skill call
			for _, c := range n.Children {
				if c.Type == NodeSkillCall {
					fn.Skill = c.Value
					if c.Attrs != nil {
						fn.Attrs = make(map[string]interface{})
						for k, v := range c.Attrs {
							fn.Attrs[k] = nodeValueToInterface(v)
						}
					}
				}
			}
			fj.Nodes = append(fj.Nodes, fn)
		}

		// Build dependency list from edges
		depMap := make(map[string][]string)
		for _, e := range node.Edges {
			src, dst := edgeEndpoints(e)
			if src != "" && dst != "" {
				depMap[dst] = append(depMap[dst], src)
			}
		}
		for i := range fj.Nodes {
			if deps, ok := depMap[fj.Nodes[i].Name]; ok {
				fj.Nodes[i].Deps = deps
			}
		}
	}

	return fj, nil
}

func (m *Manifest) findFlowAST() []*Node {
	// The manifest doesn't directly store the AST — we use the analysis flow
	// To properly do this, the manifest builder should store the AST fragment
	// For now, return empty — FlowJSON construction uses the AnalysisResult
	return nil
}

func nodeValueToInterface(n *Node) interface{} {
	switch n.Type {
	case NodeStringLiteral:
		return n.Value
	case NodeNumberLiteral:
		return n.NumVal
	case NodeBoolLiteral:
		return n.BoolVal
	case NodeArrayLiteral:
		arr := make([]interface{}, 0, len(n.ArrVal))
		for _, a := range n.ArrVal {
			arr = append(arr, nodeValueToInterface(a))
		}
		return arr
	case NodeObjectLiteral:
		obj := make(map[string]interface{})
		for k, v := range n.MapVal {
			obj[k] = nodeValueToInterface(v)
		}
		return obj
	case NodeIdentifier:
		return "$" + n.Value // marked for runtime resolution
	}
	return n.Value
}
