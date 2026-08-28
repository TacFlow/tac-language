// Package parser implements the TAC Language parser.
//
// The parser consumes a token stream from the lexer and produces
// an Abstract Syntax Tree (AST) defined in the ast package.
//
// Grammar (EBNF):
//
//	program     = { statement } ;
//	statement   = remember_stmt | recall_stmt | forget_stmt
//	            | relate_stmt | flow_def | context_block
//	            | auto_summarize_stmt ;
//	flow_def    = "flow" string "{" { flow_stmt } "}" ;
//	flow_stmt   = node_def | edge_def | trigger_def | input_stmt | agent_decl ;
//	node_def    = "node" (string | ident) "->" (skill_call | ident_list) [block] ;
//	skill_call  = ident "(" [arg_list] ")" [ "{" { named_arg } "}" ] ;
//	edge_def    = ident "->" (ident | ident_list) [ "{" [ condition ] "}" ] ;
//	trigger_def = "on" string [ "{" trigger_attrs "}" ] "->" (ident | ident_list) ;
package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/TacFlow/tac-language/ast"
	"github.com/TacFlow/tac-language/lexer"
)

// Parser converts a token stream into an AST.
type Parser struct {
	tokens []lexer.Token
	pos    int
}

// New creates a new Parser for the given token stream.
func New(tokens []lexer.Token) *Parser {
	return &Parser{tokens: tokens, pos: 0}
}

// posAt returns the current parse position for error reporting.
func (p *Parser) posAt() ast.Position {
	if p.pos < len(p.tokens) {
		tok := p.tokens[p.pos]
		return ast.Position{Line: tok.Line, Col: tok.Col}
	}
	if len(p.tokens) > 0 {
		last := p.tokens[len(p.tokens)-1]
		return ast.Position{Line: last.Line, Col: last.Col}
	}
	return ast.Position{Line: 1, Col: 1}
}

