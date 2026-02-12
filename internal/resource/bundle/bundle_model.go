// ABOUTME: Terraform state models for the zenfra_configuration_bundle resource.
// ABOUTME: Maps between API Bundle types and Terraform schema types including nested env vars and mounted files.
package bundle

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// BundleModel represents the Terraform state model for a Zenfra configuration bundle.
type BundleModel struct {
	ID                  types.String `tfsdk:"id"`
	OrganizationID      types.String `tfsdk:"organization_id"`
	SpaceID             types.String `tfsdk:"space_id"`
	Name                types.String `tfsdk:"name"`
	Slug                types.String `tfsdk:"slug"`
	Description         types.String `tfsdk:"description"`
	Labels              types.List   `tfsdk:"labels"`
	ContentVersion      types.Int64  `tfsdk:"content_version"`
	AttachedStacksCount types.Int64  `tfsdk:"attached_stacks_count"`
	EnvironmentVariable types.Set    `tfsdk:"environment_variable"`
	MountedFile         types.Set    `tfsdk:"mounted_file"`
	CreatedAt           types.String `tfsdk:"created_at"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
}

// EnvVariableModel represents an environment variable block in the bundle.
type EnvVariableModel struct {
	Key         types.String `tfsdk:"key"`
	Value       types.String `tfsdk:"value"`
	Secret      types.Bool   `tfsdk:"secret"`
	Description types.String `tfsdk:"description"`
}

// MountedFileModel represents a mounted file block in the bundle.
type MountedFileModel struct {
	Path        types.String `tfsdk:"path"`
	Content     types.String `tfsdk:"content"`
	Secret      types.Bool   `tfsdk:"secret"`
	Description types.String `tfsdk:"description"`
}

// mapBundleToState converts an API Bundle response to a BundleModel for Terraform state.
func mapBundleToState(bundle *zenfraclient.Bundle) BundleModel {
	model := BundleModel{
		ID:                  types.StringValue(bundle.ID),
		OrganizationID:      types.StringValue(bundle.OrganizationID),
		SpaceID:             types.StringValue(bundle.SpaceID),
		Name:                types.StringValue(bundle.Name),
		ContentVersion:      types.Int64Value(bundle.ContentVersion),
		AttachedStacksCount: types.Int64Value(bundle.AttachedStacksCount),
		CreatedAt:           types.StringValue(bundle.CreatedAt.Format("2006-01-02T15:04:05Z07:00")),
		UpdatedAt:           types.StringValue(bundle.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")),
	}

	if bundle.Slug != "" {
		model.Slug = types.StringValue(bundle.Slug)
	} else {
		model.Slug = types.StringNull()
	}

	if bundle.Description != "" {
		model.Description = types.StringValue(bundle.Description)
	} else {
		model.Description = types.StringNull()
	}

	return model
}
