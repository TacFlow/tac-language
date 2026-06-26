# Changelog

All notable changes to the TAC Language will be documented in this file.

## [v0.1.0] — 2026-06-27

### 🚀 Initial Release

- **Language Specification** (`SPEC.md`) — Complete v2.0 specification with 13 sections covering syntax, memory architecture, trust types, flows, skills, swarm concurrency, context management, execution pipeline, and training data format
- **Go Parser** (`parser/tac_parser.go`) — Full parser implementation that converts `.tac` source files to structured JSON AST
- **Documentation** (`README.md`, `parser/README.md`) — Build, usage, and contribution instructions
- **Examples** (`examples/`) — Three complete `.tac` programs:
  - Web Q&A flow with parallel search + synthesis + fact-check
  - Knowledge graph builder from web pages
  - Multi-agent code review orchestrator
- **Benefits Analysis** (`BENEFITS.md`) — 10 strategic benefits of adopting TAC on the TacFlow platform

### Parser Features

- Tokenizer with full Unicode support
- AST generation for all language constructs:
  - Flow definitions with nodes, edges, and triggers
  - Skill calls with positional and named arguments
  - `remember` / `recall` / `forget` / `relate` statements
  - `context` blocks with nesting
  - `input` declarations with trust types
  - `agent` declarations
  - Conditional edges (`if` / `else`)
  - `for each` iteration blocks
  - Object literals, array literals, strings, numbers, booleans
- JSON output (stdout or file via `--output`)
- Standard library only (zero external dependencies)
