package main

import (
	"fmt"
	"github.com/TacFlow/tac-language/ast"
	"github.com/TacFlow/tac-language/parser"
)

func main() {
	src := `flow "test" {
  node "a" -> skill foo()
  node "b" -> skill bar()
  a -> b { if: a.confidence > 0.8 }
}`
	program, _ := parser.ParseSource(src)
	flows := ast.CollectFlows(program)
	edges := ast.CollectEdges(flows[0])
	cond, _, hasCond := ast.EdgeCondition(edges[0])
	fmt.Printf("hasCond=%v cond=%q\n", hasCond, cond)
	
	// Show the attr node
	if condNode, ok := edges[0].Attrs["if"]; ok {
		fmt.Printf("condNode.Type=%s condNode.Value=%q\n", condNode.Type, condNode.Value)
		for i, c := range condNode.Children {
			fmt.Printf("  child[%d].Type=%s .Value=%q\n", i, c.Type, c.Value)
		}
	}
}