func (p *Parser) peek() lexer.Token {
	if p.pos >= len(p.tokens) {
		return lexer.Token{Type: lexer.EOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() lexer.Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *Parser) skipNewlines() {
	for p.peek().Type == lexer.Newline {
		p.advance()
	}
}

func (p *Parser) skipNewlinesAndComments() {
	for {
		tok := p.peek()
		if tok.Type == lexer.Newline || tok.Type == lexer.Comment {
			p.advance()
		} else {
			break
		}
	}
}

// --- Value Parsing ---

func (p *Parser) parseValue() *ast.Node {
	tok := p.peek()
	switch tok.Type {
	case lexer.String:
		vtok := p.advance()
		n := ast.NewNode(ast.NodeStringLiteral, vtok.Line, vtok.Col)
		n.Value = vtok.Value
		return n
	case lexer.Number:
		vtok := p.advance()
		n := ast.NewNode(ast.NodeNumberLiteral, vtok.Line, vtok.Col)
		n.Value = vtok.Value
		n.NumVal, _ = strconv.ParseFloat(vtok.Value, 64)
		return n
	case lexer.True:
		p.advance()
		n := ast.NewNode(ast.NodeBoolLiteral, tok.Line, tok.Col)
		n.BoolVal = true
		n.Value = "true"
		return n
	case lexer.False:
		p.advance()
		n := ast.NewNode(ast.NodeBoolLiteral, tok.Line, tok.Col)
		n.BoolVal = false
		n.Value = "false"
		return n
	case lexer.LBrace:
		return p.parseObjectLiteral()
	case lexer.LBrack:
		return p.parseArrayLiteral()
	case lexer.Ident:
		return p.parseIdent()
	}
	return nil
}

func (p *Parser) parseIdent() *ast.Node {
	tok := p.advance()
	n := ast.NewNode(ast.NodeIdentifier, tok.Line, tok.Col)
	n.Value = tok.Value
	return n
}

func (p *Parser) parseObjectLiteral() *ast.Node {
	tok := p.advance() // {
	n := ast.NewNode(ast.NodeObjectLiteral, tok.Line, tok.Col)
	n.MapVal = make(map[string]*ast.Node)
	p.skipNewlines()
	for p.peek().Type != lexer.RBrace && p.peek().Type != lexer.EOF {
		keyTok := p.peek()
		if keyTok.Type != lexer.Ident && keyTok.Type != lexer.String {
			break
		}
		key := p.advance().Value
		if p.peek().Type == lexer.Colon {
			p.advance()
		} else {
			break
		}
		val := p.parseValue()
		if val != nil {
			n.MapVal[key] = val
		}
		p.skipNewlines()
		if p.peek().Type == lexer.Comma {
			p.advance()
			p.skipNewlines()
		}
	}
	if p.peek().Type == lexer.RBrace {
		p.advance()
	}
	return n
}

func (p *Parser) parseArrayLiteral() *ast.Node {
	tok := p.advance() // [
	n := ast.NewNode(ast.NodeArrayLiteral, tok.Line, tok.Col)
	n.ArrVal = make([]*ast.Node, 0)
	p.skipNewlines()
	for p.peek().Type != lexer.RBrack && p.peek().Type != lexer.EOF {
		val := p.parseValue()
		if val != nil {
			n.ArrVal = append(n.ArrVal, val)
		}
		p.skipNewlines()
		if p.peek().Type == lexer.Comma {
			p.advance()
			p.skipNewlines()
		}
	}
	if p.peek().Type == lexer.RBrack {
		p.advance()
	}
	return n
}

// --- Expression Parsing (for conditions like "x > 0.9") ---

// isComparisonOp returns true if the token is a comparison/equality operator.
func isComparisonOp(t lexer.TokenType) bool {
	switch t {
	case lexer.Greater, lexer.Less, lexer.GreaterEq, lexer.LessEq, lexer.EqEq, lexer.NotEq:
		return true
	}
	return false
}

// parseExprValue parses a value or a comparison expression (lhs op rhs).
func (p *Parser) parseExprValue() *ast.Node {
	lhs := p.parseValue()
	if lhs == nil {
		return nil
	}
	if isComparisonOp(p.peek().Type) {
		opTok := p.advance()
		rhs := p.parseValue()
		expr := ast.NewNode(ast.NodeCondition, opTok.Line, opTok.Col)
		expr.Value = opTok.Value
		expr.Children = append(expr.Children, lhs)
		if rhs != nil {
			expr.Children = append(expr.Children, rhs)
		}
		return expr
	}
	return lhs
}

// --- Named Arguments ---

func (p *Parser) parseNamedArgs() map[string]*ast.Node {
	attrs := make(map[string]*ast.Node)
	if p.peek().Type != lexer.LBrace {
		return attrs
	}
	p.advance() // {
	p.skipNewlines()
	for p.peek().Type != lexer.RBrace && p.peek().Type != lexer.EOF {
		keyTok := p.peek()
		if keyTok.Type != lexer.Ident {
			break
		}
		key := p.advance().Value
		if p.peek().Type == lexer.Colon {
			p.advance()
		} else {
			break
		}
		val := p.parseExprValue()
		if val != nil {
			attrs[key] = val
		}
		p.skipNewlines()
		if p.peek().Type == lexer.Comma {
			p.advance()
			p.skipNewlines()
		}
	}
	if p.peek().Type == lexer.RBrace {
		p.advance()
	}
	return attrs
}

// --- Skill Call ---

func (p *Parser) parseSkillCall(nameTok lexer.Token) *ast.Node {
	n := ast.NewNode(ast.NodeSkillCall, nameTok.Line, nameTok.Col)
	n.Value = nameTok.Value
	n.Args = make([]*ast.Node, 0)

	// Optional version: @ "1.4.2"
	if p.peek().Type == lexer.At {
		p.advance() // @
		if p.peek().Type == lexer.String {
			n.Version = p.advance().Value
		}
	}

	if p.peek().Type == lexer.LParen {
		p.advance()
		p.skipNewlines()
		for p.peek().Type != lexer.RParen && p.peek().Type != lexer.EOF {
			// Check for named arg: name: value
			if p.peek().Type == lexer.Ident &&
				p.pos+1 < len(p.tokens) &&
				p.tokens[p.pos+1].Type == lexer.Colon {
				nameTok := p.advance() // consume argument name
				p.advance()            // consume colon
				val := p.parseValue()
				arg := ast.NewNode(ast.NodeNamedArg, nameTok.Line, nameTok.Col)
				arg.Value = nameTok.Value
				if val != nil {
					arg.Children = append(arg.Children, val)
				}
				n.Args = append(n.Args, arg)
			} else {
				arg := p.parseValue()
				if arg != nil {
					n.Args = append(n.Args, arg)
				} else {
					// Skip unrecognized tokens to avoid infinite loop
					p.advance()
				}
			}
			p.skipNewlines()
			if p.peek().Type == lexer.Comma {
				p.advance()
				p.skipNewlines()
			}
		}
		if p.peek().Type == lexer.RParen {
			p.advance()
		}
	}

	if p.peek().Type == lexer.LBrace {
		n.Attrs = p.parseNamedArgs()
	}

	return n
}

// --- Statements ---

func (p *Parser) parseRememberStmt() *ast.Node {
	tok := p.advance() // "remember"
	n := ast.NewNode(ast.NodeRememberStmt, tok.Line, tok.Col)
	if p.peek().Type == lexer.Ident {
		n.Children = append(n.Children, p.parseIdent())
	}
	if p.peek().Type == lexer.Equals {
		p.advance()
		val := p.parseValue()
		if val != nil {
			n.Children = append(n.Children, val)
		}
	}
	if p.peek().Type == lexer.LBrace {
		n.Attrs = p.parseNamedArgs()
	}
	return n
}

func (p *Parser) parseRecallStmt() *ast.Node {
	tok := p.advance() // "recall"
	n := ast.NewNode(ast.NodeRecallStmt, tok.Line, tok.Col)
	if p.peek().Type == lexer.Ident {
		n.Children = append(n.Children, p.parseIdent())
	}
	if p.peek().Type == lexer.LBrace {
		n.Attrs = p.parseNamedArgs()
	}
	return n
}

func (p *Parser) parseForgetStmt() *ast.Node {
	tok := p.advance() // "forget"
	n := ast.NewNode(ast.NodeForgetStmt, tok.Line, tok.Col)
	if p.peek().Type == lexer.Ident {
		n.Children = append(n.Children, p.parseIdent())
	}
	if p.peek().Type == lexer.LBrace {
		n.Attrs = p.parseNamedArgs()
	}
	return n
}

func (p *Parser) parseRelateStmt() *ast.Node {
	tok := p.advance() // "relate"
	n := ast.NewNode(ast.NodeRelateStmt, tok.Line, tok.Col)
	src := p.parseIdent()
	n.Children = append(n.Children, src)
	if p.peek().Type == lexer.Arrow {
		p.advance()
	}
	tgt := p.parseIdent()
	n.Children = append(n.Children, tgt)
	if p.peek().Type == lexer.LBrace {
		n.Attrs = p.parseNamedArgs()
	}
	return n
}

// --- Node Definition ---

func (p *Parser) parseNodeDef() *ast.Node {
	tok := p.advance() // "node"
	n := ast.NewNode(ast.NodeNode, tok.Line, tok.Col)

	// Node name
	if p.peek().Type == lexer.String {
		nameNode := p.parseValue()
		if nameNode != nil {
			n.Children = append(n.Children, nameNode)
			n.Value = nameNode.Value
		}
	} else if p.peek().Type == lexer.Ident {
		nameNode := p.parseIdent()
		n.Children = append(n.Children, nameNode)
		n.Value = nameNode.Value
	}

	// Arrow
	if p.peek().Type == lexer.Arrow {
		p.advance()

		// Consume optional "skill" keyword
		if p.peek().Type == lexer.Ident && p.peek().Value == "skill" {
			p.advance()
		}

		if p.peek().Type == lexer.Ident {
			// Check if this is a skill call (followed by ( or {)
			if p.pos+1 < len(p.tokens) &&
				(p.tokens[p.pos+1].Type == lexer.LParen || p.tokens[p.pos+1].Type == lexer.LBrace) {
				p.advance() // advance past skill name so parseSkillCall sees it
				call := p.parseSkillCall(p.tokens[p.pos-1])
				n.Children = append(n.Children, call)
			} else {
				n.Children = append(n.Children, p.parseIdent())
			}
		} else if p.peek().Type == lexer.LBrack {
			// Array of targets
			p.advance()
			p.skipNewlines()
			arr := ast.NewNode(ast.NodeArrayLiteral, tok.Line, tok.Col)
			arr.ArrVal = make([]*ast.Node, 0)
			for p.peek().Type != lexer.RBrack && p.peek().Type != lexer.EOF {
				arr.ArrVal = append(arr.ArrVal, p.parseIdent())
				p.skipNewlines()
				if p.peek().Type == lexer.Comma {
					p.advance()
					p.skipNewlines()
				}
			}
			if p.peek().Type == lexer.RBrack {
				p.advance()
			}
			n.Children = append(n.Children, arr)
		}
	}

	// Optional block (for inline nodes with sub-nodes)
	if p.peek().Type == lexer.LBrace {
		saved := p.pos
		p.advance()
		p.skipNewlines()
		// Heuristic: if the next token is an ident followed by ':', it's named args
		// (e.g. { if: condition, else: fallback })
		// Otherwise it's a block of statements (e.g. { if condition { ... } })
		isNamedArgs := false
		if p.peek().Type == lexer.Ident {
			// Look ahead for colon
			if p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == lexer.Colon {
				isNamedArgs = true
			}
		}
		p.pos = saved
		if isNamedArgs {
			n.Attrs = p.parseNamedArgs()
		} else {
			n.Nodes = p.parseBlock()
		}
	}

	return n
}

// --- Block ---

func (p *Parser) parseBlock() []*ast.Node {
	nodes := make([]*ast.Node, 0)
	if p.peek().Type != lexer.LBrace {
		return nodes
	}
	p.advance() // {
	p.skipNewlines()

	// Track brace depth for nested blocks
	depth := 1

	for depth > 0 && p.peek().Type != lexer.EOF {
		tok := p.peek()

		switch {
		case tok.Type == lexer.RBrace:
			p.advance()
			depth--
			p.skipNewlines()

		case tok.Type == lexer.LBrace:
			p.advance()
			depth++
			p.skipNewlines()

		case tok.Type == lexer.Ident && tok.Value == "skill":
			p.advance()
			if p.peek().Type == lexer.Ident {
				call := p.parseSkillCall(p.advance())
				nodes = append(nodes, call)
			}
			p.skipNewlines()

		case tok.Type == lexer.Ident && tok.Value == "for":
			p.advance()
			if p.peek().Type == lexer.Ident && p.peek().Value == "each" {
				p.advance()
			}
			forNode := ast.NewNode(ast.NodeIdentifier, tok.Line, tok.Col)
			forNode.Value = "for_each"
			if p.peek().Type == lexer.Ident {
				forNode.Children = append(forNode.Children, p.parseIdent())
			}
			if p.peek().Type == lexer.Ident && p.peek().Value == "in" {
				p.advance()
				forNode.Children = append(forNode.Children, p.parseIdent())
			}
			if p.peek().Type == lexer.LBrace {
				forNode.Nodes = p.parseBlock()
			}
			nodes = append(nodes, forNode)
			p.skipNewlines()

		case tok.Type == lexer.Ident && (tok.Value == "if" || tok.Value == "else"):
			p.advance()
			condNode := ast.NewNode(ast.NodeCondition, tok.Line, tok.Col)
			condNode.Value = tok.Value // "if" or "else"
			// Parse condition expression (until { or end of line)
			var exprParts []string
			for p.peek().Type != lexer.LBrace && p.peek().Type != lexer.EOF && p.peek().Type != lexer.Newline && p.peek().Type != lexer.RBrace {
				t := p.advance()
				exprParts = append(exprParts, t.Value)
			}
			// Store condition expression in an identifier child
			if len(exprParts) > 0 {
				cond := ast.NewNode(ast.NodeIdentifier, tok.Line, tok.Col)
				cond.Value = strings.Join(exprParts, " ")
				condNode.Children = append(condNode.Children, cond)
			}
			p.skipNewlines()
			// Parse body block
			if p.peek().Type == lexer.LBrace {
				condNode.Nodes = p.parseBlock()
			}
			nodes = append(nodes, condNode)
			p.skipNewlines()

		case tok.Type == lexer.Newline:
			p.advance()
			p.skipNewlines()

		case tok.Type == lexer.Comment:
			p.advance()
			p.skipNewlines()

		default:
			// Skip unrecognized tokens gracefully
			p.advance()
			p.skipNewlines()
		}
	}

	return nodes
}

// --- Flow Body ---

func (p *Parser) parseFlowBody() (*ast.Node, error) {
	flow := ast.NewNode(ast.NodeFlow, 0, 0)
	if p.peek().Type != lexer.LBrace {
		return flow, fmt.Errorf("expected '{' for flow body at line %d", p.posAt().Line)
	}
	p.advance() // {
	p.skipNewlines()

	for p.peek().Type != lexer.RBrace && p.peek().Type != lexer.EOF {
		tok := p.peek()

		switch {
		case tok.Type == lexer.Ident && tok.Value == "node":
			node := p.parseNodeDef()
			flow.Nodes = append(flow.Nodes, node)

		case tok.Type == lexer.Ident && tok.Value == "input":
			p.advance()
			in := ast.NewNode(ast.NodeInput, tok.Line, tok.Col)
			if p.peek().Type == lexer.Ident {
				in.Children = append(in.Children, p.parseIdent())
			}
			if p.peek().Type == lexer.Colon {
				p.advance()
				if p.peek().Type == lexer.Ident {
					typeNode := p.parseIdent()
					in.Children = append(in.Children, typeNode)
				}
			}
			flow.Children = append(flow.Children, in)

		case tok.Type == lexer.Ident && tok.Value == "agent":
			p.advance()
			ag := ast.NewNode(ast.NodeAgentDecl, tok.Line, tok.Col)
			if p.peek().Type == lexer.String {
				ag.Children = append(ag.Children, p.parseValue())
			} else if p.peek().Type == lexer.Ident {
				ag.Children = append(ag.Children, p.parseIdent())
			}
			if p.peek().Type == lexer.LBrace {
				ag.Attrs = p.parseNamedArgs()
			}
			flow.Children = append(flow.Children, ag)

		case tok.Type == lexer.Ident && tok.Value == "on":
			p.advance()
			tr := ast.NewNode(ast.NodeTrigger, tok.Line, tok.Col)
			if p.peek().Type == lexer.String {
				tr.Children = append(tr.Children, p.parseValue())
			}
			if p.peek().Type == lexer.LBrace {
				tr.Attrs = p.parseNamedArgs()
			}
			p.skipNewlines()
			if p.peek().Type == lexer.Arrow {
				p.advance()
				p.skipNewlines()
				if p.peek().Type == lexer.LBrack {
					arr := p.parseArrayLiteral()
					tr.Children = append(tr.Children, arr)
				} else if p.peek().Type == lexer.Ident {
					tr.Children = append(tr.Children, p.parseIdent())
				}
			}
			flow.Children = append(flow.Children, tr)

		case tok.Type == lexer.Ident && tok.Value == "remember":
			flow.Children = append(flow.Children, p.parseRememberStmt())

		case tok.Type == lexer.Ident && tok.Value == "recall":
			flow.Children = append(flow.Children, p.parseRecallStmt())

		case tok.Type == lexer.Newline || tok.Type == lexer.Comment:
			p.advance()
			continue

		case tok.Type == lexer.Ident:
			// Could be an edge or an inline statement
			name := tok.Value
			p.advance()
			p.skipNewlines()
			if p.peek().Type == lexer.Arrow {
				e := ast.NewNode(ast.NodeEdge, tok.Line, tok.Col)
				src := ast.NewNode(ast.NodeIdentifier, tok.Line, tok.Col)
				src.Value = name
				e.Children = append(e.Children, src)

				p.advance() // ->
				p.skipNewlines()

				if p.peek().Type == lexer.LBrack {
					e.Children = append(e.Children, p.parseArrayLiteral())
				} else if p.peek().Type == lexer.Ident {
					e.Children = append(e.Children, p.parseIdent())
				}

				if p.peek().Type == lexer.LBrace {
					e.Attrs = p.parseNamedArgs()
				}
				flow.Edges = append(flow.Edges, e)
			}

		default:
			// Skip unrecognized token
			p.advance()
		}

		p.skipNewlines()
	}

	if p.peek().Type == lexer.RBrace {
		p.advance()
	}

	return flow, nil
}

// --- Top-Level Parsing ---

// Parse converts the token stream into a complete Program AST.
// It returns the root Program node or an error.
func (p *Parser) Parse() (*ast.Node, error) {
	program := ast.NewNode(ast.NodeProgram, 1, 1)
	p.skipNewlines()

	for p.peek().Type != lexer.EOF {
		tok := p.peek()

		switch {
		case tok.Type == lexer.Newline || tok.Type == lexer.Comment:
			p.advance()

		case tok.Type == lexer.Ident && tok.Value == "flow":
			p.advance()
			flowName := ""
			if p.peek().Type == lexer.String {
				flowNameNode := p.parseValue()
				if flowNameNode != nil {
					flowName = flowNameNode.Value
				}
			}
			flowBody, err := p.parseFlowBody()
			if err != nil {
				return nil, err
			}
			flowBody.Value = flowName
			program.Nodes = append(program.Nodes, flowBody)

		case tok.Type == lexer.Ident && tok.Value == "context":
			p.advance()
			ctx := ast.NewNode(ast.NodeContextBlock, tok.Line, tok.Col)
			if p.peek().Type == lexer.String {
				ctx.Children = append(ctx.Children, p.parseValue())
			}
			if p.peek().Type == lexer.LBrace {
				p.advance() // {
				p.skipNewlines()
				for p.peek().Type != lexer.RBrace && p.peek().Type != lexer.EOF {
					switch {
					case p.peek().Type == lexer.Ident && p.peek().Value == "remember":
						ctx.Children = append(ctx.Children, p.parseRememberStmt())
					case p.peek().Type == lexer.Ident && p.peek().Value == "flow":
						p.advance()
						fn := ""
						if p.peek().Type == lexer.String {
							fnNode := p.parseValue()
							if fnNode != nil {
								fn = fnNode.Value
							}
						}
						fb, _ := p.parseFlowBody()
						fb.Value = fn
						ctx.Children = append(ctx.Children, fb)
					case p.peek().Type == lexer.Ident && p.peek().Value == "recall":
						ctx.Children = append(ctx.Children, p.parseRecallStmt())
					default:
						p.advance()
					}
					p.skipNewlines()
				}
				if p.peek().Type == lexer.RBrace {
					p.advance()
				}
			}
			program.Nodes = append(program.Nodes, ctx)

		case tok.Type == lexer.Ident && tok.Value == "remember":
			program.Nodes = append(program.Nodes, p.parseRememberStmt())

		case tok.Type == lexer.Ident && tok.Value == "recall":
			program.Nodes = append(program.Nodes, p.parseRecallStmt())

		case tok.Type == lexer.Ident && tok.Value == "relate":
			program.Nodes = append(program.Nodes, p.parseRelateStmt())

		case tok.Type == lexer.Ident && tok.Value == "forget":
			program.Nodes = append(program.Nodes, p.parseForgetStmt())

		case tok.Type == lexer.Ident && tok.Value == "auto_summarize":
			p.advance()
			as := ast.NewNode(ast.NodeAutoSummarize, tok.Line, tok.Col)
			if p.peek().Type == lexer.LParen {
				p.advance()
				p.skipNewlines()
				as.Attrs = make(map[string]*ast.Node)
				for p.peek().Type != lexer.RParen && p.peek().Type != lexer.EOF {
					if p.peek().Type == lexer.Ident {
						key := p.advance().Value
						if p.peek().Type == lexer.Colon {
							p.advance()
							val := p.parseValue()
							if val != nil {
								as.Attrs[key] = val
							}
						}
					}
					p.skipNewlines()
					if p.peek().Type == lexer.Comma {
						p.advance()
						p.skipNewlines()
					}
				}
				if p.peek().Type == lexer.RParen {
					p.advance()
				}
			}
			program.Nodes = append(program.Nodes, as)

		default:
			// Skip unrecognized top-level tokens
			p.advance()
		}

		p.skipNewlines()
	}

	return program, nil
}

// ParseSource is a convenience function that lexes and parses a TAC source
// string in one call.
func ParseSource(source string) (*ast.Node, error) {
	l := lexer.New(source)
	tokens, err := l.Scan()
	if err != nil {
		return nil, fmt.Errorf("lexer error: %w", err)
	}
	if lexer.HasErrors(tokens) {
		// Collect all errors
		var errs []string
		for _, tok := range tokens {
			if tok.Type == lexer.Error {
				errs = append(errs, fmt.Sprintf("line %d: %s", tok.Line, tok.Value))
			}
		}
		return nil, fmt.Errorf("tokenization errors: %v", errs)
	}
	p := New(tokens)
	return p.Parse()
}
