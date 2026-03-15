# List all stacks in a space
data "zenfra_stacks" "production" {
  space_id = zenfra_space.production.id
}
