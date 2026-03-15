# Bootstrap Gaps: Provider Fixes & New Resources

## Context

The zenfra-bootstrap team needs a fully declarative, data-driven bootstrap flow:
discover VCS integrations → discover repos → discover cloud integrations → attach to stacks.
This plan addresses 3 bugs and 3 feature gaps blocking that flow.

---

## Task 1: Fix `ListVCSIntegrations` response parsing

**Type:** Bug fix
**Priority:** P0 — blocks all VCS integration discovery

**Problem:**
The client expects a raw JSON array but the API returns an object wrapper:
```json
{"integrations": [...], "total": 1, "limit": 100, "offset": 0}
```

**Files to change:**
- `internal/zenfraclient/vcs_integrations.go` — `ListVCSIntegrations()` must parse `{"integrations": [...]}`
- `internal/zenfraclient/vcs_integrations.go` — also update `ListVCSIntegrations` to accept optional query params (`provider`, `status`)
- `internal/zenfraclient/client_test.go` — update test for wrapped response

**Fix:**
```go
// Before (broken):
var integrations []VCSIntegration

// After:
var resp struct {
    Integrations []VCSIntegration `json:"integrations"`
}
```

**Verification:**
- Unit test with httptest returning `{"integrations": [...], "total": 1}`
- `go test ./internal/zenfraclient/...`

---

## Task 2: Remove trigger fields from stack schema

**Type:** Bug fix
**Priority:** P0 — causes apply/readback inconsistency on every stack

**Problem:**
Triggers are `Optional + Computed` in the schema but the backend doesn't fully support them end-to-end. The API returns default/empty trigger values that differ from what was configured, causing perpetual plan diffs.

**Files to change:**
- `internal/resource/stack/stack_resource.go` — remove `triggers` from schema entirely
- `internal/resource/stack/stack_model.go` — remove `Triggers` from `StackModel`
- `internal/resource/stack/stack_resource.go` — remove trigger handling from Create, Update, Read (`buildTriggersFromModel`, `mapStackToState` trigger section)
- `internal/resource/stack/stack_resource_test.go` — remove trigger-related tests
- `internal/datasource/stack/stack_data_source.go` — remove triggers from data source schema/model
- `docs/resources/stack.md` — regenerate
- `examples/resources/zenfra_stack/resource.tf` — remove triggers from example

**Note:** When the backend fully supports triggers, they can be re-added. This is a temporary removal to unblock users.

**Verification:**
- `go build ./...`
- `go test ./internal/resource/stack/...`
- `make docs`

---

## Task 3: Prevent in-place stack source type migration

**Type:** Bug fix
**Priority:** P1 — causes API 500 errors

**Problem:**
Changing `source.type` from `raw_git` to `vcs` (or vice versa) on an existing stack causes an API 500 from `SetStackSource`. Destroy + recreate works.

**Files to change:**
- `internal/resource/stack/stack_resource.go` — add `RequiresReplace()` plan modifier to `source.type`

**Fix:**
```go
"type": schema.StringAttribute{
    Description: "Source type: 'raw_git' or 'vcs'.",
    Required:    true,
    PlanModifiers: []planmodifier.String{
        stringplanmodifier.RequiresReplace(),
    },
},
```

**Verification:**
- `go build ./...`
- `go test ./internal/resource/stack/...`

---

## Task 4: Add VCS repository data sources

**Type:** Feature
**Priority:** P0 — bootstrap can't resolve `repository_id` without this

**API endpoints (already exist):**
- `GET /api/v1/vcs/integrations/{id}/repos` — list repos for an integration
  - Response: `{"repositories": [...]}`
- `GET /api/v1/vcs/repos/{id}` — get single repo
  - Response: `VCSRepositoryResponse`

**API response shape:**
```json
{
  "id": "repo-abc",
  "integration_id": "vcs-123",
  "provider": "github",
  "provider_repo": {
    "id": "12345",
    "full_name": "ndemeshchenko/tf-base",
    "web_url": "https://github.com/ndemeshchenko/tf-base",
    "default_branch": "main",
    "visibility": "private",
    "archived": false
  },
  "enabled": true,
  "created_at": "...",
  "updated_at": "..."
}
```

