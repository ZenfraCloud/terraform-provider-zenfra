resource "zenfra_space" "production" {
  name        = "Production"
  slug        = "production"
  description = "Production infrastructure"
}

resource "zenfra_space" "production_us" {
  name            = "Production US"
  slug            = "production-us"
  description     = "US region production workloads"
  parent_id       = zenfra_space.production.id
  inherit_bundles = true
}
