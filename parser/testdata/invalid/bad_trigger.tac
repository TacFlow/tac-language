// Invalid: trigger references undefined node
flow "Bad Trigger" {
  node "start" -> skill get_current_time()

  on "user_message" -> nonexistent
}
