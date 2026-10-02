package parser

import (
	"testing"
	"time"

	"github.com/TacFlow/tac-language/ast"
)

// parseWithin fails the test if ParseSource does not return within d.
func parseWithin(t *testing.T, src string, d time.Duration) (*ast.Node, error) {
	t.Helper()
	type res struct {
		n   *ast.Node
		err error
	}
	ch := make(chan res, 1)
	go func() {
		n, err := ParseSource(src)
		ch <- res{n, err}
	}()
	select {
	case r := <-ch:
		return r.n, r.err
	case <-time.After(d):
		t.Fatalf("ParseSource(%q) did not return within %v (infinite loop)", src, d)
		return nil, nil
	}
}

// Bug d: parseArrayLiteral looped forever when an element was a token
// parseValue does not handle (it consumed nothing and never advanced).
func TestBugD_ArrayLiteralWithUnparseableElementTerminates(t *testing.T) {
	for _, src := range []string{
		`remember x = [}`,
		`remember x = [1, )]`,
		`flow "f" { node "a" -> skill web_search(query: [ -> ]) }`,
		`[`,
		`flow "f" { on "e" -> [ }`,
	} {
		parseWithin(t, src, 2*time.Second)
	}
}
func edgePairs(flow *ast.Node) []string {
	var out []string
	for _, e := range flow.Edges {
		out = append(out, ast.EdgeSource(e)+">"+ast.EdgeTarget(e))
	}
	return out
}

// Bug b: `a -> b -> c` kept only a -> b; every later hop was dropped.
func TestBugB_ChainedEdgesKeepEveryHop(t *testing.T) {
	src := `flow "f" {
  node "a" -> skill foo()
  node "b" -> skill foo()
  node "c" -> skill foo()
  node "d" -> skill foo()
  a -> b -> c -> d
  a -> d
}`
	prog, err := ParseSource(src)
	if err != nil {
		t.Fatal(err)
	}
	got := edgePairs(ast.CollectFlows(prog)[0])
	want := []string{"a>b", "b>c", "c>d", "a>d"}
	if len(got) != len(want) {
		t.Fatalf("edges = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("edges = %v, want %v", got, want)
		}
	}
	// The tail edge is positioned at its own source token: the `b` that is
	// the target of the first hop.
	edges := ast.CollectFlows(prog)[0].Edges
	if got, want := edges[1].Pos, edges[0].Children[1].Pos; got != want {
		t.Errorf("b -> c at %+v, want %+v", got, want)
	}
}
