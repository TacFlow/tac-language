task "acao" {
  question acao: choice "Qual ação o agente deve tomar neste passo?" {
    proceed: "executar o passo agora"
    reuse: "usar resultado em cache válido"
    skip: "passo desnecessário"
    ask: "ambíguo"
  }
}

flow "gate_missing_coverage" {
  node "gate" -> skill laya.decide(task: "acao", input: payload, min_confidence: 0.8)
  node "a" -> skill laya.tasks.list()
  node "b" -> skill laya.tasks.describe(task: "acao")
  node "c" -> skill laya.model.status(model: "estoque")
  gate[proceed] -> a
  gate[reuse] -> b
  gate[ask] -> c
}
