// Package formatter implements a code formatter for TAC Language.
//
// The formatter takes a parsed AST and emits canonical, consistently
// indented .tac source code. It normalizes:
//   - Indentation (tabs or spaces)
//   - Line breaks
//   - Braces placement
//   - Object/array literal formatting
//   - Skill call argument ordering
package formatter

import (
	"fmt"
	"strings"

	"github.com/tacflow1-tech/tac-language/ast"
)

// Options controls formatter behavior.
type Options struct {
	UseTabs    bool // Use tabs instead of spaces (default: false, uses 4 spaces)
	IndentSize int  // Number of spaces per indent level (default: 4)
}

func defaultOptions() Options {
	return Options{
		UseTabs:    false,
		IndentSize: 4,
	}
}

// Format converts an AST Program node back to canonical .tac source text.
func Format(program *ast.Node) string {
	return FormatWithOptions(program, defaultOptions())
}

// FormatWithOptions formats using custom options.
func FormatWithOptions(program *ast.Node, opts Options) string {
	if program == nil {
		return ""
	}
	f := &fmtWriter{opts: opts, buf: new(strings.Builder)}
	f.writeProgram(program)
	return f.buf.String()
}

type fmtWriter struct {
	opts  Options
	buf   *strings.Builder
	depth int
}

func (w *fmtWriter) indent() string {
	if w.opts.UseTabs {
		return strings.Repeat("\t", w.depth)
	}
	return strings.Repeat(" ", w.depth*w.opts.IndentSize)
}

func (w *fmtWriter) write(format string, args ...interface{}) {
	if len(args) == 0 {
		w.buf.WriteString(format)
		return
	}
	w.buf.WriteString(fmt.Sprintf(format, args...))
}

func (w *fmtWriter) raw(s string) {
	w.buf.WriteString(s)
}

func (w *fmtWriter) writeln(format string, args ...interface{}) {
	w.buf.WriteString(w.indent())
	w.buf.WriteString(fmt.Sprintf(format, args...))
	w.buf.WriteByte('\n')
}

func (w *fmtWriter) writeValue(n *ast.Node) {
	switch n.Type {
	case ast.NodeStringLiteral:
		w.write("%q", n.Value)
	case ast.NodeNumberLiteral:
		w.write("%s", n.Value)
	case ast.NodeBoolLiteral:
		if n.BoolVal {
			w.write("true")
		} else {
			w.write("false")
		}
	case ast.NodeObjectLiteral:
		w.writeObjectLiteral(n)
	case ast.NodeArrayLiteral:
		w.writeArrayLiteral(n)
	case ast.NodeIdentifier:
		w.write("%s", n.Value)
	}
}

func (w *fmtWriter) writeObjectLiteral(n *ast.Node) {
	if len(n.MapVal) == 0 {
		w.write("{}")
		return
	}
	w.write("{\n")
	w.depth++
	for key, val := range n.MapVal {
		w.writeln("%s: ", key)
		w.depth++
		w.writeln("%s", nodeValueStr(val))
		w.depth--
	}
	w.depth--
	w.write("%s}", w.indent())
}

func (w *fmtWriter) writeArrayLiteral(n *ast.Node) {
	if len(n.ArrVal) == 0 {
		w.write("[]")
		return
	}
	parts := make([]string, len(n.ArrVal))
	for i, item := range n.ArrVal {
		parts[i] = nodeValueStr(item)
	}
	w.write("[%s]", strings.Join(parts, ", "))
}

func (w *fmtWriter) writeAttrs(attrs map[string]*ast.Node) {
	if len(attrs) == 0 {
		return
	}
	w.write(" {\n")
	w.depth++
	for key, val := range attrs {
		w.writeln("%s: %s", key, nodeValueStr(val))
	}
	w.depth--
	w.write("%s}", w.indent())
}

func (w *fmtWriter) writeSkillCall(n *ast.Node) {
	w.write("%s(", n.Value)
	if len(n.Args) > 0 {
		parts := make([]string, len(n.Args))
		for i, arg := range n.Args {
			parts[i] = nodeValueStr(arg)
		}
		w.raw(strings.Join(parts, ", "))
	}
	w.raw(")")
	if len(n.Attrs) > 0 {
		w.writeAttrs(n.Attrs)
	}
}

