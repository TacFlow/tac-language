package formatter

import (
	"fmt"
	"strings"

	"github.com/TacFlow/tac-language/ast"
)

// v0.5 (LAYA) constructs. Canonical form (DESIGN §5.5): one form per
// construct, declarations and their items in file order, `context`/`meta`
// and other objects on one line when it fits in maxLine columns, otherwise
// one key per line; numbers by their source literal. A form the parser could
// not read (ast.NodeUnrecognized, TAC-PARSE-001) has nothing to print and,
// like a comment, does not survive formatting.

const maxLine = 100

// isIdent reports whether s can be written as a bare TAC identifier.
func isIdent(s string) bool {
	if s == "" || s == "true" || s == "false" {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		case (r >= '0' && r <= '9') && i > 0:
		default:
			return false
		}
	}
	return true
}

// keyStr writes an object key: bare when the lexer reads it back as one
// identifier (dots allowed after the first character, as in `a.b`), quoted
// otherwise (`"sku id"`) — an unquoted key with a space would not re-parse.
func keyStr(k string) string {
	if k == "" || k == "true" || k == "false" {
		return fmt.Sprintf("%q", k)
	}
	for i, r := range k {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		case (r >= '0' && r <= '9' || r == '.') && i > 0:
		default:
			return fmt.Sprintf("%q", k)
		}
	}
	return k
}

func labelOrString(s string) string {
	if isIdent(s) {
		return s
	}
	return fmt.Sprintf("%q", s)
}

// branchStr renders the branch of a labelled edge.
func branchStr(b *ast.Node) string {
	switch b.Type {
	case ast.NodeRange:
		if b.Value == ".." && len(b.Children) == 2 {
			return b.Children[0].Value + ".." + b.Children[1].Value
		}
		if len(b.Children) == 1 {
			return b.Value + b.Children[0].Value
		}
		return b.Value
	case ast.NodeStringLiteral:
		return fmt.Sprintf("%q", b.Value)
	case ast.NodeBoolLiteral:
		if b.BoolVal {
			return "true"
		}
		return "false"
	}
	return b.Value
}

// writeKeyed writes `key value`, breaking an object over several lines when
// the one-line form would not fit.
func (w *fmtWriter) writeKeyed(key string, v *ast.Node) {
	inline := nodeValueStr(v)
	if v.Type != ast.NodeObjectLiteral || len(w.indent())+len(key)+1+len(inline) <= maxLine || len(v.MapVal) == 0 {
		w.writeln("%s %s", key, inline)
		return
	}
	w.writeln("%s {", key)
	w.depth++
	for _, k := range sortedMapKeys(v.MapVal) {
		w.writeln("%s: %s", keyStr(k), nodeValueStr(v.MapVal[k]))
	}
	w.depth--
	w.writeln("}")
}

func (w *fmtWriter) writeQuestion(q *ast.Node) {
	typ, instr := "", ""
	if t := q.Attrs["type"]; t != nil {
		typ = t.Value
	}
	if s := q.Attrs["instructions"]; s != nil {
		instr = s.Value
	}
	line := fmt.Sprintf("question %s: %s %q", q.Value, typ, instr)
	if a := q.Attrs["attrs"]; a != nil {
		line += " " + nodeValueStr(a)
	}
	if len(q.Children) == 0 {
		w.writeln("%s", line)
		return
	}
	w.writeln("%s {", line)
	w.depth++
	for _, o := range q.Children {
		desc := ""
		if len(o.Children) > 0 {
			desc = o.Children[0].Value
		}
		w.writeln("%s: %q", labelOrString(o.Value), desc)
	}
	w.depth--
	w.writeln("}")
}

func (w *fmtWriter) writeDecl(n *ast.Node) {
	kw := map[ast.NodeType]string{ast.NodeTaskDecl: "task", ast.NodeModelDecl: "model",
		ast.NodeEpisodeDecl: "episode", ast.NodeDatasetDecl: "dataset"}[n.Type]
	w.writeln("%s %q {", kw, n.Value)
	w.depth++
	switch n.Type {
	case ast.NodeModelDecl:
		for _, k := range []string{"tasks", "base", "port"} {
			if v := n.Attrs[k]; v != nil {
				w.writeln("%s %s", k, nodeValueStr(v))
			}
		}
	case ast.NodeDatasetDecl:
		for _, k := range []string{"task", "from"} {
			if v := n.Attrs[k]; v != nil {
				w.writeln("%s %s", k, nodeValueStr(v))
			}
		}
		if inc := n.Attrs["include"]; inc != nil && len(inc.ArrVal) > 0 {
			parts := make([]string, len(inc.ArrVal))
			for i, v := range inc.ArrVal {
				parts[i] = nodeValueStr(v)
			}
			w.writeln("include %s", strings.Join(parts, ", "))
		}
	}
	for _, c := range n.Children {
		switch c.Type {
		case ast.NodeQuestion:
			w.writeQuestion(c)
		case ast.NodeTarget:
			if len(c.Children) > 0 {
				w.writeln("target %s = %s", c.Value, nodeValueStr(c.Children[0]))
			}
		case ast.NodeKeyValue:
			if len(c.Children) == 0 {
				continue
			}
			if n.Type == ast.NodeDatasetDecl {
				w.writeln("split %s = %s", c.Value, nodeValueStr(c.Children[0]))
			} else {
				w.writeKeyed(c.Value, c.Children[0])
			}
		}
	}
	if n.Type == ast.NodeTaskDecl {
		if p := n.Attrs["profile"]; p != nil {
			w.writeKeyed("profile", p)
		}
	}
	w.depth--
	w.writeln("}")
}

func (w *fmtWriter) writeSchedule(n *ast.Node) {
	if len(n.Children) == 0 {
		return
	}
	if tz := n.Attrs["tz"]; tz != nil {
		w.writeln("schedule %q tz %q", n.Children[0].Value, tz.Value)
		return
	}
	w.writeln("schedule %q", n.Children[0].Value)
}
