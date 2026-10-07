package laya

import (
	"errors"
	"fmt"
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
// [a..b] is [a, b), as DESIGN §5.4 says. ok is false when CheckRangeLabel
// reports a problem.
func ParseRangeLabel(label string) (Interval, bool) {
	iv, err := CheckRangeLabel(label)
	return iv, err == nil
}

// CheckRangeLabel is ParseRangeLabel with the reason a label is not a
// usable range: not a range at all, a bound that is not a finite number
// (`1e999`), or an empty or inverted range (`[5..5]`, `[50..10]`).
func CheckRangeLabel(label string) (Interval, error) {
	s, ok := strings.CutPrefix(label, "range:")
	if !ok {
		return Interval{}, errNotRange
	}
	inf := math.Inf(1)
	parse := func(x string) (float64, error) {
		f, err := strconv.ParseFloat(x, 64)
		if err != nil && (math.IsInf(f, 0) || math.IsNaN(f)) {
			return 0, fmt.Errorf("bound %s is not a finite number", x)
		}
		if err != nil {
			return 0, errNotRange
		}
		return f, nil
	}
	iv := Interval{Label: label}
	var err error
	switch {
	case strings.HasPrefix(s, "<="):
		iv.Hi, err = parse(s[2:])
		iv.Lo, iv.HiIn = -inf, true
		return iv, err
	case strings.HasPrefix(s, ">="):
		iv.Lo, err = parse(s[2:])
		iv.Hi, iv.LoIn = inf, true
		return iv, err
	case strings.HasPrefix(s, "<"):
		iv.Hi, err = parse(s[1:])
		iv.Lo = -inf
		return iv, err
	case strings.HasPrefix(s, ">"):
		iv.Lo, err = parse(s[1:])
		iv.Hi = inf
		return iv, err
	}
	lo, hi, found := strings.Cut(s, "..")
	if !found {
		return Interval{}, errNotRange
	}
	a, err := parse(lo)
	if err != nil {
		return iv, err
	}
	b, err := parse(hi)
	if err != nil {
		return iv, err
	}
	iv.Lo, iv.Hi, iv.LoIn = a, b, true
	if !(a < b) {
		return iv, fmt.Errorf("empty or inverted range %s..%s: [a..b] is a <= x < b, so a must be below b", lo, hi)
	}
	return iv, nil
}

var errNotRange = errors.New("not a range (<v, <=v, >v, >=v, a..b)")

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
