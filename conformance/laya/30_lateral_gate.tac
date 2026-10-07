task "acao" {
  question acao: choice "Qual ação?" {
    proceed: "agir"
    ask: "perguntar"
  }
}

flow "lateral_gate" {
  node "gate" -> laya.decide(task: "acao", input: payload)
  node "a" -> laya.tasks.list()
  node "b" -> laya.tasks.list()
  gate[*] -> a
  gate[proceed] ~> b
}
