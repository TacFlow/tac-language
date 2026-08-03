// Invalid: uses an unknown skill "fantasy_skill"
flow "Unknown Skill" {
  node "finder" -> skill fantasy_skill(location: "secret", power_level: 9000)

  on "init" -> finder
}