func (w *fmtWriter) writeNodeDef(n *ast.Node) {
	nodeName := ast.NodeName(n)
	w.write("%snode %q -> ", w.indent(), nodeName)
	if len(n.Children) > 1 {
		call := n.Children[1]
		if call.Type == ast.NodeSkillCall {
			w.write("skill ")
			w.writeSkillCall(call)
		} else {
			w.writeValue(call)
		}
	}
	if len(n.Nodes) > 0 {
		w.write(" {\n")
		w.depth++
		for _, sub := range n.Nodes {
			if sub.Type == ast.NodeSkillCall {
				w.writeln("skill %s", sub.Value)
				if sub.Attrs != nil {
					w.writeAttrs(sub.Attrs)
					w.write("\n")
				}
			}
		}
		w.depth--
		w.write("%s}", w.indent())
	}
	if n.Attrs != nil {
		if cond, ok := n.Attrs["if"]; ok {
			w.write(" {\n")
			w.depth++
			condStr := nodeValueStr(cond)
			if cond.Type == ast.NodeCondition {
				condStr = ast.ConditionToString(cond)
			}
			w.writeln("if: %s", condStr)
			if fb, ok := n.Attrs["else"]; ok {
				w.writeln("else: %s", nodeValueStr(fb))
			}
			w.depth--
			w.write("%s}", w.indent())
		}
	}
	w.write("\n")
}

func (w *fmtWriter) writeEdge(e *ast.Node) {
	src := ast.EdgeSource(e)
	tgt := ast.EdgeTarget(e)
	w.write("%s%s -> %s", w.indent(), src, tgt)
	if e.Attrs != nil {
		if cond, ok := e.Attrs["if"]; ok {
			w.write(" {\n")
			w.depth++
			condStr := nodeValueStr(cond)
			if cond.Type == ast.NodeCondition {
				condStr = ast.ConditionToString(cond)
			}
			w.writeln("if: %s", condStr)
			if fb, ok := e.Attrs["else"]; ok {
				w.writeln("else: %s", nodeValueStr(fb))
			}
			w.depth--
			w.write("%s}", w.indent())
		}
	}
	w.write("\n")
}

func (w *fmtWriter) writeTrigger(n *ast.Node) {
	eventName := ""
	if len(n.Children) > 0 && n.Children[0].Type == ast.NodeStringLiteral {
		eventName = n.Children[0].Value
	}
	w.write("%son %q", w.indent(), eventName)
	if n.Attrs != nil {
		w.write(" {\n")
		w.depth++
		for key, val := range n.Attrs {
			w.writeln("%s: %s", key, nodeValueStr(val))
		}
		w.depth--
		w.write("%s}", w.indent())
	}
	if len(n.Children) > 1 {
		last := n.Children[len(n.Children)-1]
		switch last.Type {
		case ast.NodeIdentifier:
			w.write(" -> %s", last.Value)
		case ast.NodeArrayLiteral:
			parts := make([]string, len(last.ArrVal))
			for i, item := range last.ArrVal {
				parts[i] = item.Value
			}
			w.write(" -> [%s]", strings.Join(parts, ", "))
		}
	}
	w.write("\n")
}

func (w *fmtWriter) writeFlow(flow *ast.Node) {
	flowName := flow.Value
	w.writeln("flow %q {", flowName)
	w.depth++

	// Inputs
	for _, child := range flow.Children {
		if child.Type == ast.NodeInput {
			if len(child.Children) >= 1 {
				name := child.Children[0].Value
				tt := "Untrusted"
				if len(child.Children) >= 2 {
					tt = child.Children[1].Value
				}
				w.writeln("input %s: %s", name, tt)
			}
		}
	}

	// Agent declarations
	for _, child := range flow.Children {
		if child.Type == ast.NodeAgentDecl {
			agName := ""
			if len(child.Children) >= 1 {
				agName = child.Children[0].Value
			}
			w.writeln("agent %q {", agName)
			if child.Attrs != nil {
				w.depth++
				for key, val := range child.Attrs {
					w.writeln("%s: %s", key, nodeValueStr(val))
				}
				w.depth--
			}
			w.writeln("}")
		}
	}

	// Nodes
	for _, node := range flow.Nodes {
		w.writeNodeDef(node)
	}

	// Edges
	for _, edge := range flow.Edges {
		w.writeEdge(edge)
	}

	// Triggers
	for _, child := range flow.Children {
		if child.Type == ast.NodeTrigger {
			w.writeTrigger(child)
		}
	}

	w.depth--
	w.writeln("}")
}

func (w *fmtWriter) writeContext(ctx *ast.Node) {
	ctxName := ""
	if len(ctx.Children) > 0 && ctx.Children[0].Type == ast.NodeStringLiteral {
		ctxName = ctx.Children[0].Value
	}
	w.writeln("context %q {", ctxName)
	w.depth++
	for _, child := range ctx.Children {
		switch child.Type {
		case ast.NodeRememberStmt:
			w.writeRemember(child)
		case ast.NodeRecallStmt:
			w.writeRecall(child)
		case ast.NodeFlow:
			w.writeFlow(child)
		}
	}
	w.depth--
	w.writeln("}")
}

