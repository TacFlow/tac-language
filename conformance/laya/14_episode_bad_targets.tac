episode "ep-bad-targets" {
  group "conformance"
  goal "testar targets"
  question acao: choice "Qual ação?" {
    proceed: "agir"
    ask: "perguntar"
  }
  question repor: binary "Repor?"
  target acao = talvez
  target repor = true
  target fantasma = proceed
}
