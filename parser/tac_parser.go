// tac_parser.go — TAC Language Parser
//
// Reads .tac source files and outputs an AST as JSON.
//
// Usage:
//   go run tac_parser.go input.tac              # print AST to stdout
//   go run tac_parser.go input.tac --output ast.json
//
// Build:
//   go build -o tac-parser tac_parser.go
//
// TAC is the TacFlow Agentic Code — a DSL for autonomous AI agents.
// Specification: TAC_Language_Specification.md
//
// (c) 2026 TacFlow — MIT License

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"strconv"
	"strings"
)

// ============================================================================
// AST Node Types
// ============================================================================

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
	Nodes    []*Node           `json:"nodes,omitempty"`    // For flow/context bodies
	Edges    []*Node           `json:"edges,omitempty"`    // For flow edges
	Args     []*Node           `json:"args,omitempty"`     // For skill calls
	Attrs    map[string]*Node  `json:"attrs,omitempty"`    // For blocks with named attributes
	MapVal   map[string]*Node  `json:"map_val,omitempty"`  // For object literals
	ArrVal   []*Node           `json:"arr_val,omitempty"`  // For array literals
}

// NewNode creates a new AST node.
func NewNode(typ NodeType, line, col int) *Node {
	return &Node{
		Type:     typ,
		Pos:      Position{Line: line, Col: col},
		Children: make([]*Node, 0),
	}
}

// ============================================================================
// Tokenizer
// ============================================================================

// TokenType represents a lexical token type.
type TokenType int

const (
	TEOF TokenType = iota
	TIdent
	TString
	TNumber
	TTrue
	TFalse
	TArrow
	TEquals
	TDot
	TComma
	TColon
	TLBrace
	TRBrace
	TLParen
	TRParen
	TLBrack
	TRBrack
	TNewline
	TComment
	TError
)

var keywords = map[string]TokenType{
	"true":  TTrue,
	"false": TFalse,
}

var reservedWords = map[string]bool{
	"remember": true, "recall": true, "forget": true,
	"context": true, "auto_summarize": true, "on": true, "input": true,
	"agent": true, "if": true, "else": true, "search_hybrid": true,
	"set_token_limit": true, "swarm_teach": true, "agent_task": true,
	"agent_wait": true, "swarm_status": true, "verify": true,
	"validate": true, "config_get": true, "config_set": true,
	"graph_search": true, "graph_relate": true, "memory_search": true,
	"memory_store": true, "web_search": true, "llm": true, "tts": true,
	"each": true, "in": true,
}

// Token represents a single lexical token.
type Token struct {
	Type  TokenType
	Value string
	Line  int
	Col   int
}

// Tokenizer converts source text into a token stream.
type Tokenizer struct {
	source []rune
	pos    int
	line   int
	col    int
	tokens []Token
}

func NewTokenizer(source string) *Tokenizer {
	return &Tokenizer{
		source: []rune(source),
		pos:    0,
		line:   1,
		col:    1,
		tokens: make([]Token, 0),
	}
}

func (t *Tokenizer) peek() rune {
	if t.pos >= len(t.source) {
		return 0
	}
	return t.source[t.pos]
}

func (t *Tokenizer) advance() rune {
	if t.pos >= len(t.source) {
		return 0
	}
	ch := t.source[t.pos]
	t.pos++
	if ch == '\n' {
		t.line++
		t.col = 1
	} else {
		t.col++
	}
	return ch
}

func (t *Tokenizer) skipWhitespace() {
	for t.pos < len(t.source) {
		ch := t.source[t.pos]
		if ch == ' ' || ch == '\t' || ch == '\r' {
			t.advance()
		} else {
			break
		}
	}
}

func (t *Tokenizer) addToken(typ TokenType, val string) {
	t.tokens = append(t.tokens, Token{
		Type:  typ,
		Value: val,
		Line:  t.line,
		Col:   t.col,
	})
}

