// TAC Example: a LAYA gate started by an event (v0.5).
//
// `on "<pattern>"` starts the flow when a matching event is published; the
// payload of the event is the implicit `payload` identifier.
flow "estoque_evento" {
  on "estoque.baixo" -> gate
  node "gate"      -> skill laya.decide(task: "acao", input: payload, min_confidence: 0.8)
  node "fetch"     -> skill web_search(query: payload)
  node "use_cache" -> skill memory_search(query: payload)
  node "ask_human" -> skill agent_task(agent: "humano", payload: payload)
  gate[proceed] -> fetch
  gate[reuse]   -> use_cache
  gate[*]       -> ask_human
}
