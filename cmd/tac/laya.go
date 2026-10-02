package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/TacFlow/tac-language/compiler"
	"github.com/TacFlow/tac-language/laya"
	"github.com/TacFlow/tac-language/parser"
	"github.com/TacFlow/tac-language/semantic"
)

// flagValue returns the value after `name` in os.Args[2:], or "".
func flagValue(args []string, name string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == name {
			return args[i+1]
		}
	}
	return ""
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

// loadTasks reads a --tasks file: a JSON array of {name, questions} — the
// shape of laya.tasks.list on the platform (DESIGN §5.4).
func loadTasks(path string) ([]laya.Task, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read tasks file %q: %w", path, err)
	}
	var tasks []laya.Task
	if err := json.Unmarshal(b, &tasks); err != nil {
		return nil, fmt.Errorf("invalid tasks file %q: %w", path, err)
	}
	for i, t := range tasks {
		if t.Name == "" {
			return nil, fmt.Errorf("invalid tasks file %q: entry %d has no name", path, i)
		}
	}
	return tasks, nil
}

// programJSON is `tac compile --json`: the whole program IR (flows, tasks,
// models, episodes, datasets), indented.
func programJSON(src string, reg *semantic.Registry, tasks []laya.Task, mode semantic.Mode) ([]byte, []semantic.Diagnostic, error) {
	program, err := parser.ParseSource(src)
	if err != nil {
		return nil, nil, err
	}
	a := semantic.NewWithRegistry(reg, mode)
	if tasks != nil {
		a.SetTasks(tasks)
	}
	diags := a.Analyze(program)
	if a.HasErrors() {
		return nil, diags, fmt.Errorf("semantic validation failed with %d errors", len(a.Errors()))
	}
	ir, err := compiler.CompileProgramIR(program)
	if err != nil {
		return nil, diags, err
	}
	b, err := json.MarshalIndent(ir, "", "  ")
	return b, diags, err
}

// episodeJSON is `tac episode`: the bare episode of the file (or the one
// named by id), indented. More than one episode needs --id.
func episodeJSON(src, id string) ([]byte, []semantic.Diagnostic, error) {
	program, err := parser.ParseSource(src)
	if err != nil {
		return nil, nil, err
	}
	a := semantic.New()
	diags := a.Analyze(program)
	if a.HasErrors() {
		return nil, diags, fmt.Errorf("semantic validation failed with %d errors", len(a.Errors()))
	}
	ir, err := compiler.CompileProgramIR(program)
	if err != nil {
		return nil, diags, err
	}
	var pick *laya.Episode
	switch {
	case id != "":
		for i := range ir.Episodes {
			if ir.Episodes[i].ID == id {
				pick = &ir.Episodes[i]
			}
		}
		if pick == nil {
			return nil, diags, fmt.Errorf("no episode %q in the file", id)
		}
	case len(ir.Episodes) == 1:
		pick = &ir.Episodes[0]
	case len(ir.Episodes) == 0:
		return nil, diags, fmt.Errorf("the file declares no episode")
	default:
		return nil, diags, fmt.Errorf("the file declares %d episodes; choose one with --id <id>", len(ir.Episodes))
	}
	b, err := json.MarshalIndent(pick, "", "  ")
	return b, diags, err
}