func (t *Tokenizer) scanString() TokenType {
	var buf strings.Builder
	for t.pos < len(t.source) {
		ch := t.advance()
		if ch == '\\' && t.pos < len(t.source) {
			next := t.advance()
			switch next {
			case 'n':
				buf.WriteByte('\n')
			case 't':
				buf.WriteByte('\t')
			case '\\':
				buf.WriteByte('\\')
			case '"':
				buf.WriteByte('"')
			default:
				buf.WriteByte('\\')
				buf.WriteRune(next)
			}
		} else if ch == '"' {
			t.addToken(TString, buf.String())
			return TString
		} else if ch == '\n' {
			t.addToken(TError, "unterminated string")
			return TError
		} else {
			buf.WriteRune(ch)
		}
	}
	t.addToken(TError, "unterminated string")
	return TError
}

func (t *Tokenizer) scanIdentifier() TokenType {
	start := t.pos - 1
	for t.pos < len(t.source) {
		ch := t.source[t.pos]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '_' || ch == '.' {
			t.advance()
		} else {
			break
		}
	}
	word := string(t.source[start:t.pos])
	if kw, ok := keywords[word]; ok {
		t.addToken(kw, word)
		return kw
	}
	t.addToken(TIdent, word)
	return TIdent
}

func (t *Tokenizer) scanNumber() TokenType {
	start := t.pos - 1
	isFloat := false
	for t.pos < len(t.source) {
		ch := t.source[t.pos]
		if ch >= '0' && ch <= '9' {
			t.advance()
		} else if ch == '.' && !isFloat {
			isFloat = true
			t.advance()
		} else {
			break
		}
	}
	t.addToken(TNumber, string(t.source[start:t.pos]))
	return TNumber
}

func (t *Tokenizer) Tokenize() ([]Token, error) {
	for t.pos < len(t.source) {
		ch := t.peek()
		switch {
		case ch == ' ' || ch == '\t' || ch == '\r':
			t.advance()
		case ch == '\n':
			t.advance()
			t.skipWhitespace()
			t.addToken(TNewline, "\n")
		case ch == '"':
			t.advance()
			t.scanString()
		case ch == '/' && t.pos+1 < len(t.source) && t.source[t.pos+1] == '/':
			for t.pos < len(t.source) && t.source[t.pos] != '\n' {
				t.advance()
			}
			t.addToken(TComment, "")
		case ch == '-' && t.pos+1 < len(t.source) && t.source[t.pos+1] == '>':
			t.advance()
			t.advance()
			t.addToken(TArrow, "->")
		case ch == '=':
			t.advance()
			t.addToken(TEquals, "=")
		case ch == ',':
			t.advance()
			t.addToken(TComma, ",")
		case ch == ':':
			t.advance()
			t.addToken(TColon, ":")
		case ch == '{':
			t.advance()
			t.addToken(TLBrace, "{")
		case ch == '}':
			t.advance()
			t.addToken(TRBrace, "}")
		case ch == '(':
			t.advance()
			t.addToken(TLParen, "(")
		case ch == ')':
			t.advance()
			t.addToken(TRParen, ")")
		case ch == '[':
			t.advance()
			t.addToken(TLBrack, "[")
		case ch == ']':
			t.advance()
			t.addToken(TRBrack, "]")
		case ch == '.':
			t.advance()
			t.addToken(TDot, ".")
		case (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_':
			t.advance()
			t.scanIdentifier()
		case ch >= '0' && ch <= '9':
			t.advance()
			t.scanNumber()
		default:
			t.addToken(TError, fmt.Sprintf("unexpected character: %c", ch))
			t.advance()
		}
	}
	t.addToken(TEOF, "")
	return t.tokens, nil
}

// ============================================================================
// Parser
// ============================================================================

type Parser struct {
	tokens []Token
	pos    int
}

func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens, pos: 0}
}

