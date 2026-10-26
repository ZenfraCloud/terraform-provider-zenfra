# After each successful tracked apply of the network stack, start a run of the app stack.
resource "zenfra_stack_dependency" "app_after_network" {
  parent_stack_id = zenfra_stack.network.id
  child_stack_id  = zenfra_stack.app.id
}
