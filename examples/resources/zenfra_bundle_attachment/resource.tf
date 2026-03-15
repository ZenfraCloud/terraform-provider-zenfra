resource "zenfra_bundle_attachment" "app_aws" {
  stack_id  = zenfra_stack.app.id
  bundle_id = zenfra_configuration_bundle.aws_credentials.id
}
