package compiler

import (
	"github.com/TacFlow/tac-language/ast"
	"github.com/TacFlow/tac-language/laya"
)

// ProgramIR is the whole compiled program (`tac compile --json`): every flow
// plus the LAYA data declared in the file. Every list is present, empty when
// nothing is declared.
type ProgramIR struct {
	Flows    []*FlowJSON    `json:"flows"`
	Tasks    []laya.Task    `json:"tasks"`
	Models   []laya.Model   `json:"models"`
	Episodes []laya.Episode `json:"episodes"`
	Datasets []laya.Dataset `json:"datasets"`
}

// CompileProgramIR compiles every flow and every task/model/episode/dataset
// declaration of program. Declarations keep file order; when two episodes
// share an id the last one wins (the analyzer warns TAC-LAYA-013).
//
// It does not run semantic analysis: it compiles whatever the parser read,
// including programs the analyzer rejects. Run semantic.Analyze first and
// compile only when it reports no errors (as `tac compile --json` does).
func CompileProgramIR(program *ast.Node) (*ProgramIR, error) {
	flows, err := CompileProgram(program)
	if err != nil {
		return nil, err
	}
	ir := &ProgramIR{
		Flows:    flows,
		Tasks:    []laya.Task{},
		Models:   []laya.Model{},
		Episodes: []laya.Episode{},
		Datasets: []laya.Dataset{},
	}
	if program == nil {
		return ir, nil
	}
	lastEpisode := map[string]int{} // id -> index in program.Nodes of its last declaration
	for i, n := range program.Nodes {
		if n.Type == ast.NodeEpisodeDecl {
			lastEpisode[n.Value] = i
		}
	}
	for i, n := range program.Nodes {
		switch n.Type {
		case ast.NodeTaskDecl:
			ir.Tasks = append(ir.Tasks, laya.TaskFromAST(n))
		case ast.NodeModelDecl:
			ir.Models = append(ir.Models, laya.ModelFromAST(n))
		case ast.NodeEpisodeDecl:
			if lastEpisode[n.Value] == i {
				ir.Episodes = append(ir.Episodes, laya.EpisodeFromAST(n))
			}
		case ast.NodeDatasetDecl:
			ir.Datasets = append(ir.Datasets, laya.DatasetFromAST(n))
		}
	}
	return ir, nil
}
