# Stack using a raw git source
resource "zenfra_stack" "app" {
  name     = "Application Stack"
  space_id = zenfra_space.production.id

  iac = {
    engine  = "terraform"
    version = "1.9.0"
  }

  source = {
    type = "raw_git"
    raw_git = {
      url = "https://github.com/example/infra.git"
      ref = {
        type = "branch"
        name = "main"
      }
      path = "stacks/app"
    }
  }

}

# Stack using a VCS integration
resource "zenfra_stack" "network" {
  name           = "Network Stack"
  space_id       = zenfra_space.production.id
  worker_pool_id = zenfra_worker_pool.private.id

  iac = {
    engine  = "opentofu"
    version = "1.8.0"
  }

  source = {
    type = "vcs"
    vcs = {
      provider       = "github"
      integration_id = data.zenfra_vcs_integration.github.id
      repository_id  = "example/network-infra"
      ref = {
        type = "branch"
        name = "main"
      }
    }
  }
}

# Stack that keeps its own Terraform state.
#
# State ownership is chosen at creation and cannot be changed afterwards. The
# provider refuses a mode change on an existing stack rather than replacing it,
# because there is no in-place migration: destroy the stack and create a new
# one, handling the state yourself in between.
#
# Anything that actually deletes and recreates a managed stack — an explicit
# destroy, `terraform apply -replace`, or a tainted resource — soft-deletes the
# Zenfra stack, and the Terraform state Zenfra held for it becomes inaccessible.
# There is no self-service export.
#
# `terraform taint` can defeat the plan-time comparison, because Terraform
# presents a tainted object as a creation. It is not a migration mechanism and
# must not be used as one.
#
# An external-state stack only runs on a worker advertising the
# `external-state-v1` capability, and:
#
#   - the configuration must resolve to exactly the `s3` backend; a run that
#     resolves to anything else fails before plan or apply;
#   - the worker owns `TF_DATA_DIR`, `HOME` and `TF_CLI_CONFIG_FILE`, and
#     refuses those plus `TF_PLUGIN_CACHE_DIR`,
#     `TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE`, `TF_CLI_ARGS`,
#     `TF_CLI_ARGS_<subcommand>`, `TERRAFORM_CONFIG` and `TF_REATTACH_PROVIDERS`
#     if they are supplied through stack, bundle or run variables;
#   - the backend credentials and state locking are yours; Zenfra never reads
#     the state;
#   - the backend block is source code, so the destination can change in a later
#     commit, but migrating between backends is your job and happens out of
#     band: every run starts in a clean workspace with a non-interactive init,
#     so Zenfra cannot move state for you.
resource "zenfra_stack" "external_state" {
  name     = "External State Stack"
  space_id = zenfra_space.production.id

  # Omit this attribute for a Zenfra-managed stack, which is the default.
  state_management = "external"

  iac = {
    engine  = "terraform"
    version = "1.9.0"
  }

  source = {
    type = "raw_git"
    raw_git = {
      # This repository's own backend "s3" block decides where the state lives.
      url = "https://github.com/example/infra.git"
      ref = {
        type = "branch"
        name = "main"
      }
      path = "stacks/external"
    }
  }
}
