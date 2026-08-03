// TAC Example: Knowledge Graph Builder
// Extracts concepts and relationships from a web page and builds a knowledge graph.
flow "Graph Builder" {
  input url: Untrusted

  node "fetch"     -> skill web_search(query: url, count: 1)
  node "extract"   -> skill llm.chat(
    prompt: "Extract key concepts and relationships from this text",
    context: fetch.result
  )
  node "verify"    -> skill verify(source: extract.result)
  node "store_nodes" -> skill memory_store(
    text: verify.result,
    tags: ["concept"]
  )
  node "relate_nodes" {
    for each relationship in extract.result.relationships {
      skill graph_relate(
        source: relationship.from,
        target: relationship.to,
        type: relationship.type
      )
    }
  }

  fetch -> extract
  extract -> verify
  verify -> store_nodes
  verify -> relate_nodes

  on "init" -> fetch
}
