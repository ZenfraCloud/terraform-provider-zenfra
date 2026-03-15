data "zenfra_vcs_repository" "tf_base" {
  integration_id = data.zenfra_vcs_integration.github.id
  full_name      = "ndemeshchenko/tf-base"
}
