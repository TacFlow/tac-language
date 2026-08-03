// Package ast defines the Abstract Syntax Tree node types for the TAC language.
//
// TAC — The TacFlow Agentic Code. A DSL for autonomous AI agents.
// (c) 2026 TacFlow — MIT License
package ast

import (
	"encoding/json"
	"fmt"
)

// NodeType identifies the kind of AST node.
type NodeType string

// AST node type constants. These strings are part of the stable AST JSON
// contract: the canonical formatter, golden tests and AST hashing all depend
// on them. Do not rename without bumping the LANGUAGE_VERSION.
const (
	NodeProgram       NodeType = "Program"
	NodeFlow          NodeType = "Flow"
	NodeNode          NodeType = "Node"
	NodeSkillCall     NodeType = "SkillCall"
	NodeRememberStmt  NodeType = "RememberStmt"
	NodeRecallStmt    NodeType = "RecallStmt"
	NodeForgetStmt    NodeType = "ForgetStmt"
	NodeRelateStmt    NodeType = "RelateStmt"
	NodeEdge          NodeType = "Edge"
	NodeCondition     NodeType = "Condition"
	NodeTrigger       NodeType = "Trigger"
	NodeContextBlock  NodeType = "ContextBlock"
	NodeAutoSummarize NodeType = "AutoSummarize"
	NodeInput         NodeType = "Input"
	NodeAgentDecl     NodeType = "AgentDecl"
	NodeIdentifier    NodeType = "Identifier"
	NodeStringLiteral NodeType = "StringLiteral"
	NodeNumberLiteral NodeType = "NumberLiteral"
	NodeBoolLiteral   NodeType = "BoolLiteral"
	NodeObjectLiteral NodeType = "ObjectLiteral"
	NodeArrayLiteral  NodeType = "ArrayLiteral"
	NodeKeyValue      NodeType = "KeyValue"
	NodeNamedArg      NodeType = "NamedArg"
)

// Position represents a source location (1-based line and column).
type Position struct {
	Line int `json:"line"`
	Col  int `json:"col"`
}

// IsZero reports whether the position is unset (0,0).
func (p Position) IsZero() bool { return p.Line == 0 && p.Col == 0 }

// String renders the position as "line:col".
func (p Position) String() string { return fmt.Sprintf("%d:%d", p.Line, p.Col) }

// Node is a generic AST node. The JSON serialization is the stable contract
// consumed by the compiler stage (agent skill tac_compile) and by the TacFlow
// runtime. Fields are omitted when empty so golden files stay compact.
type Node struct {
	Type     NodeType         `json:"type"`
	Pos      Position         `json:"pos"`
	Children []*Node          `json:"children,omitempty"`
	Value    string           `json:"value,omitempty"`
	NumVal   float64          `json:"num_val,omitempty"`
	BoolVal  bool             `json:"bool_val,omitempty"`
	Nodes    []*Node          `json:"nodes,omitempty"`    // For flow/context bodies
	Edges    []*Node          `json:"edges,omitempty"`    // For flow edges
	Args     []*Node          `json:"args,omitempty"`     // For skill calls
	Attrs    map[string]*Node `json:"attrs,omitempty"`    // For blocks with named attributes
	MapVal   map[string]*Node `json:"map_val,omitempty"`  // For object literals
	ArrVal   []*Node          `json:"arr_val,omitempty"`  // For array literals
}

// NewNode creates a new AST node with the given type and source position.
func NewNode(typ NodeType, line, col int) *Node {
	return &Node{
		Type:     typ,
		Pos:      Position{Line: line, Col: col},
		Children: make([]*Node, 0),
	}
}

// MarshalJSON strips zero-valued positions so serialized output is stable
// across programmatic AST construction (used by the canonical formatter and
// golden tests).
func (n *Node) MarshalJSON() ([]byte, error) {
	type alias Node
	aux := &struct {
		*alias
		Pos *Position `json:"pos,omitempty"`
	}{
		alias: (*alias)(n),
	}
	if !n.Pos.IsZero() {
		aux.Pos = &n.Pos
	}
	return json.Marshal(aux)
}

// Walk visits every node in the tree in pre-order, invoking fn on each node.
// If fn returns false the children of that node are skipped.
func Walk(n *Node, fn func(*Node) bool) {
	if n == nil {
		return
	}
	if !fn(n) {
		return
	}
	for _, c := range n.Children {
		Walk(c, fn)
	}
	for _, c := range n.Nodes {
		Walk(c, fn)
	}
	for _, c := range n.Edges {
		Walk(c, fn)
	}
	for _, c := range n.Args {
		Walk(c, fn)
	}
	for _, c := range n.ArrVal {
		Walk(c, fn)
	}
	for _, kv := range n.MapVal {
		Walk(kv, fn)
	}
	for _, kv := range n.Attrs {
		Walk(kv, fn)
	}
}

// Find returns the first node matching predicate fn in pre-order, or nil.
func Find(n *Node, fn func(*Node) bool) *Node {
	var found *Node
	Walk(n, func(cur *Node) bool {
		if found != nil {
			return false
		}
		if fn(cur) {
			found = cur
			return false
		}
		return true
	})
	return found
}

// Collect returns all nodes matching predicate fn in pre-order.
func Collect(n *Node, fn func(*Node) bool) []*Node {
	var out []*Node
	Walk(n, func(cur *Node) bool {
		if fn(cur) {
			out = append(out, cur)
		}
		return true
	})
	return out
}
