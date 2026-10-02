flow "normalizar" {
  node "n" -> skill laya.normalize(input: payload, task: "acao", content_type: "text/csv")
  node "r" -> skill laya.episode.record(episode: n.episode, task: "acao")
  n -> r
}
