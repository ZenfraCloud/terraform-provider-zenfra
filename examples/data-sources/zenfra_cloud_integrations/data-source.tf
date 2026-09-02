# List all AWS cloud integrations
data "zenfra_cloud_integrations" "aws" {
  provider_type = "aws"
}

# List all cloud integrations
data "zenfra_cloud_integrations" "all" {}
