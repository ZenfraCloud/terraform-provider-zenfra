// ABOUTME: Shared request/response type definitions matching Zenfra API DTOs.
// ABOUTME: All JSON field names exactly mirror the Zenfra API handler DTOs for wire compatibility.

package zenfraclient

import "time"

// --- Space types ---

// Space represents a logical grouping of stacks.
type Space struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	Name           string     `json:"name"`
	Slug           string     `json:"slug"`
	Description    string     `json:"description,omitempty"`
	ParentID       *string    `json:"parent_id,omitempty"`
	Depth          int        `json:"depth"`
	InheritBundles bool       `json:"inherit_bundles"`
	ChildCount     int        `json:"child_count"`
	StackCount     int        `json:"stack_count"`
	CreatedBy      string     `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	UpdatedBy      string     `json:"updated_by"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

// CreateSpaceRequest is the request body for creating a space.
type CreateSpaceRequest struct {
	Name           string  `json:"name"`
	Slug           string  `json:"slug"`
	Description    string  `json:"description,omitempty"`
	ParentID       *string `json:"parent_id,omitempty"`
	InheritBundles bool    `json:"inherit_bundles,omitempty"`
}

// UpdateSpaceRequest is the request body for updating a space.
type UpdateSpaceRequest struct {
	Name           *string `json:"name,omitempty"`
	Slug           *string `json:"slug,omitempty"`
	Description    *string `json:"description,omitempty"`
	InheritBundles *bool   `json:"inherit_bundles,omitempty"`
}

// --- Stack types ---

// IACConfig represents the Infrastructure as Code tool configuration.
type IACConfig struct {
	Engine  string `json:"engine"`
	Version string `json:"version"`
}

// StackSourceRef identifies what to check out.
type StackSourceRef struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// StackSourceRawGit is a public HTTPS git source.
type StackSourceRawGit struct {
	URL  string         `json:"url"`
	Ref  StackSourceRef `json:"ref"`
	Path string         `json:"path,omitempty"`
}

// StackSourceVCS is an integration-backed VCS source.
type StackSourceVCS struct {
	Provider      string         `json:"provider"`
	IntegrationID string         `json:"integration_id"`
	RepositoryID  string         `json:"repository_id"`
	Ref           StackSourceRef `json:"ref"`
	Path          string         `json:"path,omitempty"`
}

// StackSource is a discriminated union for stack code source.
type StackSource struct {
	Type   string              `json:"type"`
	RawGit *StackSourceRawGit  `json:"raw_git,omitempty"`
	VCS    *StackSourceVCS     `json:"vcs,omitempty"`
}

// StackTriggerOnPush configures push-based automation triggers.
type StackTriggerOnPush struct {
	Enabled bool     `json:"enabled"`
	Paths   []string `json:"paths,omitempty"`
}

// StackTriggers configures what events can automatically create runs.
type StackTriggers struct {
	OnPush StackTriggerOnPush `json:"on_push"`
}

// LastRunInfo contains summary information about the most recent run.
type LastRunInfo struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Status      string  `json:"status"`
	TriggeredBy string  `json:"triggered_by"`
	TriggeredAt string  `json:"triggered_at"`
	FinishedAt  *string `json:"finished_at,omitempty"`
}

