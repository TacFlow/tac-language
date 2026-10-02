// Compatibility fixture (bug b): a chained edge line.
// v0.4.0 kept only the first hop (fetch -> rank) and dropped rank -> store.
flow "Chained" {
  input q: Untrusted
  node "fetch" -> skill web_search(query: q)
  node "rank"  -> skill llm.classify(text: fetch.result)
  node "store" -> skill memory_search(query: rank.result)
  fetch -> rank -> store
  on "user_message" -> fetch
}
