// ABOUTME: Shared model types for VCS repository data sources.
// ABOUTME: Maps between API VCSRepository types and Terraform data source schema types.
package vcs_repository

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// vcsRepositoryDataSourceModel represents the Terraform state for a singular VCS repository data source.
type vcsRepositoryDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	IntegrationID types.String `tfsdk:"integration_id"`
	FullName      types.String `tfsdk:"full_name"`
	ProviderType  types.String `tfsdk:"provider_type"`
	WebURL        types.String `tfsdk:"web_url"`
	DefaultBranch types.String `tfsdk:"default_branch"`
	Visibility    types.String `tfsdk:"visibility"`
	Archived      types.Bool   `tfsdk:"archived"`
	Enabled       types.Bool   `tfsdk:"enabled"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

// vcsRepositoriesDataSourceModel represents the Terraform state for the plural VCS repositories data source.
type vcsRepositoriesDataSourceModel struct {
	IntegrationID types.String                 `tfsdk:"integration_id"`
	Repositories  []vcsRepositoryListItemModel `tfsdk:"repositories"`
}

// vcsRepositoryListItemModel represents a single item in the repositories list.
type vcsRepositoryListItemModel struct {
	ID            types.String `tfsdk:"id"`
	IntegrationID types.String `tfsdk:"integration_id"`
	Provider      types.String `tfsdk:"provider"`
	FullName      types.String `tfsdk:"full_name"`
	DefaultBranch types.String `tfsdk:"default_branch"`
	Visibility    types.String `tfsdk:"visibility"`
	Enabled       types.Bool   `tfsdk:"enabled"`
}

// mapVCSRepositoryToDataSource converts an API VCSRepository to the singular data source model.
func mapVCSRepositoryToDataSource(repo *zenfraclient.VCSRepository) vcsRepositoryDataSourceModel {
	return vcsRepositoryDataSourceModel{
		ID:            types.StringValue(repo.ID),
		IntegrationID: types.StringValue(repo.IntegrationID),
		ProviderType:  types.StringValue(repo.Provider),
		FullName:      types.StringValue(repo.ProviderRepo.FullName),
		WebURL:        types.StringValue(repo.ProviderRepo.WebURL),
		DefaultBranch: types.StringValue(repo.ProviderRepo.DefaultBranch),
		Visibility:    types.StringValue(repo.ProviderRepo.Visibility),
		Archived:      types.BoolValue(repo.ProviderRepo.Archived),
		Enabled:       types.BoolValue(repo.Enabled),
		CreatedAt:     types.StringValue(repo.CreatedAt),
		UpdatedAt:     types.StringValue(repo.UpdatedAt),
	}
}
