data "zenfra_current_organization" "this" {}

output "org_name" {
  value = data.zenfra_current_organization.this.name
}
