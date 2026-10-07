// TAC Example: train and promote a private LAYA model (v0.5).
//
// The nightly flow exports the dataset and starts training (asynchronous).
// laya.train publishes laya.train.completed when it finishes; the second flow
// evaluates the run and promotes it only if it beats the current model.
requires "0.5"
model "estoque" { tasks ["acao"] base "laya-multilingual" }

flow "estoque_treino" {
  schedule "0 3 * * *" tz "Europe/Lisbon"
  node "exp"   -> skill laya.dataset.export(task: "acao")
  node "train" -> skill laya.train(model: "estoque", dataset: exp.path)
  exp -> train
}

flow "estoque_promover" {
  on "laya.train.completed" -> ev
  node "ev"   -> skill laya.eval(model: "estoque", run: payload.run)
  node "prom" -> skill laya.model.promote(model: "estoque", run: payload.run, if_better: true, max_drop_pp: 1)
  ev -> prom
}
