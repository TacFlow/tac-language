// TAC Example: a LAYA gate (v0.5).
//
// laya.decide answers the task's question; each labelled edge is one branch.
// low_confidence and error are pseudo-labels every gate understands.
flow "estoque_gate" {
  node "gate"      -> skill laya.decide(task: "acao", input: payload, min_confidence: 0.8)
  node "fetch"     -> skill web_search(query: payload)
  node "use_cache" -> skill memory_search(query: payload)
  node "ask_human" -> skill agent_task(agent: "humano", payload: payload)
  node "done"      -> skill memory_store(text: "skip")
  gate[proceed] -> fetch
  gate[reuse]   -> use_cache
  gate[skip]    -> done
  gate[ask]     -> ask_human
  gate[low_confidence] -> ask_human
  gate[error]   -> ask_human
}
