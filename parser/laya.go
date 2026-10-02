package parser

import (
	"strconv"

	"github.com/TacFlow/tac-language/ast"
	"github.com/TacFlow/tac-language/lexer"
)

// v0.5 (LAYA) syntax: requires, task, model, episode, dataset, schedule,
// labelled edges and the bookkeeping for forms the parser does not
// recognise (TAC-PARSE-001). The grammar is DESIGN §5.2 of the LAYA mission.

func tokText(t lexer.Token) string {
	switch t.Type {
	case lexer.String:
		return strconv.Quote(t.Value)
	case lexer.EOF:
		return "end of file"
	case lexer.Newline:
		return "newline"
	}
	if t.Value != "" {
		return t.Value
	}
	return t.String()
}

// unrecognized consumes a form the parser cannot read — up to the end of the
// line outside braces, or an unmatched '}' (which belongs to the enclosing
// block) — and returns a node recording it. Only `{ }` can carry the form
// over several lines: an unclosed '(' or '[' never swallows the next line.
// It always consumes at least one token, so no caller can loop on it.
func (p *Parser) unrecognized() *ast.Node {
	tok := p.peek()
	n := ast.NewNode(ast.NodeUnrecognized, tok.Line, tok.Col)
	n.Value = tokText(tok)
	depth := 0 // open '{'
	first := true
	for {
		t := p.peek()
		if t.Type == lexer.EOF {
			return n
		}
		if !first && depth == 0 && (t.Type == lexer.Newline || t.Type == lexer.RBrace) {
			return n
		}
		switch t.Type {
		case lexer.LBrace:
			depth++
		case lexer.RBrace:
			if depth > 0 {
				depth--
			}
		}
		p.advance()
		first = false
	}
}

func (p *Parser) parseString() *ast.Node {
	tok := p.advance()
	n := ast.NewNode(ast.NodeStringLiteral, tok.Line, tok.Col)
	n.Value = tok.Value
	return n
}

// negate turns a number literal node into its negative (for `[<-5]`, which
// the lexer reads as `<-` `5`).
func negate(n *ast.Node) *ast.Node {
	if len(n.Value) > 0 && n.Value[0] == '-' {
		n.Value = n.Value[1:]
	} else {
		n.Value = "-" + n.Value
	}
	n.NumVal = -n.NumVal
	return n
}

// parseBranch reads what sits between `[` and `]` in `gate[branch] -> x`:
//
//	branch = ident | string | "true" | "false" | "*" | range
//	range  = ( "<" | "<=" | ">" | ">=" ) number | number ".." number
//
// It returns nil (consuming nothing) when the tokens are not a branch.
func (p *Parser) parseBranch() *ast.Node {
	tok := p.peek()
	switch tok.Type {
	case lexer.Ident:
		return p.parseIdent()
	case lexer.String:
		return p.parseString()
	case lexer.True, lexer.False:
		return p.parseValue()
	case lexer.Star:
		p.advance()
		n := ast.NewNode(ast.NodeIdentifier, tok.Line, tok.Col)
		n.Value = ast.LabelAny
		return n
	case lexer.Less, lexer.LessEq, lexer.Greater, lexer.GreaterEq:
		if p.pos+1 >= len(p.tokens) || p.tokens[p.pos+1].Type != lexer.Number {
			return nil
		}
		p.advance()
		r := ast.NewNode(ast.NodeRange, tok.Line, tok.Col)
		r.Value = tok.Value
		r.Children = append(r.Children, p.parseValue())
		return r
	case lexer.Assign:
		// `[<-5]` lexes as `<-` `5`: inside a branch it means `< -5`.
		if p.pos+1 >= len(p.tokens) || p.tokens[p.pos+1].Type != lexer.Number {
			return nil
		}
		p.advance()
		r := ast.NewNode(ast.NodeRange, tok.Line, tok.Col)
		r.Value = "<"
		r.Children = append(r.Children, negate(p.parseValue()))
		return r
	case lexer.Number:
		if p.pos+2 >= len(p.tokens) || p.tokens[p.pos+1].Type != lexer.DotDot || p.tokens[p.pos+2].Type != lexer.Number {
			return nil
		}
		lo := p.parseValue()
		p.advance() // ..
		hi := p.parseValue()
		r := ast.NewNode(ast.NodeRange, tok.Line, tok.Col)
		r.Value = ".."
		r.Children = append(r.Children, lo, hi)
		return r
	}
	return nil
}

