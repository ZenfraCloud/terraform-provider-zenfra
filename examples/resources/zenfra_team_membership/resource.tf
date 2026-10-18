# A team of one gives a single person access to a subtree.
resource "zenfra_team_membership" "alice" {
  team_id = zenfra_team.platform.id
  user_id = var.alice_user_id
}
