resource "zenfra_webhook_endpoint" "on_call" {
  url         = "https://hooks.example.com/zenfra"
  description = "Failures and approvals to the on-call channel"
  event_types = ["run.failed", "run.awaiting_approval"]
}

# The signing secret is only available when Terraform creates the endpoint.
# Use it to verify the webhook-signature header on deliveries.
output "on_call_webhook_secret" {
  value     = zenfra_webhook_endpoint.on_call.secret
  sensitive = true
}
