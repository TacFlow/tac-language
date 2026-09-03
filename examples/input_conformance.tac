// The conformance rule for input types (v0.4.0).
//
//   An unrecognised type name means "unconstrained", with a warning —
//   NEVER an error.
//
// Every declaration below is valid TAC. An implementation that rejects any of
// them is not conformant.
//
// WHY THIS RULE EXISTS
//
// The type slot on an `input` holds a trust type OR a value type, and the set
// of names in it grows. A toolchain pinned to an older TAC, or one that does
// not implement a dialect's extensions, will meet names it does not know. If
// an unknown name were an error, source would stop compiling the moment it
// referenced anything newer than the oldest implementation reading it, and the
// two type systems could never have shared a slot in the first place —
// `Untrusted` is not a value type, and `string` is not a trust type.
//
// Degrading to "unconstrained plus a warning" keeps such source compiling
// while still telling the author their constraint is not being enforced.

flow "Conformance" {
  input a: string          // value type — recognised
  input b: Untrusted       // trust type — recognised
  input c                  // no type at all — unconstrained, no warning
  input d: Frobnicate      // unknown — unconstrained, WARNING, not an error
  input e: int             // a near miss: the type is `integer`, not `int`
  input f: String          // case matters: the type is `string`

  node "clean"  -> skill validate(value: b)
  node "answer" -> skill llm.chat(prompt: "Answer using the supplied inputs.")
  node "check"  -> skill verify(source: answer.result)

  clean -> answer
  answer -> check

  on "user_message" -> clean
}

// `d`, `e` and `f` all end up unconstrained. `e` and `f` are the cases worth
// noticing: they LOOK like value types and are not, so the warning is the only
// signal that `e` will accept prose and `f` will accept a number.
