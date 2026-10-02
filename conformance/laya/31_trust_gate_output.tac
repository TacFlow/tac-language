task "acao" {
  question acao: choice "Qual ação?" {
    proceed: "agir"
    ask: "perguntar"
  }
}

flow "trust_gate_output" {
  node "gate"  -> skill laya.decide(task: "acao", input: payload)
  node "store" -> skill memory_store(text: gate.label)
  gate[*] -> store
}
