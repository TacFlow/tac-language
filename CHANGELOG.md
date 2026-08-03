# Changelog

All notable changes to the TAC Language will be documented in this file.

## [v0.2.0] — 2026-08-03

### 🏗️ Modular Architecture (Major Refactor)

- **9-module architecture** — split monolithic parser into reusable Go packages:
  - `ast/` — AST types with Walk, CollectFlows, NodeName, EdgeCondition helpers
  - `lexer/` — Tokenizer with comparison operators (>, <, >=, <=, ==, !=), channel op (<-), and @ version qualifier
  - `parser/` — Modular parser using ast + lexer packages; supports `skill@version` syntax
  - `semantic/` — Semantic analyzer with trust-type dataflow checking, strict/production mode, dynamic skill registry
  - `types/` — Trust type system (Secret, Untrusted, Fact, Hallucinable, Control)
  - `compiler/` — AST → Flow JSON compiler with structured Condition IR and version metadata
  - `formatter/` — Deterministic canonical .tac formatter (sorted map keys for byte-stable output)
  - `manifest/` — Flow manifest generator (inputs, skills, agents)
  - `cmd/tac/` — Unified CLI (parse, compile, fmt, validate) with --mode flag

### 🔒 Security Hardening — Trust Type Dataflow Checker

- **Dataflow type checker**: semantic analyzer now tracks trust types across the DAG and verifies every value crossing a skill boundary satisfies the skill's `ArgTypes` requirements.
- **Strict built-in rules**: `memory_store.text` → Fact, `verify.source` → Hallucinable, `validate.value` → Untrusted, `swarm_teach.content` → Fact, `graph_relate.source/target` → Fact.
- **Explicit conversion enforcement**: Untrusted → Fact requires explicit `validate()`, Hallucinable → Fact requires explicit `verify()`, Secret → Fact is forbidden.
- **Examples fixed**: `web_qa.tac` learn node now uses `verify.result` (not raw `synthesize.result`); `graph_builder.tac` added verify step between extract and store_nodes.

### 🎯 Validation Modes — Development vs Production

- `tac validate --mode development flow.tac` — unknown skills are warnings
- `tac validate --mode production flow.tac` — unknown skills, unsigned/unversioned skills, missing schemas, missing artifact digests are all **compile errors**

### 🧬 Dynamic Skill Registry

- **Registry type** with `Register`/`Lookup`/`LookupVersioned` — supports Python/containerized skills with full metadata (runtime, entrypoint, artifact digest, signature, input/output schemas, capabilities, permissions, execution constraints)
- Production mode enforces: digest required, signature required, input/output schemas required for dynamic skills

### 📐 Dead Node Detection — Trigger-Based Reachability

- Reachability now starts from **trigger targets and entry points**, not all indegree-zero nodes. In a disconnected DAG, each component has a zero-indegree node; only trigger-reachable nodes are considered alive.

### 🔧 Condition IR — Structured Expressions

- Conditions no longer compiled as opaque strings; the compiler emits a `ConditionIR` with operator, left operand (kind + node + path), and right operand (kind + value) — structured, machine-readable, no reparsing needed at runtime.

### 📦 Version Consistency

- **Language version**: 2.0 (SPEC)
- **Compiler version**: 0.2.0 (CLI)
- **IR version**: 1.0 (Flow JSON)
- **Manifest version**: 1.0
- Every compiled Flow JSON now includes a `language` metadata block with name, language_version, compiler_version, ir_version.

### 🧪 Test Suite

- **14 new tests**: compile golden, compile deterministic, formatter byte-stable, unknown skill strict mode, trust flow rejected, validated chain, secret forbidden, condition IR, Python skill manifest, dead node trigger-based, Hallucinable requires verify, version metadata, fuzz pipeline

### 🚀 CLI Improvements — Strict Mode & External Registry

- `--strict` flag added as alias for `--mode production` (more ergonomic: `tac compile flow.tac --strict`)
- `--registry <file.json>` loads custom Python/container skills from an external JSON file, merged on top of builtins
- `skills.json` template included: Python skill packages require runtime, entrypoint, artifact digest, signature, input/output schemas

### 📋 SPEC §7.2 — External Python Skill Registry

- Custom skills section rewritten: defines JSON-based Python skill packages with full metadata (runtime, entrypoint, digest, signature, schemas, permissions, execution constraints)
- Production mode enforces: unknown skill → error, unversioned → error, unsigned → error, missing schema → error

---

## [v0.1.0] — 2026-06-27

### 🎉 Initial commit: TAC Language v0.1.0

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
