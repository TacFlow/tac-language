// TAC Example: a LAYA task, three episodes and a dataset (v0.5).
task "acao" {
  question acao: choice "Qual ação o agente deve tomar neste passo?" {
    proceed: "executar o passo agora (dado ausente/obsoleto)"
    reuse:   "usar resultado em cache válido"
    skip:    "passo provadamente desnecessário"
    ask:     "ambíguo, faltam dados ou risco/custo sem justificação"
  }
}

episode "ds-estoque-001" {
  group "ds-estoque"
  task "acao"
  goal "decidir se reabastece o SKU antes do próximo ciclo de compra"
  context { sku: "REF-2L-COLA", estoque_atual: 2, ponto_reposicao: 10, pedido_em_aberto: false }
  question acao: choice "Qual ação o agente deve tomar neste passo?" {
    proceed: "executar o passo agora (dado ausente/obsoleto)"
    reuse:   "usar resultado em cache válido"
    skip:    "passo provadamente desnecessário"
    ask:     "ambíguo, faltam dados ou risco/custo sem justificação"
  }
  target acao = proceed
}

episode "ds-estoque-002" {
  group "ds-estoque"
  task "acao"
  goal "decidir se reabastece o SKU antes do próximo ciclo de compra"
  context { sku: "AGUA-500", estoque_atual: 40, ponto_reposicao: 10, pedido_em_aberto: false }
  cache_available true
  question acao: choice "Qual ação o agente deve tomar neste passo?" {
    proceed: "executar o passo agora (dado ausente/obsoleto)"
    reuse:   "usar resultado em cache válido"
    skip:    "passo provadamente desnecessário"
    ask:     "ambíguo, faltam dados ou risco/custo sem justificação"
  }
  target acao = skip
}

episode "ds-estoque-003" {
  group "ds-estoque"
  task "acao"
  goal "decidir se reabastece o SKU antes do próximo ciclo de compra"
  context { sku: "SUCO-1L", estoque_atual: 3, ponto_reposicao: 10, pedido_em_aberto: true }
  question acao: choice "Qual ação o agente deve tomar neste passo?" {
    proceed: "executar o passo agora (dado ausente/obsoleto)"
    reuse:   "usar resultado em cache válido"
    skip:    "passo provadamente desnecessário"
    ask:     "ambíguo, faltam dados ou risco/custo sem justificação"
  }
  target acao = ask
}

dataset "acao_v1" {
  task "acao"
  from "db"
  include "ds-estoque-001", "ds-estoque-002", "ds-estoque-003"
  split train = 0.7
  split validation = 0.1
  split calibration = 0.1
  split test = 0.1
}
