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
