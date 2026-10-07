context "c" {
  x = 1 remember y = "v"
  z { k: 1 } flow "f" { node "a" -> skill web_search(query: "q") }
}
flow "g" { node "b" -> skill web_search(query: "q") }
