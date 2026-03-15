resource "zenfra_cloud_integration_attachment" "app_aws" {
  integration_id = data.zenfra_cloud_integration.aws_prod.id
  stack_id       = zenfra_stack.app.id
  read           = true
  write          = true
}
