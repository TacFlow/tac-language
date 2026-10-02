# LAYA conformance corpus (TAC v0.5)

Each `NN_name.tac` compiles in BOTH TAC compilers: this repository (upstream)
and the TacFlow platform dialect (`client-app/api/pkg/tac`). Flows here are
made only of `laya.*` nodes and labelled edges, so the dialect accepts them.

`expected/NN_name.json`:

| key | meaning |
|---|---|
| `only` | `""` = both compilers; `"upstream"` / `"dialect"` = the other one skips the case (e.g. trust analysis exists only upstream; `~>` exists only in the dialect) |
| `ok_upstream`, `ok_dialect` | the compile succeeds (no error diagnostics) |
| `codes_upstream`, `codes_dialect` | sorted multiset of the diagnostic codes (errors and warnings) |
| `shape` | when a compiler succeeds, its output reduced to this common shape must be structurally equal (JCS) |

Shape: `{flows: [{name, nodes: [{name, skill, args}], edges: [{from, to, label?}], triggers: [{event, targets}], schedules: [{cron, tz?}]}], tasks, models, episodes, datasets}`; edge labels are `ast.LabelString` (`proceed`, `*`, `true`, `range:<10`, `range:10..50`); only declared edges appear (the dialect's start/end and synthesized edges do not; its `evt:` start edges become `triggers`).

The platform keeps a hash-pinned copy (`client-app/api/pkg/tac/conformance/laya/SHA256SUMS`). Change a case here first, then copy it there.
