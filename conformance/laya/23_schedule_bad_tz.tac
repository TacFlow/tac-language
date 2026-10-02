flow "schedule_bad_tz" {
  schedule "0 3 * * *" tz "Mars/Phobos"
  node "a" -> skill laya.dataset.export(task: "acao")
}
