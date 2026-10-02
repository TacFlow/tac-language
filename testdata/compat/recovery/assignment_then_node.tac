flow "f" {
  node "a" -> skill web_search(query: "q")
  x = 1 node "c" -> skill web_search(query: "c")
  a -> c
}
