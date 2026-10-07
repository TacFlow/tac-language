flow "f" {
  node "a" -> skill web_search(query: "q")
  node "b" -> skill web_search(query: "q")
  foo { k: 1 } a -> b
  x = [1, 2] b -> a
}
