package semantic

import "github.com/TacFlow/tac-language/types"

// LayaSkills are the `laya.*` skills of TAC v0.5 (DESIGN §5.3 of the LAYA
// mission). They are part of the standard library (merged into BuiltinSkills
// at init) and are listed in skills.json for registry consumers.
// decide/classify/normalize return Hallucinable (a model's answer, D16); the
// rest return Fact.
var LayaSkills = map[string]SkillSpec{
	"laya.decide": {Name: "laya.decide", Version: "1.0", ReturnType: types.Hallucinable,
		Args:        []string{"task", "input", "min_confidence", "question", "allow_uncalibrated"},
		Description: "Ask a LAYA model a task's question; with labelled edges the node is a gate"},
	"laya.classify": {Name: "laya.classify", Version: "1.0", ReturnType: types.Hallucinable,
		Args: []string{"task", "input"}, Description: "Answer every question of a LAYA task"},
	"laya.normalize": {Name: "laya.normalize", Version: "1.0", ReturnType: types.Hallucinable,
		Args: []string{"input", "task", "content_type"}, Description: "Turn raw input into a LAYA episode"},
	"laya.episode.record": {Name: "laya.episode.record", Version: "1.0", ReturnType: types.Fact,
		Args: []string{"from", "episode", "target", "task"}, Description: "Store a LAYA episode (optionally with targets)"},
	"laya.dataset.export": {Name: "laya.dataset.export", Version: "1.0", ReturnType: types.Fact,
		Args:        []string{"task", "dataset", "exclude_inferred", "exclude_fixed_test"},
		Description: "Export a training dataset (JSONL) of a task or a declared dataset"},
	"laya.train": {Name: "laya.train", Version: "1.0", ReturnType: types.Fact,
		Args: []string{"model", "dataset", "run"}, Description: "Start training a private LAYA model (asynchronous)"},
	"laya.eval": {Name: "laya.eval", Version: "1.0", ReturnType: types.Fact,
		Args: []string{"model", "run", "test"}, Description: "Evaluate a run against the current model and the base"},
	"laya.model.create": {Name: "laya.model.create", Version: "1.0", ReturnType: types.Fact,
		Args: []string{"name", "tasks", "base"}, Description: "Declare a private LAYA model"},
	"laya.model.status": {Name: "laya.model.status", Version: "1.0", ReturnType: types.Fact,
		Args: []string{"model"}, Description: "Serving run, previous run, port, memory and job of a model"},
	"laya.model.promote": {Name: "laya.model.promote", Version: "1.0", ReturnType: types.Fact,
		Args: []string{"model", "run", "if_better", "max_drop_pp"}, Description: "Serve a run (optionally only if it is better)"},
	"laya.model.rollback": {Name: "laya.model.rollback", Version: "1.0", ReturnType: types.Fact,
		Args: []string{"model"}, Description: "Serve the previous run again"},
	"laya.tasks.list": {Name: "laya.tasks.list", Version: "1.0", ReturnType: types.Fact,
		Args: nil, Description: "List the registered LAYA tasks"},
	"laya.tasks.describe": {Name: "laya.tasks.describe", Version: "1.0", ReturnType: types.Fact,
		Args: []string{"task"}, Description: "Questions, labels, counts and model of a task"},
}

func init() {
	for name, spec := range LayaSkills {
		spec.ArgTypes = map[string]types.TrustType{}
		builtinSkills[name] = spec
	}
}
