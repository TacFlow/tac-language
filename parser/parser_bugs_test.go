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
