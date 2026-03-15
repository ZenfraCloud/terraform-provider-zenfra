data "zenfra_vcs_repositories" "all" {
  integration_id = data.zenfra_vcs_integration.github.id
}
