// Invalid: duplicate node names in the same flow
flow "Duplicate Nodes" {
  node "search" -> skill web_search(query: "abc")
  node "search" -> skill web_search(query: "xyz")

  on "init" -> search
}
