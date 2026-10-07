episode "ep-dup" {
  group "conformance"
  goal "primeira versão"
  question repor: binary "Repor?"
  target repor = false
}

episode "ep-dup" {
  group "conformance"
  goal "segunda versão: esta vence"
  question repor: binary "Repor?"
  target repor = true
}
