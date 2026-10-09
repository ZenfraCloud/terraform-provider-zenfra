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
	Type   string             `json:"type"`
	RawGit *StackSourceRawGit `json:"raw_git,omitempty"`
	VCS    *StackSourceVCS    `json:"vcs,omitempty"`
}

// StackTriggerOnPush configures push-based automation triggers.
type StackTriggerOnPush struct {
	Enabled bool     `json:"enabled"`
	Paths   []string `json:"paths,omitempty"`
}

// StackTriggerOnPullRequest configures pull-request planning. Paths are shared
// with OnPush: one path list scopes the stack for both events.
type StackTriggerOnPullRequest struct {
	Enabled bool `json:"enabled"`
}

// StackTriggers configures what events can automatically create runs.
type StackTriggers struct {
	OnPush        StackTriggerOnPush        `json:"on_push"`
	OnPullRequest StackTriggerOnPullRequest `json:"on_pull_request"`
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
	ID              string          `json:"id"`
	OrganizationID  string          `json:"organization_id"`
	SpaceID         string          `json:"space_id"`
	Name            string          `json:"name"`
	WorkerPoolID    *string         `json:"worker_pool_id,omitempty"`
	AllowPublicPool bool            `json:"allow_public_pool"`
	IAC             IACConfig       `json:"iac"`
	Source          StackSource     `json:"source"`
	Triggers        StackTriggers   `json:"triggers"`
	PRComment       *StackPRComment `json:"pr_comment,omitempty"`
	// Hooks are the stack's own per-phase commands (ZenfraCloud/zenfra-cloud#737).
	// Absent when the stack has none.
	Hooks *Hooks `json:"hooks,omitempty"`
	// Labels select the bundles that auto-attach to the stack
	// (ZenfraCloud/zenfra-cloud#736). The API answers [] when there are none.
	Labels    []string     `json:"labels"`
	LastRun   *LastRunInfo `json:"last_run,omitempty"`
	CreatedBy string       `json:"created_by"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	UpdatedBy string       `json:"updated_by"`
	DeletedAt *time.Time   `json:"deleted_at,omitempty"`
}

// Hooks holds ordered shell commands per phase, the shape the API uses for
// both stack hooks and bundle hooks. The API drops empty phases, so every
// field is omitempty and Hooks{} marshals as {}, which the API reads as
// "no commands": on a bundle content write it clears the stored hooks.
type Hooks struct {
	BeforeInit  []string `json:"before_init,omitempty"`
	AfterInit   []string `json:"after_init,omitempty"`
	BeforePlan  []string `json:"before_plan,omitempty"`
	AfterPlan   []string `json:"after_plan,omitempty"`
	BeforeApply []string `json:"before_apply,omitempty"`
	AfterApply  []string `json:"after_apply,omitempty"`
}

// StackPRComment is the stack's pull request comment setting
// (ZenfraCloud/zenfra-cloud#833): whether plan comments list resource
// addresses, "off", "private_or_internal_repos" or "always".
type StackPRComment struct {
	ResourceAddresses string `json:"resource_addresses"`
}

// StackPRCommentRequest carries the setting in a request: an absent leaf
// leaves the stored value unchanged.
type StackPRCommentRequest struct {
	ResourceAddresses *string `json:"resource_addresses,omitempty"`
}

// CreateStackRequest is the request body for creating a stack.
type CreateStackRequest struct {
	SpaceID         string      `json:"space_id"`
	Name            string      `json:"name"`
	WorkerPoolID    *string     `json:"worker_pool_id,omitempty"`
	AllowPublicPool bool        `json:"allow_public_pool"`
	IAC             IACConfig   `json:"iac"`
	Source          StackSource `json:"source"`
	// Labels absent, null or [] all mean no labels.
	Labels []string `json:"labels,omitempty"`
	// PRComment absent: the API's default, private_or_internal_repos.
	PRComment *StackPRCommentRequest `json:"pr_comment,omitempty"`
}

// UpdateStackRequest is the request body for updating a stack.
type UpdateStackRequest struct {
	Name            *string      `json:"name,omitempty"`
	WorkerPoolID    *string      `json:"worker_pool_id,omitempty"`
	AllowPublicPool *bool        `json:"allow_public_pool,omitempty"`
	IAC             *IACConfig   `json:"iac,omitempty"`
	Source          *StackSource `json:"source,omitempty"`
	// Labels: nil leaves the stored labels unchanged; a pointer to an empty,
	// non-nil slice sends [] and clears them. A pointer to a nil slice would
	// marshal as null and leave them unchanged, so never send one to clear.
	Labels    *[]string              `json:"labels,omitempty"`
	PRComment *StackPRCommentRequest `json:"pr_comment,omitempty"`
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
	ID                 string        `json:"id"`
	OrganizationID     string        `json:"organization_id"`
	Name               string        `json:"name"`
	PoolType           string        `json:"pool_type"`
	APIKeyID           *string       `json:"api_key_id,omitempty"`
	KeyVersion         int           `json:"key_version"`
	Active             bool          `json:"active"`
	ActiveWorkersCount int64         `json:"active_workers_count"`
	Capacity           *PoolCapacity `json:"capacity,omitempty"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
	LastUsedAt         *time.Time    `json:"last_used_at,omitempty"`
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
	ID             string   `json:"id"`
	OrganizationID string   `json:"organization_id"`
	SpaceID        string   `json:"space_id"`
	Name           string   `json:"name"`
	Slug           string   `json:"slug"`
	Description    string   `json:"description"`
	Labels         []string `json:"labels"`
	// AutoAttachLabels attach the bundle to every stack carrying one of them
	// (ZenfraCloud/zenfra-cloud#736). Metadata, not content. [] when none.
	AutoAttachLabels     []string      `json:"auto_attach_labels"`
	ContentVersion       int64         `json:"content_version"`
	AttachedStacksCount  int64         `json:"attached_stacks_count"`
	HMACFingerprint      string        `json:"hmac_fingerprint,omitempty"`
	HMACKeyVersion       int           `json:"hmac_key_version,omitempty"`
	EnvironmentVariables []EnvVariable `json:"environment_variables"`
	MountedFiles         []MountedFile `json:"mounted_files"`
	// Hooks are content (ZenfraCloud/zenfra-cloud#736). Absent when none.
	Hooks     *Hooks    `json:"hooks,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy string    `json:"created_by"`
	UpdatedBy string    `json:"updated_by"`
}

// CreateBundleRequest is the request body for creating a bundle.
type CreateBundleRequest struct {
	Name             string   `json:"name"`
	Slug             string   `json:"slug"`
	Description      string   `json:"description,omitempty"`
	Labels           []string `json:"labels,omitempty"`
	SpaceID          string   `json:"space_id,omitempty"`
	AutoAttachLabels []string `json:"auto_attach_labels,omitempty"`
}

// UpdateBundleRequest is the request body for updating bundle metadata.
// A nil pointer leaves the field unchanged. For Labels and AutoAttachLabels a
// pointer to an empty, non-nil slice sends [] and clears them; a pointer to a
// nil slice marshals as null, which the API reads as "unchanged".
type UpdateBundleRequest struct {
	Description      *string   `json:"description,omitempty"`
	Labels           *[]string `json:"labels,omitempty"`
	SpaceID          *string   `json:"space_id,omitempty"`
	AutoAttachLabels *[]string `json:"auto_attach_labels,omitempty"`
}

// BundleContent is the body of a content write. The write replaces the
// environment variables and mounted files. Hooks nil leaves the stored hooks
// unchanged; &Hooks{} clears them. The API also accepts a secret_access list,
// which it stores but never reads and never returns; this provider does not
// send it, and since zenfra-cloud#865 an absent list keeps the stored one.
// A secret is always sent with its configured value (an empty one is refused
// before the request), so the API's keep-on-empty rule never applies here.
type BundleContent struct {
	EnvironmentVariables []EnvVariable `json:"environment_variables"`
	MountedFiles         []MountedFile `json:"mounted_files"`
	Hooks                *Hooks        `json:"hooks,omitempty"`
}

// UpdateBundleContentRequest is the request body for updating bundle content.
//
// ExpectedVersion fences the write on content_version: a non-zero value must
// match or the API answers 409 version_conflict. 0 is omitted; the API then
// still fences the write on the version it read whenever the write reuses
// stored data (zenfra-cloud#865). This provider never sends secret_access, so
// the API keeps the stored list and every content write is fenced, a write at
// 0 included: the first content write to a bundle, whose content_version
// starts at 0, fails with 409 if another writer got there first.
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

// Attachment sources (ZenfraCloud/zenfra-cloud#736).
const (
	// AttachmentSourceExplicit is a persisted attachment, the only kind the
	// attachment endpoints can detach or reprioritise.
	AttachmentSourceExplicit = "explicit"
	// AttachmentSourceAuto is a bundle that reaches the stack by label.
	AttachmentSourceAuto = "auto"
)

// BundleAttachment is one bundle a run of the stack would attach. An explicit
// row carries ID, Priority, AttachedAt and AttachedBy; an auto row has none of
// them and carries BundleSlug instead, which orders it.
type BundleAttachment struct {
	ID             string     `json:"id,omitempty"`
	OrganizationID string     `json:"organization_id"`
	StackID        string     `json:"stack_id"`
	BundleID       string     `json:"bundle_id"`
	Source         string     `json:"source"`
	BundleSlug     string     `json:"bundle_slug,omitempty"`
	Priority       *int       `json:"priority,omitempty"`
	AttachedAt     *time.Time `json:"attached_at,omitempty"`
	AttachedBy     string     `json:"attached_by,omitempty"`
}

// AttachBundleRequest is the request body for attaching a bundle to a stack.
type AttachBundleRequest struct {
	BundleID string `json:"bundle_id"`
}

// UpdateBundlePriorityRequest is the request body for
// PATCH /stacks/:stack_id/bundles/:bundle_id. Bundles apply in ascending
// priority, so on a conflicting env var or file the higher value wins.
type UpdateBundlePriorityRequest struct {
	Priority int `json:"priority"`
}

// ListAttachmentsResponse is the response for GET /stacks/:id/bundles.
// Attachments and Total are the explicit attachments only; AutoAttached are
// the bundles that attach by label and AutoMatchCount is their number.
type ListAttachmentsResponse struct {
	Attachments    []BundleAttachment `json:"attachments"`
	Total          int                `json:"total"`
	AutoAttached   []BundleAttachment `json:"auto_attached"`
	AutoMatchCount int                `json:"auto_match_count"`
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

// IACToolConfig represents the default IaC tool configuration.
type IACToolConfig struct {
	Engine  string `json:"engine"`
	Version string `json:"version"`
}

// OrganizationSettings represents organization-level settings.
type OrganizationSettings struct {
	DefaultIACTool     IACToolConfig `json:"default_iac_tool"`
	RunTimeoutMinutes  int           `json:"run_timeout_minutes"`
	PlanTimeoutMinutes int           `json:"plan_timeout_minutes"`
	AuditRetentionDays int           `json:"audit_retention_days"`
}

// OrganizationBilling represents billing information for an organization.
type OrganizationBilling struct {
	Plan            string `json:"plan"`
	SlotLimit       int    `json:"slot_limit"`
	SlotsUsed       int    `json:"slots_used"`
	SlotsAvailable  int    `json:"slots_available"`
	EnforcementMode string `json:"enforcement_mode"`
}

// Organization represents the current user's organization.
type Organization struct {
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	Slug      string               `json:"slug"`
	Settings  OrganizationSettings `json:"settings"`
	Billing   *OrganizationBilling `json:"billing,omitempty"`
	CreatedAt string               `json:"created_at"`
	UpdatedAt string               `json:"updated_at,omitempty"`
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
	Provider    string                  `json:"provider"`
	DisplayName string                  `json:"display_name,omitempty"`
	GitLab      *CreateVCSGitLabRequest `json:"gitlab,omitempty"`
	GitHub      *CreateVCSGitHubRequest `json:"github,omitempty"`
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

// --- VCS Repository types ---

// VCSProviderRepo holds provider-specific repository metadata.
type VCSProviderRepo struct {
	ID            string `json:"id"`
	FullName      string `json:"full_name"`
	WebURL        string `json:"web_url"`
	DefaultBranch string `json:"default_branch"`
	Visibility    string `json:"visibility"`
	Archived      bool   `json:"archived"`
}

// VCSRepository represents a repository discovered via a VCS integration.
type VCSRepository struct {
	ID            string          `json:"id"`
	IntegrationID string          `json:"integration_id"`
	Provider      string          `json:"provider"`
	ProviderRepo  VCSProviderRepo `json:"provider_repo"`
	Enabled       bool            `json:"enabled"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
}

