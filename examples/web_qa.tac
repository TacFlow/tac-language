// TAC Example: Web Q&A
// A complete question-answering flow that searches web, memory, and graph in parallel.
flow "Web Q&A" {
  input question: Untrusted

  // Phase 1: Parallel search (web + memory + graph)
  node "search_web"    -> skill web_search(query: question, count: 3)
  node "search_memory" -> skill memory_search(query: question, scope: "shared")
  node "search_graph"  -> skill graph_search(query: question, depth: 2)

  // Phase 2: Synthesize (after ALL searches complete)
  node "synthesize" -> skill llm.chat(
    prompt: "Use these sources to answer the question accurately:",
    context: [search_web.result, search_memory.result, search_graph.result]
  )

  // Phase 3: Fact-check
  node "verify" -> skill verify(source: synthesize.result)

  // Phase 4: Respond
  node "speak" {
    if verify.confidence > 0.8 {
      skill tts.speak(text: synthesize.result)
    } else {
      skill tts.speak(text: "I am not confident enough to answer this question accurately.")
    }
  }

  // Phase 5: Learn (if highly confident)
  node "learn" {
    if verify.confidence > 0.9 {
      skill memory_store(
        text: verify.result,
        tags: ["qa", "verified"]
      )
    }
  }

  // Edges — define the DAG
  search_web    -> synthesize
  search_memory -> synthesize
  search_graph  -> synthesize
  synthesize    -> verify
  verify        -> speak
  verify        -> learn  { if: verify.confidence > 0.9 }

  // Triggers
  on "user_message" -> search_web
}
