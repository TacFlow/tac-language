// Every value type, in one flow (v0.4.0).
//
// The six value types describe the SHAPE of a declared input:
//
//   string   text
//   integer  a whole number
//   number   any number, integral or not
//   boolean  true / false
//   list     an ordered sequence
//   object   a set of named fields
//
// They constrain DECLARATIONS only. There is no runtime coercion, no
// arithmetic, and no expressions — an input's type says what a caller or a
// trigger must supply, and nothing more.
//
// Contrast with `question` below, which carries a TRUST type instead. Both
// kinds share one slot; see SPEC.md §5.2.

flow "Research Digest" {
  input question: Untrusted     // trust type — arrived from outside

  input headline: string        // text
  input max_results: integer    // a whole number
  input min_score: number       // 0.75 is valid here, but not for an integer
  input include_sources: bool   // NOTE: `bool` is NOT one of the six — see below
  input topics: list            // an ordered sequence
  input options: object         // a set of named fields

  node "clean"  -> skill validate(value: question)
  node "search" -> skill web_search(query: clean.result, count: max_results)
  node "graph"  -> skill graph_search(query: clean.result, depth: max_results)
  node "draft"  -> skill llm.chat(prompt: "Draft a digest.")
  node "check"  -> skill verify(source: draft.result)
  node "save"   -> skill memory_store(text: check.result, tags: topics, shared: true)

  clean -> search
  search -> graph
  graph -> draft
  draft -> check
  check -> save

  on "user_message" -> clean
}

// `include_sources: bool` is deliberate. The type is `boolean`, not `bool`, so
// an implementation treats `bool` as an unrecognised name: the input becomes
// unconstrained and a warning is raised. It is NEVER an error — see
// examples/input_conformance.tac for why that rule exists.
