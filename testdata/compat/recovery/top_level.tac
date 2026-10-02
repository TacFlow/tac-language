x = 1 flow "f" {
  node "a" -> skill web_search(query: "q")
}
junk { k: 1 } flow "g" { node "b" -> skill web_search(query: "q") }
junk } flow "h" { node "c" -> skill web_search(query: "q") }
junk { flow "i" { node "d" -> skill web_search(query: "q") } }