func (p *Parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *Parser) skipNewlines() {
	for p.peek().Type == TNewline {
		p.advance()
	}
}

func (p *Parser) parseValue() *Node {
	tok := p.peek()
	switch tok.Type {
	case TString:
		vtok := p.advance()
		n := NewNode(NodeStringLiteral, vtok.Line, vtok.Col)
		n.Value = vtok.Value
		return n
	case TNumber:
		vtok := p.advance()
		n := NewNode(NodeNumberLiteral, vtok.Line, vtok.Col)
		n.Value = vtok.Value
		n.NumVal, _ = strconv.ParseFloat(vtok.Value, 64)
		return n
	case TTrue:
		p.advance()
		n := NewNode(NodeBoolLiteral, tok.Line, tok.Col)
		n.BoolVal = true
		n.Value = "true"
		return n
	case TFalse:
		p.advance()
		n := NewNode(NodeBoolLiteral, tok.Line, tok.Col)
		n.BoolVal = false
		n.Value = "false"
		return n
	case TLBrace:
		return p.parseObjectLiteral()
	case TLBrack:
		return p.parseArrayLiteral()
	case TIdent:
		return p.parseIdent()
	}
	return nil
}

func (p *Parser) parseIdent() *Node {
	tok := p.advance()
	n := NewNode(NodeIdentifier, tok.Line, tok.Col)
	n.Value = tok.Value
	return n
}

func (p *Parser) parseObjectLiteral() *Node {
	tok := p.advance()
	n := NewNode(NodeObjectLiteral, tok.Line, tok.Col)
	n.MapVal = make(map[string]*Node)
	p.skipNewlines()
	for p.peek().Type != TRBrace && p.peek().Type != TEOF {
		keyTok := p.peek()
		if keyTok.Type != TIdent && keyTok.Type != TString {
			break
		}
		key := p.advance().Value
		if p.peek().Type == TColon {
			p.advance()
		} else {
			break
		}
		val := p.parseValue()
		if val != nil {
			n.MapVal[key] = val
		}
		p.skipNewlines()
		if p.peek().Type == TComma {
			p.advance()
			p.skipNewlines()
		}
	}
	if p.peek().Type == TRBrace {
		p.advance()
	}
	return n
}

func (p *Parser) parseArrayLiteral() *Node {
	tok := p.advance()
	n := NewNode(NodeArrayLiteral, tok.Line, tok.Col)
	n.ArrVal = make([]*Node, 0)
	p.skipNewlines()
	for p.peek().Type != TRBrack && p.peek().Type != TEOF {
		val := p.parseValue()
		if val != nil {
			n.ArrVal = append(n.ArrVal, val)
		}
		p.skipNewlines()
		if p.peek().Type == TComma {
			p.advance()
			p.skipNewlines()
		}
	}
	if p.peek().Type == TRBrack {
		p.advance()
	}
	return n
}

func (p *Parser) parseNamedArgs() map[string]*Node {
	attrs := make(map[string]*Node)
	if p.peek().Type != TLBrace {
		return attrs
	}
	p.advance()
	p.skipNewlines()
	for p.peek().Type != TRBrace && p.peek().Type != TEOF {
		keyTok := p.peek()
		if keyTok.Type != TIdent {
			break
		}
		key := p.advance().Value
		if p.peek().Type == TColon {
			p.advance()
		} else {
			break
		}
		val := p.parseValue()
		if val != nil {
			attrs[key] = val
		}
		p.skipNewlines()
		if p.peek().Type == TComma {
			p.advance()
			p.skipNewlines()
		}
	}
	if p.peek().Type == TRBrace {
		p.advance()
	}
	return attrs
}

