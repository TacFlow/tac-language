flow "f" {
  node "a" -> skill web_search(query: "q")
  node "b" -> skill web_search(query: "q")
  node "c" -> skill web_search(query: "q")
  a -> b x y
  b -> c { if: a.score > 1 } junk -> c
}
