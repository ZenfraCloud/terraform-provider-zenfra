# Teams, members and grants are available after the organization's
# access-boundary activation; until then the API refuses these writes.
resource "zenfra_team" "platform" {
  name        = "Platform"
  slug        = "platform"
  description = "Owns the shared networking and base stacks"
}