func (p *Parser) parseSkillCall() *Node {
	nameTok := p.tokens[p.pos-1]
	n := NewNode(NodeSkillCall, nameTok.Line, nameTok.Col)
	n.Value = nameTok.Value
	n.Args = make([]*Node, 0)
	if p.peek().Type == TLParen {
		p.advance()
		p.skipNewlines()
		for p.peek().Type != TRParen && p.peek().Type != TEOF {
			arg := p.parseValue()
			if arg != nil {
				n.Args = append(n.Args, arg)
			}
			p.skipNewlines()
			if p.peek().Type == TComma {
				p.advance()
				p.skipNewlines()
			}
		}
		if p.peek().Type == TRParen {
			p.advance()
		}
	}
	if p.peek().Type == TLBrace {
		n.Attrs = p.parseNamedArgs()
	}
	return n
}

func (p *Parser) parseRememberStmt() *Node {
	tok := p.advance()
	n := NewNode(NodeRememberStmt, tok.Line, tok.Col)
	if p.peek().Type == TIdent {
		n.Children = append(n.Children, p.parseIdent())
	}
	if p.peek().Type == TEquals {
		p.advance()
		val := p.parseValue()
		if val != nil {
			n.Children = append(n.Children, val)
		}
	}
	if p.peek().Type == TLBrace {
		n.Attrs = p.parseNamedArgs()
	}
	return n
}

func (p *Parser) parseRecallStmt() *Node {
	tok := p.advance()
	n := NewNode(NodeRecallStmt, tok.Line, tok.Col)
	if p.peek().Type == TIdent {
		n.Children = append(n.Children, p.parseIdent())
	}
	if p.peek().Type == TLBrace {
		n.Attrs = p.parseNamedArgs()
	}
	return n
}

func (p *Parser) parseForgetStmt() *Node {
	tok := p.advance()
	n := NewNode(NodeForgetStmt, tok.Line, tok.Col)
	if p.peek().Type == TIdent {
		n.Children = append(n.Children, p.parseIdent())
	}
	if p.peek().Type == TLBrace {
		n.Attrs = p.parseNamedArgs()
	}
	return n
}

func (p *Parser) parseRelateStmt() *Node {
	tok := p.advance()
	n := NewNode(NodeRelateStmt, tok.Line, tok.Col)
	src := p.parseIdent()
	n.Children = append(n.Children, src)
	if p.peek().Type == TArrow {
		p.advance()
	}
	tgt := p.parseIdent()
	n.Children = append(n.Children, tgt)
	if p.peek().Type == TLBrace {
		n.Attrs = p.parseNamedArgs()
	}
	return n
}

func (p *Parser) parseNodeDef() *Node {
	tok := p.advance()
	n := NewNode(NodeNode, tok.Line, tok.Col)
	if p.peek().Type == TString {
		nameNode := p.parseValue()
		if nameNode != nil {
			n.Children = append(n.Children, nameNode)
			n.Value = nameNode.Value
		}
	} else if p.peek().Type == TIdent {
		nameNode := p.parseIdent()
		n.Children = append(n.Children, nameNode)
		n.Value = nameNode.Value
	}
	if p.peek().Type == TArrow {
		p.advance()
		if p.peek().Type == TIdent {
			_ = p.peek()
			if p.pos+1 < len(p.tokens) &&
				(p.tokens[p.pos+1].Type == TLParen || p.tokens[p.pos+1].Type == TLBrace) {
				call := p.parseSkillCall()
				n.Children = append(n.Children, call)
			} else {
				n.Children = append(n.Children, p.parseIdent())
			}
		} else if p.peek().Type == TLBrack {
			p.advance()
			p.skipNewlines()
			arr := NewNode(NodeArrayLiteral, tok.Line, tok.Col)
			arr.ArrVal = make([]*Node, 0)
			for p.peek().Type != TRBrack && p.peek().Type != TEOF {
				arr.ArrVal = append(arr.ArrVal, p.parseIdent())
				p.skipNewlines()
				if p.peek().Type == TComma {
					p.advance()
					p.skipNewlines()
				}
			}
			if p.peek().Type == TRBrack {
				p.advance()
			}
			n.Children = append(n.Children, arr)
		}
	}
	if p.peek().Type == TLBrace {
		saved := p.pos
		p.advance()
		p.skipNewlines()
		if p.peek().Type == TIdent && (p.peek().Value == "if" || p.peek().Value == "else") {
			p.pos = saved
			n.Attrs = p.parseNamedArgs()
		} else {
			p.pos = saved
			n.Nodes = p.parseBlock()
		}
	}
	return n
}

