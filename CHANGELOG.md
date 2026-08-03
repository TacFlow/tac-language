# Changelog

All notable changes to the TAC Language will be documented in this file.

## [v0.2.0] — 2026-08-03

### 🏗️ Modular Architecture (Major Refactor)

- **9-module architecture** — split monolithic parser into reusable Go packages:
  - `ast/` — AST types with Walk, CollectFlows, NodeName, EdgeCondition helpers
  - `lexer/` — Tokenizer with comparison operators (>, <, >=, <=, ==, !=) and channel op (<-)
  - `parser/` — Modular parser using ast + lexer packages
  - `semantic/` — Semantic analyzer (DAG validation, cycle detection, skill registry)
  - `types/` — Trust type system (Secret, Untrusted, Fact, Hallucinable, Control)
  - `compiler/` — AST → Flow JSON compiler (TacFlow engine compatible)
  - `formatter/` — Canonical .tac formatter (round-trip: parse → format → parse)
  - `manifest/` — Flow manifest generator (inputs, skills, agents)
  - `cmd/tac/` — Unified CLI (parse, compile, fmt, validate)

### ✨ New Features

- **Comparison operators**: `>`, `<`, `>=`, `<=`, `==`, `!=` — lexed and parsed
- **Channel operator**: `<-` (assignment-style) — lexed and parsed
- **Expression parsing**: `parseExprValue()` handles `lhs op rhs` comparison expressions
- **Inline if/else blocks**: `parseBlock()` with brace-depth tracking for nested conditionals
- **Named arguments**: `skill foo(name: value)` properly parsed as NamedArg nodes
- **Semantic validation**: DAG cycle detection (Kahn's algorithm), dead node detection, skill registry lookup
- **Fuzz harness**: `FuzzParser` — resilient to arbitrary input

### 🧪 Test Suite

- **17 unit tests**: lexer ops, parser constructs, semantic analysis, trust types, formatter round-trip
- **Golden files**: 3 golden JSON files for official examples
- **Fuzz testing**: 819K+ iterations, zero crashes

### 🔧 Fixes

- Infinite loop in `parseFlowBody` when encountering unrecognized tokens
- `parseBlock()` not handling nested `{ }` brace blocks
- Edge conditions with comparison operators (`>`, `<`, etc.)
- Formatter not handling NamedArg and Condition nodes
- Self-assignment warning in `parser.go`

---

## [v0.1.0] — 2026-06-27

### 🚀 Initial Release

- **Language Specification** (`SPEC.md`) — Complete v2.0 specification with 13 sections
- **Go Parser** (`parser/tac_parser.go`) — Full parser implementation (monolithic)
- **Documentation** (`README.md`, `parser/README.md`) — Build, usage, and contribution instructions
- **Examples** (`examples/`) — Three complete `.tac` programs:
  - Web Q&A flow with parallel search + synthesis + fact-check
  - Knowledge graph builder from web pages
  - Multi-agent code review orchestrator
- **Benefits Analysis** (`BENEFITS.md`) — 10 strategic benefits of adopting TAC

### Parser Features

- Tokenizer with full Unicode support
- AST generation for all language constructs
- JSON output (stdout or file via `--output`)
- Standard library only (zero external dependencies)
