// Invalid: edge references undefined node "missing_node"
flow "Undefined Ref" {
  node "search" -> skill web_search(query: "test")
  node "answer" -> skill llm.chat(prompt: "result")

  search -> missing_node
  missing_node -> answer
}
