// Invalid: contains a cycle (search_web -> synthesize -> verify -> search_web)
flow "Cycle Test" {
  node "search_web" -> skill web_search(query: "test")
  node "synthesize" -> skill llm.chat(prompt: "test")
  node "verify" -> skill verify(source: synthesize.result)

  // These three edges form a cycle: search_web -> synthesize -> verify -> search_web
  search_web -> synthesize
  synthesize -> verify
  verify -> search_web
}
