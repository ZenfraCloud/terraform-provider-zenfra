resource "zenfra_worker_pool" "private" {
  name = "Private Workers"
}

# The api_key is returned only on creation — store it securely.
output "worker_pool_api_key" {
  value     = zenfra_worker_pool.private.api_key
  sensitive = true
}
