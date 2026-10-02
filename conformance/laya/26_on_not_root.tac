flow "on_not_root" {
  on "estoque.baixo" -> b
  node "a" -> skill laya.tasks.list()
  node "b" -> skill laya.tasks.list()
  a -> b
}
