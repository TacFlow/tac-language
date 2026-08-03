flow "Parallel Fanout" {
  node "a" -> skill web_search(query: "a", count: 1)
  node "b" -> skill web_search(query: "b", count: 1)
  node "c" -> skill web_search(query: "c", count: 1)
  node "merge" -> skill llm.chat(prompt: "merge results", context: [a.result, b.result, c.result])

  a -> merge
  b -> merge
  c -> merge

  on "init" -> [a, b, c]
}
