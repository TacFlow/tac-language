// Parameterized sub-flow invocation — declaring typed inputs on a callee and
// binding them at the call site with `flow.run` (v0.4.0).
//
// TAC has no user-defined functions and no dedicated call construct: §2.2
// stands, and a node target is always a `skill`. Running another flow is a
// CAPABILITY, not a language feature — `flow.run(flow, params)`, listed in the
// standard library at SPEC.md §7.1.
//
// Value types are what make that call checkable. The callee below declares the
// SHAPE it expects; the caller supplies it. A toolchain can compare the two
// before either flow runs, instead of discovering at execution time that
// `depth` arrived as prose.

// ── Callee ───────────────────────────────────────────────────────────────────
// Declares what it needs. `target` carries a trust type because it arrives
// from outside and must be validated; `depth` and `strict` carry value types
// because what matters about them is their shape.

flow "Code Review" {
  input target: Untrusted     // provenance — validate before use
  input depth: integer        // shape — how far to recurse
  input strict: boolean       // shape — fail on style warnings?

  node "clean"   -> skill validate(value: target)
  node "scan"    -> skill memory_search(query: clean.result, top_k: depth)
  node "report"  -> skill llm.chat(prompt: "Summarize the findings.")
  node "confirm" -> skill verify(source: report.result)

  clean -> scan
  scan -> report
  report -> confirm

  on "review_requested" -> clean
}

// ── Caller ───────────────────────────────────────────────────────────────────
// Binds every declared input. `params` is an object literal whose keys are the
// callee's input names.

flow "Nightly Review" {
  input branch: Untrusted

  node "run_review" -> skill flow.run(flow: "Code Review", params: { target: branch, depth: 3, strict: true })
  node "announce"   -> skill tts.speak(text: "Nightly review finished.")

  run_review -> announce

  on "schedule_fired" -> run_review
}
