flow "f" {
  node "a" -> skill web_search(query: "q")
  node "b" -> skill web_search(query: "q")
  a -> b x y
  b -> a { if: a.score > 1 } junk -> b
}
