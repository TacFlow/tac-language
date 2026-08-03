// Package ast defines the Abstract Syntax Tree types for the TAC Language.
//
// TAC is The TacFlow Agentic Code — a DSL for autonomous AI agents.
package ast

import "fmt"

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
