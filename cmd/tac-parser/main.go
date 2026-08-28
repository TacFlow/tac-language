// Command tac-parser parses TAC Language (.tac) source files into AST JSON.
//
// Usage:
//
//	tac-parser <input.tac> [--output ast.json] [--validate]
//
// With --validate, the parser also runs semantic analysis and prints diagnostics.
//
// Build:
//
//	go build -o tac-parser ./cmd/tac-parser
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/TacFlow/tac-language/parser"
	"github.com/TacFlow/tac-language/semantic"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input.tac> [--output ast.json] [--validate]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nTAC — The TacFlow Agentic Code Parser\n")
		fmt.Fprintf(os.Stderr, "Parses .tac files and outputs the AST as JSON.\n")
		os.Exit(1)
	}

	inputPath := os.Args[1]
	outputPath := ""
	validate := false

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--output":
			if i+1 < len(os.Args) {
				outputPath = os.Args[i+1]
				i++
			}
		case "--validate":
			validate = true
		}
	}

	// Read source
	source, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", inputPath, err)
		os.Exit(1)
	}

	// Parse
	ast, err := parser.ParseSource(string(source))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		os.Exit(1)
	}

	// Semantic analysis (if requested)
	exitCode := 0
	if validate {
		analyzer := semantic.New()
		diags := analyzer.Analyze(ast)
		if len(diags) > 0 {
			for _, d := range diags {
				fmt.Fprintf(os.Stderr, "%s\n", d.String())
			}
			if analyzer.HasErrors() {
				exitCode = 2
			}
		}
	}

	// Serialize to JSON
	jsonBytes, err := json.MarshalIndent(ast, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON serialization error: %v\n", err)
		os.Exit(1)
	}

	if outputPath != "" {
		err = os.WriteFile(outputPath, jsonBytes, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", outputPath, err)
			os.Exit(1)
		}
		fmt.Printf("AST written to %s (%d bytes)\n", outputPath, len(jsonBytes))
	} else {
		out := strings.TrimSpace(string(jsonBytes))
		if out == "" {
			out = "{}"
		}
		fmt.Println(out)
	}

	os.Exit(exitCode)
}
