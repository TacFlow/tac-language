task "acao" {
  question acao: choice "Qual ação?" { proceed: "agir", ask: "perguntar" }
  quesiton extra: choice "Outra?" { a: "1", b: "2" }
}

episode "ep-typos" {
  group "conformance"
  goal "erros de escrita são reportados, nunca reinterpretados"
  limite 1e3
  question repor: binary "Repor?"
  targte repor = true
  target repor = false
}

dataset "ds-typos" {
  task "acao"
  splt train = 1
  taks "outra"
}