func (w *fmtWriter) writeRemember(n *ast.Node) {
	name := ""
	if len(n.Children) > 0 {
		name = n.Children[0].Value
	}
	w.write("%sremember %s = ", w.indent(), name)
	if len(n.Children) > 1 {
		w.writeValue(n.Children[1])
	}
	if n.Attrs != nil {
		w.write(" {\n")
		w.depth++
		for key, val := range n.Attrs {
			w.writeln("%s: %s", key, nodeValueStr(val))
		}
		w.depth--
		w.write("%s}", w.indent())
	}
	w.write("\n")
}

func (w *fmtWriter) writeRecall(n *ast.Node) {
	name := ""
	if len(n.Children) > 0 {
		name = n.Children[0].Value
	}
	w.write("%srecall %s", w.indent(), name)
	if n.Attrs != nil {
		w.write(" {\n")
		w.depth++
		for key, val := range n.Attrs {
			w.writeln("%s: %s", key, nodeValueStr(val))
		}
		w.depth--
		w.write("%s}", w.indent())
	}
	w.write("\n")
}

func (w *fmtWriter) writeRelate(n *ast.Node) {
	src := ""
	tgt := ""
	if len(n.Children) > 0 {
		src = n.Children[0].Value
	}
	if len(n.Children) > 1 {
		tgt = n.Children[1].Value
	}
	w.writeln("relate %s -> %s {", src, tgt)
	if n.Attrs != nil {
		w.depth++
		for key, val := range n.Attrs {
			w.writeln("%s: %s", key, nodeValueStr(val))
		}
		w.depth--
	}
	w.writeln("}")
}

func (w *fmtWriter) writeAutoSummarize(n *ast.Node) {
	w.write("%sauto_summarize(", w.indent())
	if n.Attrs != nil {
		parts := make([]string, 0, len(n.Attrs))
		for key, val := range n.Attrs {
			parts = append(parts, fmt.Sprintf("%s: %s", key, nodeValueStr(val)))
		}
		w.raw(strings.Join(parts, ", "))
	}
	w.raw(")\n")
}

func (w *fmtWriter) writeProgram(program *ast.Node) {
	for _, n := range program.Nodes {
		switch n.Type {
		case ast.NodeFlow:
			w.writeFlow(n)
		case ast.NodeContextBlock:
			w.writeContext(n)
		case ast.NodeRememberStmt:
			w.writeRemember(n)
		case ast.NodeRecallStmt:
			w.writeRecall(n)
		case ast.NodeRelateStmt:
			w.writeRelate(n)
		case ast.NodeForgetStmt:
			name := ""
			if len(n.Children) > 0 {
				name = n.Children[0].Value
			}
			w.writeln("forget %s", name)
		case ast.NodeAutoSummarize:
			w.writeAutoSummarize(n)
		}
		w.write("\n")
	}
}

// nodeValueStr converts an AST value node to its string representation.
func nodeValueStr(n *ast.Node) string {
	if n == nil {
		return ""
	}
	switch n.Type {
	case ast.NodeStringLiteral:
		return fmt.Sprintf("%q", n.Value)
	case ast.NodeNumberLiteral:
		return n.Value
	case ast.NodeBoolLiteral:
		if n.BoolVal {
			return "true"
		}
		return "false"
	case ast.NodeIdentifier:
		return n.Value
	case ast.NodeNamedArg:
		if len(n.Children) > 0 {
			return fmt.Sprintf("%s: %s", n.Value, nodeValueStr(n.Children[0]))
		}
		return fmt.Sprintf("%s: ?", n.Value)
	case ast.NodeCondition:
		if len(n.Children) >= 2 {
			return fmt.Sprintf("%s %s %s", nodeValueStr(n.Children[0]), n.Value, nodeValueStr(n.Children[1]))
		}
		return n.Value
	case ast.NodeObjectLiteral:
		parts := make([]string, 0, len(n.MapVal))
		for key, val := range n.MapVal {
			parts = append(parts, fmt.Sprintf("%s: %s", key, nodeValueStr(val)))
		}
		return fmt.Sprintf("{%s}", strings.Join(parts, ", "))
	case ast.NodeArrayLiteral:
		parts := make([]string, len(n.ArrVal))
		for i, item := range n.ArrVal {
			parts[i] = nodeValueStr(item)
		}
		return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
	default:
		return n.Value
	}
}
