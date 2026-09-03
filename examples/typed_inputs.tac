// Typed inputs — value types on a flow's declared inputs (v0.4.0).
//
// An `input` declaration names a value the flow expects before it runs.
// Until v0.4.0 the type slot held a TRUST type, describing where a value came
// from and how far it may be relied upon. It now also accepts a VALUE type,
// describing the SHAPE of the value.
//
// The two answer different questions and share one slot:
//
//   input question: Untrusted     // provenance — came from outside, validate it
//   input max_results: integer    // shape — a whole number, not prose
//
// Value types are declarative. They document what a trigger or an embedding
// host must supply, so a toolchain can reject a mismatch before the flow runs
// rather than midway through it. They do not replace trust types and do not
// change how a skill is invoked.
//
// The two systems are visibly independent here. `include_sources: boolean`
// constrains a SHAPE and nothing else. The `check` node exists for the other
// reason entirely: llm.chat returns Hallucinable, memory_store requires Fact,
// and only an explicit verify() bridges them. Remove it and the analyzer
// rejects the flow with TAC-TRUST-001 — a value type would never have caught
// that, and a trust type would never have caught a string passed where an
// integer was meant.
//
// Every node target here is a `skill`, per the grammar in SPEC.md §12.3.

flow "Search And Summarize" {
  input question: Untrusted        // trust type — user-supplied, unvalidated
  input max_results: integer       // value type — how many hits to fetch
  input include_sources: boolean   // value type — append a citation list?

  node "search"    -> skill web_search(query: question, count: max_results)
  node "summarize" -> skill llm.chat(prompt: "Summarize the findings.")
  node "check"     -> skill verify(source: summarize.result)
  node "remember"  -> skill memory_store(text: check.result, shared: include_sources)

  search -> summarize
  summarize -> check
  check -> remember

  on "user_message" -> search
}
