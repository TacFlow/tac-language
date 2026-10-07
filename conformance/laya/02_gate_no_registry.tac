flow "gate_no_registry" {
  node "gate" -> skill laya.decide(task: "acao", input: payload, min_confidence: 0.8)
  node "a" -> skill laya.tasks.list()
  node "b" -> skill laya.tasks.describe(task: "acao")
  node "c" -> skill laya.model.status(model: "estoque")
  gate[proceed] -> a
  gate[*] -> b
  gate[error] -> c
}
