resource "zenfra_api_token" "ci" {
  name            = "CI Pipeline Token"
  description     = "Token for CI/CD automation"
  role            = "write"
  expires_in_days = 90
}

# The token value is only available at creation time.
output "ci_token" {
  value     = zenfra_api_token.ci.token
  sensitive = true
}
