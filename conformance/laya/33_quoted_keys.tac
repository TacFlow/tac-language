episode "ep-quoted-keys" {
  group "conformance"
  goal "chaves de context que não são identificadores"
  context { "sku id": "REF-2L", "lote/validade": "2026-12", qtd: 3 }
  question repor: binary "Repor?"
  target repor = true
}