// parseLabelledEdge parses `src[branch] -> tgt [-> tgt2 …]` once src has
// been consumed and '[' is current. A malformed branch is a TAC-PARSE-001
// form: the line is skipped and recorded, never silently dropped.
func (p *Parser) parseLabelledEdge(srcTok lexer.Token) ([]*ast.Node, *ast.Node) {
	save := p.pos
	p.advance() // [
	branch := p.parseBranch()
	if branch == nil || p.peek().Type != lexer.RBrack {
		p.pos = save - 1
		return nil, p.unrecognized()
	}
	p.advance() // ]
	if p.peek().Type != lexer.Arrow {
		p.pos = save - 1
		return nil, p.unrecognized()
	}
	return p.parseEdgeChainLabelled(srcTok, branch), nil
}

// parseSchedule parses `schedule "<cron>" [tz "<IANA>"]` (keyword current).
func (p *Parser) parseSchedule() *ast.Node {
	save := p.pos
	kw := p.advance()
	if p.peek().Type != lexer.String {
		p.pos = save
		return p.unrecognized()
	}
	n := ast.NewNode(ast.NodeSchedule, kw.Line, kw.Col)
	n.Children = append(n.Children, p.parseString())
	if p.peek().Type == lexer.Ident && p.peek().Value == "tz" {
		if p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == lexer.String {
			p.advance()
			n.Attrs = map[string]*ast.Node{"tz": p.parseString()}
		}
	}
	return n
}

// isDeclStart reports whether the current token starts a v0.5 top-level
// declaration: the keyword followed by a string.
func (p *Parser) isDeclStart() bool {
	tok := p.peek()
	if tok.Type != lexer.Ident || p.pos+1 >= len(p.tokens) || p.tokens[p.pos+1].Type != lexer.String {
		return false
	}
	switch tok.Value {
	case "requires", "task", "model", "episode", "dataset":
		return true
	}
	return false
}

// parseDecl parses one top-level v0.5 declaration (isDeclStart is true).
func (p *Parser) parseDecl() *ast.Node {
	kw := p.advance()
	name := p.advance() // string
	var n *ast.Node
	switch kw.Value {
	case "requires":
		n = ast.NewNode(ast.NodeRequires, kw.Line, kw.Col)
		n.Value = name.Value
		return n
	case "task":
		n = ast.NewNode(ast.NodeTaskDecl, kw.Line, kw.Col)
	case "model":
		n = ast.NewNode(ast.NodeModelDecl, kw.Line, kw.Col)
	case "episode":
		n = ast.NewNode(ast.NodeEpisodeDecl, kw.Line, kw.Col)
	case "dataset":
		n = ast.NewNode(ast.NodeDatasetDecl, kw.Line, kw.Col)
	}
	n.Value = name.Value
	n.Attrs = map[string]*ast.Node{}
	if p.peek().Type != lexer.LBrace {
		return n
	}
	p.advance() // {
	for {
		p.skipNewlinesAndComments()
		t := p.peek()
		if t.Type == lexer.RBrace {
			p.advance()
			break
		}
		if t.Type == lexer.EOF {
			break
		}
		if t.Type == lexer.Comma {
			p.advance()
			continue
		}
		var item *ast.Node
		switch n.Type {
		case ast.NodeTaskDecl:
			item = p.parseTaskItem(n)
		case ast.NodeModelDecl:
			item = p.parseModelItem(n)
		case ast.NodeEpisodeDecl:
			item = p.parseEpisodeItem()
		case ast.NodeDatasetDecl:
			item = p.parseDatasetItem(n)
		}
		if item != nil {
			n.Children = append(n.Children, item)
		}
	}
	return n
}

// keyValue builds a KeyValue node: Value = key, Children[0] = value.
func keyValue(tok lexer.Token, val *ast.Node) *ast.Node {
	kv := ast.NewNode(ast.NodeKeyValue, tok.Line, tok.Col)
	kv.Value = tok.Value
	kv.Children = append(kv.Children, val)
	return kv
}

