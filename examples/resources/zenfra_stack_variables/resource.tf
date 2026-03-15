# Manages the complete set of variables on a stack.
# Variables not listed here will be deleted (replace-all semantics).
resource "zenfra_stack_variables" "app" {
  stack_id = zenfra_stack.app.id

  variable {
    key    = "TF_VAR_environment"
    value  = "production"
    secret = false
  }

  variable {
    key    = "TF_VAR_db_password"
    value  = var.db_password
    secret = true
  }
}
