package parser

import (
	"strings"
	"testing"
	"time"
)

// I-4: values and blocks nest by recursion; without a limit a deep enough
// source overflows the goroutine stack, which no caller can recover from.
// MaxDepth levels parse; one more is a parse error.
func TestNestingDepthLimit(t *testing.T) {
	shapes := map[string]func(n int) string{
		"array": func(n int) string {
			return "remember x = " + strings.Repeat("[", n) + strings.Repeat("]", n)
		},
		"object": func(n int) string {
			return "remember x = " + strings.Repeat("{a: ", n) + "1" + strings.Repeat("}", n)
		},
		"arg": func(n int) string {
			return `flow "f" { node "a" -> skill s(x: ` + strings.Repeat("[", n) + strings.Repeat("]", n) + `) }`
		},
		"block": func(n int) string {
			return `flow "f" { node "a" ` + strings.Repeat("{ if x ", n) + strings.Repeat("}", n) + ` }`
		},
	}
	for name, src := range shapes {
		if _, err := parseWithin(t, src(MaxDepth), 5*time.Second); err != nil {
			t.Errorf("%s: %d levels must parse: %v", name, MaxDepth, err)
		}
		_, err := parseWithin(t, src(MaxDepth+1), 5*time.Second)
		if err == nil || !strings.Contains(err.Error(), "nest") {
			t.Errorf("%s: %d levels: err = %v, want a nesting error", name, MaxDepth+1, err)
		}
		if _, err := parseWithin(t, src(200000), 10*time.Second); err == nil {
			t.Errorf("%s: 200000 levels must be a parse error", name)
		}
	}
	// Unclosed: the error, not a crash or a hang.
	for _, src := range []string{"remember x = " + strings.Repeat("[", 100000), "remember x = " + strings.Repeat("{a:", 100000)} {
		if _, err := parseWithin(t, src, 10*time.Second); err == nil {
			t.Errorf("%d unclosed levels must be a parse error", len(src))
		}
	}
}
