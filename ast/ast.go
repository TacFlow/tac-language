// Package ast defines the Abstract Syntax Tree types for the TAC Language.
//
// TAC is The TacFlow Agentic Code — a DSL for autonomous AI agents.
package ast

import (
	"fmt"
	"strings"
)

// NodeType identifies the kind of AST node.
type NodeType string

const (
	NodeProgram        NodeType = "Program"
	NodeFlow           NodeType = "Flow"
	NodeNode           NodeType = "Node"
	NodeSkillCall      NodeType = "SkillCall"
	NodeRememberStmt   NodeType = "RememberStmt"
	NodeRecallStmt     NodeType = "RecallStmt"
	NodeForgetStmt     NodeType = "ForgetStmt"
	NodeRelateStmt     NodeType = "RelateStmt"
	NodeEdge           NodeType = "Edge"
	NodeCondition      NodeType = "Condition"
	NodeTrigger        NodeType = "Trigger"
	NodeContextBlock   NodeType = "ContextBlock"
	NodeAutoSummarize  NodeType = "AutoSummarize"
	NodeInput          NodeType = "Input"
	NodeAgentDecl      NodeType = "AgentDecl"
	NodeIdentifier     NodeType = "Identifier"
	NodeStringLiteral  NodeType = "StringLiteral"
	NodeNumberLiteral  NodeType = "NumberLiteral"
	NodeBoolLiteral    NodeType = "BoolLiteral"
	NodeObjectLiteral  NodeType = "ObjectLiteral"
	NodeArrayLiteral   NodeType = "ArrayLiteral"
	NodeKeyValue       NodeType = "KeyValue"
	NodeNamedArg       NodeType = "NamedArg"

	// v0.5 (LAYA). Nothing in a v0.4 source produces these, so v0.4 ASTs
	// (and the testdata/*.golden.json files) are unchanged.
	NodeRequires     NodeType = "Requires"     // requires "0.5" (Value = version)
	NodeSchedule     NodeType = "Schedule"     // schedule "<cron>" [tz "<IANA>"]: Children[0] cron, Attrs["tz"]
	NodeRange        NodeType = "Range"        // gate[<10] / gate[10..50]: Value = op, Children = bounds
	NodeTaskDecl     NodeType = "TaskDecl"     // task "name" { question… profile {…} }
	NodeModelDecl    NodeType = "ModelDecl"    // model "name" { tasks […] base "…" port N }
	NodeEpisodeDecl  NodeType = "EpisodeDecl"  // episode "id" { … }
	NodeDatasetDecl  NodeType = "DatasetDecl"  // dataset "name" { task "…" … }
	NodeQuestion     NodeType = "Question"     // question id: type "instructions" [ {attrs} ] [ {options} ]
	NodeTarget       NodeType = "Target"       // target id = value
	NodeUnrecognized NodeType = "Unrecognized" // a form the parser skipped (TAC-PARSE-001)
	NodeGluedNumber  NodeType = "GluedNumber"  // `3x`: a number glued to text, read as in v0.4 (TAC-PARSE-001)
)

// Pseudo-labels every gate understands besides the task's own labels.
const (
	LabelLowConfidence = "low_confidence"
	LabelError         = "error"
	LabelAny           = "*"
)

// Position represents a source location.
type Position struct {
	Line int `json:"line"`
	Col  int `json:"col"`
}

// Node is a generic AST node.
type Node struct {
	Type     NodeType          `json:"type"`
	Pos      Position          `json:"pos"`
	Children []*Node           `json:"children,omitempty"`
	Value    string            `json:"value,omitempty"`
	NumVal   float64           `json:"num_val,omitempty"`
	BoolVal  bool              `json:"bool_val,omitempty"`
	Version  string            `json:"version,omitempty"` // Optional @ "x.y.z" on skill calls
	Nodes    []*Node           `json:"nodes,omitempty"`   // For flow/context bodies
	Edges    []*Node           `json:"edges,omitempty"`   // For flow edges
	Args     []*Node           `json:"args,omitempty"`    // For skill calls
	Attrs    map[string]*Node  `json:"attrs,omitempty"`   // For blocks with named attributes
	MapVal   map[string]*Node  `json:"map_val,omitempty"` // For object literals
	ArrVal   []*Node           `json:"arr_val,omitempty"` // For array literals
}

// NewNode creates a new AST node.
func NewNode(typ NodeType, line, col int) *Node {
	return &Node{
		Type:     typ,
		Pos:      Position{Line: line, Col: col},
		Children: make([]*Node, 0),
	}
}

// Walk traverses the AST depth-first, calling fn for each node.
// If fn returns false, traversal stops.
func Walk(root *Node, fn func(*Node, int) bool) {
	walk(root, fn, 0)
}

func walk(n *Node, fn func(*Node, int) bool, depth int) {
	if n == nil {
		return
	}
	if !fn(n, depth) {
		return
	}
	for _, child := range n.Children {
		walk(child, fn, depth+1)
	}
	for _, node := range n.Nodes {
		walk(node, fn, depth+1)
	}
	for _, edge := range n.Edges {
		walk(edge, fn, depth+1)
	}
	for _, arg := range n.Args {
		walk(arg, fn, depth+1)
	}
	for _, attr := range n.Attrs {
		walk(attr, fn, depth+1)
	}
	for _, mapVal := range n.MapVal {
		walk(mapVal, fn, depth+1)
	}
	for _, arrVal := range n.ArrVal {
		walk(arrVal, fn, depth+1)
	}
}

