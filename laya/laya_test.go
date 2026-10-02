package laya

import "testing"

func TestCanonical_StructuralEquality(t *testing.T) {
	eq := [][2]string{
		{`{"b":1,"a":[2.0,"x"]}`, `{ "a": [2, "x"], "b": 1e0 }`},
		{`{"n":12345678901234567890}`, `{"n":12345678901234567890}`},
		{`{"v":0.80}`, `{"v":0.8}`},
		{`{"n":1e20}`, `{"n":100000000000000000000}`},
		{`{"n":-2.50e1}`, `{"n":-25}`},
	}
	for _, c := range eq {
		a, err1 := Canonical([]byte(c[0]))
		b, err2 := Canonical([]byte(c[1]))
		if err1 != nil || err2 != nil || string(a) != string(b) {
			t.Errorf("%s vs %s: %s / %s (%v %v)", c[0], c[1], a, b, err1, err2)
		}
	}
	ne := [][2]string{
		{`{"n":12345678901234567890}`, `{"n":12345678901234567891}`},
		{`{"a":"1"}`, `{"a":1}`},
		{`[1,2]`, `[2,1]`},
	}
	for _, c := range ne {
		a, _ := Canonical([]byte(c[0]))
		b, _ := Canonical([]byte(c[1]))
		if string(a) == string(b) {
			t.Errorf("%s and %s must differ", c[0], c[1])
		}
	}
	if _, err := Canonical([]byte(`{} {}`)); err == nil {
		t.Error("trailing data must be an error")
	}
}

func TestRanges_ParseAndOverlap(t *testing.T) {
	iv := func(s string) Interval {
		t.Helper()
		v, ok := ParseRangeLabel(s)
		if !ok {
			t.Fatalf("ParseRangeLabel(%q) failed", s)
		}
		return v
	}
	cases := []struct {
		a, b string
		want bool
	}{
		{"range:<10", "range:10..50", false},  // (-inf,10) vs [10,50)
		{"range:<=10", "range:10..50", true},  // 10 in both
		{"range:10..50", "range:>=50", false}, // [10,50) vs [50,inf)
		{"range:10..50", "range:>49", true},   //
		{"range:<-5", "range:-5..-1", false},  //
		{"range:5..50", "range:<10", true},    //
		{"range:>50", "range:<=50", false},    //
	}
	for _, c := range cases {
		if got := Overlap(iv(c.a), iv(c.b)); got != c.want {
			t.Errorf("Overlap(%s, %s) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
	for _, bad := range []string{"proceed", "range:", "range:50..10", "range:a..b", "range:<x"} {
		if _, ok := ParseRangeLabel(bad); ok {
			t.Errorf("ParseRangeLabel(%q) should fail", bad)
		}
	}
}
