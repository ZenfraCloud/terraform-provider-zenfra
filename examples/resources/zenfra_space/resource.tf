resource "zenfra_space" "production" {
  name        = "Production"
  description = "Production infrastructure"
}

resource "zenfra_space" "production_us" {
  name            = "Production US"
  description     = "US region production workloads"
  parent_space_id = zenfra_space.production.id
  inherit_bundles = true
}
