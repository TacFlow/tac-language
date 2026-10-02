package laya

import (
	"math"
	"strconv"
	"strings"
)

// TaskLabels are the branch labels a gate on q can take, without the
// pseudo-labels: option labels (choice, multi_choice, ranking — the 1st
// place), "true"/"false" (binary), level names (scale). Empty for number
// (ranges), extraction, text and unknown types.
func TaskLabels(q Question) []string {
	switch q.Type {
	case TypeChoice, TypeMultiChoice, TypeRanking:
		out := make([]string, len(q.Options))
		for i, o := range q.Options {
			out[i] = o.Label
		}
		return out
	case TypeBinary:
		return []string{"true", "false"}
	case TypeScale:
		return append([]string(nil), q.Levels...)
	}
	return nil
}

// Gateable reports whether a gate may branch on a question of type t:
// false for extraction and text (TAC-LAYA-005).
func Gateable(t string) bool { return t != TypeExtraction && t != TypeText }

// KnownType reports whether t is one of the eight question types.
func KnownType(t string) bool {
	switch t {
	case TypeChoice, TypeMultiChoice, TypeBinary, TypeNumber, TypeScale, TypeRanking, TypeExtraction, TypeText:
		return true
	}
	return false
}

// Interval is a numeric branch: Lo/Hi with inclusiveness. ±Inf for open ends.
type Interval struct {
	Lo, Hi     float64
	LoIn, HiIn bool
	Label      string
}

// ParseRangeLabel turns a canonical range label ("range:<10",
// "range:10..50", "range:>=50" — ast.LabelString) into an interval.
// [a..b] is [a, b), as DESIGN §5.4 says.
func ParseRangeLabel(label string) (Interval, bool) {
	s, ok := strings.CutPrefix(label, "range:")
	if !ok {
		return Interval{}, false
	}
	inf := math.Inf(1)
	parse := func(x string) (float64, bool) {
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	}
	iv := Interval{Label: label}
	switch {
	case strings.HasPrefix(s, "<="):
		v, ok := parse(s[2:])
		iv.Lo, iv.Hi, iv.HiIn = -inf, v, true
		return iv, ok
	case strings.HasPrefix(s, ">="):
		v, ok := parse(s[2:])
		iv.Lo, iv.Hi, iv.LoIn = v, inf, true
		return iv, ok
	case strings.HasPrefix(s, "<"):
		v, ok := parse(s[1:])
		iv.Lo, iv.Hi = -inf, v
		return iv, ok
	case strings.HasPrefix(s, ">"):
		v, ok := parse(s[1:])
		iv.Lo, iv.Hi = v, inf
		return iv, ok
	}
	lo, hi, found := strings.Cut(s, "..")
	if !found {
		return Interval{}, false
	}
	a, ok1 := parse(lo)
	b, ok2 := parse(hi)
	iv.Lo, iv.Hi, iv.LoIn = a, b, true
	return iv, ok1 && ok2 && a < b
}

// Overlap reports whether two intervals share at least one number.
func Overlap(a, b Interval) bool {
	// a ends before b starts?
	if a.Hi < b.Lo || (a.Hi == b.Lo && !(a.HiIn && b.LoIn)) {
		return false
	}
	if b.Hi < a.Lo || (b.Hi == a.Lo && !(b.HiIn && a.LoIn)) {
		return false
	}
	return true
}
