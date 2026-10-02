package lexer

import "testing"

func scanTypes(t *testing.T, src string) ([]TokenType, []string) {
	t.Helper()
	toks, err := New(src).Scan()
	if err != nil {
		t.Fatalf("Scan(%q): %v", src, err)
	}
	var types []TokenType
	var vals []string
	for _, tk := range toks {
		types = append(types, tk.Type)
		vals = append(vals, tk.Value)
	}
	return types, vals
}

func TestLexer_V05Tokens(t *testing.T) {
	cases := []struct {
		src  string
		want []TokenType
		vals []string // nil = do not check values
	}{
		{"*", []TokenType{Star, EOF}, nil},
		{"..", []TokenType{DotDot, EOF}, nil},
		{"10..50", []TokenType{Number, DotDot, Number, EOF}, []string{"10", "..", "50", ""}},
		{"1.5..2.5", []TokenType{Number, DotDot, Number, EOF}, []string{"1.5", "..", "2.5", ""}},
		{"-5", []TokenType{Number, EOF}, []string{"-5", ""}},
		{"-0.25", []TokenType{Number, EOF}, []string{"-0.25", ""}},
		{"[-5..-1]", []TokenType{LBrack, Number, DotDot, Number, RBrack, EOF}, []string{"[", "-5", "..", "-1", "]", ""}},
		{"x: -3", []TokenType{Ident, Colon, Number, EOF}, []string{"x", ":", "-3", ""}},
		{"gate[*]", []TokenType{Ident, LBrack, Star, RBrack, EOF}, nil},
		{"gate[>=50]", []TokenType{Ident, LBrack, GreaterEq, Number, RBrack, EOF}, nil},
		// `<-` stays the Assign token; the parser splits it inside a branch.
		{"[<-5]", []TokenType{LBrack, Assign, Number, RBrack, EOF}, []string{"[", "<-", "5", "]", ""}},
		{"a -> b", []TokenType{Ident, Arrow, Ident, EOF}, nil},
	}
	for _, c := range cases {
		got, vals := scanTypes(t, c.src)
		if len(got) != len(c.want) {
			t.Fatalf("%q: got %v, want %v", c.src, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("%q: token %d = %v, want %v (all %v)", c.src, i, got[i], c.want[i], got)
			}
			if c.vals != nil && vals[i] != c.vals[i] {
				t.Fatalf("%q: token %d value %q, want %q", c.src, i, vals[i], c.vals[i])
			}
		}
	}
}

// After an operand a '-' cannot start a number: TAC has no subtraction, so it
// is still a lexical error, exactly as in v0.4.
func TestLexer_MinusAfterOperandIsStillAnError(t *testing.T) {
	for _, src := range []string{"x -3", "3 -3", `"a" -3`, ") -3"} {
		if _, err := New(src).Scan(); err == nil {
			t.Errorf("%q: expected a lexical error", src)
		}
	}
}

// Scientific notation is one number (valid JSON number syntax). Any other
// text glued to a number lexes exactly as in v0.4 — the number, then the
// rest — and is recorded so the analyzer can warn (TAC-PARSE-001).
func TestLexer_ExponentAndGluedNumbers(t *testing.T) {
	for src, want := range map[string]string{"1e3": "1e3", "2.5E-4": "2.5E-4", "1e+21": "1e+21", "-3e-2": "-3e-2"} {
		l := New(src)
		toks, err := l.Scan()
		if err != nil || len(toks) != 2 || toks[0].Type != Number || toks[0].Value != want || len(l.Glued()) != 0 {
			t.Errorf("%q: %v %v glued=%v", src, toks, err, l.Glued())
		}
	}
	cases := []struct {
		src           string
		types         []TokenType
		number, glued string
	}{
		{"3x", []TokenType{Number, Ident, EOF}, "3", "3x"},
		{"2.5kg", []TokenType{Number, Ident, EOF}, "2.5", "2.5kg"},
		{"1_000", []TokenType{Number, Ident, EOF}, "1", "1_000"},
		{"1e", []TokenType{Number, Ident, EOF}, "1", "1e"},
		{"1e3x", []TokenType{Number, Ident, EOF}, "1e3", "1e3x"},
	}
	for _, c := range cases {
		l := New(c.src)
		toks, err := l.Scan()
		if err != nil || len(toks) != len(c.types) {
			t.Fatalf("%q: %v %v", c.src, toks, err)
		}
		for i := range toks {
			if toks[i].Type != c.types[i] {
				t.Fatalf("%q: tokens %v", c.src, toks)
			}
		}
		g := l.Glued()
		if toks[0].Value != c.number || len(g) != 1 || g[0].Number+g[0].Rest != c.glued {
			t.Errorf("%q: number %q glued %+v", c.src, toks[0].Value, g)
		}
	}
	for _, src := range []string{"x1", "a.b2", "1 x", "10..50", "3, x"} {
		l := New(src)
		if _, err := l.Scan(); err != nil || len(l.Glued()) != 0 {
			t.Errorf("%q: err %v glued %v", src, err, l.Glued())
		}
	}
}
