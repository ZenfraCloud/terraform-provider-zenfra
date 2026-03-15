resource "zenfra_configuration_bundle" "aws_credentials" {
  name        = "AWS Credentials"
  slug        = "aws-credentials"
  space_id    = zenfra_space.production.id
  description = "AWS credentials for production workloads"
  labels      = ["aws", "production"]

  environment_variable {
    key    = "AWS_REGION"
    value  = "us-east-1"
    secret = false
  }

  environment_variable {
    key    = "AWS_ACCESS_KEY_ID"
    value  = var.aws_access_key_id
    secret = true
  }

  mounted_file {
    path        = "/etc/config/settings.json"
    content     = file("${path.module}/settings.json")
    description = "Application settings"
    secret      = false
  }
}
