relate "TAC Language" -> "Flow Management" {
  type: "depends_on",
  weight: 0.95,
  description: "TAC requires the flow management system"
}
relate "Parser" -> "AST" {
  type: "produces",
  weight: 1.0
}
