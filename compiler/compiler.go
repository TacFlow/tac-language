// Package compiler converts a TAC AST into Flow Definition JSON
// compatible with the TacFlow flow engine (SPEC §10, Stage 3).
//
// It produces the structured JSON consumed by flow-management,
// with node-by-node skill invocations, edge dependencies, and
// triggers mapped to their flow-engine equivalents.
package compiler

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/TacFlow/tac-language/ast"
	"github.com/TacFlow/tac-language/manifest"
	"github.com/TacFlow/tac-language/semantic"
)

// FlowJSON is the output format consumed by the TacFlow flow engine.
// Enhanced per SPEC v0.3 §17 with registry snapshot, fingerprint, skill metadata.
type FlowJSON struct {
	FlowID            string            `json:"flow_id"`
	Name              string            `json:"name"`
	Version           string            `json:"version"`
	Language          LanguageMeta      `json:"language"`
	RegistrySnapshot  string            `json:"registry_snapshot,omitempty"`
	TrustPolicyVersion string           `json:"trust_policy_version,omitempty"`
	Fingerprint       string            `json:"fingerprint,omitempty"`
	Nodes             []FlowNode        `json:"nodes"`
	Edges             []FlowEdge        `json:"edges"`
	Triggers          []FlowTrigger     `json:"triggers,omitempty"`
	Manifest          *manifest.Manifest `json:"manifest,omitempty"`
}

// LanguageMeta stamps the language and compiler versions used to produce the
// Flow JSON, ensuring reproducibility and auditability (SPEC v0.3 §8).
type LanguageMeta struct {
	Name            string `json:"name"`
	LanguageVersion string `json:"language_version"`
	CompilerVersion string `json:"compiler_version"`
	IRVersion       string `json:"ir_version"`
}

// ConditionIR is a structured condition expression (SPEC §10, Stage 2).
// Instead of flattening conditions to opaque strings, the compiler emits
// a structured form the runtime can evaluate without reparsing.
type ConditionIR struct {
	Operator string      `json:"operator"`
	Left     ConditionOp `json:"left"`
	Right    ConditionOp `json:"right"`
}