// CollectFlows gathers all top-level Flow nodes from a Program.
func CollectFlows(program *Node) []*Node {
	if program == nil || program.Type != NodeProgram {
		return nil
	}
	var flows []*Node
	for _, n := range program.Nodes {
		if n.Type == NodeFlow {
			flows = append(flows, n)
		}
	}
	return flows
}

// CollectNodes gathers all Node definitions within a Flow.
func CollectNodes(flow *Node) []*Node {
	if flow == nil || flow.Type != NodeFlow {
		return nil
	}
	return flow.Nodes
}

// CollectEdges gathers all Edge definitions within a Flow.
func CollectEdges(flow *Node) []*Node {
	if flow == nil || flow.Type != NodeFlow {
		return nil
	}
	return flow.Edges
}

// NodeName extracts the name from a Node definition.
func NodeName(n *Node) string {
	if n == nil {
		return ""
	}
	if n.Value != "" {
		return n.Value
	}
	if n.Type == NodeNode && len(n.Children) > 0 {
		return n.Children[0].Value
	}
	return ""
}

// EdgeSource returns the source node name of an Edge.
func EdgeSource(e *Node) string {
	if e == nil || e.Type != NodeEdge || len(e.Children) < 1 {
		return ""
	}
	return e.Children[0].Value
}

// EdgeTarget returns the target node name of an Edge.
func EdgeTarget(e *Node) string {
	if e == nil || e.Type != NodeEdge || len(e.Children) < 2 {
		return ""
	}
	return e.Children[1].Value
}

// EdgeCondition returns the condition of a conditional edge, if any.
// It handles both simple string conditions (legacy) and structured
// Condition nodes from expression parsing.
func EdgeCondition(e *Node) (condition string, fallback string, hasCondition bool) {
	if e == nil || e.Type != NodeEdge || e.Attrs == nil {
		return "", "", false
	}
	if cond, ok := e.Attrs["if"]; ok && cond != nil {
		hasCondition = true
		if cond.Type == NodeCondition {
			// Structured expression: reconstruct from children
			condition = conditionToString(cond)
		} else {
			condition = cond.Value
		}
	}
	if fb, ok := e.Attrs["else"]; ok && fb != nil {
		fallback = fb.Value
	}
	return
}

// ConditionToString reconstructs a condition expression string from a Condition node.
// The first child is LHS, second child is RHS, and node.Value is the operator.
func ConditionToString(n *Node) string {
	if n == nil {
		return ""
	}
	var lhs, rhs string
	if len(n.Children) >= 1 && n.Children[0] != nil {
		lhs = n.Children[0].Value
	}
	if len(n.Children) >= 2 && n.Children[1] != nil {
		rhs = n.Children[1].Value
	}
	op := n.Value
	if lhs == "" {
		return op
	}
	if rhs == "" {
		return lhs + " " + op
	}
	return lhs + " " + op + " " + rhs
}

// conditionToString is the internal helper used by EdgeCondition.
func conditionToString(n *Node) string {
	return ConditionToString(n)
}

// ValidateNode performs basic validation of an AST node structure.
func ValidateNode(n *Node) error {
	if n == nil {
		return fmt.Errorf("nil node")
	}
	if n.Type == "" {
		return fmt.Errorf("node at %d:%d has no type", n.Pos.Line, n.Pos.Col)
	}
	return nil
}

// LabelAttr is the Attrs key under which a labelled edge stores its branch.
// It is not a valid identifier, so `a -> b { label: x }` cannot forge it.
const LabelAttr = "[label]"

// EdgeLabel returns the branch of a labelled edge (`gate[branch] -> x`), or
// nil for an ordinary edge.
func EdgeLabel(e *Node) *Node {
	if e == nil || e.Type != NodeEdge || e.Attrs == nil {
		return nil
	}
	return e.Attrs[LabelAttr]
}

// LabelString is the canonical text of a branch, shared by the IR, the
// conformance corpus and the platform dialect (flow_edges.source_handle
// "laya:" + LabelString):
//
//	proceed / "texto" -> the text      true / false -> "true" / "false"
//	*                 -> "*"           <10 / >=50   -> "range:<10" / "range:>=50"
//	10..50            -> "range:10..50"
//
// Numbers keep their source literal.
func LabelString(b *Node) string {
	if b == nil {
		return ""
	}
	switch b.Type {
	case NodeRange:
		if b.Value == ".." && len(b.Children) == 2 {
			return "range:" + b.Children[0].Value + ".." + b.Children[1].Value
		}
		if len(b.Children) == 1 {
			return "range:" + b.Value + b.Children[0].Value
		}
		return "range:"
	case NodeBoolLiteral:
		if b.BoolVal {
			return "true"
		}
		return "false"
	default:
		return b.Value
	}
}

// JSONNumber returns a number literal as valid JSON number text. The lexer
// accepts leading zeros (`007`, `-00.5`, `0080`), which JSON forbids; they
// are dropped (`7`, `-0.5`, `80`), as v0.4 did when it read the value as a
// float. Every other spelling (`1.50`, `1e3`) is kept as written.
func JSONNumber(lit string) string {
	sign := ""
	if strings.HasPrefix(lit, "-") {
		sign, lit = "-", lit[1:]
	}
	i := 0
	for i+1 < len(lit) && lit[i] == '0' && lit[i+1] >= '0' && lit[i+1] <= '9' {
		i++
	}
	return sign + lit[i:]
}
