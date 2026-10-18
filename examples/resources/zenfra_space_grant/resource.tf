# Write on the networking space and every space below it.
resource "zenfra_space_grant" "platform_networking" {
  space_id = zenfra_space.networking.id
  team_id  = zenfra_team.platform.id
  role     = "write"
}
