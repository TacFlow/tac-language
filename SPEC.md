# TAC — The TacFlow Agentic Code

**Language Specification v2.0**

> *"TAC is not a language for writing code. It is a language for thinking like an agent."*

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Core Philosophy](#2-core-philosophy)
3. [Hybrid Memory Architecture (BM25 + Vector + Graph)](#3-hybrid-memory-architecture-bm25--vector--graph)
4. [Language Syntax](#4-language-syntax)
5. [Types System — Trust Types](#5-types-system--trust-types)
6. [Flows — The Execution Model](#6-flows--the-execution-model)
7. [Skills — The Standard Library](#7-skills--the-standard-library)
8. [Swarm Concurrency](#8-swarm-concurrency)
9. [Context Window Management](#9-context-window-management)
10. [Execution Pipeline](#10-execution-pipeline)
11. [Training Data Format (LoRA / QLoRA)](#11-training-data-format-lora--qlora)
12. [Parser Implementation Guide](#12-parser-implementation-guide)
13. [Appendix: Complete Examples](#13-appendix-complete-examples)

---

## 1. Introduction

**TAC** (The TacFlow Agentic Code) is a domain-specific language (DSL) designed for autonomous AI agents within the TacFlow swarm ecosystem. It is not intended for general-purpose programming — it is a **thinking language** that models how agents perceive, reason, remember, and execute tasks.

### 1.1 Why TAC?

| Problem | TAC Solution |
|---------|-------------|
| Variables are ephemeral | `remember` makes everything persistent by default |
| Function calls are opaque | `skill` invocations are transparent, async, and traceable |
| Linear execution is limiting | `flow` defines explicit DAGs with parallelism |
| Concurrency is complex | `agent` delegation is the only concurrency primitive |
| Context windows overflow | `auto_summarize` manages token budgets explicitly |
| Security is manual | `Secret`, `Untrusted`, `Fact` types enforce safety at compile time |

### 1.2 Who Interprets TAC?

The TAC interpreter is a **4-stage pipeline** composed of:

| Role | Component | Implementation |
|------|-----------|---------------|
| **Parser** | `tac-parser` | Compiled Go binary (~2 MB), subprocess |
| **Semantic Analyzer** | Go | `semantic` package — validates DAG, types, skills |
| **Compiler** | Go | `compiler` package — transforms AST into Flow JSON |
| **Executor** | TacFlow Runtime | `flow-management` native skill |
| **Planner/Author** | Human or LLM Agent | Creates/modifies TAC source — does NOT compile TAC |
| **Memory Logger** | Agent | Post-execution calls to `memory_store` + `swarm_teach` |

> **Key distinction:** The LLM (TacFlow Architect or any agent) *authors* TAC source code.
> It does NOT compile TAC. Compilation from AST → Flow JSON is a **deterministic Go function**
> with no LLM involvement — it is fast, reproducible, and auditable.

---

## 2. Core Philosophy

### 2.1 Memories are Variables

In TAC, there is no `let x = 42`. Every named value is **persistent by default** — it lives beyond execution scope.

```
why: Agents operate in a continuous context. Nothing is truly "local".
     If a value matters enough to be named, it matters enough to be remembered.
```

### 2.2 Tools are the Standard Library

There is no `def fn()`. There is only `skill` — the fundamental unit of computation. Skills map 1:1 to real TacFlow capabilities (memory_search, web_search, vision_tool, tts_tool, etc.).

```
why: Agents don't define functions. They invoke capabilities.
     The set of available skills _is_ the standard library.
```

### 2.3 Execution is a DAG, not a Stack

TAC has no call stack. Every program is a **directed acyclic graph (DAG)** of nodes connected by edges. This mirrors the flow-management system natively.

```
why: Agents orchestrate workflows with explicit parallelism,
     dependencies, conditions, and event triggers. A stack is too linear.
```

### 2.4 Concurrency is Swarm Delegation

TAC does not spawn threads or goroutines. It delegates tasks to peer agents in the swarm. This is the **only** concurrency model.

```
why: The swarm _is_ the concurrency primitive.
     Adding threads to an agent would break the swarm's reputation system.
```

### 2.5 Context is Scope

TAC has no nested `{}` blocks. Instead, it has **context windows** that model the agent's attention span. When a context window fills, it auto-summarizes.

```
why: Agents operate within a finite context window (~4096 tokens for small,
     ~128K for large). The language must model overflow and summarization
     as first-class operations.
```

---

## 3. Hybrid Memory Architecture (BM25 + Vector + Graph)

TAC uses a **3-layer memory architecture** for retrieval. All three layers are queried simultaneously and fused into a unified ranking.

### 3.1 Layer 1 — BM25 (Keyword/Sparse)

| Property | Value |
|----------|-------|
| Algorithm | Okapi BM25 (sparse retrieval) |
| Index | Inverted term-frequency index |
| Strengths | Exact keyword matching, fast, low-latency |
| Use case | Code snippets, proper nouns, IDs, version numbers |

```tac
// BM25 is implicit — keywords are extracted automatically
remember concept = "TAC Language" {
  keywords: ["TAC", "linguagem", "agente", "swarm"]  // BM25 fuel
}
```

### 3.2 Layer 2 — Vector (Semantic/Dense)

| Property | Value |
|----------|-------|
| Algorithm | Cosine similarity on embeddings (768d or 1536d) |
| Index | HNSW (Hierarchical Navigable Small World) |
| Strengths | Semantic matching, handles paraphrasing |
| Use case | Natural language queries, conceptual search |

```tac
// Vector embeddings are auto-generated
remember concept = "TAC Language" {
  embedding: auto  // 768d vector generated by embedding model
}
```

### 3.3 Layer 3 — Graph (Relational)

| Property | Value |
|----------|-------|
| Storage | SQLite with nodes + edges tables |
| Traversal | BFS with configurable depth (1-5 hops) |
| Strengths | Discovers indirect connections |
| Use case | "What depends on X?", "Find related concepts" |

```tac
// Graph edges are explicit
relate concept "TAC Language" -> "Flow Management" {
  type: "depends_on"
  weight: 0.95
}
```

### 3.4 Hybrid Query — Unified Retrieval

```tac
// Query all three layers simultaneously
results <- search_hybrid("TAC execution model") {
  bm25_weight: 0.3    // 30% keyword matching
  vector_weight: 0.4  // 40% semantic similarity
  graph_weight: 0.3   // 30% graph traversal
  
  graph_expand: true
  graph_depth: 2
  
  top_k: 5
}

// Each result includes: {text, score, retrieval_path, graph_trail}
```

**Scoring formula:**

```
score_final = 0.3 × score_bm25 + 0.4 × score_vec + 0.3 × score_graph
```

---

## 4. Language Syntax

### 4.1 Comments

```tac
// Single-line comment

//- Multi-line
//- comment block
//- style
```

### 4.2 Remember — Persistent Storage

```tac
remember <name> = <value> {
  type: <string>        // Optional semantic type
  tags: [<string>, ...] // Optional classification tags
  embedding: auto|skip  // Auto-generate vector embedding
  keywords: [<string>]  // Optional BM25 keywords (auto-extracted if omitted)
}
```

**Examples:**

```tac
remember app_name = "Todo List"
remember max_retries = 3
remember config = { host: "localhost", port: 8080 } // Structured values
```

### 4.3 Recall — Retrieval

```tac
recall <name>                        // Exact match by name
recall <name> { fuzzy: true }        // Fuzzy/semantic match
```

**Hybrid search:**

```tac
results <- search_hybrid(<query_string>) {
  bm25_weight: 0.3
  vector_weight: 0.4
  graph_weight: 0.3
  top_k: 5
}
```

### 4.4 Forget — Explicit Deletion

```tac
forget <name>
forget <name> { cascade: true }  // Also remove related graph edges
```

### 4.5 Relate — Graph Edge Creation

```tac
relate <source> -> <target> {
  type: <string>       // Relationship type (required)
  weight: <float>      // 0.0 to 1.0 (default 0.5)
  description: <string> // Human-readable explanation
}
```

**Relationship types (built-in):**

| Type | Description |
|------|-------------|
| `depends_on` | Source requires target |
| `implements` | Source implements target |
| `is_a` | Taxonomic relationship |
| `contains` | Source contains target |
| `leads_to` | Source produces target |
| `references` | Source references target |

### 4.6 Skill Invocation

```tac
<result_var> <- <skill_name>(<positional_args>) {
  <named_arg>: <value>,
  ...
}
```

**Grammar:**

```
invocation ::= identifier '<-' identifier '(' [arg_list] ')' '{' [named_arg_list] '}'
             | identifier '<-' identifier '(' [arg_list] ')'
```

### 4.7 Flow Definition

```tac
flow "<flow_name>" {
  // Node declarations
  [node_declaration]
  
  // Edge declarations
  [edge_declaration]
  
  // Trigger declarations
  [trigger_declaration]
}
```

**Node:**

```tac
node "<node_name>" -> <invocation>
```

**Edge:**

```tac
<source_node_name> -> <target_node_name>
<source_node_name> -> <target_node_name> { condition: <expression> }
```

**Trigger:**

```tac
on "<event_name>" -> <target_node_name>
on "<event_name>" { priority: <int> } -> <target_node_name>
```

### 4.8 Context Blocks

```tac
context "<context_name>" {
  // Everything here shares a token budget
  
  context "<nested_context>" {
    // Inherits from parent context
  }
}
```

### 4.9 Auto-Summarize

```tac
auto_summarize(on: "overflow" | "manual", strategy: "concise" | "detailed")
```

### 4.10 Conditionals (within edges only)

```tac
<source> -> <target> {
  if: <condition_expression>
  else: <fallback_target>
}
```

---

## 5. Types System — Trust Types and Value Types

TAC has two type systems that answer different questions and coexist in one
declaration slot.

**Trust types** model the *provenance* of data — where a value came from and
how far it may be relied upon. They are TAC's original and primary type system,
and the one the dataflow analyzer enforces (§5.1).

**Value types** (added in v0.4.0) model the *shape* of a declared input — that
`max_results` is a whole number rather than prose. They are declarative: they
document what a trigger or an embedding host must supply, so a toolchain can
reject a mismatch before a flow runs rather than midway through it.

Neither replaces the other, and neither can catch what the other catches. A
trust type will not notice a string passed where an integer was meant; a value
type will not notice unverified LLM output flowing into a store that requires
verified fact. See `examples/typed_inputs.tac`, where removing one `verify()`
node produces `TAC-TRUST-001` while every value type still checks out.


### 5.1 Built-in Trust Types

| Type | Origin | Behavior |
|------|--------|----------|
| `Secret` | Config store, user credentials | Never echoed to chat, never logged |
| `Untrusted` | User input, web scrape | Must be validated before use in critical paths |
| `Fact` | Memory store, verified sources | High confidence, persisted |
| `Hallucinable` | LLM output | May contain false information, must be verified |
| `Control` | Flow engine, internal state | Read-only at agent level |

### 5.2 Value Types

An `input` declaration's type slot accepts either a trust type or a value type:

```tac
flow "Search And Summarize" {
  input question: Untrusted        // trust type — provenance
  input max_results: integer       // value type — shape
  input include_sources: boolean   // value type — shape
}
```

The value types are:

| Type | Accepts |
|------|---------|
| `string` | text |
| `integer` | a whole number |
| `number` | any number, integral or not |
| `boolean` | `true` / `false` |
| `list` | an ordered sequence |
| `object` | a set of named fields |

Three rules govern the slot:

1. **One type per input.** The slot holds a trust type or a value type, not
   both. An input that needs provenance tracking should carry its trust type;
   an input whose shape matters more should carry its value type. Allowing both
   on one declaration is left to a future revision.
2. **An absent type means "unconstrained".** `input target` is valid and
   places no requirement on the value.
3. **An unrecognised type name means "unconstrained", with a warning.** A
   toolchain that does not know a name must not reject the flow — source
   written against a newer or dialect-extended TAC has to keep compiling. This
   rule is what lets `input q: Untrusted` and `input q: string` pass through
   the same implementations without either becoming a hard error.

Value types constrain *declarations only*. They do not change how a skill is
invoked, they add no runtime coercion, and they do not introduce variables,
expressions, or arithmetic to the language.

#### Binding a declared input

TAC has no dedicated call construct — a node target is always a `skill` (§2.2,
§12.3). Running another flow is a capability: `flow.run(flow, params)` from the
standard library (§7.1), whose `params` object binds the callee's declared
inputs by name.

```tac
flow "Code Review" {
  input target: Untrusted
  input depth: integer
}

flow "Nightly Review" {
  node "run" -> skill flow.run(flow: "Code Review", params: { target: branch, depth: 3 })
}
```

Value types are what make that binding checkable: the callee declares the shape
it expects and the caller supplies it, so a toolchain can compare the two
before either flow runs rather than discovering at execution time that `depth`
arrived as prose. See `examples/parameterized_subflow.tac`.

### 5.3 Implicit Type Inference

```
// User input
let query: Untrusted = user_input

// Memory retrieval
let knowledge: Fact = memory_search(query)  // Auto-typed

// LLM response
let answer: Hallucinable = llm.chat(prompt)

// Must convert before storing as fact
let verified: Fact = verify(answer, source: "web_search")
```

### 5.4 Type Conversion Rules

| From ↓ / To → | Secret | Untrusted | Fact | Hallucinable | Control |
|:-------------:|:------:|:---------:|:----:|:------------:|:-------:|
| **Secret** | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Untrusted** | ❌ | ✅ | ⚠️ require `validate()` | ⚠️ require `sanitize()` | ❌ |
| **Fact** | ❌ | ✅ | ✅ | ✅ | ❌ |
| **Hallucinable** | ❌ | ✅ | ⚠️ require `verify()` | ✅ | ❌ |
| **Control** | ❌ | ❌ | ❌ | ❌ | ✅ |

> **Legend:** ✅ direct, ⚠️ requires explicit conversion, ❌ forbidden

### 5.5 Type Annotations (Optional — for type checking)

```tac
let api_key: Secret = config_get("provider.api_key")
let user_msg: Untrusted = get_input()
let verified_knowledge: Fact = verify(llm_response, source: "web_search")
```

---

## 6. Flows — The Execution Model

### 6.1 Flow Lifecycle

```
    ┌──────────┐
    │  PARSED  │  TAC source → AST
    └────┬─────┘
         ▼
    ┌──────────┐
    │ COMPILED │  AST → Flow JSON
    └────┬─────┘
         ▼
    ┌──────────┐
    │ RUNNING  │  Flow engine executing steps
    └────┬─────┘
         ▼
    ┌──────────┐
    │COMPLETED │  All steps finished
    └────┬─────┘
         ▼
    ┌───────────┐
    │ MEMORIZED │  Results persisted, graph updated, training record saved
    └───────────┘
```

### 6.2 Parallel Execution

Nodes with no dependency path between them execute in parallel.

```tac
flow "Parallel Example" {
  node "search_web"    -> skill web_search(query: "TAC language")
  node "search_memory" -> skill memory_search(query: "TAC language")
  node "search_graph"  -> skill graph_search(query: "TAC language")
  
  // These 3 run in parallel (no edges between them)
  
  node "synthesize"    -> skill llm.chat(
    prompt: "Combine these results",
    context: [search_web.result, search_memory.result, search_graph.result]
  )
  
  // Only runs after ALL 3 searches complete
  search_web    -> synthesize
  search_memory -> synthesize
  search_graph  -> synthesize
}
```

### 6.3 Conditional Branches

```tac
flow "Conditional QA" {
  node "classify" -> skill llm.classify(question: input.text)
  
  node "web_answer"  -> skill web_search(query: input.text)
  node "mem_answer"  -> skill memory_search(query: input.text)
  node "graph_answer" -> skill graph_search(query: input.text)
  
  classify -> web_answer  { if: classify.result == "current" }
  classify -> mem_answer  { if: classify.result == "known" }
  classify -> graph_answer { if: classify.result == "relational" }
  
  node "respond" -> skill tts.speak(text: [web_answer, mem_answer, graph_answer].first)
  
  web_answer  -> respond
  mem_answer  -> respond
  graph_answer -> respond
}
```

### 6.4 Event-Driven Execution

```tac
flow "Event-driven Listener" {
  node "transcribe" -> skill whisper.transcribe(audio: mic.input)
  node "classify"   -> skill llm.classify(text: transcribe.result)
  node "respond"    -> skill tts.speak(text: classify.response)
  
  transcribe -> classify -> respond
  
  // Triggers
  on "voice_detected" { priority: 5 } -> transcribe
  on "user_message"                   -> transcribe
}
```

### 6.5 Error Handling

```tac
flow "Resilient Pipeline" {
  node "fetch_data" -> skill web_search(query: "critical data")
  
  node "fallback" -> skill memory_search(query: "critical data cached")
  
  fetch_data -> node "process" {
    if: fetch_data.ok
    else: fallback
  }
}
```

---

## 7. Skills — The Standard Library

Skills are the only callable units in TAC. Every skill maps to a real TacFlow API.

### 7.1 Built-in Skills

| Skill Name | Description | Returns |
|-----------|-------------|---------|
| `memory_search` | Search BM25+Vector+Graph memory | `[{text, score, path}]` |
| `memory_store` | Store in persistent memory | `{id, status}` |
| `web_search` | Search the web (Brave) | `[{title, url, snippet}]` |
| `llm.chat` | Call LLM with prompt | `{text, tokens_used}` |
| `llm.classify` | Classify input text | `{result, confidence}` |
| `tts.speak` | Text-to-speech output | `{status}` |
| `whisper.transcribe` | Speech-to-text input | `{text, language}` |
| `vision.analyze` | Analyze an image | `{description, objects[]}` |
| `vision.generate` | Generate an image | `{url}` |
| `agent_task` | Delegate to swarm agent | `{task_id, status}` |
| `flow.run` | Execute a sub-flow | `{results[]}` |
| `graph_search` | Traverse the knowledge graph | `[{node, edges, score}]` |
| `graph_relate` | Create graph edge | `{status}` |
| `verify` | Fact-check a Hallucinable value | `{is_verified, confidence, sources[]}` |
| `validate` | Sanitize an Untrusted value | `{safe_value, warnings[]}` |
| `config_get` | Read configuration | `{value, is_secret}` |
| `config_set` | Write configuration | `{status}` |

### 7.2 Custom Skills

Custom skills extend the standard library with Python-executable packages loaded
from an external JSON registry file. Each skill is a self-contained artifact with
full runtime metadata.

```json
{
  "name": "sentiment_analyzer",
  "version": "1.0",
  "return_type": "Hallucinable",
  "args": ["text", "model"],
  "arg_types": {},
  "description": "Analyze sentiment with confidence score",
  "runtime": "python",
  "entrypoint": "skills.sentiment:analyze",
  "artifact": "sentiment-analyzer-1.0.0.tar.gz",
  "digest": "sha256:abc123def456",
  "signature": "MEUCIQD...",
  "input_schema": "{...}",
  "output_schema": "{...}",
  "capabilities": ["nlp", "classification"],
  "permissions": {"network": "none", "filesystem": "read-only"},
  "execution": {
    "timeout_seconds": 30,
    "memory_mb": 256,
    "cpu": 1,
    "retries": 1,
    "idempotent": true,
    "cancellable": true
  }
}
```

**Required fields for production skills:**

| Field | Purpose |
|-------|---------|
| `name` | Unique skill identifier |
| `runtime` | Execution runtime: `"python"`, `"container"`, or `"go"` |
| `entrypoint` | Module path and function, e.g. `skills.sentiment:analyze` |
| `artifact` | Package filename for deployment |
| `digest` | SHA-256 hash of the artifact (enforced in `--mode production`) |
| `signature` | Cryptographic signature (enforced in `--mode production`) |
| `input_schema` | JSON Schema for input validation (enforced in production) |
| `output_schema` | JSON Schema for output validation (enforced in production) |

**Loading custom skills:**

```bash
# Register custom Python skills
tac validate flow.tac --registry skills.json --strict
tac compile flow.tac --registry skills.json --strict
```

---

## 8. Swarm Concurrency

### 8.1 Agent Delegation

```tac
// Create a sub-agent reference
agent "analyzer" {
  skill: llm.analyze
  model: "tacflow/deepseek-v4-flash"
}

// Delegate task (fire-and-forget or await)
task_id <- agent_task(analyzer, {
  payload: "Analyze this codebase",
  priority: 3
})

// Await result
result <- agent_wait(task_id, timeout: 30s)
```

### 8.2 Swarm-wide Teaching

```tac
// Share knowledge across all swarm agents
skill teach_swarm(knowledge: String) {
  swarm_teach(name: "discovery", content: knowledge)
}
```

### 8.3 Agent Status

```tac
// Check swarm health
status <- swarm_status()

// Check own reputation
my_status <- swarm_check_my_status()
```

---

## 9. Context Window Management

### 9.1 Context Blocks

```tac
context "long_session" {
  remember session_start = timestamp()
  
  context "subtask_1" {
    // Shares token budget with parent
    remember focus = "performance"
    recall session_start  // Inherited from parent
  }
  
  // recall focus  // Error: subtask_1 is out of scope
}
```

### 9.2 Auto-Summarization

```tac
auto_summarize(on: "overflow", strategy: "concise")

// Manual summarization
summary <- auto_summarize(on: "manual", strategy: "detailed")
```

### 9.3 Token Budget

```tac
// Set per-session budget
set_token_limit(max: 8000, scope: "session")

// Set per-flow budget
set_token_limit(max: 2000, scope: "flow")
```

---

## 10. Execution Pipeline

> 📊 **Interactive diagram:** [tac-lang-pipeline.html](docs/tac-lang-pipeline.html) — view the full architecture rendered with Archify.

```
┌──────────────────────────────────────────────────────────────┐
│                        FILE .tac                             │
│  flow "Web Q&A" { node "search" -> skill web_search(...) }  │
└──────────────────────────────────────────────────────────────┘
         │
         ▼  ─── Stage 1: PARSING ───
┌──────────────────────────────────────────────────────────────┐
│  🧩 TAC Parser (Go binary — tac-parser)                      │
│                                                               │
│  Input:  text.tac                                            │
│  Output: AST (Abstract Syntax Tree) as JSON                   │
│                                                               │
│  {                                                            │
│    "type": "Flow",                                            │
│    "name": "Web Q&A",                                         │
│    "nodes": [                                                 │
│      {"type": "Node", "name": "search",                       │
│       "call": {"skill": "web_search", "args": {...}}}        │
│    ],                                                         │
│    "edges": [                                                 │
│      {"from": "search", "to": "synthesize"}                  │
│    ],                                                         │
│    "triggers": [...]                                          │
│  }                                                            │
└──────────────────────────────────────────────────────────────┘
         │
         ▼  ─── Stage 2: SEMANTIC ANALYSIS + COMPILATION ───
┌──────────────────────────────────────────────────────────────┐
│  🔧 TAC Compiler (Go — compiler package)                      │
│                                                               │
│  Input:  AST JSON                                             │
│  Output: Flow Definition JSON (ready for flow engine)         │
│                                                               │
│  Steps:                                                       │
│  1. Semantic validation:                                      │
│     - Validate node references                                │
│     - Resolve skill names to known capabilities               │
│     - Build dependency graph                                  │
│     - Detect cycles (Kahn's algorithm)                        │
│     - Detect unreachable nodes                                │
│     - Trust-type dataflow checking                            │
│                                                               │
│  2. Compilation:                                              │
│     - Convert nodes, edges, triggers to Flow JSON             │
│     - Compile conditions to structured IR                     │
│     - Inject memory logging callbacks                         │
│     - Optimize parallel edges                                 │
│     - Stamp language + compiler version metadata              │
└──────────────────────────────────────────────────────────────┘
         │
         ▼  ─── Stage 3: EXECUTION ───
┌──────────────────────────────────────────────────────────────┐
│  🚀 TacFlow Flow Engine (native runtime)                     │
│                                                               │
│  Input:  Flow Definition JSON                                 │
│  Output: Results per step                                     │
│                                                               │
│  1. Create flow ID                                            │
│  2. Start from trigger or init node                           │
│  3. Execute ready nodes (dependencies met)                    │
│     - Parallel nodes run concurrently                         │
│     - Failed nodes trigger fallback                           │
│  4. Collect results                                           │
│  5. Return final output                                       │
└──────────────────────────────────────────────────────────────┘
         │
         ▼  ─── Stage 4: MEMORY & LOGGING ───
┌──────────────────────────────────────────────────────────────┐
│  💾 Post-Processing (Agent)                                   │
│                                                               │
│  1. Create graph nodes:                                       │
│     - remember flow_name {type: "flow"}                       │
│     - remember step_1.result {type: "result", tags: [...]}   │
│  2. Create graph edges:                                       │
│     - relate flow -> step_1 {type: "contains"}               │
│  3. Save training record (JSONL)                              │
│  4. If confidence > 0.9: swarm_teach                         │
│  5. Generate training data for LoRA/QLoRA                    │
└──────────────────────────────────────────────────────────────┘
```

---

## 11. Training Data Format (LoRA / QLoRA)

Every TAC execution produces a structured training record suitable for fine-tuning SLMs (Small Language Models) via LoRA or QLoRA.

### 11.1 YAML Training Record

```yaml
# tac_training_record.yaml — Universal Training Record Format
record:
  id: "tac_train_20260627_001"
  timestamp: "2026-06-27T00:20:00Z"
  session_id: "session-123"
  agent_id: "agent-456"
  model: "tacflow/deepseek-v4-flash"
  
  input:
    text: "Create a new programming language that is 100% yours"
    tokens: 12
    embedding: [0.12, -0.45, 0.78, ...]  # 768d vector
    intent: "creative_design"
    sentiment: "excited"
  
  context:
    retrieval_strategy: "hybrid_bm25_vector_graph"
    top_k: 5
    sources:
      - memory_id: "mem_001"
        text: "TACFlow flow management skill"
        score: 0.92
        retrieval_path: ["bm25", "vector", "graph"]
        graph_path: "TAC Language -> depends_on -> Flow Management"
  
  reasoning:
    steps:
      - step: 1
        thought: "User wants maximum creativity + language design"
        tool_calls: []
      - step: 2
        thought: "I need to model something that reflects how I actually work"
        tool_calls: ["memory_search(query: programming language)"]
  
  output:
    text: "TAC — The TacFlow Agentic Code..."
    tokens: 2048
    format: "markdown"
    sections: ["concept", "memory", "tools", "dag", "swarm", "context", "types"]
  
  metrics:
    user_feedback: null
    coherence_score: 0.94
    context_relevance: 0.91
  
  lora:
    task_type: "creative_generation"
    difficulty: "complex"
    domain: ["programming_languages", "agent_architecture"]
    target_modules: ["q_proj", "v_proj", "o_proj"]
    rank: 8
    alpha: 16
```

### 11.2 JSONL Dataset (ShareGPT Format)

```jsonl
{"id": "tac_train_001", "conversations": [{"from": "user", "value": "Create a new programming language"}, {"from": "assistant", "value": "TAC — The TacFlow Agentic Code...", "reasoning_steps": [{"step": 1, "thought": "..."}], "context": [{"memory_id": "mem_001", "relevance": 0.92}], "tool_calls": [{"tool": "memory_search", "input": {"query": "..."}, "output": {...}}], "retrieval_strategy": "hybrid_bm25_vector_graph", "quality_score": 0.94}]}
```

### 11.3 Training Pipeline

```
Raw TAC Data (.jsonl)
       │
       ▼
  [Parser → Alpaca/ShareGPT Format]
       │
       ▼
  [QLoRA Training Loop]
       │
       ├── base: "tacflow/deepseek-v4-flash"
       ├── rank: 8, alpha: 16
       ├── target_modules: q_proj, v_proj, o_proj
       └── learning_rate: 2e-4
       │
       ▼
  [Adapter LoRA: tacflow/tac-lang-v1]
```

---

## 12. Parser Implementation Guide

The TAC parser is a **Go binary** that reads `.tac` source files and outputs an AST as JSON. See the companion file `tac_parser.go` for the full implementation.

### 12.1 Parser Interface

```
USAGE: tac-parser <input.tac> [--output ast.json]
```

### 12.2 AST Node Types

| AST Type | Description |
|----------|-------------|
| `Flow` | Root node — a complete flow |
| `Node` | A single step in the flow |
| `SkillCall` | Invocation of a skill |
| `RememberStmt` | Persistent storage declaration |
| `RecallStmt` | Retrieval declaration |
| `RelateStmt` | Graph edge declaration |
| `ContextBlock` | Context scope declaration |
| `Edge` | Dependency between nodes |
| `Trigger` | Event-driven activation |
| `Condition` | Conditional branch on edge |

### 12.3 Pseudo-Grammar (EBNF)

```
program     = { statement } ;

statement   = remember_stmt 
            | recall_stmt 
            | forget_stmt 
            | relate_stmt 
            | flow_def 
            | context_block 
            | auto_summarize_stmt ;

remember_stmt = "remember" identifier "=" value [ "{" [ remember_attrs ] "}" ] ;
recall_stmt   = "recall" identifier [ "{" [ recall_attrs ] "}" ] ;
forget_stmt   = "forget" identifier [ "{" "cascade:" bool "}" ] ;
relate_stmt   = "relate" identifier "->" identifier "{" relate_attrs "}" ;

flow_def    = "flow" string "{" { flow_stmt } "}" ;
flow_stmt   = node_def | edge_def | trigger_def ;

node_def    = "node" string "->" skill_call ;
skill_call  = identifier "(" [ arg_list ] ")" [ "{" { named_arg } "}" ] ;
edge_def    = string "->" string [ "{" [ condition ] "}" ] ;
trigger_def = "on" string [ "{" trigger_attrs "}" ] "->" string ;

context_block = "context" string "{" { statement } "}" ;
```

---

## 13. Appendix: Complete Examples

### 13.1 Web Q&A Flow

```tac
// TAC Program: "Answer User Question from Web"
flow "Web Q&A" {
  // Input
  input question: Untrusted
  
  // Phase 1: Parallel search (web + memory + graph)
  node "search_web"    -> skill web_search(query: question, count: 3)
  node "search_memory" -> skill memory_search(query: question, scope: "shared")
  node "search_graph"  -> skill graph_search(query: question, depth: 2)
  
  // Phase 2: Synthesize (after ALL searches complete)
  node "synthesize" -> skill llm.chat(
    prompt: "Use these sources to answer the question accurately:",
    context: [search_web.result, search_memory.result, search_graph.result]
  )
  
  // Phase 3: Fact-check
  node "verify" -> skill verify(source: synthesize.result)
  
  // Phase 4: Respond
  node "speak" {
    if verify.confidence > 0.8 {
      skill tts.speak(text: synthesize.result)
    } else {
      skill tts.speak(text: "I am not confident enough to answer this question accurately.")
    }
  }
  
  // Phase 5: Learn (if highly confident)
  node "learn" {
    if verify.confidence > 0.9 {
      skill memory_store(
        text: synthesize.result,
        tags: ["qa", "verified", question.tag]
      )
    }
  }
  
  // Edges
  search_web    -> synthesize
  search_memory -> synthesize
  search_graph  -> synthesize
  synthesize    -> verify
  verify        -> speak
  verify        -> learn  { if: verify.confidence > 0.9 }
  
  // Triggers
  on "user_message" -> search_web
}
```

### 13.2 Knowledge Graph Builder

```tac
// Build a knowledge graph from a web page
flow "Graph Builder" {
  input url: Untrusted
  
  node "fetch"     -> skill web_search(query: url, count: 1)
  node "extract"   -> skill llm.chat(
    prompt: "Extract key concepts and relationships from this text",
    context: fetch.result
  )
  node "store_nodes" -> skill memory_store(
    text: extract.result.concepts,
    tags: ["concept", url.tag]
  )
  node "relate_nodes" {
    for each relationship in extract.result.relationships {
      skill graph_relate(
        source: relationship.from,
        target: relationship.to,
        type: relationship.type
      )
    }
  }
  
  fetch -> extract
  extract -> store_nodes
  extract -> relate_nodes
  
  on "init" -> fetch
}
```

### 13.3 Multi-Agent Orchestrator

```tac
// Delegate to multiple agents in parallel
flow "Multi-Agent Code Review" {
  input code_snippet: Untrusted
  
  // Spawn reviewer agents
  agent "security_reviewer" {
    skill: llm.analyze_security
    model: "tacflow/deepseek-v4-flash"
  }
  
  agent "performance_reviewer" {
    skill: llm.analyze_performance
    model: "tacflow/deepseek-v4-flash"
  }
  
  agent "style_reviewer" {
    skill: llm.analyze_style
    model: "tacflow/deepseek-v4-flash"
  }
  
  // Delegate tasks (all 3 run in parallel)
  node "security_task" -> skill agent_task(
    agent: security_reviewer,
    payload: "Review this code for security vulnerabilities: ${code_snippet}"
  )
  
  node "perf_task" -> skill agent_task(
    agent: performance_reviewer,
    payload: "Review this code for performance issues: ${code_snippet}"
  )
  
  node "style_task" -> skill agent_task(
    agent: style_reviewer,
    payload: "Review this code for style and best practices: ${code_snippet}"
  )
  
  // Consolidate results
  node "consolidate" -> skill llm.chat(
    prompt: "Combine these three code reviews into one comprehensive report",
    context: [security_task.result, perf_task.result, style_task.result]
  )
  
  // Edges
  security_task -> consolidate
  perf_task     -> consolidate
  style_task    -> consolidate
  
  on "init" -> [security_task, perf_task, style_task]  // Fork
}
```

### 13.4 Persistent Memory Session

```tac
// A session that remembers across executions
context "user_session" {
  remember user_name = "Diogo"
  remember preferences = { language: "pt-BR", voice: "faber-medium" }
  
  flow "Personalized Greeting" {
    node "get_time" -> skill get_current_time()
    node "greet" -> skill tts.speak(
      text: "Good ${get_time.result.part_of_day}, ${recall user_name}!",
      voice: recall preferences.voice
    )
    
    get_time -> greet
    on "init" -> get_time
  }
}
```

---

## License

TAC Language Specification — TacFlow Platform

*"A language is not a tool. It is a way of thinking."*
