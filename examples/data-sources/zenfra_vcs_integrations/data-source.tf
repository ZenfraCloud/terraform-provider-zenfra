# List all GitHub integrations
data "zenfra_vcs_integrations" "github" {
  provider_type = "github"
}

# List all integrations
data "zenfra_vcs_integrations" "all" {}