func (p *Parser) parseBlock() []*Node {
	nodes := make([]*Node, 0)
	if p.peek().Type != TLBrace {
		return nodes
	}
	p.advance()
	p.skipNewlines()
	for p.peek().Type != TRBrace && p.peek().Type != TEOF {
		tok := p.peek()
		switch {
		case tok.Type == TIdent && tok.Value == "skill":
			p.advance()
			if p.peek().Type == TIdent {
				call := p.parseSkillCall()
				nodes = append(nodes, call)
			}
			p.skipNewlines()
		case tok.Type == TIdent && tok.Value == "for":
			p.advance()
			if p.peek().Type == TIdent && p.peek().Value == "each" {
				p.advance()
			}
			forNode := NewNode(NodeIdentifier, tok.Line, tok.Col)
			forNode.Value = "for_each"
			if p.peek().Type == TIdent {
				forNode.Children = append(forNode.Children, p.parseIdent())
			}
			if p.peek().Type == TIdent && p.peek().Value == "in" {
				p.advance()
				forNode.Children = append(forNode.Children, p.parseIdent())
			}
			if p.peek().Type == TLBrace {
				forNode.Nodes = p.parseBlock()
			}
			nodes = append(nodes, forNode)
			p.skipNewlines()
		case tok.Type == TNewline:
			p.advance()
			p.skipNewlines()
		case tok.Type == TComment:
			p.advance()
			p.skipNewlines()
		default:
			p.skipNewlines()
			if p.peek().Type == TRBrace || p.peek().Type == TEOF {
				break
			}
			p.advance()
			p.skipNewlines()
		}
	}
	if p.peek().Type == TRBrace {
		p.advance()
	}
	return nodes
}

