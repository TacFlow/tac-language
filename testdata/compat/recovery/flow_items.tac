flow "f" {
  node "a" -> skill web_search(query: "q")
  node "b" -> skill web_search(query: "q")
  "str" a -> b
  123 on "e" -> a
  x = 1 input q: Untrusted
  y = 2 agent "z"
  z = 3 remember k = "v"
  w = 4 recall k
  foo bar
  -> b
  q ( a -> b )
}
