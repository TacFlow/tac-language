// TAC Example: normalize a raw payload into a LAYA episode and record it (v0.5).
flow "normalizar" {
  node "n" -> skill laya.normalize(input: payload, task: "acao")
  node "r" -> skill laya.episode.record(episode: n.episode)
  n -> r
}
