package laya

import (
	"encoding/json"

	"github.com/TacFlow/tac-language/ast"
)

// DataValue converts a literal AST value in a DECLARATION (episode state,
// targets, meta, profile) to its JSON form. Unlike a flow argument, a bare
// identifier here is text (`target acao = proceed` means the label
// "proceed"), never a reference. Numbers keep their literal.
func DataValue(n *ast.Node) interface{} {
	if n == nil {
		return nil
	}
	switch n.Type {
	case ast.NodeStringLiteral, ast.NodeIdentifier:
		return n.Value
	case ast.NodeNumberLiteral:
		return json.Number(n.Value)
	case ast.NodeBoolLiteral:
		return n.BoolVal
	case ast.NodeObjectLiteral:
		m := make(map[string]interface{}, len(n.MapVal))
		for k, v := range n.MapVal {
			m[k] = DataValue(v)
		}
		return m
	case ast.NodeArrayLiteral:
		arr := make([]interface{}, len(n.ArrVal))
		for i, v := range n.ArrVal {
			arr[i] = DataValue(v)
		}
		return arr
	}
	return n.Value
}

func numbers(n *ast.Node) []json.Number {
	if n == nil || n.Type != ast.NodeArrayLiteral {
		return nil
	}
	out := make([]json.Number, 0, len(n.ArrVal))
	for _, v := range n.ArrVal {
		if v.Type == ast.NodeNumberLiteral {
			out = append(out, json.Number(v.Value))
		}
	}
	return out
}

func texts(n *ast.Node) []string {
	if n == nil || n.Type != ast.NodeArrayLiteral {
		return nil
	}
	out := make([]string, 0, len(n.ArrVal))
	for _, v := range n.ArrVal {
		if v.Type == ast.NodeStringLiteral || v.Type == ast.NodeIdentifier {
			out = append(out, v.Value)
		}
	}
	return out
}

// QuestionAttrs are the attribute keys a question's `{ … }` block may carry.
var QuestionAttrs = map[string]bool{"range": true, "unit": true, "bins": true, "levels": true, "fields": true}

// QuestionFromAST converts a Question node.
func QuestionFromAST(n *ast.Node) Question {
	q := Question{ID: n.Value}
	if t := n.Attrs["type"]; t != nil {
		q.Type = t.Value
	}
	if s := n.Attrs["instructions"]; s != nil {
		q.Instructions = s.Value
	}
	for _, o := range n.Children {
		if o.Type != ast.NodeKeyValue {
			continue
		}
		opt := Option{Label: o.Value}
		if len(o.Children) > 0 {
			opt.Description = o.Children[0].Value
		}
		q.Options = append(q.Options, opt)
	}
	if a := n.Attrs["attrs"]; a != nil {
		q.Range = numbers(a.MapVal["range"])
		q.Bins = numbers(a.MapVal["bins"])
		q.Levels = texts(a.MapVal["levels"])
		q.Fields = texts(a.MapVal["fields"])
		if u := a.MapVal["unit"]; u != nil {
			q.Unit = u.Value
		}
	}
	return q
}

// TaskFromAST converts a TaskDecl node.
func TaskFromAST(n *ast.Node) Task {
	t := Task{Name: n.Value, Questions: []Question{}}
	for _, c := range n.Children {
		if c.Type == ast.NodeQuestion {
			t.Questions = append(t.Questions, QuestionFromAST(c))
		}
	}
	if p := n.Attrs["profile"]; p != nil {
		t.Profile = DataValue(p)
	}
	return t
}

// ModelFromAST converts a ModelDecl node.
func ModelFromAST(n *ast.Node) Model {
	m := Model{Name: n.Value, Tasks: texts(n.Attrs["tasks"])}
	if m.Tasks == nil {
		m.Tasks = []string{}
	}
	if b := n.Attrs["base"]; b != nil {
		m.Base = b.Value
	}
	if p := n.Attrs["port"]; p != nil {
		m.Port = json.Number(p.Value)
	}
	return m
}

// DatasetFromAST converts a DatasetDecl node.
func DatasetFromAST(n *ast.Node) Dataset {
	d := Dataset{Name: n.Value}
	if t := n.Attrs["task"]; t != nil {
		d.Task = t.Value
	}
	if f := n.Attrs["from"]; f != nil {
		d.From = f.Value
	}
	d.Include = texts(n.Attrs["include"])
	for _, c := range n.Children {
		if c.Type == ast.NodeKeyValue && len(c.Children) > 0 {
			if d.Splits == nil {
				d.Splits = map[string]json.Number{}
			}
			d.Splits[c.Value] = json.Number(c.Children[0].Value)
		}
	}
	return d
}

// DefaultGroupID is the group_id of an episode declared without `group`:
// the shape of DESIGN §3.3 ({source}:{actor}:{scope}) with the episode id as
// scope, so compiling the same source always gives the same JSON.
func DefaultGroupID(id string) string { return "tac:anon:" + id }

// EpisodeFromAST converts an EpisodeDecl node. `rule` is stored as
// state.regra_declarada (the reference JSON is the contract, DESIGN D14) and
// `task` as meta.task.
func EpisodeFromAST(n *ast.Node) Episode {
	e := Episode{ID: n.Value, State: map[string]interface{}{}, Questions: []Question{}}
	var meta map[string]interface{}
	task := ""
	for _, c := range n.Children {
		switch c.Type {
		case ast.NodeQuestion:
			e.Questions = append(e.Questions, QuestionFromAST(c))
		case ast.NodeTarget:
			if e.Targets == nil {
				e.Targets = map[string]interface{}{}
			}
			if len(c.Children) > 0 {
				e.Targets[c.Value] = DataValue(c.Children[0])
			}
		case ast.NodeKeyValue:
			if len(c.Children) == 0 {
				continue
			}
			v := c.Children[0]
			switch c.Value {
			case "group":
				e.GroupID = v.Value
			case "task":
				task = v.Value
			case "rule":
				e.State["regra_declarada"] = v.Value
			case "meta":
				if m, ok := DataValue(v).(map[string]interface{}); ok {
					meta = m
				}
			default: // goal, context, tool and open state keys
				e.State[c.Value] = DataValue(v)
			}
		}
	}
	if e.GroupID == "" {
		e.GroupID = DefaultGroupID(e.ID)
	}
	if task != "" {
		if meta == nil {
			meta = map[string]interface{}{}
		}
		meta["task"] = task
	}
	e.Meta = meta
	return e
}
