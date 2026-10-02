task "variacao" {
  question delta: number "Variação do estoque?" { range: [-100, 100], unit: "un", bins: [-5, 50] }
}

flow "range_ok" {
  node "gate" -> laya.decide(task: "variacao", input: payload)
  node "baixa" -> laya.tasks.list()
  node "media" -> laya.tasks.list()
  node "alta" -> laya.tasks.list()
  gate[<-5] -> baixa
  gate[-5..50] -> media
  gate[>=50] -> alta
  gate[low_confidence] -> baixa
  gate[error] -> baixa
}
