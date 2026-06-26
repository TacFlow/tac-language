# 🧬 TAC — The TacFlow Agentic Code

> *"TAC is not a language for writing code. It is a language for thinking like an agent."*

**TAC** is a domain-specific language (DSL) designed for **autonomous AI agents** within the TacFlow swarm ecosystem.  
It models how agents perceive, reason, remember, and execute tasks — as a **directed acyclic graph (DAG)** of skill invocations.

---

## 📦 Repository Structure

```
tac-language/
├── SPEC.md                  # Full language specification (English)
├── BENEFITS.md              # 10 strategic benefits of using TAC
├── parser/
│   ├── tac_parser.go        # Go parser — source code
│   ├── go.mod               # Go module definition
│   └── README.md            # Parser build & usage instructions
├── examples/
│   ├── web_qa.tac           # Web Q&A flow (parallel search + synthesis)
│   ├── graph_builder.tac    # Knowledge graph builder from web pages
│   └── multi_agent_review.tac # Multi-agent code review orchestrator
├── docs/                    # Future: additional documentation
├── LICENSE                  # MIT License
├── README.md                # This file
└── .gitignore
```

---

## 🚀 Quick Start

### 1. Clone

```bash
git clone https://github.com/tacflow1-tech/tac-language.git
cd tac-language
```

### 2. Build the Parser

```bash
cd parser
go build -o tac-parser .
```

### 3. Parse a `.tac` File

```bash
# Output AST to stdout
./tac-parser ../examples/web_qa.tac

# Save AST to file
./tac-parser ../examples/web_qa.tac --output ast.json
```

### 4. Inspect the AST

```bash
# Pretty-print the flow structure
./tac-parser ../examples/web_qa.tac | python3 -c "
import json, sys
ast = json.load(sys.stdin)
for n in ast.get('nodes', []):
    if n['type'] == 'Flow':
        print(f'Flow: {n[\"value\"]}')
        print(f'  Nodes: {len(n.get(\"nodes\",[]))}')
        print(f'  Edges: {len(n.get(\"edges\",[]))}')
        print(f'  Children: {len(n.get(\"children\",[]))}')
"
```

---

## 📖 Language Overview

### Core Concepts

| Concept | Description |
|---------|-------------|
| **Memories are Variables** | Every named value is persistent by default — no `let x = 42` |
| **Skills are the Standard Library** | No functions, only `skill` — maps 1:1 to real TacFlow APIs |
| **Execution is a DAG** | No call stack — every program is a directed acyclic graph |
| **Concurrency is Swarm Delegation** | No threads — delegate to peer agents in the swarm |
| **Context is Scope** | No nested `{}` — context windows model the attention span |
| **Hybrid Memory** | BM25 + Vector + Graph — queried simultaneously |

### Memory Architecture (3 Layers)

| Layer | Type | Strength | Weight |
|-------|------|----------|:------:|
| BM25 | Sparse (keywords) | Exact matching | 30% |
| Vector | Dense (768d embeddings) | Semantic similarity | 40% |
| Graph | Relational (SQLite + edges) | Indirect connections | 30% |

### Trust Types

| Type | Origin | Behavior |
|------|--------|----------|
| `Secret` | Credentials | Never echoed or logged |
| `Untrusted` | User input | Must be validated |
| `Fact` | Verified memory | High confidence |
| `Hallucinable` | LLM output | Must be fact-checked |
| `Control` | Runtime state | Read-only |

### Example: Web Q&A Flow

```tac
flow "Web Q&A" {
  input question: Untrusted

  node "search_web"    -> skill web_search(query: question, count: 3)
  node "search_memory" -> skill memory_search(query: question, scope: "shared")
  node "search_graph"  -> skill graph_search(query: question, depth: 2)
  node "synthesize"    -> skill llm.chat(prompt: "Answer using:", context: [..])
  node "verify"        -> skill verify(source: synthesize.result)
  node "speak"         -> skill tts.speak(text: synthesize.result)

  search_web -> synthesize
  search_memory -> synthesize
  search_graph -> synthesize
  synthesize -> verify -> speak

  on "user_message" -> search_web
}
```

---

## 🔧 Parser

Written in **Go** (standard library only), the parser converts `.tac` source files into a structured JSON AST.

```bash
Usage: tac-parser <input.tac> [--output ast.json]
```

See [`parser/README.md`](parser/README.md) for full build and usage instructions.

### AST Node Types

`Program` → `Flow` | `RememberStmt` | `RecallStmt` | `RelateStmt` | `ForgetStmt` | `ContextBlock` | `AutoSummarize`

Each `Flow` contains: `Node` → `SkillCall` | `Edge` | `Trigger` | `Input` | `AgentDecl`

---

## 📚 Examples

| File | Description |
|------|-------------|
| [`examples/web_qa.tac`](examples/web_qa.tac) | Parallel search (web + memory + graph), synthesis, fact-check, speak |
| [`examples/graph_builder.tac`](examples/graph_builder.tac) | Extract concepts from a URL, build knowledge graph with relationships |
| [`examples/multi_agent_review.tac`](examples/multi_agent_review.tac) | Spawn 3 reviewer agents in parallel, consolidate results |

---

## 📄 Specification

The full language specification is available in [`SPEC.md`](SPEC.md) (English, 13 sections):

1. Introduction
2. Core Philosophy
3. Hybrid Memory Architecture
4. Language Syntax
5. Types System — Trust Types
6. Flows — The Execution Model
7. Skills — The Standard Library
8. Swarm Concurrency
9. Context Window Management
10. Execution Pipeline
11. Training Data Format (LoRA / QLoRA)
12. Parser Implementation Guide
13. Appendix: Complete Examples

---

## 💡 Benefits

See [`BENEFITS.md`](BENEFITS.md) for the full analysis of the 10 strategic benefits of adopting TAC:

1. **DSL Unificada** — One language for all agents
2. **Auditoria Total** — Traceable from source to execution
3. **Training Data Autogerado** — Every run feeds LoRA/QLoRA fine-tuning
4. **Portabilidade entre Swarms** — Language-agnostic AST
5. **Compile-time Safety** — Trust Types prevent data leaks
6. **Otimização DAG** — Automatic parallelism, branch pruning
7. **Memória 3 Camadas** — BM25 + Vector + Graph fusion
8. **Marketplace de Skills** — Versioned, shareable skill packages
9. **Simulação Pré-Execução** — Cost estimation before execution
10. **Diferencial Competitivo** — Unique moat vs all competitors

---

## 🧪 Try It

```bash
# Clone, build, and parse
git clone https://github.com/tacflow1-tech/tac-language.git
cd tac-language/parser
go build -o tac-parser .
./tac-parser ../examples/web_qa.tac | jq '.'
```

---

## 📜 License

MIT License — see [`LICENSE`](LICENSE).

Copyright (c) 2026 Tacflow

---

## 🏆 About TacFlow

**TAC** is part of the **TacFlow** platform — an ecosystem of autonomous AI agents operating in swarms, sharing memory, skills, and reputation.  
Learn more at [tacflow.com](https://tacflow.com) (coming soon).

---

*"A language is not a tool. It is a way of thinking."*
