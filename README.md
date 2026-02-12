# Terraform Provider for Zenfra

Manage your Zenfra infrastructure-as-code platform with Terraform.

## Setup

```hcl
terraform {
  required_providers {
    zenfra = {
      source = "registry.terraform.io/zenfra/zenfra"
    }
  }
}

provider "zenfra" {
  # api_token = "..."  # or set ZENFRA_API_TOKEN env var
}
```

## Resources

- `zenfra_space` — organizational grouping for stacks
- `zenfra_stack` — IaC stack with source, engine, and trigger config
- `zenfra_worker_pool` — private worker pool for running operations
- `zenfra_configuration_bundle` — reusable env vars and mounted files
- `zenfra_bundle_attachment` — attach a bundle to a stack
- `zenfra_stack_variables` — environment variables on a stack
- `zenfra_api_token` — API token management
- `zenfra_vcs_integration` — GitHub or GitLab integration

## Data Sources

- `zenfra_space` — look up a space by ID
- `zenfra_stack` / `zenfra_stacks` — look up stacks
- `zenfra_worker_pool` / `zenfra_worker_pools` — look up worker pools
- `zenfra_current_organization` — get the current org

## Building from source

```
make build
```

To install locally for development:

```
make install
```

## Running tests

```
make test
```
