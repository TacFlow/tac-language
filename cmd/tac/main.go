// tac is the TAC Language CLI — parse, validate, compile, and format .tac files.
//
// Usage:
//
//	tac parse <input.tac>              Parse and print AST as JSON
//	tac compile <input.tac>            Parse, validate, and output Flow JSON
//	tac fmt <input.tac>                Format .tac source (canonical style)
//	tac validate <input.tac>           Parse and run semantic analysis
//	tac version                        Print version
//
// Build:
//
//	go build -o tac ./cmd/tac
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tacflow1-tech/tac-language/compiler"
	"github.com/tacflow1-tech/tac-language/formatter"
	"github.com/tacflow1-tech/tac-language/parser"
	"github.com/tacflow1-tech/tac-language/semantic"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "parse":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: tac parse <input.tac>")
			os.Exit(1)
		}
		cmdParse(os.Args[2])

	case "compile":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: tac compile <input.tac>")
			os.Exit(1)
		}
		cmdCompile(os.Args[2])

	case "fmt":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: tac fmt <input.tac>")
			os.Exit(1)
		}
		cmdFmt(os.Args[2])

	case "validate":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: tac validate <input.tac>")
			os.Exit(1)
		}
		cmdValidate(os.Args[2])

	case "version", "--version", "-v":
		fmt.Printf("tac %s\n", version)

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`tac — TAC Language CLI v` + version + `

tac is The TacFlow Agentic Code — a DSL for autonomous AI agents.

Usage:
  tac parse <input.tac>        Parse and print AST as JSON
  tac compile <input.tac>      Parse, validate, and output Flow JSON
  tac fmt <input.tac>          Format .tac source (canonical style)
  tac validate <input.tac>     Parse and run semantic analysis
  tac version                  Print version
  tac help                     Show this help

Examples:
  tac parse my_flow.tac
  tac compile my_flow.tac | jq
  tac fmt my_flow.tac > formatted.tac
  tac validate my_flow.tac`)
}

func readSource(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	return string(data), nil
}

func cmdParse(path string) {
	source, err := readSource(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ast, err := parser.ParseSource(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		os.Exit(1)
	}

	jsonBytes, err := json.MarshalIndent(ast, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(jsonBytes))
}

func cmdCompile(path string) {
	source, err := readSource(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	program, err := parser.ParseSource(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		os.Exit(1)
	}

	flows, diags, err := compiler.CompileAndValidate(program)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Compile error: %v\n", err)
		for _, d := range diags {
			if d.Severity == semantic.SeverityError {
				fmt.Fprintf(os.Stderr, "  %s\n", d)
			}
		}
		os.Exit(1)
	}

	jsonBytes, err := json.MarshalIndent(flows, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(jsonBytes))
}

func cmdFmt(path string) {
	source, err := readSource(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	program, err := parser.ParseSource(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(formatter.Format(program))
}

func cmdValidate(path string) {
	source, err := readSource(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	program, err := parser.ParseSource(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		os.Exit(1)
	}

	analyzer := semantic.New()
	diags := analyzer.Analyze(program)

	if len(diags) == 0 {
		fmt.Println("✅ No issues found.")
		return
	}

	errors := 0
	warnings := 0
	for _, d := range diags {
		if d.Severity == semantic.SeverityError {
			errors++
			fmt.Printf("❌ %s\n", d)
		} else {
			warnings++
			fmt.Printf("⚠️  %s\n", d)
		}
	}

	fmt.Printf("\n%d error(s), %d warning(s)\n", errors, warnings)
	if errors > 0 {
		os.Exit(1)
	}
}
