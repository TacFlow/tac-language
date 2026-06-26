# TAC Language Parser

Go implementation of the TAC (The TacFlow Agentic Code) language parser.  
Converts `.tac` source files into structured JSON AST (Abstract Syntax Tree).

## Quick Start

```bash
# Build
cd parser
go build -o tac-parser .

# Parse a .tac file (output to stdout)
./tac-parser ../examples/web_qa.tac

# Parse and save AST to file
./tac-parser ../examples/web_qa.tac --output ast.json

# Pretty-print with jq
./tac-parser ../examples/web_qa.tac | jq '.nodes[0].nodes | length'
```

## Installation

```bash
# Option A: Build from source
git clone https://github.com/tacflow1-tech/tac-language.git
cd tac-language/parser
go build -o tac-parser .

# Option B: Copy to PATH
cp tac-parser /usr/local/bin/
```

## Usage

```text
Usage: tac-parser <input.tac> [--output ast.json]

Arguments:
  input.tac          Path to the TAC source file (required)
  --output ast.json  Write AST JSON to file (optional, defaults to stdout)

Exit codes:
  0  Success
  1  Error (file not found, parse error, etc.)
```

## AST Output Structure

The parser produces a JSON AST with the following node types:

| Node Type        | Description                          |
|------------------|--------------------------------------|
| `Program`        | Root node — contains all statements  |
| `Flow`           | A complete flow with nodes + edges   |
| `Node`           | A single step in the flow            |
| `SkillCall`      | Invocation of a skill                |
| `RememberStmt`   | Persistent storage declaration       |
| `RecallStmt`     | Retrieval declaration                |
| `ForgetStmt`     | Deletion declaration                 |
| `RelateStmt`     | Graph edge creation                  |
| `Edge`           | Dependency between nodes             |
| `Trigger`        | Event-driven activation              |
| `Condition`      | Conditional branch on edge           |
| `ContextBlock`   | Context scope declaration            |
| `AutoSummarize`  | Auto-summarization directive         |
| `Input`          | Flow input declaration               |
| `AgentDecl`      | Agent declaration                    |

## Supported Syntax

- ✅ `flow "name" { ... }` — Flow definitions
- ✅ `node "name" -> skill name(args) { attrs }` — Node definitions with skill calls
- ✅ `source -> target` — Edge definitions (DAG)
- ✅ `source -> target { if: condition }` — Conditional edges
- ✅ `on "event" -> node` — Event-driven triggers
- ✅ `on "event" { priority: 5 } -> node` — Trigger with attributes
- ✅ `input var: Type` — Input declarations with trust types
- ✅ `remember name = value { attrs }` — Persistent storage
- ✅ `recall name { attrs }` — Memory retrieval
- ✅ `forget name { attrs }` — Memory deletion
- ✅ `relate source -> target { attrs }` — Graph edge creation
- ✅ `context "name" { ... }` — Context blocks
- ✅ `auto_summarize(on: "overflow")` — Auto-summarize directives
- ✅ `agent "name" { attrs }` — Agent declarations
- ✅ `for each x in list { ... }` — Iteration blocks
- ✅ `{ key: value, key: "string" }` — Object literals
- ✅ `[item1, item2]` — Array literals
- ✅ `// comments` — Line comments
- ✅ Strings, numbers, booleans, identifiers
- ✅ `skill identifier(args) { named_attrs }` — Full skill call syntax

## Test

```bash
cd parser
go build -o tac-parser .
./tac-parser ../examples/web_qa.tac
```

## Dependencies

- Go 1.26+
- Standard library only (no external dependencies)