// --- Cloud Integration types ---

// CloudAWSConfig holds AWS-specific configuration for a cloud integration.
type CloudAWSConfig struct {
	RoleARN          string `json:"role_arn"`
	SessionDuration  int    `json:"session_duration,omitempty"`
	Region           string `json:"region,omitempty"`
	GenerateOnWorker bool   `json:"generate_on_worker"`
}

// CloudIntegrationError represents an error from a cloud integration verification.
type CloudIntegrationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	At      string `json:"at"`
}

// CloudIntegration represents a cloud provider integration resource.
type CloudIntegration struct {
	ID              string                 `json:"id"`
	OrganizationID  string                 `json:"organization_id"`
	SpaceID         string                 `json:"space_id"`
	Name            string                 `json:"name"`
	Provider        string                 `json:"provider"`
	Status          string                 `json:"status"`
	AWS             *CloudAWSConfig        `json:"aws,omitempty"`
	AutoAttachLabel string                 `json:"auto_attach_label,omitempty"`
	CreatedAt       string                 `json:"created_at"`
	UpdatedAt       string                 `json:"updated_at"`
	CreatedBy       string                 `json:"created_by"`
	LastVerifiedAt  *string                `json:"last_verified_at,omitempty"`
	LastError       *CloudIntegrationError `json:"last_error,omitempty"`
}

// --- Cloud Attachment types ---

// CloudAttachment represents a cloud integration attached to a stack.
type CloudAttachment struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	IntegrationID  string `json:"integration_id"`
	StackID        string `json:"stack_id"`
	Read           bool   `json:"read"`
	Write          bool   `json:"write"`
	IsAutoAttached bool   `json:"is_auto_attached"`
	CreatedAt      string `json:"created_at"`
	CreatedBy      string `json:"created_by"`
	ExternalID     string `json:"external_id"`
}

// AttachCloudIntegrationRequest is the request body for attaching a cloud integration to a stack.
type AttachCloudIntegrationRequest struct {
	StackID string `json:"stack_id"`
	Read    bool   `json:"read"`
	Write   bool   `json:"write"`
}

// --- Paginated response wrapper ---

// PaginatedResponse wraps paginated list responses from the API.
type PaginatedResponse[T any] struct {
	Items  []T   `json:"items"`
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}