func (p *Parser) parseFlowBody() (*Node, error) {
	flow := NewNode(NodeFlow, 0, 0)
	if p.peek().Type != TLBrace {
		return flow, fmt.Errorf("expected { for flow body")
	}
	p.advance()
	p.skipNewlines()

	for p.peek().Type != TRBrace && p.peek().Type != TEOF {
		tok := p.peek()
		switch {
		case tok.Type == TIdent && tok.Value == "node":
			node := p.parseNodeDef()
			flow.Nodes = append(flow.Nodes, node)
			p.skipNewlines()
		case tok.Type == TIdent && tok.Value == "input":
			p.advance()
			in := NewNode(NodeInput, tok.Line, tok.Col)
			if p.peek().Type == TIdent {
				in.Children = append(in.Children, p.parseIdent())
			}
			if p.peek().Type == TColon {
				p.advance()
				if p.peek().Type == TIdent {
					typeNode := p.parseIdent()
					typeNode.Value = strings.ToLower(typeNode.Value)
					in.Children = append(in.Children, typeNode)
				}
			}
			flow.Children = append(flow.Children, in)
			p.skipNewlines()
		case tok.Type == TIdent && tok.Value == "agent":
			p.advance()
			ag := NewNode(NodeAgentDecl, tok.Line, tok.Col)
			if p.peek().Type == TString {
				ag.Children = append(ag.Children, p.parseValue())
			} else if p.peek().Type == TIdent {
				ag.Children = append(ag.Children, p.parseIdent())
			}
			if p.peek().Type == TLBrace {
				ag.Attrs = p.parseNamedArgs()
			}
			flow.Children = append(flow.Children, ag)
			p.skipNewlines()
		case tok.Type == TIdent && tok.Value == "on":
			p.advance()
			tr := NewNode(NodeTrigger, tok.Line, tok.Col)
			if p.peek().Type == TString {
				tr.Children = append(tr.Children, p.parseValue())
			}
			if p.peek().Type == TLBrace {
				tr.Attrs = p.parseNamedArgs()
			}
			p.skipNewlines()
			if p.peek().Type == TArrow {
				p.advance()
				p.skipNewlines()
				if p.peek().Type == TLBrack {
					arr := p.parseArrayLiteral()
					tr.Children = append(tr.Children, arr)
				} else if p.peek().Type == TIdent {
					tr.Children = append(tr.Children, p.parseIdent())
				}
			}
			flow.Children = append(flow.Children, tr)
			p.skipNewlines()
		case tok.Type == TNewline || tok.Type == TComment:
			p.advance()
			p.skipNewlines()
		case tok.Type == TIdent:
			name := tok.Value
			p.advance()
			p.skipNewlines()
			if p.peek().Type == TArrow {
				e := NewNode(NodeEdge, tok.Line, tok.Col)
				src := NewNode(NodeIdentifier, tok.Line, tok.Col)
				src.Value = name
				e.Children = append(e.Children, src)
				p.advance()
				p.skipNewlines()
				if p.peek().Type == TLBrack {
					e.Children = append(e.Children, p.parseArrayLiteral())
				} else if p.peek().Type == TIdent {
					e.Children = append(e.Children, p.parseIdent())
				}
				if p.peek().Type == TLBrace {
					e.Attrs = p.parseNamedArgs()
				}
				flow.Edges = append(flow.Edges, e)
				p.skipNewlines()
			} else {
				flow.Children = append(flow.Children, NewNode(NodeIdentifier, tok.Line, tok.Col))
				p.skipNewlines()
			}
		case tok.Type == TIdent && tok.Value == "remember":
			flow.Children = append(flow.Children, p.parseRememberStmt())
			p.skipNewlines()
		case tok.Type == TIdent && tok.Value == "recall":
			flow.Children = append(flow.Children, p.parseRecallStmt())
			p.skipNewlines()
		default:
			p.advance()
			p.skipNewlines()
		}
	}
	if p.peek().Type == TRBrace {
		p.advance()
	}
	return flow, nil
}

