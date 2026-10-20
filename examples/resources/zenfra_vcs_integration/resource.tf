# GitHub integration (via GitHub App installation)
resource "zenfra_vcs_integration" "github" {
  name            = "GitHub Org"
  provider_type   = "github"
  installation_id = 12345678
}

# GitLab integration (via personal access token)
resource "zenfra_vcs_integration" "gitlab" {
  name                  = "GitLab Self-Hosted"
  provider_type         = "gitlab"
  personal_access_token = var.gitlab_pat
  api_url               = "https://gitlab.example.com"
}

# Scoped to a space: only stacks in the platform space or a space below it
# may use this integration. Remove space_id to make it organization-wide.
resource "zenfra_vcs_integration" "platform_github" {
  name            = "Platform GitHub"
  provider_type   = "github"
  installation_id = 87654321
  space_id        = zenfra_space.platform.id
}
