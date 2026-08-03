// Invalid: trust type issue — but this is semantic, not syntax
flow "Trust Type Flow" {
  input secret_key: secret

  node "fetch_config" -> skill config_get(key: "api_key")
  // fetch_config returns Secret, but we're feeding it directly to tts.speak which is risky
  // The semantic analyzer should flag this eventually

  on "init" -> fetch_config
}
