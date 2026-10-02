// Compatibility fixture (bug e): the target of an `else:` fallback.
// v0.4.0 reported "fallback" as unreachable (TAC-GRAPH-003).
flow "Else Target" {
  node "check"    -> skill web_search(query: "status")
  node "proceed"  -> skill memory_search(query: "ok")
  node "fallback" -> skill memory_search(query: "degraded")
  check -> proceed { if: check.confidence > 0.5, else: fallback }
  on "tick" -> check
}