// parseTaskItem: question_def | "profile" object. Returns the node to append
// to Children (questions, unrecognised forms) or nil (stored in Attrs).
func (p *Parser) parseTaskItem(decl *ast.Node) *ast.Node {
	t := p.peek()
	switch {
	case t.Type == lexer.Ident && t.Value == "question":
		return p.parseQuestion()
	case t.Type == lexer.Ident && t.Value == "profile" && p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == lexer.LBrace:
		p.advance()
		decl.Attrs["profile"] = p.parseObjectLiteral()
		return nil
	}
	return p.unrecognized()
}

// parseModelItem: "tasks" [ strings ] | "base" string | "port" number.
func (p *Parser) parseModelItem(decl *ast.Node) *ast.Node {
	t := p.peek()
	if t.Type == lexer.Ident && p.pos+1 < len(p.tokens) {
		next := p.tokens[p.pos+1].Type
		switch {
		case t.Value == "tasks" && next == lexer.LBrack:
			p.advance()
			decl.Attrs["tasks"] = p.parseArrayLiteral()
			return nil
		case t.Value == "base" && next == lexer.String:
			p.advance()
			decl.Attrs["base"] = p.parseString()
			return nil
		case t.Value == "port" && next == lexer.Number:
			p.advance()
			decl.Attrs["port"] = p.parseValue()
			return nil
		}
	}
	return p.unrecognized()
}

// episodeStringKeys are the episode items that take one string.
var episodeStringKeys = map[string]bool{"group": true, "goal": true, "rule": true, "tool": true, "task": true}

// parseEpisodeItem: group/goal/rule/tool/task string | context/meta object |
// question_def | target ident "=" value | ident value (open state key).
func (p *Parser) parseEpisodeItem() *ast.Node {
	t := p.peek()
	if t.Type != lexer.Ident {
		return p.unrecognized()
	}
	var next lexer.TokenType = lexer.EOF
	if p.pos+1 < len(p.tokens) {
		next = p.tokens[p.pos+1].Type
	}
	switch {
	case t.Value == "question":
		return p.parseQuestion()
	case t.Value == "target":
		save := p.pos
		kw := p.advance()
		if p.peek().Type != lexer.Ident || p.pos+1 >= len(p.tokens) || p.tokens[p.pos+1].Type != lexer.Equals {
			p.pos = save
			return p.unrecognized()
		}
		id := p.advance()
		p.advance() // =
		val := p.parseValue()
		if val == nil {
			p.pos = save
			return p.unrecognized()
		}
		tg := ast.NewNode(ast.NodeTarget, kw.Line, kw.Col)
		tg.Value = id.Value
		tg.Children = append(tg.Children, val)
		return tg
	case episodeStringKeys[t.Value] && next == lexer.String:
		kw := p.advance()
		return keyValue(kw, p.parseString())
	case (t.Value == "context" || t.Value == "meta") && next == lexer.LBrace:
		kw := p.advance()
		return keyValue(kw, p.parseObjectLiteral())
	case !episodeStringKeys[t.Value] && t.Value != "context" && t.Value != "meta":
		// open state key: `cache_available false` — the value must end the
		// item, so a typo like `targte q = true` is reported, not stored as
		// state.targte = "q".
		save := p.pos
		kw := p.advance()
		val := p.parseValue()
		if val == nil || !p.atItemEnd() {
			p.pos = save
			return p.unrecognized()
		}
		return keyValue(kw, val)
	}
	return p.unrecognized()
}

// atItemEnd reports whether the current token ends a declaration item.
func (p *Parser) atItemEnd() bool {
	switch p.peek().Type {
	case lexer.Newline, lexer.RBrace, lexer.Comment, lexer.Comma, lexer.EOF:
		return true
	}
	return false
}

