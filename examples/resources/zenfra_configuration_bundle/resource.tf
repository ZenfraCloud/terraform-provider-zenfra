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

# A bundle that attaches itself, by label, to every stack labelled
# "production", and runs hooks around plan and apply on those stacks.
resource "zenfra_configuration_bundle" "production_guardrails" {
  name     = "Production Guardrails"
  slug     = "production-guardrails"
  space_id = zenfra_space.production.id

  auto_attach_labels = ["production"]

  hooks = {
    before_plan = [
      "terraform fmt -check -recursive",
    ]
    after_apply = [
      "./scripts/notify.sh \"$ZENFRA_STACK_ID\"",
    ]
  }
}
