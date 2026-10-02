// TAC Example: a LAYA episode (v0.5).
//
// `tac episode examples/laya_episode.tac` prints the bare episode. It is
// structurally equal (RFC 8785 JCS) to the mission's reference JSON,
// testdata/laya_episode.reference.json.
episode "evt-c1-estoque-0007-a" {
  group "conv-estoque-reposicao-2026-10"
  goal "decidir se reabastece o SKU antes do próximo ciclo de compra"
  context { sku: "REF-2L-COLA", estoque_atual: 2, ponto_reposicao: 10, consumo_medio_dia: 6,
            lead_time_dias: 3, validade_dias: 180, pedido_em_aberto: false }
  rule "reabastecer SE estoque_atual < ponto_reposicao E pedido_em_aberto == false"
  tool "estoque.consultar_sku"
  cache_available false
  fresh_result_required true
  question acao: choice "Qual ação o agente deve tomar neste passo?" {
    proceed: "executar o passo agora (dado ausente/obsoleto)"
    reuse:   "usar resultado em cache válido"
    skip:    "passo provadamente desnecessário"
    ask:     "ambíguo, faltam dados ou risco/custo sem justificação"
  }
  target acao = proceed
}
