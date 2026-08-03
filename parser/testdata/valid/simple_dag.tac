flow "Simple DAG" {
  node "start" -> skill get_current_time()
  node "process" -> skill llm.chat(prompt: "hello")

  start -> process

  on "init" -> start
}
