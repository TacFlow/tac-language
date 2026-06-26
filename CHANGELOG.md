# Changelog

All notable changes to the TAC Language project will be documented in this file.

## [v1.0.0] — 2026-06-27

### Added
- **TAC Language Specification v2.0** — Full language specification in English (`SPEC.md`)
  - Syntax, types, flows, memory architecture, execution pipeline, training data format
- **Go Parser** — Complete recursive-descent parser in Go
  - Tokenizer → Parser → JSON AST output pipeline
  - Supports all major TAC syntax: flows, nodes, skill calls, triggers, edges, conditions
  - Supports remember/recall/forget/relate statements
  - Supports context blocks, auto-summarize, agent declarations
  - Supports iterators (for each), object/array literals
- **Examples** — 3 complete `.tac` example files
  - Web Q&A with parallel search + fact-check
  - Knowledge Graph Builder
  - Multi-Agent Code Review Orchestrator
- **Benefits Analysis** — 10 strategic benefits of TAC (`BENEFITS.md`)
- **Documentation** — Full README with quick start, architecture, and usage instructions
