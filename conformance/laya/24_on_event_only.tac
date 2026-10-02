flow "on_event_only" {
  on "estoque.baixo" -> ev
  node "ev" -> skill laya.eval(model: "estoque", run: payload.run)
  node "prom" -> skill laya.model.promote(model: "estoque", run: payload.run, if_better: true, max_drop_pp: 1)
  ev -> prom
}
