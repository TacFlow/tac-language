package laya

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/TacFlow/tac-language/ast"
)

// Diagnostic codes owned by this package (DESIGN §5.9).
const (
	CodeTargetUnknown = "TAC-LAYA-006"
	CodeTargetInvalid = "TAC-LAYA-007"
	CodeUnknownType   = "TAC-LAYA-008"
	CodeLimits        = "TAC-LAYA-014"
	CodeReservedLabel = "TAC-LAYA-017"
)

// MaxReducedQuestions is the total of model questions one episode may reduce to.
const MaxReducedQuestions = 32

var (
	stateKeyRE      = regexp.MustCompile(`^(_raw|[A-Za-z][A-Za-z0-9_]*)$`)
	questionIDRE    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,47}$`)
	reducedSuffixRE = regexp.MustCompile(`__(o[0-9]+|p[0-9]+_[0-9]+)$`)
	metaKeys        = map[string]bool{"schema_version": true, "source": true, "provenance": true, "agent_id": true,
		"swarm_id": true, "task": true, "created_at": true, "synthetic": true, "language": true, "nonce": true}
)

// PseudoLabels are the branches every gate understands besides the task's
// own labels (defined once, in ast). A task may not use them as labels
// (TAC-LAYA-017).
var PseudoLabels = []string{ast.LabelLowConfidence, ast.LabelError, ast.LabelAny}

// IsPseudoLabel reports whether s is a reserved pseudo-label.
func IsPseudoLabel(s string) bool {
	for _, p := range PseudoLabels {
		if p == s {
			return true
		}
	}
	return false
}

func num(n interface{}) (float64, bool) {
	switch t := n.(type) {
	case interface{ Float64() (float64, error) }:
		f, err := t.Float64()
		return f, err == nil
	case float64:
		return t, true
	}
	return 0, false
}

func toFloats(q Question) (rng, bins []float64, ok bool) {
	ok = true
	for _, n := range q.Range {
		f, err := strconv.ParseFloat(string(n), 64)
		if err != nil {
			ok = false
		}
		rng = append(rng, f)
	}
	for _, n := range q.Bins {
		f, err := strconv.ParseFloat(string(n), 64)
		if err != nil {
			ok = false
		}
		bins = append(bins, f)
	}
	return rng, bins, ok
}

// ReducedCount is how many model questions (choice/score/noul) qs reduce to
// (DESIGN §3.2). Non-learnable types count 0.
func ReducedCount(qs []Question) int {
	n := 0
	for _, q := range qs {
		switch q.Type {
		case TypeChoice, TypeBinary, TypeNumber, TypeScale:
			n++
		case TypeMultiChoice:
			n += len(q.Options)
		case TypeRanking:
			k := len(q.Options)
			n += k * (k - 1) / 2
		}
	}
	return n
}

// ValidateQuestion checks the shape of one question (DESIGN §3.2). path
// prefixes every Issue path.
func ValidateQuestion(q Question, path string) []Issue {
	var out []Issue
	bad := func(code, p, f string, a ...interface{}) {
		out = append(out, Issue{Code: code, Path: p, Msg: fmt.Sprintf(f, a...)})
	}
	if !questionIDRE.MatchString(q.ID) {
		bad("", path+".id", "invalid question id %q", q.ID)
	}
	if reducedSuffixRE.MatchString(q.ID) {
		bad("", path+".id", "question id %q ends with a reserved reduced-question suffix", q.ID)
	}
	if strings.TrimSpace(q.Instructions) == "" || len(q.Instructions) > 4096 {
		bad("", path+".instructions", "instructions must have 1..4096 bytes")
	}
	optRange := func(min, max int) {
		if len(q.Options) < min || len(q.Options) > max {
			bad(CodeLimits, path+".options", "%s needs %d..%d options, got %d", q.Type, min, max, len(q.Options))
		}
	}
	switch q.Type {
	case TypeChoice:
		optRange(2, 16)
	case TypeMultiChoice:
		optRange(2, 8)
	case TypeRanking:
		optRange(2, 4)
	case TypeBinary:
		if len(q.Options) != 0 && len(q.Options) != 2 {
			bad(CodeLimits, path+".options", "binary takes no options or exactly 2")
		}
	case TypeScale:
		if len(q.Levels) < 2 || len(q.Levels) > 16 {
			bad(CodeLimits, path+".levels", "scale needs 2..16 levels, got %d", len(q.Levels))
		}
		seen := map[string]bool{}
		for i, l := range q.Levels {
			if strings.TrimSpace(l) == "" || seen[l] {
				bad("", fmt.Sprintf("%s.levels[%d]", path, i), "empty or duplicate level")
			}
			seen[l] = true
		}
	case TypeNumber:
		rng, bins, ok := toFloats(q)
		if !ok || len(rng) != 2 || !(rng[0] < rng[1]) {
			bad("", path+".range", "number needs range [min, max] with min < max")
			break
		}
		if len(bins) > 15 {
			bad(CodeLimits, path+".bins", "at most 15 cuts")
		}
		prev := rng[0]
		for i, c := range bins {
			if !(c > prev) || !(c < rng[1]) {
				bad("", fmt.Sprintf("%s.bins[%d]", path, i), "cuts must be increasing and inside the open range")
			}
			prev = c
		}
	case TypeExtraction, TypeText:
	default:
		out = append(out, Issue{Code: CodeUnknownType, Warning: true, Path: path + ".type",
			Msg: fmt.Sprintf("unknown question type %q: stored, not learnable", q.Type)})
	}
	seen := map[string]bool{}
	for i, o := range q.Options {
		op := fmt.Sprintf("%s.options[%d]", path, i)
		if strings.TrimSpace(o.Label) == "" || len(o.Label) > 128 || len(o.Description) > 2048 {
			bad("", op, "invalid option")
		}
		if seen[o.Label] {
			bad("", op, "duplicate label %q", o.Label)
		}
		seen[o.Label] = true
	}
	return out
}

// ReservedLabelIssues reports a task question that uses a pseudo-label as
// one of its own labels (TAC-LAYA-017).
func ReservedLabelIssues(q Question, path string) []Issue {
	var out []Issue
	for _, l := range TaskLabels(q) {
		if IsPseudoLabel(l) {
			out = append(out, Issue{Code: CodeReservedLabel, Path: path,
				Msg: fmt.Sprintf("question %q uses the reserved pseudo-label %q", q.ID, l)})
		}
	}
	return out
}

// ValidateEpisode checks an episode (DESIGN §3): structure, per-type limits,
// and every target against its question's type.
func ValidateEpisode(e Episode) []Issue {
	var out []Issue
	bad := func(code, p, f string, a ...interface{}) {
		out = append(out, Issue{Code: code, Path: p, Msg: fmt.Sprintf(f, a...)})
	}
	if n := utf8.RuneCountInString(e.ID); strings.TrimSpace(e.ID) == "" || n > 128 {
		bad("", "id", "must have 1..128 characters")
	}
	if n := utf8.RuneCountInString(e.GroupID); strings.TrimSpace(e.GroupID) == "" || n > 256 {
		bad("", "group_id", "must have 1..256 characters")
	}
	for k, v := range e.State {
		if !stateKeyRE.MatchString(k) {
			bad("", "state."+k, "invalid state key")
			continue
		}
		switch k {
		case "goal", "regra_declarada", "tool", "_raw":
			if _, ok := v.(string); !ok {
				bad("", "state."+k, "must be a string")
			}
		case "cache_available", "fresh_result_required":
			if _, ok := v.(bool); !ok {
				bad("", "state."+k, "must be a boolean")
			}
		case "context":
			if _, ok := v.(map[string]interface{}); !ok {
				bad("", "state.context", "must be an object")
			}
		}
	}
	if len(e.Questions) < 1 || len(e.Questions) > 8 {
		bad("", "questions", "1..8 questions required, got %d", len(e.Questions))
	}
	byID := map[string]Question{}
	for i, q := range e.Questions {
		p := fmt.Sprintf("questions[%d]", i)
		if _, dup := byID[q.ID]; dup {
			bad("", p+".id", "duplicate question id %q", q.ID)
		}
		byID[q.ID] = q
		out = append(out, ValidateQuestion(q, p)...)
	}
	if n := ReducedCount(e.Questions); n > MaxReducedQuestions {
		bad(CodeLimits, "questions", "%d reduced questions, max %d", n, MaxReducedQuestions)
	}
	for qid, v := range e.Targets {
		q, ok := byID[qid]
		if !ok {
			bad(CodeTargetUnknown, "targets."+qid, "target for a question that is not declared")
			continue
		}
		if msg := TargetError(q, v); msg != "" {
			bad(CodeTargetInvalid, "targets."+qid, "%s", msg)
		}
	}
	for k := range e.Meta {
		if !metaKeys[k] {
			bad("", "meta."+k, "unknown meta key")
		}
	}
	return out
}

func hasLabel(opts []Option, s string) bool {
	for _, o := range opts {
		if o.Label == s {
			return true
		}
	}
	return false
}

// TargetError returns "" if v is a valid target for q, else the reason.
func TargetError(q Question, v interface{}) string {
	switch q.Type {
	case TypeChoice:
		s, ok := v.(string)
		if !ok || !hasLabel(q.Options, s) {
			return "must be one of the option labels"
		}
	case TypeMultiChoice:
		arr, ok := v.([]interface{})
		if !ok {
			return "must be a list of option labels"
		}
		seen := map[string]bool{}
		for _, x := range arr {
			s, ok := x.(string)
			if !ok || !hasLabel(q.Options, s) || seen[s] {
				return "must be a subset of the option labels without repeats"
			}
			seen[s] = true
		}
	case TypeBinary:
		if _, ok := v.(bool); !ok {
			return "must be true or false"
		}
	case TypeNumber:
		rng, _, ok := toFloats(q)
		f, isNum := num(v)
		if !ok || !isNum || len(rng) != 2 || f < rng[0] || f > rng[1] {
			return "must be a number inside range"
		}
	case TypeScale:
		if s, ok := v.(string); ok {
			for _, l := range q.Levels {
				if l == s {
					return ""
				}
			}
			return "unknown level"
		}
		f, ok := num(v)
		if !ok || f != math.Trunc(f) || f < 0 || int(f) >= len(q.Levels) {
			return "must be a level name or index 0..n-1"
		}
	case TypeRanking:
		arr, ok := v.([]interface{})
		if !ok || len(arr) != len(q.Options) {
			return "must be a full permutation of the options"
		}
		seen := map[string]bool{}
		for _, x := range arr {
			s, ok := x.(string)
			if !ok || !hasLabel(q.Options, s) || seen[s] {
				return "must be a full permutation of the options"
			}
			seen[s] = true
		}
	case TypeExtraction:
		obj, ok := v.(map[string]interface{})
		if !ok {
			return "must be an object"
		}
		if len(obj) == 2 {
			s, ok1 := num(obj["start"])
			e, ok2 := num(obj["end"])
			if ok1 && ok2 && s == math.Trunc(s) && e == math.Trunc(e) {
				if s < 0 || s >= e {
					return "span must satisfy 0 <= start < end"
				}
				return ""
			}
		}
		if len(q.Fields) > 0 {
			allowed := map[string]bool{}
			for _, f := range q.Fields {
				allowed[f] = true
			}
			for k := range obj {
				if !allowed[k] {
					return fmt.Sprintf("field %q is not declared", k)
				}
			}
		}
	case TypeText:
		s, ok := v.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return "must be a non-empty string"
		}
	}
	return ""
}
