flow "on_with_schedule" {
  schedule "0 3 * * *" tz "Europe/Lisbon"
  on "laya.train.completed" -> ev
  node "ev" -> skill laya.eval(model: "estoque", run: payload.run)
}
