task "repor" {
  question repor: binary "Repor agora?" { nao: "não repor", sim: "repor" }
}

flow "binary_gate" {
  node "gate" -> skill laya.decide(task: "repor", input: payload, allow_uncalibrated: true)
  node "sim" -> skill laya.train(model: "estoque", dataset: "acao_v1")
  node "nao" -> skill laya.tasks.list()
  gate[true] -> sim
  gate[false] -> nao
  gate[*] -> nao
}
