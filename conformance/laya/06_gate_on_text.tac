task "explicar" {
  question porque: text "Explique a decisão"
}

flow "gate_on_text" {
  node "gate" -> skill laya.decide(task: "explicar", input: payload)
  node "a" -> skill laya.tasks.list()
  gate[*] -> a
}
