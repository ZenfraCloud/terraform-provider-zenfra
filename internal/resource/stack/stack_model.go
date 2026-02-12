// ABOUTME: Terraform state models for the zenfra_stack resource.
// ABOUTME: Maps between API Stack types and Terraform schema types including nested source/iac/triggers.
package stack

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// StackModel represents the Terraform state model for a Zenfra stack.
type StackModel struct {
	ID              types.String `tfsdk:"id"`
	OrganizationID  types.String `tfsdk:"organization_id"`
	SpaceID         types.String `tfsdk:"space_id"`
	Name            types.String `tfsdk:"name"`
	WorkerPoolID    types.String `tfsdk:"worker_pool_id"`
	AllowPublicPool types.Bool   `tfsdk:"allow_public_pool"`
	IAC             types.Object `tfsdk:"iac"`
	Source          types.Object `tfsdk:"source"`
	Triggers        types.Object `tfsdk:"triggers"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
	CreatedBy       types.String `tfsdk:"created_by"`
	UpdatedBy       types.String `tfsdk:"updated_by"`
}

// IACModel represents the IAC configuration.
type IACModel struct {
	Engine  types.String `tfsdk:"engine"`
	Version types.String `tfsdk:"version"`
}

// RefModel represents a source reference (branch, tag, or commit).
type RefModel struct {
	Type types.String `tfsdk:"type"`
	Name types.String `tfsdk:"name"`
}

// RawGitModel represents a raw HTTPS git source.
type RawGitModel struct {
	URL  types.String `tfsdk:"url"`
	Ref  types.Object `tfsdk:"ref"`
	Path types.String `tfsdk:"path"`
}

// VCSModel represents an integration-backed VCS source.
type VCSModel struct {
	Provider      types.String `tfsdk:"provider"`
	IntegrationID types.String `tfsdk:"integration_id"`
	RepositoryID  types.String `tfsdk:"repository_id"`
	Ref           types.Object `tfsdk:"ref"`
	Path          types.String `tfsdk:"path"`
}

// SourceModel represents the stack source configuration.
type SourceModel struct {
	Type   types.String `tfsdk:"type"`
	RawGit types.Object `tfsdk:"raw_git"`
	VCS    types.Object `tfsdk:"vcs"`
}

// OnPushModel represents the on_push trigger configuration.
type OnPushModel struct {
	Enabled types.Bool `tfsdk:"enabled"`
	Paths   types.List `tfsdk:"paths"`
}

// TriggersModel represents the stack trigger configuration.
type TriggersModel struct {
	OnPush types.Object `tfsdk:"on_push"`
}

// IACModelAttrTypes defines the attribute types for IACModel.
var IACModelAttrTypes = map[string]attr.Type{
	"engine":  types.StringType,
	"version": types.StringType,
}

// RefModelAttrTypes defines the attribute types for RefModel.
var RefModelAttrTypes = map[string]attr.Type{
	"type": types.StringType,
	"name": types.StringType,
}

// RawGitModelAttrTypes defines the attribute types for RawGitModel.
var RawGitModelAttrTypes = map[string]attr.Type{
	"url":  types.StringType,
	"ref":  types.ObjectType{AttrTypes: RefModelAttrTypes},
	"path": types.StringType,
}

// VCSModelAttrTypes defines the attribute types for VCSModel.
var VCSModelAttrTypes = map[string]attr.Type{
	"provider":       types.StringType,
	"integration_id": types.StringType,
	"repository_id":  types.StringType,
	"ref":            types.ObjectType{AttrTypes: RefModelAttrTypes},
	"path":           types.StringType,
}

// SourceModelAttrTypes defines the attribute types for SourceModel.
var SourceModelAttrTypes = map[string]attr.Type{
	"type":    types.StringType,
	"raw_git": types.ObjectType{AttrTypes: RawGitModelAttrTypes},
	"vcs":     types.ObjectType{AttrTypes: VCSModelAttrTypes},
}

// OnPushModelAttrTypes defines the attribute types for OnPushModel.
var OnPushModelAttrTypes = map[string]attr.Type{
	"enabled": types.BoolType,
	"paths":   types.ListType{ElemType: types.StringType},
}

// TriggersModelAttrTypes defines the attribute types for TriggersModel.
var TriggersModelAttrTypes = map[string]attr.Type{
	"on_push": types.ObjectType{AttrTypes: OnPushModelAttrTypes},
}
