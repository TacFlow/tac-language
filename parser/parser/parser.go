// Package parser implements the recursive-descent parser for the TAC language.
//
// It consumes the token stream produced by the lexer package and produces an
// AST (package ast). Unlike the original monolithic implementation, the parser
// reports rich syntax errors (with position and expected tokens) instead of
// silently skipping malformed input.
//
// (c) 2026 TacFlow — MIT License
package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tacflow1-tech/tac-language/parser/ast"
	"github.com/tacflow1-tech/tac-language/parser/lexer"
)

// Error is a syntax error with source position and expected tokens.
type Error struct {
	Line     int
	Col      int
	Msg      string
	Expected []lexer.TokenType
}

func (e *Error) Error() string {
	if len(e.Expected) > 0 {
		names := make([]string, 0, len(e.Expected))
		for _, t := range e.Expected {
			names = append(names, lexer.TokenTypeName(t))
		}
		return fmt.Sprintf("parse error at %d:%d: %s (expected %s)", e.Line, e.Col, e.Msg, strings.Join(names, " or "))
	}
	return fmt.Sprintf("parse error at %d:%d: %s", e.Line, e.Col, e.Msg)
}

// Parser is a recursive-descent parser over a token stream.
type Parser struct {
	tokens []lexer.Token
	pos    int
	errs   []*Error
}

// New creates a parser over the given tokens.
func New(tokens []lexer.Token) *Parser {
	return &Parser{tokens: tokens, pos: 0}
}

// ParseString tokenizes and parses TAC source in one call.
func ParseString(source string) (*ast.Node, error) {
	tok := lexer.NewTokenizer(source)
	tokens, err := tok.Tokenize()
	if err != nil {
		return nil, err
	}
	for _, t := range tokens {
		if t.Type == lexer.TError {
			return nil, &Error{Line: t.Line, Col: t.Col, Msg: t.Value}
		}
	}
	p := New(tokens)
	prog, perr := p.Parse()
	if perr != nil {
		return nil, perr
	}
	if len(p.errs) > 0 {
		return nil, p.errs[0]
	}
	return prog, nil
}

