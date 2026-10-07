// Package laya holds the LAYA data declared in TAC v0.5 — tasks, models,
// episodes and datasets — in their compiled (JSON) form, the conversion from
// the AST, and the checks the analyzer runs on them.
//
// The episode form is the canonical `laya.episode/1` of the LAYA mission
// (DESIGN §3): core (id, group_id, state, questions, targets) plus optional
// meta. Numbers keep their source literal (json.Number).
package laya

import "encoding/json"

// Question types (DESIGN §3.2). Any other name is kept and not learnable
// (warning TAC-LAYA-008).
const (
	TypeChoice      = "choice"
	TypeMultiChoice = "multi_choice"
	TypeBinary      = "binary"
	TypeNumber      = "number"
	TypeScale       = "scale"
	TypeRanking     = "ranking"
	TypeExtraction  = "extraction"
	TypeText        = "text"
)

// Option is a labelled option of a choice, multi_choice, binary or ranking.
type Option struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// Question is one question of a task or an episode.
type Question struct {
	ID           string        `json:"id"`
	Type         string        `json:"type"`
	Instructions string        `json:"instructions"`
	Options      []Option      `json:"options,omitempty"`
	Levels       []string      `json:"levels,omitempty"`
	Range        []json.Number `json:"range,omitempty"`
	Unit         string        `json:"unit,omitempty"`
	Bins         []json.Number `json:"bins,omitempty"`
	Fields       []string      `json:"fields,omitempty"`
}

// Episode is a `laya.episode/1` episode.
type Episode struct {
	ID        string                 `json:"id"`
	GroupID   string                 `json:"group_id"`
	State     map[string]interface{} `json:"state"`
	Questions []Question             `json:"questions"`
	Targets   map[string]interface{} `json:"targets,omitempty"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
}

// Task is a `task` declaration: the questions a gate or an episode answers.
// It is also the shape of one entry of a --tasks file (and of
// laya.tasks.list on the platform).
type Task struct {
	Name      string      `json:"name"`
	Questions []Question  `json:"questions"`
	Profile   interface{} `json:"profile,omitempty"`
}

// Model is a `model` declaration: a private model trained on some tasks.
type Model struct {
	Name  string      `json:"name"`
	Tasks []string    `json:"tasks"`
	Base  string      `json:"base,omitempty"`
	Port  json.Number `json:"port,omitempty"`
}

// Dataset is a `dataset` declaration.
type Dataset struct {
	Name    string                 `json:"name"`
	Task    string                 `json:"task"`
	From    string                 `json:"from,omitempty"`
	Include []string               `json:"include,omitempty"`
	Splits  map[string]json.Number `json:"splits,omitempty"`
}

// Issue is one finding of a check in this package. Code is a TAC-LAYA-*
// code, or "" for a structural problem without a dedicated code.
type Issue struct {
	Code    string
	Warning bool
	Path    string
	Msg     string
}