// ConditionOp is one operand of a condition expression.
type ConditionOp struct {
	Kind  string      `json:"kind"`  // "node_output", "number", "string", "bool"
	Node  string      `json:"node,omitempty"`
	Path  string      `json:"path,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

// FlowNode represents a single executable step.
type FlowNode struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Skill     string            `json:"skill"`
	Args      map[string]interface{} `json:"args,omitempty"`
	Pos       ast.Position      `json:"pos,omitempty"`
}

// FlowEdge represents a dependency between two steps.
type FlowEdge struct {
	From      string        `json:"from"`
	To        string        `json:"to"`
	Condition *ConditionIR  `json:"condition,omitempty"`
	Fallback  string        `json:"fallback,omitempty"`
}

// FlowTrigger represents an event-driven activation.
type FlowTrigger struct {
	Event    string   `json:"event"`
	Priority int      `json:"priority,omitempty"`
	Targets  []string `json:"targets"`
}

// Compile converts a flow AST node into a FlowJSON definition.
func Compile(flow *ast.Node) (*FlowJSON, error) {
	if flow == nil || flow.Type != ast.NodeFlow {
		return nil, fmt.Errorf("expected Flow node, got %v", flow.Type)
	}

	fj := &FlowJSON{
		Name:    flow.Value,
		Version: "1.0",
		Language: LanguageMeta{
			Name:            "TAC",
			LanguageVersion: "0.3",
			CompilerVersion: "0.3.0",
			IRVersion:       "1.1",
		},
		Nodes:    make([]FlowNode, 0),
		Edges:    make([]FlowEdge, 0),
		Manifest: manifest.ExtractManifest(flow),
	}

	// Compile nodes
	nodeIDs := make(map[string]string) // display name -> id
	for i, node := range ast.CollectNodes(flow) {
		name := ast.NodeName(node)
		id := fmt.Sprintf("n%d", i)
		nodeIDs[name] = id

		skillName := ""
		var args map[string]interface{}

		for _, child := range node.Children {
			if child.Type == ast.NodeSkillCall {
				skillName = child.Value
				args = compileArgs(child)
				break
			}
		}
		// Also check nested blocks for skill calls
		for _, sub := range node.Nodes {
			if sub.Type == ast.NodeSkillCall {
				skillName = sub.Value
				if args == nil {
					args = compileArgs(sub)
				}
			}
		}

		fj.Nodes = append(fj.Nodes, FlowNode{
			ID:    id,
			Name:  name,
			Skill: skillName,
			Args:  args,
			Pos:   node.Pos,
		})
	}

	// Compile edges
	for _, edge := range ast.CollectEdges(flow) {
		src := ast.EdgeSource(edge)
		tgt := ast.EdgeTarget(edge)

		fe := FlowEdge{
			From: nodeIDs[src],
			To:   nodeIDs[tgt],
		}

		if _, fb, hasCond := ast.EdgeCondition(edge); hasCond {
			fe.Condition = compileCondition(edge.Attrs["if"])
			if fb != "" {
				if fbID, ok := nodeIDs[fb]; ok {
					fe.Fallback = fbID
				} else {
					fe.Fallback = fb
				}
			}
		}

		fj.Edges = append(fj.Edges, fe)
	}

	// Compile triggers
	for _, child := range flow.Children {
		if child.Type != ast.NodeTrigger {
			continue
		}
		var eventName string
		if len(child.Children) > 0 && child.Children[0].Type == ast.NodeStringLiteral {
			eventName = child.Children[0].Value
		}

		ft := FlowTrigger{
			Event: eventName,
		}

		// Priority
		if child.Attrs != nil {
			if prio, ok := child.Attrs["priority"]; ok && prio != nil && prio.NumVal > 0 {
				ft.Priority = int(prio.NumVal)
			}
		}

		// Targets
		if len(child.Children) > 1 {
			last := child.Children[len(child.Children)-1]
			switch last.Type {
			case ast.NodeIdentifier:
				if id, ok := nodeIDs[last.Value]; ok {
					ft.Targets = []string{id}
				} else {
					ft.Targets = []string{last.Value}
				}
			case ast.NodeArrayLiteral:
				for _, item := range last.ArrVal {
					if id, ok := nodeIDs[item.Value]; ok {
						ft.Targets = append(ft.Targets, id)
					} else {
						ft.Targets = append(ft.Targets, item.Value)
					}
				}
			}
		}

		fj.Triggers = append(fj.Triggers, ft)
	}

	return fj, nil
}

// CompileProgram compiles all flows in a program.
func CompileProgram(program *ast.Node) ([]*FlowJSON, error) {
	flows := ast.CollectFlows(program)
	result := make([]*FlowJSON, 0, len(flows))
	for _, flow := range flows {
		fj, err := Compile(flow)
		if err != nil {
			return nil, fmt.Errorf("flow %q: %w", flow.Value, err)
		}
		result = append(result, fj)
	}
	return result, nil
}

// CompileAndValidate compiles and runs semantic validation in one step.
func CompileAndValidate(program *ast.Node) ([]*FlowJSON, []semantic.Diagnostic, error) {
	// Validate
	analyzer := semantic.New()
	diags := analyzer.Analyze(program)
	if analyzer.HasErrors() {
		return nil, diags, fmt.Errorf("semantic validation failed with %d errors", len(analyzer.Errors()))
	}

	// Compile
	flows, err := CompileProgram(program)
	if err != nil {
		return nil, diags, err
	}

	return flows, diags, nil
}

// compileArgs converts a SkillCall's positional + named args into a flat map.
func compileArgs(call *ast.Node) map[string]interface{} {
	args := make(map[string]interface{})

	// Positional args become numeric keys
	for i, arg := range call.Args {
		key := fmt.Sprintf("arg%d", i)
		args[key] = nodeToValue(arg)
	}

	// Named args
	for key, val := range call.Attrs {
		args[key] = nodeToValue(val)
	}

	return args
}

// nodeToValue converts an AST value node to its Go representation.
func nodeToValue(n *ast.Node) interface{} {
	if n == nil {
		return nil
	}
	switch n.Type {
	case ast.NodeStringLiteral:
		return n.Value
	case ast.NodeNumberLiteral:
		return n.NumVal
	case ast.NodeBoolLiteral:
		return n.BoolVal
	case ast.NodeIdentifier:
		return n.Value
	case ast.NodeObjectLiteral:
		m := make(map[string]interface{})
		for key, val := range n.MapVal {
			m[key] = nodeToValue(val)
		}
		return m
	case ast.NodeArrayLiteral:
		arr := make([]interface{}, len(n.ArrVal))
		for i, val := range n.ArrVal {
			arr[i] = nodeToValue(val)
		}
		return arr
	default:
		return n.Value
	}
}

// compileCondition converts a structured AST Condition node into a ConditionIR
// that the runtime can evaluate without reparsing.
func compileCondition(condNode *ast.Node) *ConditionIR {
	if condNode == nil || condNode.Type != ast.NodeCondition || len(condNode.Children) < 2 {
		return nil
	}
	lhs := condNode.Children[0]
	rhs := condNode.Children[1]
	return &ConditionIR{
		Operator: condNode.Value,
		Left:     compileConditionOp(lhs),
		Right:    compileConditionOp(rhs),
	}
}

// compileConditionOp converts an AST node into a structured ConditionOp.
func compileConditionOp(n *ast.Node) ConditionOp {
	if n == nil {
		return ConditionOp{}
	}
	switch n.Type {
	case ast.NodeIdentifier:
		// Dotted identifier like "verify.confidence" or "a.confidence"
		v := n.Value
		op := ConditionOp{Kind: "node_output", Node: v}
		if idx := strings.Index(v, "."); idx > 0 {
			op.Node = v[:idx]
			op.Path = v[idx+1:]
		}
		return op
	case ast.NodeNumberLiteral:
		return ConditionOp{Kind: "number", Value: n.NumVal}
	case ast.NodeStringLiteral:
		return ConditionOp{Kind: "string", Value: n.Value}
	case ast.NodeBoolLiteral:
		return ConditionOp{Kind: "bool", Value: n.BoolVal}
	default:
		return ConditionOp{Kind: "unknown", Value: n.Value}
	}
}

// ToJSON serializes a FlowJSON to indented JSON bytes.
func ToJSON(fj *FlowJSON) ([]byte, error) {
	return json.MarshalIndent(fj, "", "  ")
}

// ToJSONString is like ToJSON but returns a string.
func ToJSONString(fj *FlowJSON) (string, error) {
	b, err := ToJSON(fj)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