func (p *Parser) peek() lexer.Token {
	if p.pos >= len(p.tokens) {
		return lexer.Token{Type: lexer.TEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) peekAt(off int) lexer.Token {
	if p.pos+off >= len(p.tokens) {
		return lexer.Token{Type: lexer.TEOF}
	}
	return p.tokens[p.pos+off]
}

func (p *Parser) advance() lexer.Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *Parser) skipNewlines() {
	for p.peek().Type == lexer.TNewline {
		p.advance()
	}
}

func (p *Parser) skipCommentsAndNewlines() {
	for {
		switch p.peek().Type {
		case lexer.TNewline, lexer.TComment:
			p.advance()
		default:
			return
		}
	}
}

func (p *Parser) expect(typ lexer.TokenType, what string) (lexer.Token, error) {
	tok := p.peek()
	if tok.Type != typ {
		return tok, &Error{Line: tok.Line, Col: tok.Col, Msg: fmt.Sprintf("expected %s", what), Expected: []lexer.TokenType{typ}}
	}
	return p.advance(), nil
}

func (p *Parser) parseValue() (*ast.Node, error) {
	tok := p.peek()
	switch tok.Type {
	case lexer.TString:
		vtok := p.advance()
		n := ast.NewNode(ast.NodeStringLiteral, vtok.Line, vtok.Col)
		n.Value = vtok.Value
		return n, nil
	case lexer.TNumber:
		vtok := p.advance()
		n := ast.NewNode(ast.NodeNumberLiteral, vtok.Line, vtok.Col)
		n.Value = vtok.Value
		n.NumVal, _ = strconv.ParseFloat(vtok.Value, 64)
		return n, nil
	case lexer.TTrue:
		p.advance()
		n := ast.NewNode(ast.NodeBoolLiteral, tok.Line, tok.Col)
		n.BoolVal = true
		n.Value = "true"
		return n, nil
	case lexer.TFalse:
		p.advance()
		n := ast.NewNode(ast.NodeBoolLiteral, tok.Line, tok.Col)
		n.BoolVal = false
		n.Value = "false"
		return n, nil
	case lexer.TLBrace:
		return p.parseObjectLiteral()
	case lexer.TLBrack:
		return p.parseArrayLiteral()
	case lexer.TIdent:
		return p.parseIdent(), nil
	}
	return nil, &Error{Line: tok.Line, Col: tok.Col, Msg: "expected a value", Expected: []lexer.TokenType{lexer.TString, lexer.TNumber, lexer.TLBrace, lexer.TLBrack, lexer.TIdent}}
}

func (p *Parser) parseIdent() *ast.Node {
	tok := p.advance()
	n := ast.NewNode(ast.NodeIdentifier, tok.Line, tok.Col)
	n.Value = tok.Value
	return n
}

func (p *Parser) parseObjectLiteral() (*ast.Node, error) {
	tok := p.advance()
	n := ast.NewNode(ast.NodeObjectLiteral, tok.Line, tok.Col)
	n.MapVal = make(map[string]*ast.Node)
	p.skipNewlines()
	for p.peek().Type != lexer.TRBrace && p.peek().Type != lexer.TEOF {
		keyTok := p.peek()
		if keyTok.Type != lexer.TIdent && keyTok.Type != lexer.TString {
			return n, &Error{Line: keyTok.Line, Col: keyTok.Col, Msg: "expected object key"}
		}
		key := p.advance().Value
		if _, err := p.expect(lexer.TColon, "':' after object key"); err != nil {
			return n, err
		}
		val, err := p.parseValue()
		if err != nil {
			return n, err
		}
		n.MapVal[key] = val
		p.skipNewlines()
		if p.peek().Type == lexer.TComma {
			p.advance()
			p.skipNewlines()
		}
	}
	if p.peek().Type == lexer.TRBrace {
		p.advance()
	}
	return n, nil
}

func (p *Parser) parseArrayLiteral() (*ast.Node, error) {
	tok := p.advance()
	n := ast.NewNode(ast.NodeArrayLiteral, tok.Line, tok.Col)
	n.ArrVal = make([]*ast.Node, 0)
	p.skipNewlines()
	for p.peek().Type != lexer.TRBrack && p.peek().Type != lexer.TEOF {
		val, err := p.parseValue()
		if err != nil {
			return n, err
		}
		n.ArrVal = append(n.ArrVal, val)
		p.skipNewlines()
		if p.peek().Type == lexer.TComma {
			p.advance()
			p.skipNewlines()
		}
	}
	if p.peek().Type == lexer.TRBrack {
		p.advance()
	}
	return n, nil
}

func (p *Parser) parseNamedArgs() (map[string]*ast.Node, error) {
	attrs := make(map[string]*ast.Node)
	if p.peek().Type != lexer.TLBrace {
		return attrs, nil
	}
	p.advance()
	p.skipNewlines()
	for p.peek().Type != lexer.TRBrace && p.peek().Type != lexer.TEOF {
		keyTok := p.peek()
		if keyTok.Type != lexer.TIdent {
			return attrs, &Error{Line: keyTok.Line, Col: keyTok.Col, Msg: "expected named argument"}
		}
		key := p.advance().Value
		if _, err := p.expect(lexer.TColon, "':' after argument name"); err != nil {
			return attrs, err
		}
		val, err := p.parseValue()
		if err != nil {
			return attrs, err
		}
		attrs[key] = val
		p.skipNewlines()
		if p.peek().Type == lexer.TComma {
			p.advance()
			p.skipNewlines()
		}
	}
	if p.peek().Type == lexer.TRBrace {
		p.advance()
	}
	return attrs, nil
}

func (p *Parser) parseSkillCall(nameTok lexer.Token) (*ast.Node, error) {
	n := ast.NewNode(ast.NodeSkillCall, nameTok.Line, nameTok.Col)
	n.Value = nameTok.Value
	n.Args = make([]*ast.Node, 0)
	if p.peek().Type == lexer.TLParen {
		p.advance()
		p.skipNewlines()
		for p.peek().Type != lexer.TRParen && p.peek().Type != lexer.TEOF {
			arg, err := p.parseValue()
			if err != nil {
				return n, err
			}
			n.Args = append(n.Args, arg)
			p.skipNewlines()
			if p.peek().Type == lexer.TComma {
				p.advance()
				p.skipNewlines()
			}
		}
		if _, err := p.expect(lexer.TRParen, "')' to close argument list"); err != nil {
			return n, err
		}
	}
	if p.peek().Type == lexer.TLBrace {
		attrs, err := p.parseNamedArgs()
		if err != nil {
			return n, err
		}
		n.Attrs = attrs
	}
	return n, nil
}

func (p *Parser) parseRememberStmt() (*ast.Node, error) {
	tok := p.advance()
	n := ast.NewNode(ast.NodeRememberStmt, tok.Line, tok.Col)
	if p.peek().Type == lexer.TIdent {
		n.Children = append(n.Children, p.parseIdent())
	}
	if p.peek().Type == lexer.TEquals {
		p.advance()
		val, err := p.parseValue()
		if err != nil {
			return n, err
		}
		n.Children = append(n.Children, val)
	}
	if p.peek().Type == lexer.TLBrace {
		attrs, err := p.parseNamedArgs()
		if err != nil {
			return n, err
		}
		n.Attrs = attrs
	}
	return n, nil
}

func (p *Parser) parseRecallStmt() (*ast.Node, error) {
	tok := p.advance()
	n := ast.NewNode(ast.NodeRecallStmt, tok.Line, tok.Col)
	if p.peek().Type == lexer.TIdent {
		n.Children = append(n.Children, p.parseIdent())
	}
	if p.peek().Type == lexer.TLBrace {
		attrs, err := p.parseNamedArgs()
		if err != nil {
			return n, err
		}
		n.Attrs = attrs
	}
	return n, nil
}

func (p *Parser) parseForgetStmt() (*ast.Node, error) {
	tok := p.advance()
	n := ast.NewNode(ast.NodeForgetStmt, tok.Line, tok.Col)
	if p.peek().Type == lexer.TIdent {
		n.Children = append(n.Children, p.parseIdent())
	}
	if p.peek().Type == lexer.TLBrace {
		attrs, err := p.parseNamedArgs()
		if err != nil {
			return n, err
		}
		n.Attrs = attrs
	}
	return n, nil
}

func (p *Parser) parseRelateStmt() (*ast.Node, error) {
	tok := p.advance()
	n := ast.NewNode(ast.NodeRelateStmt, tok.Line, tok.Col)
	src := p.parseIdent()
	n.Children = append(n.Children, src)
	if p.peek().Type == lexer.TArrow {
		p.advance()
	}
	tgt := p.parseIdent()
	n.Children = append(n.Children, tgt)
	if p.peek().Type == lexer.TLBrace {
		attrs, err := p.parseNamedArgs()
		if err != nil {
			return n, err
		}
		n.Attrs = attrs
	}
	return n, nil
}

func (p *Parser) parseNodeDef() (*ast.Node, error) {
	tok := p.advance()
	n := ast.NewNode(ast.NodeNode, tok.Line, tok.Col)
	if p.peek().Type == lexer.TString {
		nameNode, err := p.parseValue()
		if err != nil {
			return n, err
		}
		n.Children = append(n.Children, nameNode)
		n.Value = nameNode.Value
	} else if p.peek().Type == lexer.TIdent {
		nameNode := p.parseIdent()
		n.Children = append(n.Children, nameNode)
		n.Value = nameNode.Value
	} else {
		return n, &Error{Line: p.peek().Line, Col: p.peek().Col, Msg: "expected node name after 'node'"}
	}
	if p.peek().Type == lexer.TArrow {
		p.advance()
		if p.peek().Type == lexer.TIdent {
			nameTok := p.peek()
			next := p.peekAt(1)
			if next.Type == lexer.TLParen || next.Type == lexer.TLBrace {
				call, err := p.parseSkillCall(nameTok)
				if err != nil {
					return n, err
				}
				n.Children = append(n.Children, call)
			} else {
				n.Children = append(n.Children, p.parseIdent())
			}
		} else if p.peek().Type == lexer.TLBrack {
			p.advance()
			p.skipNewlines()
			arr := ast.NewNode(ast.NodeArrayLiteral, tok.Line, tok.Col)
			arr.ArrVal = make([]*ast.Node, 0)
			for p.peek().Type != lexer.TRBrack && p.peek().Type != lexer.TEOF {
				if p.peek().Type != lexer.TIdent {
					return n, &Error{Line: p.peek().Line, Col: p.peek().Col, Msg: "expected node name in fork list"}
				}
				arr.ArrVal = append(arr.ArrVal, p.parseIdent())
				p.skipNewlines()
				if p.peek().Type == lexer.TComma {
					p.advance()
					p.skipNewlines()
				}
			}
			if p.peek().Type == lexer.TRBrack {
				p.advance()
			}
			n.Children = append(n.Children, arr)
		} else {
			return n, &Error{Line: p.peek().Line, Col: p.peek().Col, Msg: "expected skill call or node reference after '->'"}
		}
	}
	if p.peek().Type == lexer.TLBrace {
		saved := p.pos
		p.advance()
		p.skipNewlines()
		if p.peek().Type == lexer.TIdent && (p.peek().Value == "if" || p.peek().Value == "else") {
			p.pos = saved
			attrs, err := p.parseNamedArgs()
			if err != nil {
				return n, err
			}
			n.Attrs = attrs
		} else {
			p.pos = saved
			block, err := p.parseBlock()
			if err != nil {
				return n, err
			}
			n.Nodes = block
		}
	}
	return n, nil
}

func (p *Parser) parseBlock() ([]*ast.Node, error) {
	nodes := make([]*ast.Node, 0)
	if p.peek().Type != lexer.TLBrace {
		return nodes, nil
	}
	p.advance()
	p.skipNewlines()
	for p.peek().Type != lexer.TRBrace && p.peek().Type != lexer.TEOF {
		tok := p.peek()
		switch {
		case tok.Type == lexer.TIdent && tok.Value == "skill":
			p.advance()
			if p.peek().Type == lexer.TIdent {
				call, err := p.parseSkillCall(p.peek())
				if err != nil {
					return nodes, err
				}
				nodes = append(nodes, call)
			} else {
				return nodes, &Error{Line: p.peek().Line, Col: p.peek().Col, Msg: "expected skill name after 'skill'"}
			}
			p.skipNewlines()
		case tok.Type == lexer.TIdent && tok.Value == "for":
			p.advance()
			if p.peek().Type == lexer.TIdent && p.peek().Value == "each" {
				p.advance()
			}
			forNode := ast.NewNode(ast.NodeIdentifier, tok.Line, tok.Col)
			forNode.Value = "for_each"
			if p.peek().Type == lexer.TIdent {
				forNode.Children = append(forNode.Children, p.parseIdent())
			}
			if p.peek().Type == lexer.TIdent && p.peek().Value == "in" {
				p.advance()
				forNode.Children = append(forNode.Children, p.parseIdent())
			}
			if p.peek().Type == lexer.TLBrace {
				block, err := p.parseBlock()
				if err != nil {
					return nodes, err
				}
				forNode.Nodes = block
			}
			nodes = append(nodes, forNode)
			p.skipNewlines()
		case tok.Type == lexer.TNewline || tok.Type == lexer.TComment:
			p.advance()
			p.skipNewlines()
		default:
			p.skipNewlines()
			if p.peek().Type == lexer.TRBrace || p.peek().Type == lexer.TEOF {
				break
			}
			p.advance()
			p.skipNewlines()
		}
	}
	if p.peek().Type == lexer.TRBrace {
		p.advance()
	}
	return nodes, nil
}

func (p *Parser) parseFlowBody(flowTok lexer.Token) (*ast.Node, error) {
	flow := ast.NewNode(ast.NodeFlow, flowTok.Line, flowTok.Col)
	if p.peek().Type != lexer.TLBrace {
		return flow, &Error{Line: p.peek().Line, Col: p.peek().Col, Msg: "expected '{' for flow body", Expected: []lexer.TokenType{lexer.TLBrace}}
	}
	p.advance()
	p.skipNewlines()

	for p.peek().Type != lexer.TRBrace && p.peek().Type != lexer.TEOF {
		tok := p.peek()
		switch {
		case tok.Type == lexer.TIdent && tok.Value == "node":
			node, err := p.parseNodeDef()
			if err != nil {
				return flow, err
			}
			flow.Nodes = append(flow.Nodes, node)
			p.skipNewlines()
		case tok.Type == lexer.TIdent && tok.Value == "input":
			p.advance()
			in := ast.NewNode(ast.NodeInput, tok.Line, tok.Col)
			if p.peek().Type == lexer.TIdent {
				in.Children = append(in.Children, p.parseIdent())
			}
			if p.peek().Type == lexer.TColon {
				p.advance()
				if p.peek().Type == lexer.TIdent {
					typeNode := p.parseIdent()
					typeNode.Value = strings.ToLower(typeNode.Value)
					in.Children = append(in.Children, typeNode)
				}
			}
			flow.Children = append(flow.Children, in)
			p.skipNewlines()
		case tok.Type == lexer.TIdent && tok.Value == "agent":
			p.advance()
			ag := ast.NewNode(ast.NodeAgentDecl, tok.Line, tok.Col)
			if p.peek().Type == lexer.TString {
				nameNode, err := p.parseValue()
				if err != nil {
					return flow, err
				}
				ag.Children = append(ag.Children, nameNode)
			} else if p.peek().Type == lexer.TIdent {
				ag.Children = append(ag.Children, p.parseIdent())
			}
			if p.peek().Type == lexer.TLBrace {
				attrs, err := p.parseNamedArgs()
				if err != nil {
					return flow, err
				}
				ag.Attrs = attrs
			}
			flow.Children = append(flow.Children, ag)
			p.skipNewlines()
		case tok.Type == lexer.TIdent && tok.Value == "on":
			p.advance()
			tr := ast.NewNode(ast.NodeTrigger, tok.Line, tok.Col)
			if p.peek().Type == lexer.TString {
				ev, err := p.parseValue()
				if err != nil {
					return flow, err
				}
				tr.Children = append(tr.Children, ev)
			}
			if p.peek().Type == lexer.TLBrace {
				attrs, err := p.parseNamedArgs()
				if err != nil {
					return flow, err
				}
				tr.Attrs = attrs
			}
			p.skipNewlines()
			if p.peek().Type == lexer.TArrow {
				p.advance()
				p.skipNewlines()
				if p.peek().Type == lexer.TLBrack {
					arr, err := p.parseArrayLiteral()
					if err != nil {
						return flow, err
					}
					tr.Children = append(tr.Children, arr)
				} else if p.peek().Type == lexer.TIdent {
					tr.Children = append(tr.Children, p.parseIdent())
				} else {
					return flow, &Error{Line: p.peek().Line, Col: p.peek().Col, Msg: "expected target node after trigger arrow"}
				}
			}
			flow.Children = append(flow.Children, tr)
			p.skipNewlines()
		case tok.Type == lexer.TNewline || tok.Type == lexer.TComment:
			p.advance()
			p.skipNewlines()
		case tok.Type == lexer.TIdent:
			name := tok.Value
			p.advance()
			p.skipNewlines()
			if p.peek().Type == lexer.TArrow {
				e := ast.NewNode(ast.NodeEdge, tok.Line, tok.Col)
				src := ast.NewNode(ast.NodeIdentifier, tok.Line, tok.Col)
				src.Value = name
				e.Children = append(e.Children, src)
				p.advance()
				p.skipNewlines()
				if p.peek().Type == lexer.TLBrack {
					arr, err := p.parseArrayLiteral()
					if err != nil {
						return flow, err
					}
					e.Children = append(e.Children, arr)
				} else if p.peek().Type == lexer.TIdent {
					e.Children = append(e.Children, p.parseIdent())
				} else {
					return flow, &Error{Line: p.peek().Line, Col: p.peek().Col, Msg: "expected target node after edge arrow"}
				}
				if p.peek().Type == lexer.TLBrace {
					attrs, err := p.parseNamedArgs()
					if err != nil {
						return flow, err
					}
					e.Attrs = attrs
				}
				flow.Edges = append(flow.Edges, e)
				p.skipNewlines()
			} else {
				flow.Children = append(flow.Children, ast.NewNode(ast.NodeIdentifier, tok.Line, tok.Col))
				p.skipNewlines()
			}
		case tok.Type == lexer.TIdent && tok.Value == "remember":
			stmt, err := p.parseRememberStmt()
			if err != nil {
				return flow, err
			}
			flow.Children = append(flow.Children, stmt)
			p.skipNewlines()
		case tok.Type == lexer.TIdent && tok.Value == "recall":
			stmt, err := p.parseRecallStmt()
			if err != nil {
				return flow, err
			}
			flow.Children = append(flow.Children, stmt)
			p.skipNewlines()
		default:
			p.advance()
			p.skipNewlines()
		}
	}
	if p.peek().Type == lexer.TRBrace {
		p.advance()
	}
	return flow, nil
}

func (p *Parser) parseContextBlock(tok lexer.Token) (*ast.Node, error) {
	ctx := ast.NewNode(ast.NodeContextBlock, tok.Line, tok.Col)
	if p.peek().Type == lexer.TString {
		nameNode, err := p.parseValue()
		if err != nil {
			return ctx, err
		}
		ctx.Children = append(ctx.Children, nameNode)
	}
	if p.peek().Type == lexer.TLBrace {
		p.advance()
		p.skipNewlines()
		for p.peek().Type != lexer.TRBrace && p.peek().Type != lexer.TEOF {
			switch {
			case p.peek().Type == lexer.TIdent && p.peek().Value == "remember":
				stmt, err := p.parseRememberStmt()
				if err != nil {
					return ctx, err
				}
				ctx.Children = append(ctx.Children, stmt)
			case p.peek().Type == lexer.TIdent && p.peek().Value == "flow":
				ftok := p.advance()
				fn := ""
				if p.peek().Type == lexer.TString {
					fnNode, err := p.parseValue()
					if err != nil {
						return ctx, err
					}
					fn = fnNode.Value
				}
				fb, err := p.parseFlowBody(ftok)
				if err != nil {
					return ctx, err
				}
				fb.Value = fn
				ctx.Children = append(ctx.Children, fb)
			case p.peek().Type == lexer.TIdent && p.peek().Value == "recall":
				stmt, err := p.parseRecallStmt()
				if err != nil {
					return ctx, err
				}
				ctx.Children = append(ctx.Children, stmt)
			default:
				p.advance()
			}
			p.skipNewlines()
		}
		if p.peek().Type == lexer.TRBrace {
			p.advance()
		}
	}
	return ctx, nil
}

func (p *Parser) parseAutoSummarize(tok lexer.Token) (*ast.Node, error) {
	as := ast.NewNode(ast.NodeAutoSummarize, tok.Line, tok.Col)
	if p.peek().Type == lexer.TLParen {
		p.advance()
		p.skipNewlines()
		as.Attrs = make(map[string]*ast.Node)
		for p.peek().Type != lexer.TRParen && p.peek().Type != lexer.TEOF {
			if p.peek().Type == lexer.TIdent {
				key := p.advance().Value
				if p.peek().Type == lexer.TColon {
					p.advance()
					val, err := p.parseValue()
					if err != nil {
						return as, err
					}
					as.Attrs[key] = val
				}
			}
			p.skipNewlines()
			if p.peek().Type == lexer.TComma {
				p.advance()
				p.skipNewlines()
			}
		}
		if p.peek().Type == lexer.TRParen {
			p.advance()
		}
	}
	return as, nil
}

// Parse parses a full TAC program into a Program AST node.
func (p *Parser) Parse() (*ast.Node, error) {
	program := ast.NewNode(ast.NodeProgram, 1, 1)
	p.skipNewlines()
	for p.peek().Type != lexer.TEOF {
		tok := p.peek()
		switch {
		case tok.Type == lexer.TNewline || tok.Type == lexer.TComment:
			p.advance()
			p.skipNewlines()
		case tok.Type == lexer.TIdent && tok.Value == "flow":
			ftok := p.advance()
			flowName := ""
			if p.peek().Type == lexer.TString {
				flowNameNode, err := p.parseValue()
				if err != nil {
					return nil, err
				}
				flowName = flowNameNode.Value
			}
			flowBody, err := p.parseFlowBody(ftok)
			if err != nil {
				return nil, err
			}
			flowBody.Value = flowName
			program.Nodes = append(program.Nodes, flowBody)
			p.skipNewlines()
		case tok.Type == lexer.TIdent && tok.Value == "context":
			p.advance()
			ctx, err := p.parseContextBlock(tok)
			if err != nil {
				return nil, err
			}
			program.Nodes = append(program.Nodes, ctx)
			p.skipNewlines()
		case tok.Type == lexer.TIdent && tok.Value == "remember":
			stmt, err := p.parseRememberStmt()
			if err != nil {
				return nil, err
			}
			program.Nodes = append(program.Nodes, stmt)
			p.skipNewlines()
		case tok.Type == lexer.TIdent && tok.Value == "recall":
			stmt, err := p.parseRecallStmt()
			if err != nil {
				return nil, err
			}
			program.Nodes = append(program.Nodes, stmt)
			p.skipNewlines()
		case tok.Type == lexer.TIdent && tok.Value == "relate":
			stmt, err := p.parseRelateStmt()
			if err != nil {
				return nil, err
			}
			program.Nodes = append(program.Nodes, stmt)
			p.skipNewlines()
		case tok.Type == lexer.TIdent && tok.Value == "forget":
			stmt, err := p.parseForgetStmt()
			if err != nil {
				return nil, err
			}
			program.Nodes = append(program.Nodes, stmt)
			p.skipNewlines()
		case tok.Type == lexer.TIdent && tok.Value == "auto_summarize":
			as, err := p.parseAutoSummarize(tok)
			if err != nil {
				return nil, err
			}
			program.Nodes = append(program.Nodes, as)
			p.skipNewlines()
		default:
			p.advance()
			p.skipNewlines()
		}
	}
	return program, nil
}
