// ABOUTME: Terraform state model for the zenfra_vcs_integration resource.
// ABOUTME: Maps between API VCSIntegration types and Terraform schema types for GitHub and GitLab providers.
package vcs_integration

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// VCSIntegrationModel represents the Terraform state model for a VCS integration.
type VCSIntegrationModel struct {
	ID                  types.String `tfsdk:"id"`
	OrganizationID      types.String `tfsdk:"organization_id"`
	Name                types.String `tfsdk:"name"`
	ProviderType        types.String `tfsdk:"provider_type"`
	PersonalAccessToken types.String `tfsdk:"personal_access_token"`
	APIURL              types.String `tfsdk:"api_url"`
	InstallationID      types.Int64  `tfsdk:"installation_id"`
	Status              types.String `tfsdk:"status"`
	CreatedAt           types.String `tfsdk:"created_at"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
}

// mapVCSIntegrationToState converts an API VCSIntegration response to a VCSIntegrationModel.
func mapVCSIntegrationToState(vcs *zenfraclient.VCSIntegration) VCSIntegrationModel {
	model := VCSIntegrationModel{
		ID:             types.StringValue(vcs.ID),
		OrganizationID: types.StringValue(vcs.OrganizationID),
		Name:           types.StringValue(vcs.DisplayName),
		ProviderType:   types.StringValue(vcs.Provider),
		Status:         types.StringValue(vcs.Status),
		CreatedAt:      types.StringValue(vcs.CreatedAt),
		UpdatedAt:      types.StringValue(vcs.UpdatedAt),
	}

	if vcs.GitHub != nil {
		model.InstallationID = types.Int64Value(vcs.GitHub.InstallationID)
	} else {
		model.InstallationID = types.Int64Null()
	}

	if vcs.GitLab != nil && vcs.GitLab.BaseURL != "" {
		model.APIURL = types.StringValue(vcs.GitLab.BaseURL)
	} else {
		model.APIURL = types.StringNull()
	}

	return model
}