**New files:**
- `internal/zenfraclient/vcs_repositories.go` — `ListVCSRepositories(integrationID)`, `GetVCSRepository(id)`
- `internal/zenfraclient/types.go` — add `VCSRepository`, `VCSProviderRepo` types
- `internal/datasource/vcs_repository/vcs_repository_data_source.go` — singular, lookup by `id` or by `integration_id` + `full_name`
- `internal/datasource/vcs_repository/vcs_repositories_data_source.go` — plural, list by `integration_id`
- `internal/datasource/vcs_repository/vcs_repository_model.go` — shared model
- `internal/datasource/vcs_repository/vcs_repository_data_source_test.go`
- `internal/datasource/vcs_repository/vcs_repositories_data_source_test.go`
- `examples/data-sources/zenfra_vcs_repository/data-source.tf`
- `examples/data-sources/zenfra_vcs_repositories/data-source.tf`

**Modified files:**
- `internal/provider/provider.go` — register both data sources

**Target UX:**
```hcl
data "zenfra_vcs_repository" "tf_base" {
  integration_id = data.zenfra_vcs_integration.github.id
  full_name      = "ndemeshchenko/tf-base"
}

resource "zenfra_stack" "app" {
  source {
    type = "vcs"
    vcs {
      provider       = "github"
      integration_id = data.zenfra_vcs_integration.github.id
      repository_id  = data.zenfra_vcs_repository.tf_base.id
      ref { type = "branch"; name = "main" }
    }
  }
}
```

**Verification:**
- Unit tests for model mapping
- `go build ./...`
- `go test ./internal/datasource/vcs_repository/...`

---

## Task 5: Add cloud integration data sources

**Type:** Feature
**Priority:** P0 — bootstrap can't discover AWS integrations without this

**API endpoints (already exist):**
- `GET /api/v1/cloud/integrations` — list all cloud integrations
  - Query params: `space_id` (optional filter)
  - Response: `{"integrations": [...], "total": N}`
- `GET /api/v1/cloud/integrations/{id}` — get single integration
  - Response: `CloudIntegrationResponse`

**API response shape:**
```json
{
  "id": "ci-abc",
  "organization_id": "org-123",
  "space_id": "sp-456",
  "name": "AWS Production",
  "provider": "aws",
  "status": "active",
  "aws": {
    "role_arn": "arn:aws:iam::123456789:role/zenfra-exec",
    "session_duration": 3600,
    "region": "us-east-1",
    "generate_on_worker": false
  },
  "auto_attach_label": "",
  "created_at": "...",
  "updated_at": "...",
  "last_verified_at": "...",
  "last_error": null
}
```

**New files:**
- `internal/zenfraclient/cloud_integrations.go` — `GetCloudIntegration(id)`, `ListCloudIntegrations(spaceID)`
- `internal/zenfraclient/types.go` — add `CloudIntegration`, `AWSConfig` types
- `internal/datasource/cloud_integration/cloud_integration_data_source.go` — singular, lookup by `id` or `name`
- `internal/datasource/cloud_integration/cloud_integrations_data_source.go` — plural, list with optional `space_id` filter
- `internal/datasource/cloud_integration/cloud_integration_model.go` — shared model
- `internal/datasource/cloud_integration/cloud_integration_data_source_test.go`
- `internal/datasource/cloud_integration/cloud_integrations_data_source_test.go`
- `examples/data-sources/zenfra_cloud_integration/data-source.tf`
- `examples/data-sources/zenfra_cloud_integrations/data-source.tf`

**Modified files:**
- `internal/provider/provider.go` — register both data sources

**Target UX:**
```hcl
data "zenfra_cloud_integration" "aws_prod" {
  name = "AWS Production"
}
```

**Verification:**
- Unit tests for model mapping
- `go build ./...`
- `go test ./internal/datasource/cloud_integration/...`

---

## Task 6: Add cloud integration attachment resource