func (p *Parser) Parse() (*Node, error) {
	program := NewNode(NodeProgram, 1, 1)
	p.skipNewlines()
	for p.peek().Type != TEOF {
		tok := p.peek()
		switch {
		case tok.Type == TNewline || tok.Type == TComment:
			p.advance()
			p.skipNewlines()
		case tok.Type == TIdent && tok.Value == "flow":
			p.advance()
			flowName := ""
			if p.peek().Type == TString {
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
			p.skipNewlines()
		case tok.Type == TIdent && tok.Value == "context":
			p.advance()
			ctx := NewNode(NodeContextBlock, tok.Line, tok.Col)
			if p.peek().Type == TString {
				ctx.Children = append(ctx.Children, p.parseValue())
			}
			if p.peek().Type == TLBrace {
				_ = p.pos
				p.advance()
				p.skipNewlines()
				// Parse nested statements as children
				for p.peek().Type != TRBrace && p.peek().Type != TEOF {
					if p.peek().Type == TIdent && p.peek().Value == "remember" {
						ctx.Children = append(ctx.Children, p.parseRememberStmt())
					} else if p.peek().Type == TIdent && p.peek().Value == "flow" {
						p.advance()
						fn := ""
						if p.peek().Type == TString {
							fnNode := p.parseValue()
							if fnNode != nil {
								fn = fnNode.Value
							}
						}
						fb, _ := p.parseFlowBody()
						fb.Value = fn
						ctx.Children = append(ctx.Children, fb)
					} else if p.peek().Type == TIdent && p.peek().Value == "recall" {
						ctx.Children = append(ctx.Children, p.parseRecallStmt())
					} else {
						p.advance()
					}
					p.skipNewlines()
				}
				if p.peek().Type == TRBrace {
					p.advance()
				}
			}
			program.Nodes = append(program.Nodes, ctx)
			p.skipNewlines()
		case tok.Type == TIdent && tok.Value == "remember":
			program.Nodes = append(program.Nodes, p.parseRememberStmt())
			p.skipNewlines()
		case tok.Type == TIdent && tok.Value == "recall":
			program.Nodes = append(program.Nodes, p.parseRecallStmt())
			p.skipNewlines()
		case tok.Type == TIdent && tok.Value == "relate":
			program.Nodes = append(program.Nodes, p.parseRelateStmt())
			p.skipNewlines()
		case tok.Type == TIdent && tok.Value == "forget":
			program.Nodes = append(program.Nodes, p.parseForgetStmt())
			p.skipNewlines()
		case tok.Type == TIdent && tok.Value == "auto_summarize":
			p.advance()
			as := NewNode(NodeAutoSummarize, tok.Line, tok.Col)
			if p.peek().Type == TLParen {
				p.advance()
				p.skipNewlines()
				as.Attrs = make(map[string]*Node)
				for p.peek().Type != TRParen && p.peek().Type != TEOF {
					if p.peek().Type == TIdent {
						key := p.advance().Value
						if p.peek().Type == TColon {
							p.advance()
							val := p.parseValue()
							if val != nil {
								as.Attrs[key] = val
							}
						}
					}
					p.skipNewlines()
					if p.peek().Type == TComma {
						p.advance()
						p.skipNewlines()
					}
				}
				if p.peek().Type == TRParen {
					p.advance()
				}
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

// ============================================================================
// JSON output formatting (pretty with indentation)
// ============================================================================

func tokenTypeName(t TokenType) string {
	switch t {
	case TIdent:
		return "identifier"
	case TString:
		return "string"
	case TNumber:
		return "number"
	case TArrow:
		return "->"
	case TLBrace:
		return "{"
	case TRBrace:
		return "}"
	case TLParen:
		return "("
	case TRParen:
		return ")"
	case TLBrack:
		return "["
	case TRBrack:
		return "]"
	case TEOF:
		return "end of file"
	default:
		return fmt.Sprintf("token(%d)", t)
	}
}

// ============================================================================
// Main
// ============================================================================

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input.tac> [--output ast.json]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nTAC — The TacFlow Agentic Code Parser\n")
		fmt.Fprintf(os.Stderr, "Parses .tac files and outputs the AST as JSON.\n")
		os.Exit(1)
	}

	inputPath := os.Args[1]
	outputPath := ""
	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == "--output" && i+1 < len(os.Args) {
			outputPath = os.Args[i+1]
			i++
		}
	}

	// Read source
	source, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", inputPath, err)
		os.Exit(1)
	}

	// Tokenize
	tokenizer := NewTokenizer(string(source))
	tokens, err := tokenizer.Tokenize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Tokenization error: %v\n", err)
		os.Exit(1)
	}

	// Check for tokenization errors
	for _, tok := range tokens {
		if tok.Type == TError {
			fmt.Fprintf(os.Stderr, "Tokenization error at line %d: %s\n", tok.Line, tok.Value)
			os.Exit(1)
		}
	}

	// Parse
	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		os.Exit(1)
	}

	// Serialize to JSON
	jsonBytes, err := json.MarshalIndent(ast, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON serialization error: %v\n", err)
		os.Exit(1)
	}

	if outputPath != "" {
		err = os.WriteFile(outputPath, jsonBytes, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", outputPath, err)
			os.Exit(1)
		}
		fmt.Printf("AST written to %s\n", outputPath)
	} else {
		fmt.Println(string(jsonBytes))
	}
}
