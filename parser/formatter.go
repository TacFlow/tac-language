// formatter.go — TAC Canonical Formatter
//
// Produces a deterministic, canonical representation of any valid TAC source.
// The canonical form:
//   - Normalizes whitespace (2-space indent, consistent newlines)
//   - Alphabetically sorts attributes within blocks
//   - Normalizes string quoting (always double quotes)
//   - Strips comments (canonical form is executable-only)
//   - Produces a stable output for the same semantics
//
// Usage: the formatter is used for:
//   - Golden tests (round-trip: parse → format → parse yields identical AST hash)
//   - Before computing AST hash (stripping comments + normalizing whitespace)
//   - Code review / CI enforcement
//
// (c) 2026 TacFlow — MIT License

package main

import (
	"fmt"
	"sort"
	"strings"
)

// FormatCanonical converts a parsed AST to its canonical string form.
func FormatCanonical(program *Node) string {
	var buf strings.Builder

	for _, node := range program.Nodes {
		formatNode(&buf, node, 0)
	}

	return buf.String()
}

func formatNode(buf *strings.Builder, node *Node, indent int) {
	if node == nil {
		return
	}

	switch node.Type {
	case NodeFlow:
		formatFlow(buf, node, indent)
	case NodeContextBlock:
		formatContextBlock(buf, node, indent)
	case NodeRememberStmt:
		formatRemember(buf, node, indent)
	case NodeRecallStmt:
		formatRecall(buf, node, indent)
	case NodeForgetStmt:
		formatForget(buf, node, indent)
	case NodeRelateStmt:
		formatRelate(buf, node, indent)
	case NodeAutoSummarize:
		formatAutoSummarize(buf, node, indent)
	case NodeInput:
		// Inputs are part of flow bodies, handled there
	default:
		// Skip unknown top-level nodes silently
	}
}

func writeIndent(buf *strings.Builder, indent int) {
	for i := 0; i < indent; i++ {
		buf.WriteByte(' ')
	}
}

func formatFlow(buf *strings.Builder, flow *Node, indent int) {
	writeIndent(buf, indent)
	fmt.Fprintf(buf, "flow %q {\n", flow.Value)

	// Inputs
	for _, c := range flow.Children {
		if c.Type == NodeInput {
			writeIndent(buf, indent+2)
			buf.WriteString("input ")
			for i, ch := range c.Children {
				if i > 0 {
					buf.WriteString(": ")
				}
				buf.WriteString(ch.Value)
			}
			buf.WriteByte('\n')
		}
	}

	// Agent declarations
	for _, c := range flow.Children {
		if c.Type == NodeAgentDecl {
			writeIndent(buf, indent+2)
			buf.WriteString("agent ")
			if len(c.Children) > 0 {
				formatValue(buf, c.Children[0])
			}
			if len(c.Attrs) > 0 {
				buf.WriteString(" ")
				formatAttrs(buf, c.Attrs, indent+2)
				buf.WriteByte('\n')
			} else {
				buf.WriteByte('\n')
			}
		}
	}

	// Nodes
	for _, n := range flow.Nodes {
		if n.Type == NodeNode {
			writeIndent(buf, indent+2)
			buf.WriteString("node ")
			formatValue(buf, &Node{Type: NodeStringLiteral, Value: n.Value})
			buf.WriteString(" -> ")
			for _, c := range n.Children {
				if c.Type == NodeSkillCall {
					formatSkillCall(buf, c)
				}
			}
			if n.Attrs != nil && len(n.Attrs) > 0 {
				buf.WriteByte(' ')
				formatAttrs(buf, n.Attrs, indent+2)
			}
			buf.WriteByte('\n')
		}
	}

	// Edges
	if len(flow.Edges) > 0 {
		buf.WriteByte('\n')
		for _, e := range flow.Edges {
			writeIndent(buf, indent+2)
			for i, c := range e.Children {
				if i > 0 {
					buf.WriteString(" -> ")
				}
				if c.Type == NodeArrayLiteral {
					formatArrayValue(buf, c)
				} else {
					buf.WriteString(c.Value)
				}
			}
			if e.Attrs != nil && len(e.Attrs) > 0 {
				buf.WriteByte(' ')
				formatAttrs(buf, e.Attrs, indent+2)
			}
			buf.WriteByte('\n')
		}
	}

	// Triggers
	for _, c := range flow.Children {
		if c.Type == NodeTrigger {
			buf.WriteByte('\n')
			writeIndent(buf, indent+2)
			buf.WriteString("on ")
			for _, ch := range c.Children {
				if ch.Type == NodeStringLiteral {
					formatValue(buf, ch)
				} else if ch.Type == NodeIdentifier {
					buf.WriteString(ch.Value)
				} else if ch.Type == NodeArrayLiteral {
					formatArrayValue(buf, ch)
				}
			}
			if c.Attrs != nil && len(c.Attrs) > 0 {
				buf.WriteByte(' ')
				formatAttrs(buf, c.Attrs, indent+2)
			}
			buf.WriteByte('\n')
		}
	}

	writeIndent(buf, indent)
	buf.WriteString("}\n")
}