**Type:** Feature
**Priority:** P0 — bootstrap can't attach AWS integrations to stacks without this

**API endpoints (already exist):**
- `POST /api/v1/cloud/integrations/{id}/attachments` — attach stack
  - Request: `{"stack_id": "...", "read": true, "write": true}`
  - Response: `CloudAttachmentResponse`
- `GET /api/v1/cloud/integrations/{id}/attachments` — list attachments
  - Response: `{"attachments": [...], "total": N}`
- `DELETE /api/v1/cloud/integrations/{id}/attachments/{attachment_id}` — detach

**New files:**
- `internal/zenfraclient/cloud_integrations.go` — add `AttachCloudIntegration()`, `ListCloudAttachments()`, `DetachCloudIntegration()`
- `internal/zenfraclient/types.go` — add `CloudAttachment`, `AttachCloudIntegrationRequest` types
- `internal/resource/cloud_integration_attachment/cloud_integration_attachment_resource.go` — CRUD resource
- `internal/resource/cloud_integration_attachment/cloud_integration_attachment_model.go` — model
- `internal/resource/cloud_integration_attachment/cloud_integration_attachment_resource_test.go`
- `examples/resources/zenfra_cloud_integration_attachment/resource.tf`
- `examples/resources/zenfra_cloud_integration_attachment/import.sh`

**Modified files:**
- `internal/provider/provider.go` — register resource

**Schema:**
```hcl
resource "zenfra_cloud_integration_attachment" "app_aws" {
  integration_id = data.zenfra_cloud_integration.aws_prod.id
  stack_id       = zenfra_stack.app.id
  read           = true
  write          = true
}
```

- `id` — computed (attachment ID)
- `integration_id` — required, ForceNew
- `stack_id` — required, ForceNew
- `read` — optional, defaults to true
- `write` — optional, defaults to true
- No update — detach + reattach (ForceNew on both IDs)
- Import via composite ID: `integration_id:attachment_id`

**Verification:**
- Unit tests for model mapping
- `go build ./...`
- `go test ./internal/resource/cloud_integration_attachment/...`

---

## Execution Order

Tasks have dependencies:

```
Task 1 (fix VCS list) ──→ Task 4 (VCS repo data sources)
Task 2 (remove triggers) ─┐
Task 3 (source.type ForceNew) ──→ independent
                           └──→ Task 5 (cloud integration data sources)
                                     └──→ Task 6 (cloud attachment resource)
```

**Recommended batch order:**
1. **Batch 1** (bugs, no deps): Tasks 1, 2, 3 — in parallel
2. **Batch 2** (features, after bug fixes): Tasks 4, 5 — in parallel
3. **Batch 3** (depends on Task 5): Task 6

**After all tasks:**
- Run `make docs` to regenerate all documentation
- Run full `go test ./...`
- Run `go build ./...`

---

## End-to-end bootstrap flow after completion

```hcl
# Discover VCS integration
data "zenfra_vcs_integration" "github" {
  name = "GitHub Org"
}

# Discover repository
data "zenfra_vcs_repository" "tf_base" {
  integration_id = data.zenfra_vcs_integration.github.id
  full_name      = "ndemeshchenko/tf-base"
}

# Discover cloud integration
data "zenfra_cloud_integration" "aws_prod" {
  name = "AWS Production"
}

# Create stack with VCS source
resource "zenfra_stack" "demo" {
  name     = "Demo Stack"
  space_id = zenfra_space.demo.id

  iac {
    engine  = "terraform"
    version = "1.9.0"
  }

  source {
    type = "vcs"
    vcs {
      provider       = "github"
      integration_id = data.zenfra_vcs_integration.github.id
      repository_id  = data.zenfra_vcs_repository.tf_base.id
      ref { type = "branch"; name = "main" }
    }
  }
}

# Attach cloud integration
resource "zenfra_cloud_integration_attachment" "demo_aws" {
  integration_id = data.zenfra_cloud_integration.aws_prod.id
  stack_id       = zenfra_stack.demo.id
  read           = true
  write          = true
}
```
