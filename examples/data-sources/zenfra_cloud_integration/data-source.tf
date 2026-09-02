# Look up a cloud integration by name
data "zenfra_cloud_integration" "aws_prod" {
  name = "AWS Production"
}

# Or by ID
data "zenfra_cloud_integration" "by_id" {
  id = "ci-abc123"
}
