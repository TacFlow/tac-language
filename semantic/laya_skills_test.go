package semantic

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/TacFlow/tac-language/types"
)

// The 13 laya.* skills are in the standard library with DESIGN §5.3's
// arguments and trust types (D16: decide/classify/normalize are Hallucinable).
func TestLayaSkills_InStandardLibrary(t *testing.T) {
	if len(LayaSkills) != 13 {
		t.Fatalf("LayaSkills = %d, want 13", len(LayaSkills))
	}
	hallucinable := map[string]bool{"laya.decide": true, "laya.classify": true, "laya.normalize": true}
	for name, spec := range LayaSkills {
		got, ok := BuiltinRegistry().Lookup(name)
		if !ok {
			t.Fatalf("%s not in BuiltinRegistry", name)
		}
		want := types.Fact
		if hallucinable[name] {
			want = types.Hallucinable
		}
		if got.ReturnType != want || !reflect.DeepEqual(got.Args, spec.Args) || got.IsDynamic() {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if args := LayaSkills["laya.decide"].Args; !reflect.DeepEqual(args, []string{"task", "input", "min_confidence", "question", "allow_uncalibrated"}) {
		t.Errorf("laya.decide args = %v", args)
	}
}

// skills.json (the sample --registry file) lists the same laya.* skills.
func TestLayaSkills_SkillsJSONMatchesBuiltins(t *testing.T) {
	b, err := os.ReadFile("../skills.json")
	if err != nil {
		t.Fatal(err)
	}
	var specs []SkillSpec
	if err := json.Unmarshal(b, &specs); err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, s := range specs {
		want, ok := LayaSkills[s.Name]
		if !ok {
			continue
		}
		seen++
		if s.ReturnType != want.ReturnType || s.Version != want.Version || s.IsDynamic() ||
			!reflect.DeepEqual(append([]string{}, s.Args...), append([]string{}, want.Args...)) {
			t.Errorf("skills.json %s = %+v, want %+v", s.Name, s, want)
		}
	}
	if seen != len(LayaSkills) {
		t.Fatalf("skills.json lists %d laya.* skills, want %d", seen, len(LayaSkills))
	}
}