func formatContextBlock(buf *strings.Builder, ctx *Node, indent int) {
	writeIndent(buf, indent)
	buf.WriteString("context ")
	for _, c := range ctx.Children {
		if c.Type == NodeStringLiteral {
			formatValue(buf, c)
		}
	}
	buf.WriteString(" {\n")
	for _, c := range ctx.Children {
		if c.Type == NodeRememberStmt {
			formatRemember(buf, c, indent+2)
		} else if c.Type == NodeRecallStmt {
			formatRecall(buf, c, indent+2)
		} else if c.Type == NodeFlow {
			formatFlow(buf, c, indent+2)
		}
	}
	writeIndent(buf, indent)
	buf.WriteString("}\n")
}

func formatRemember(buf *strings.Builder, node *Node, indent int) {
	writeIndent(buf, indent)
	buf.WriteString("remember ")
	for i, c := range node.Children {
		if i == 0 {
			buf.WriteString(c.Value)
		} else if i == 1 {
			buf.WriteString(" = ")
			formatValue(buf, c)
		}
	}
	if len(node.Attrs) > 0 {
		buf.WriteByte(' ')
		formatAttrs(buf, node.Attrs, indent)
	}
	buf.WriteByte('\n')
}

func formatRecall(buf *strings.Builder, node *Node, indent int) {
	writeIndent(buf, indent)
	buf.WriteString("recall ")
	for _, c := range node.Children {
		buf.WriteString(c.Value)
	}
	if len(node.Attrs) > 0 {
		buf.WriteByte(' ')
		formatAttrs(buf, node.Attrs, indent)
	}
	buf.WriteByte('\n')
}

func formatForget(buf *strings.Builder, node *Node, indent int) {
	writeIndent(buf, indent)
	buf.WriteString("forget ")
	for _, c := range node.Children {
		buf.WriteString(c.Value)
	}
	if len(node.Attrs) > 0 {
		buf.WriteByte(' ')
		formatAttrs(buf, node.Attrs, indent)
	}
	buf.WriteByte('\n')
}

func formatRelate(buf *strings.Builder, node *Node, indent int) {
	writeIndent(buf, indent)
	buf.WriteString("relate ")
	for i, c := range node.Children {
		if i == 0 {
			buf.WriteString(c.Value)
		} else if i == 1 {
			buf.WriteString(" -> ")
			buf.WriteString(c.Value)
		}
	}
	if len(node.Attrs) > 0 {
		buf.WriteByte(' ')
		formatAttrs(buf, node.Attrs, indent)
	}
	buf.WriteByte('\n')
}

func formatAutoSummarize(buf *strings.Builder, node *Node, indent int) {
	writeIndent(buf, indent)
	buf.WriteString("auto_summarize(")
	if node.Attrs != nil {
		keys := make([]string, 0, len(node.Attrs))
		for k := range node.Attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(k)
			buf.WriteString(": ")
			formatValue(buf, node.Attrs[k])
		}
	}
	buf.WriteString(")\n")
}

func formatSkillCall(buf *strings.Builder, call *Node) {
	buf.WriteString(call.Value)
	if len(call.Args) > 0 {
		buf.WriteByte('(')
		for i, a := range call.Args {
			if i > 0 {
				buf.WriteString(", ")
			}
			formatValue(buf, a)
		}
		buf.WriteByte(')')
	} else if len(call.Attrs) > 0 {
		buf.WriteByte('(')
		formatArgList(buf, call.Attrs)
		buf.WriteByte(')')
	}
}

func formatArgList(buf *strings.Builder, attrs map[string]*Node) {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(k)
		buf.WriteString(": ")
		formatValue(buf, attrs[k])
	}
}

func formatAttrs(buf *strings.Builder, attrs map[string]*Node, indent int) {
	buf.WriteString("{\n")
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		writeIndent(buf, indent+4)
		buf.WriteString(k)
		buf.WriteString(": ")
		formatValue(buf, attrs[k])
		buf.WriteString(",\n")
	}
	writeIndent(buf, indent+2)
	buf.WriteByte('}')
}

func formatValue(buf *strings.Builder, node *Node) {
	switch node.Type {
	case NodeStringLiteral:
		fmt.Fprintf(buf, "%q", node.Value)
	case NodeNumberLiteral:
		buf.WriteString(node.Value)
	case NodeBoolLiteral:
		buf.WriteString(node.Value)
	case NodeIdentifier:
		buf.WriteString(node.Value)
	case NodeObjectLiteral:
		buf.WriteByte('{')
		keys := make([]string, 0, len(node.MapVal))
		for k := range node.MapVal {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(k)
			buf.WriteString(": ")
			formatValue(buf, node.MapVal[k])
		}
		buf.WriteByte('}')
	case NodeArrayLiteral:
		formatArrayValue(buf, node)
	default:
		buf.WriteString(node.Value)
	}
}

func formatArrayValue(buf *strings.Builder, node *Node) {
	buf.WriteByte('[')
	for i, v := range node.ArrVal {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(v.Value)
	}
	buf.WriteByte(']')
}
