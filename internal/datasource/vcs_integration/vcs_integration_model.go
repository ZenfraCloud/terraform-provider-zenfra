// ABOUTME: Shared model types for VCS integration data sources.
// ABOUTME: Maps between API VCSIntegration types and Terraform data source schema types.
package vcs_integration

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// vcsIntegrationDataSourceModel represents the Terraform state for a singular VCS integration data source.
type vcsIntegrationDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	OrganizationID types.String `tfsdk:"organization_id"`
	ProviderType   types.String `tfsdk:"provider_type"`
	Status         types.String `tfsdk:"status"`
	InstallationID types.Int64  `tfsdk:"installation_id"`
	APIURL         types.String `tfsdk:"api_url"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

// vcsIntegrationsDataSourceModel represents the Terraform state for the plural VCS integrations data source.
type vcsIntegrationsDataSourceModel struct {
	ProviderType types.String                  `tfsdk:"provider_type"`
	Integrations []vcsIntegrationListItemModel `tfsdk:"integrations"`
}

// vcsIntegrationListItemModel represents a single item in the integrations list.
type vcsIntegrationListItemModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	OrganizationID types.String `tfsdk:"organization_id"`
	ProviderType   types.String `tfsdk:"provider_type"`
	Status         types.String `tfsdk:"status"`
}

// mapVCSIntegrationToDataSource converts an API VCSIntegration to the singular data source model.
func mapVCSIntegrationToDataSource(vcs *zenfraclient.VCSIntegration) vcsIntegrationDataSourceModel {
	model := vcsIntegrationDataSourceModel{
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
