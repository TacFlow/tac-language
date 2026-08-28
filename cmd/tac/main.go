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
	"sort"

	"github.com/TacFlow/tac-language/ast"
	"github.com/TacFlow/tac-language/compiler"
	"github.com/TacFlow/tac-language/formatter"
	"github.com/TacFlow/tac-language/parser"
	"github.com/TacFlow/tac-language/semantic"
)

const version     = "0.3.0"
const langVersion = "0.3"
const irVersion   = "1.1"

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
			fmt.Fprintln(os.Stderr, "Usage: tac compile <input.tac> [--mode production|development]")
			os.Exit(1)
		}
		mode := parseMode()
		cmdCompile(os.Args[2], mode)

	case "fmt":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: tac fmt <input.tac>")
			os.Exit(1)
		}
		cmdFmt(os.Args[2])

	case "validate":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: tac validate <input.tac> [--mode production|development]")
			os.Exit(1)
		}
		mode := parseMode()
		cmdValidate(os.Args[2], mode)

	case "inspect":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: tac inspect <input.tac>")
			os.Exit(1)
		}
		cmdInspect(os.Args[2])

	case "fingerprint":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: tac fingerprint <input.tac>")
			os.Exit(1)
		}
		mode := parseMode()
		cmdFingerprint(os.Args[2], mode)

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
  tac parse <input.tac>                  Parse and print AST as JSON
  tac compile <input.tac> [flags]        Parse, validate, and output Flow JSON
  tac fmt <input.tac>                    Format .tac source (canonical style)
  tac validate <input.tac> [flags]       Parse and run semantic analysis
  tac inspect <input.tac>                Show flow structure summary
  tac fingerprint <input.tac> [flags]    Show flow compilation fingerprint
  tac version                            Print version
  tac help                               Show this help

Flags:
  --strict                  Alias for --mode production
  --mode production|development   Strictness mode (default: development)
  --registry <file.json>    Load custom skill registry from JSON file

Examples:
  tac parse my_flow.tac
  tac compile my_flow.tac --strict | jq
  tac compile my_flow.tac --registry skills.json
  tac fmt my_flow.tac > formatted.tac
  tac validate my_flow.tac --mode production
  tac inspect my_flow.tac
  tac fingerprint my_flow.tac --strict`)
}

func readSource(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	return string(data), nil
}

func parseMode() semantic.Mode {
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--strict":
			return semantic.ModeProduction
		case "--mode":
			if i+1 < len(os.Args) {
				switch os.Args[i+1] {
				case "production":
					return semantic.ModeProduction
				case "development":
					return semantic.ModeDevelopment
				}
			}
		}
	}
	return semantic.ModeDevelopment // default
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

func cmdCompile(path string, mode semantic.Mode) {
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

	registry := loadRegistry()
	analyzer := semantic.NewWithRegistry(registry, mode)
	diags := analyzer.Analyze(program)
	if analyzer.HasErrors() {
		fmt.Fprintf(os.Stderr, "Semantic validation failed (%s mode):\n", mode)
		for _, d := range diags {
			if d.Severity == semantic.SeverityError {
				fmt.Fprintf(os.Stderr, "  %s\n", d)
			}
		}
		os.Exit(1)
	}
	for _, d := range diags {
		if d.Severity == semantic.SeverityWarning {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", d)
		}
	}

	flows, err := compiler.CompileProgram(program)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Compile error: %v\n", err)
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

func cmdValidate(path string, mode semantic.Mode) {
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

	registry := loadRegistry()
	analyzer := semantic.NewWithRegistry(registry, mode)
	diags := analyzer.Analyze(program)

	fmt.Printf("Mode: %s", mode)
	if registry != nil {
		fmt.Printf(" (registry: %d skills)", len(registry.Names()))
	}
	fmt.Println()
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

func cmdInspect(path string) {
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
	flows := ast.CollectFlows(program)
	fmt.Printf("File:     %s\n", path)
	fmt.Printf("Flows:    %d\n", len(flows))
	for _, flow := range flows {
		nodes := ast.CollectNodes(flow)
		edges := ast.CollectEdges(flow)
		fmt.Printf("\n  Flow %q:\n", flow.Value)
		fmt.Printf("    Nodes:   %d\n", len(nodes))
		fmt.Printf("    Edges:   %d\n", len(edges))

		skills := make(map[string]bool)
		for _, node := range nodes {
			for _, child := range node.Children {
				if child.Type == ast.NodeSkillCall {
					skills[child.Value] = true
				}
			}
		}
		fmt.Printf("    Skills:  %d", len(skills))
		if len(skills) > 0 {
			fmt.Printf(" (%v)", sortedKeys(skills))
		}
		fmt.Println()
	}
}

func cmdFingerprint(path string, mode semantic.Mode) {
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
	registry := loadRegistry()

	analyzer := semantic.NewWithRegistry(registry, mode)
	diags := analyzer.Analyze(program)
	if analyzer.HasErrors() {
		fmt.Fprintf(os.Stderr, "Validation failed (%s mode):\n", mode)
		for _, d := range diags {
			if d.Severity == semantic.SeverityError {
				fmt.Fprintf(os.Stderr, "  %s\n", d)
			}
		}
		os.Exit(1)
	}

	flows, err := compiler.CompileProgram(program)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Compile error: %v\n", err)
		os.Exit(1)
	}

	// Compute registry snapshot
	regSnapshot := ""
	if registry != nil {
		if d, err := registry.SnapshotDigest(nil); err == nil {
			regSnapshot = d
		}
	}

	for i, fj := range flows {
		irJSON, err := compiler.ToJSON(fj)
		if err != nil {
			fmt.Fprintf(os.Stderr, "JSON error: %v\n", err)
			continue
		}
		fp := semantic.Fingerprint(irJSON, version, regSnapshot, "1.0")
		fj.Fingerprint = fp
		fmt.Printf("Flow %d (%q):\n", i+1, fj.Name)
		fmt.Printf("  Fingerprint:       %s\n", fp)
		fmt.Printf("  Language version:  %s\n", langVersion)
		fmt.Printf("  Compiler version:  %s\n", version)
		fmt.Printf("  IR version:        %s\n", irVersion)
		if regSnapshot != "" {
			fmt.Printf("  Registry snapshot: %s\n", regSnapshot)
		}
		fmt.Printf("  Trust policy:      1.0\n")
		fmt.Println()
	}
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// loadRegistry reads a --registry <file.json> argument if present and
// returns a Registry preloaded with the file's skills merged on top of
// the built-in standard library. Returns nil (use builtins only) when
// no --registry argument is given.
func loadRegistry() *semantic.Registry {
	path := registryPath()
	if path == "" {
		return nil // use builtins only
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot read registry file %q: %v (using builtins only)\n", path, err)
		return nil
	}
	var skills []semantic.SkillSpec
	if err := json.Unmarshal(data, &skills); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: invalid registry JSON in %q: %v (using builtins only)\n", path, err)
		return nil
	}
	reg := semantic.BuiltinRegistry()
	for _, s := range skills {
		if s.Name == "" {
			fmt.Fprintf(os.Stderr, "Warning: skipping registry entry with empty name\n")
			continue
		}
		reg.Register(s)
	}
	fmt.Fprintf(os.Stderr, "Loaded %d custom skills from %s\n", len(skills), path)
	return reg
}

// registryPath extracts the --registry <file.json> value from os.Args.
func registryPath() string {
	for i := 2; i < len(os.Args)-1; i++ {
		if os.Args[i] == "--registry" {
			return os.Args[i+1]
		}
	}
	return ""
}
