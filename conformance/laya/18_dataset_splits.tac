dataset "acao_v2" {
  task "acao"
  from "db"
  split train = 0.8
  split test = 0.1
}