// Stack represents an IaC stack resource.
type Stack struct {
	ID              string        `json:"id"`
	OrganizationID  string        `json:"organization_id"`
	SpaceID         string        `json:"space_id"`
	Name            string        `json:"name"`
	WorkerPoolID    *string       `json:"worker_pool_id,omitempty"`
	AllowPublicPool bool          `json:"allow_public_pool"`
	IAC             IACConfig     `json:"iac"`
	Source          StackSource   `json:"source"`
	Triggers        StackTriggers `json:"triggers"`
	LastRun         *LastRunInfo  `json:"last_run,omitempty"`
	CreatedBy       string        `json:"created_by"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	UpdatedBy       string        `json:"updated_by"`
	DeletedAt       *time.Time    `json:"deleted_at,omitempty"`
}

// CreateStackRequest is the request body for creating a stack.
type CreateStackRequest struct {
	SpaceID         string      `json:"space_id"`
	Name            string      `json:"name"`
	WorkerPoolID    *string     `json:"worker_pool_id,omitempty"`
	AllowPublicPool bool        `json:"allow_public_pool"`
	IAC             IACConfig   `json:"iac"`
	Source          StackSource `json:"source"`
}

// UpdateStackRequest is the request body for updating a stack.
type UpdateStackRequest struct {
	Name            *string      `json:"name,omitempty"`
	WorkerPoolID    *string      `json:"worker_pool_id,omitempty"`
	AllowPublicPool *bool        `json:"allow_public_pool,omitempty"`
	IAC             *IACConfig   `json:"iac,omitempty"`
	Source          *StackSource `json:"source,omitempty"`
}

// StackVariable represents a single environment variable on a stack.
type StackVariable struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

// GetStackVariablesResponse is the response for GET /stacks/:id/variables.
type GetStackVariablesResponse struct {
	Variables []StackVariable `json:"variables"`
}

// SetStackVariablesRequest is the request for PUT /stacks/:id/variables.
type SetStackVariablesRequest struct {
	Variables []StackVariable `json:"variables"`
}

// --- Worker Pool types ---

// PoolCapacity shows org-level slot capacity.
type PoolCapacity struct {
	TotalSlots    int `json:"total_slots"`
	UsedSlots     int `json:"used_slots"`
	OnlineWorkers int `json:"online_workers"`
}

// WorkerPool represents a worker pool resource.
type WorkerPool struct {
	ID                 string       `json:"id"`
	OrganizationID     string       `json:"organization_id"`
	Name               string       `json:"name"`
	PoolType           string       `json:"pool_type"`
	APIKeyID           *string      `json:"api_key_id,omitempty"`
	KeyVersion         int          `json:"key_version"`
	Active             bool         `json:"active"`
	ActiveWorkersCount int64        `json:"active_workers_count"`
	Capacity           *PoolCapacity `json:"capacity,omitempty"`
	CreatedAt          time.Time    `json:"created_at"`
	UpdatedAt          time.Time    `json:"updated_at"`
	LastUsedAt         *time.Time   `json:"last_used_at,omitempty"`
}

// CreateWorkerPoolRequest is the request body for creating a worker pool.
type CreateWorkerPoolRequest struct {
	Name string `json:"name"`
}

// UpdateWorkerPoolRequest is the request body for updating a worker pool.
type UpdateWorkerPoolRequest struct {
	Name   *string `json:"name,omitempty"`
	Active *bool   `json:"active,omitempty"`
}

// CreateWorkerPoolResponse includes the pool and the write-once API key.
type CreateWorkerPoolResponse struct {
	Pool   WorkerPool `json:"pool"`
	APIKey string     `json:"api_key"`
}

// --- Bundle types ---

// EnvVariable represents an environment variable in a bundle.
type EnvVariable struct {
	Key         string `json:"key"`
	Value       string `json:"value,omitempty"`
	Description string `json:"description,omitempty"`
	Secret      bool   `json:"secret"`
}

// MountedFile represents a mounted file in a bundle.
type MountedFile struct {
	Path        string `json:"path"`
	Content     string `json:"content,omitempty"`
	Description string `json:"description,omitempty"`
	Secret      bool   `json:"secret"`
}

// Bundle represents a configuration bundle resource.
type Bundle struct {
	ID                   string        `json:"id"`
	OrganizationID       string        `json:"organization_id"`
	SpaceID              string        `json:"space_id"`
	Name                 string        `json:"name"`
	Slug                 string        `json:"slug"`
	Description          string        `json:"description"`
	Labels               []string      `json:"labels"`
	ContentVersion       int64         `json:"content_version"`
	AttachedStacksCount  int64         `json:"attached_stacks_count"`
	EnvironmentVariables []EnvVariable `json:"environment_variables"`
	MountedFiles         []MountedFile `json:"mounted_files"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
	CreatedBy            string        `json:"created_by"`
	UpdatedBy            string        `json:"updated_by"`
}