// parseDatasetItem: "task" string | "from" string | "include" string {"," string}
// | "split" ident "=" number.
func (p *Parser) parseDatasetItem(decl *ast.Node) *ast.Node {
	t := p.peek()
	if t.Type != lexer.Ident || p.pos+1 >= len(p.tokens) {
		return p.unrecognized()
	}
	next := p.tokens[p.pos+1].Type
	switch {
	case (t.Value == "task" || t.Value == "from") && next == lexer.String:
		p.advance()
		decl.Attrs[t.Value] = p.parseString()
		return nil
	case t.Value == "include" && next == lexer.String:
		p.advance()
		arr := decl.Attrs["include"]
		if arr == nil {
			arr = ast.NewNode(ast.NodeArrayLiteral, t.Line, t.Col)
			arr.ArrVal = []*ast.Node{}
			decl.Attrs["include"] = arr
		}
		for p.peek().Type == lexer.String {
			arr.ArrVal = append(arr.ArrVal, p.parseString())
			if p.peek().Type != lexer.Comma || p.pos+1 >= len(p.tokens) || p.tokens[p.pos+1].Type != lexer.String {
				break
			}
			p.advance() // ,
		}
		return nil
	case t.Value == "split" && next == lexer.Ident:
		if p.pos+3 < len(p.tokens) && p.tokens[p.pos+2].Type == lexer.Equals && p.tokens[p.pos+3].Type == lexer.Number {
			p.advance()
			name := p.advance()
			p.advance() // =
			return keyValue(name, p.parseValue())
		}
	}
	return p.unrecognized()
}

// optionTypes are the question types whose single `{ … }` block lists
// options; for every other type a single block holds attributes.
var optionTypes = map[string]bool{"choice": true, "multi_choice": true, "binary": true, "ranking": true}

// parseQuestion parses
//
//	question id ":" qtype "instructions" [ {attrs} ] [ {options} ]
//
// Attrs: "type" (Identifier), "instructions" (StringLiteral), "attrs"
// (ObjectLiteral, optional). Children: the options, in order, as KeyValue
// (Value = label, Children[0] = description). With two blocks the first holds
// attributes and the second options; with one block the question type
// decides (choice/multi_choice/binary/ranking: options; others: attributes).
func (p *Parser) parseQuestion() *ast.Node {
	save := p.pos
	kw := p.advance()
	if p.pos+3 >= len(p.tokens) ||
		p.tokens[p.pos].Type != lexer.Ident || p.tokens[p.pos+1].Type != lexer.Colon ||
		p.tokens[p.pos+2].Type != lexer.Ident || p.tokens[p.pos+3].Type != lexer.String {
		p.pos = save
		return p.unrecognized()
	}
	id := p.advance()
	p.advance() // :
	q := ast.NewNode(ast.NodeQuestion, kw.Line, kw.Col)
	q.Value = id.Value
	q.Attrs = map[string]*ast.Node{"type": p.parseIdent(), "instructions": p.parseString()}

	if p.peek().Type != lexer.LBrace {
		return q
	}
	// Two blocks: attributes, then options.
	isOptions := optionTypes[q.Attrs["type"].Value]
	if !isOptions || p.blockFollowedByBlock() {
		q.Attrs["attrs"] = p.parseObjectLiteral()
		if p.peek().Type != lexer.LBrace {
			return q
		}
	}
	opts, ok := p.parseOptions()
	if !ok {
		p.pos = save
		return p.unrecognized()
	}
	q.Children = opts
	return q
}

// blockFollowedByBlock reports whether the '{…}' block at the current
// position is immediately followed (same line) by another '{'.
func (p *Parser) blockFollowedByBlock() bool {
	depth := 0
	for i := p.pos; i < len(p.tokens); i++ {
		switch p.tokens[i].Type {
		case lexer.LBrace:
			depth++
		case lexer.RBrace:
			depth--
			if depth == 0 {
				return i+1 < len(p.tokens) && p.tokens[i+1].Type == lexer.LBrace
			}
		case lexer.EOF:
			return false
		}
	}
	return false
}

// parseOptions parses `{ label: "description" … }` keeping declaration
// order. Labels are identifiers or strings; separators are newlines or commas.
func (p *Parser) parseOptions() ([]*ast.Node, bool) {
	p.advance() // {
	var out []*ast.Node
	for {
		p.skipNewlinesAndComments()
		t := p.peek()
		switch t.Type {
		case lexer.RBrace:
			p.advance()
			return out, true
		case lexer.Comma:
			p.advance()
			continue
		case lexer.Ident, lexer.String:
			if p.pos+2 >= len(p.tokens) || p.tokens[p.pos+1].Type != lexer.Colon || p.tokens[p.pos+2].Type != lexer.String {
				return nil, false
			}
			label := p.advance()
			p.advance() // :
			out = append(out, keyValue(label, p.parseString()))
		default:
			return nil, false
		}
	}
}
