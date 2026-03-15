# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build              # Build binary: terraform-provider-zenfra
make install            # Install to ~/.terraform.d/plugins/.../
make test               # Unit tests with race detector
make testacc            # Acceptance tests (requires TF_ACC=1)
make lint               # golangci-lint check
make fmt                # gofmt + goimports formatting
make clean              # Remove artifacts
```

## Architecture

Terraform provider for the Zenfra IaC platform, built with HashiCorp's Terraform Plugin Framework (`terraform-plugin-framework v1.14.0`).

### Project Structure
```
cmd/terraform-provider-zenfra/    # Entry point (gRPC provider server)
internal/
  provider/                       # Provider config (endpoint, api_token)
  resource/                       # Managed resources (CRUD lifecycle)
    api_token/
    bundle/
    bundle_attachment/
    space/
    stack/
    stack_variables/
    vcs_integration/
    worker_pool/
  datasource/                     # Data sources (read-only)
    current_organization/
    space/
    stack/                        # Includes zenfra_stack and zenfra_stacks (list)
    worker_pool/                  # Includes zenfra_worker_pool and zenfra_worker_pools (list)
  zenfraclient/                   # HTTP client to Zenfra API
    client.go                     # HTTP client with Bearer auth, 30s timeout
    retry.go                      # Exponential backoff with jitter (3 retries, handles 429/502/503/504)
    errors.go                     # Typed errors: NotFoundError, ConflictError, ValidationError, etc.
    types.go                      # All request/response DTOs (must match zenfra-api handler DTOs)
    spaces.go, stacks.go, ...     # Per-resource API methods
examples/provider/main.tf         # Example usage
```

### Resources (8)
| Resource | Key Notes |
|----------|-----------|
| `zenfra_space` | Hierarchical (parent_id), bundle inheritance |
| `zenfra_stack` | Nested config: `iac`, `source` (raw_git or vcs), `triggers` |
| `zenfra_worker_pool` | Write-once `api_key` (only on create) |
| `zenfra_configuration_bundle` | Env vars + mounted files, content versioning with `expected_version` |
| `zenfra_bundle_attachment` | Priority-ordered stack↔bundle link |
| `zenfra_stack_variables` | Replace-all semantics (PUT replaces entire list) |
| `zenfra_api_token` | Write-once `token` value, role-based, expiration |
| `zenfra_vcs_integration` | GitHub (installation_id) or GitLab (base_url + token) |

### Data Sources (6)
`zenfra_space`, `zenfra_stack`, `zenfra_stacks` (list), `zenfra_worker_pool`, `zenfra_worker_pools` (list), `zenfra_current_organization`

### Provider Configuration
```hcl
provider "zenfra" {
  endpoint  = "https://api.zenfra.cloud"  # Optional
  api_token = "..."                       # Or ZENFRA_API_TOKEN env var
}
```

## Development Patterns

### Resource Implementation Pattern
Each resource follows: `{type}_resource.go` (CRUD + ImportState) + `{type}_model.go` (Terraform types ↔ API types).

All resources implement `resource.ResourceWithImportState` for `terraform import` support.

### Write-Once Secrets
API tokens and worker pool keys are `Computed: true, Sensitive: true` — only returned on creation, never re-readable.

### Client ↔ API Contract
`zenfraclient/types.go` DTOs must match `zenfra-api` handler request/response structs. When API DTOs change, update types.go accordingly.

### ABOUTME Comments
All files must start with 2-line `// ABOUTME:` comments describing the file's purpose.

### Testing
- Unit tests alongside implementation files
- Race detector always enabled (`-race`)
- Acceptance tests gated behind `TF_ACC=1`

### Linting
Uses `.golangci.yml` with: govet, errcheck, staticcheck, gosec, gocyclo (max 15), gocognit (max 20), goconst, gocritic, errorlint. Test files exempt from complexity checks.