// CreateBundleRequest is the request body for creating a bundle.
type CreateBundleRequest struct {
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Description string   `json:"description,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	SpaceID     string   `json:"space_id,omitempty"`
}

// UpdateBundleRequest is the request body for updating bundle metadata.
type UpdateBundleRequest struct {
	Description *string   `json:"description,omitempty"`
	Labels      *[]string `json:"labels,omitempty"`
	SpaceID     *string   `json:"space_id,omitempty"`
}

// UpdateBundleContentRequest is the request body for updating bundle content.
type UpdateBundleContentRequest struct {
	Content         any   `json:"content"`
	ExpectedVersion int64 `json:"expected_version,omitempty"`
}

// UpdateBundleContentResponse includes the updated bundle and dedup status.
type UpdateBundleContentResponse struct {
	Bundle          Bundle `json:"bundle"`
	WasDeduplicated bool   `json:"was_deduplicated"`
}

// --- Bundle Attachment types ---

// BundleAttachment represents a bundle attached to a stack.
type BundleAttachment struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	StackID        string    `json:"stack_id"`
	BundleID       string    `json:"bundle_id"`
	Priority       int       `json:"priority"`
	AttachedAt     time.Time `json:"attached_at"`
	AttachedBy     string    `json:"attached_by"`
}

// AttachBundleRequest is the request body for attaching a bundle to a stack.
type AttachBundleRequest struct {
	BundleID string `json:"bundle_id"`
}

// ListAttachmentsResponse is the response for listing stack bundle attachments.
type ListAttachmentsResponse struct {
	Attachments []BundleAttachment `json:"attachments"`
	Total       int                `json:"total"`
}

// --- API Token types ---

// Token represents an API token resource.
type Token struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	TokenPrefix string     `json:"token_prefix"`
	Role        string     `json:"role"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	UsageCount  int64      `json:"usage_count"`
	Active      bool       `json:"active"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}

// CreateTokenRequest is the request body for creating an API token.
type CreateTokenRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Role        string `json:"role"`
	ExpiresIn   *int64 `json:"expires_in_days,omitempty"`
}

// CreateTokenResponse includes the write-once token value.
type CreateTokenResponse struct {
	Token    string `json:"token"`
	TokenObj Token  `json:"token_obj"`
}

// --- Organization types ---

// Organization represents the current user's organization.
type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// --- VCS Integration types ---

// VCSExternalAccount holds provider account info.
type VCSExternalAccount struct {
	ID    string `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name,omitempty"`
}

// VCSGitHubConfig is the response for GitHub config (no sensitive fields).
type VCSGitHubConfig struct {
	InstallationID int64 `json:"installation_id"`
}

// VCSGitLabConfig is the response for GitLab config (no sensitive fields).
type VCSGitLabConfig struct {
	BaseURL string `json:"base_url"`
}

// VCSIntegration represents a VCS integration resource.
type VCSIntegration struct {
	ID              string             `json:"id"`
	OrganizationID  string             `json:"organization_id"`
	Provider        string             `json:"provider"`
	Status          string             `json:"status"`
	DisplayName     string             `json:"display_name"`
	ExternalAccount VCSExternalAccount `json:"external_account"`
	GitHub          *VCSGitHubConfig   `json:"github,omitempty"`
	GitLab          *VCSGitLabConfig   `json:"gitlab,omitempty"`
	CreatedAt       string             `json:"created_at"`
	UpdatedAt       string             `json:"updated_at"`
}

// CreateVCSIntegrationRequest is the request body for creating a VCS integration.
type CreateVCSIntegrationRequest struct {
	Provider    string                    `json:"provider"`
	DisplayName string                   `json:"display_name,omitempty"`
	GitLab      *CreateVCSGitLabRequest  `json:"gitlab,omitempty"`
	GitHub      *CreateVCSGitHubRequest  `json:"github,omitempty"`
}

// CreateVCSGitLabRequest contains GitLab-specific configuration.
type CreateVCSGitLabRequest struct {
	BaseURL     string `json:"base_url"`
	AccessToken string `json:"access_token"`
}

// CreateVCSGitHubRequest contains GitHub-specific configuration.
type CreateVCSGitHubRequest struct {
	InstallationID int64 `json:"installation_id"`
}

// UpdateVCSIntegrationRequest is the request body for updating a VCS integration.
type UpdateVCSIntegrationRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	Status      *string `json:"status,omitempty"`
}

// --- Paginated response wrapper ---

// PaginatedResponse wraps paginated list responses from the API.
type PaginatedResponse[T any] struct {
	Items  []T   `json:"items"`
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}
