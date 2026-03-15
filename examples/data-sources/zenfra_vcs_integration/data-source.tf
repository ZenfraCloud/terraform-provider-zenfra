# Look up a VCS integration by name
data "zenfra_vcs_integration" "github" {
  name = "GitHub Org"
}

# Or by ID
data "zenfra_vcs_integration" "by_id" {
  id = "vcs-abc123"
}
