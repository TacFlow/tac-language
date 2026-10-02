// Compatibility fixture: words that are keywords in TAC v0.5 (schedule,
// task, model, episode, dataset, requires, question, target, tz) used as
// v0.4 node names and edge endpoints. They must keep working unchanged.
flow "Keywords" {
  node "schedule" -> skill web_search(query: "a")
  node "task"     -> skill web_search(query: "b")
  node "model"    -> skill web_search(query: "c")
  node "episode"  -> skill web_search(query: "d")
  node "dataset"  -> skill web_search(query: "e")
  node "requires" -> skill web_search(query: "f")
  node "question" -> skill web_search(query: "g")
  node "target"   -> skill web_search(query: "h")
  node "tz"       -> skill memory_search(query: "i")
  schedule -> task
  task -> model
  model -> episode
  episode -> dataset
  dataset -> requires
  requires -> question
  question -> target
  target -> tz
  on "go" -> schedule
}
