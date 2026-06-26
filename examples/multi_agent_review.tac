// TAC Example: Multi-Agent Orchestrator
// Spawns multiple reviewer agents in parallel and consolidates results.
flow "Multi-Agent Code Review" {
  input code_snippet: Untrusted

  // Spawn reviewer agents
  agent "security_reviewer" {
    skill: llm.analyze_security
    model: "tacflow/deepseek-v4-flash"
  }

  agent "performance_reviewer" {
    skill: llm.analyze_performance
    model: "tacflow/deepseek-v4-flash"
  }

  agent "style_reviewer" {
    skill: llm.analyze_style
    model: "tacflow/deepseek-v4-flash"
  }

  // Delegate tasks (all 3 run in parallel)
  node "security_task" -> skill agent_task(
    agent: security_reviewer,
    payload: "Review this code for security vulnerabilities: ${code_snippet}"
  )

  node "perf_task" -> skill agent_task(
    agent: performance_reviewer,
    payload: "Review this code for performance issues: ${code_snippet}"
  )

  node "style_task" -> skill agent_task(
    agent: style_reviewer,
    payload: "Review this code for style and best practices: ${code_snippet}"
  )

  // Consolidate results
  node "consolidate" -> skill llm.chat(
    prompt: "Combine these three code reviews into one comprehensive report",
    context: [security_task.result, perf_task.result, style_task.result]
  )

  // Edges
  security_task -> consolidate
  perf_task     -> consolidate
  style_task    -> consolidate

  on "init" -> [security_task, perf_task, style_task]  // Fork
}
