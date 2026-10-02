task "quantidade" {
  question qtd: number "Quantas unidades repor?" { range: [0, 100] }
}

flow "range_overlap" {
  node "gate" -> skill laya.decide(task: "quantidade", input: payload)
  node "a" -> skill laya.tasks.list()
  node "b" -> skill laya.tasks.list()
  gate[<10] -> a
  gate[5..50] -> b
  gate[*] -> a
}
