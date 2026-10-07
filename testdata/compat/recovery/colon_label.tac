flow "f" {
  node "a" -> skill web_search(query: "q")
  node "b" -> skill web_search(query: "q")
  step: a -> b
}
