# ABOUTME: Example Terraform configuration demonstrating Zenfra provider setup.
# ABOUTME: Shows required_providers block and provider configuration with endpoint and api_token.

terraform {
  required_providers {
    zenfra = {
      source = "registry.terraform.io/zenfra/zenfra"
    }
  }
}

provider "zenfra" {
  # endpoint  = "https://api.zenfra.cloud"  # Optional: defaults to https://api.zenfra.cloud
  # api_token = "your-api-token"          # Or set ZENFRA_API_TOKEN env var
}
