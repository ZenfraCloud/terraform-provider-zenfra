// ABOUTME: Shared model types for cloud integration data sources.
// ABOUTME: Maps between API CloudIntegration types and Terraform data source schema types.
package cloud_integration

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// cloudIntegrationDataSourceModel represents the Terraform state for a singular cloud integration data source.
type cloudIntegrationDataSourceModel struct {
	ID              types.String         `tfsdk:"id"`
	Name            types.String         `tfsdk:"name"`
	OrganizationID  types.String         `tfsdk:"organization_id"`
	SpaceID         types.String         `tfsdk:"space_id"`
	ProviderType    types.String         `tfsdk:"provider_type"`
	Status          types.String         `tfsdk:"status"`
	AWS             *cloudAWSConfigModel `tfsdk:"aws"`
	AutoAttachLabel types.String         `tfsdk:"auto_attach_label"`
	CreatedAt       types.String         `tfsdk:"created_at"`
	UpdatedAt       types.String         `tfsdk:"updated_at"`
	LastVerifiedAt  types.String         `tfsdk:"last_verified_at"`
}

// cloudAWSConfigModel represents the nested AWS configuration block.
type cloudAWSConfigModel struct {
	RoleARN          types.String `tfsdk:"role_arn"`
	SessionDuration  types.Int64  `tfsdk:"session_duration"`
	Region           types.String `tfsdk:"region"`
	GenerateOnWorker types.Bool   `tfsdk:"generate_on_worker"`
}

// cloudIntegrationsDataSourceModel represents the Terraform state for the plural cloud integrations data source.
type cloudIntegrationsDataSourceModel struct {
	SpaceID      types.String                    `tfsdk:"space_id"`
	ProviderType types.String                    `tfsdk:"provider_type"`
	Integrations []cloudIntegrationListItemModel `tfsdk:"integrations"`
}

// cloudIntegrationListItemModel represents a single item in the integrations list.
type cloudIntegrationListItemModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	SpaceID      types.String `tfsdk:"space_id"`
	ProviderType types.String `tfsdk:"provider_type"`
	Status       types.String `tfsdk:"status"`
}

// mapCloudIntegrationToDataSource converts an API CloudIntegration to the singular data source model.
func mapCloudIntegrationToDataSource(ci *zenfraclient.CloudIntegration) cloudIntegrationDataSourceModel {
	model := cloudIntegrationDataSourceModel{
		ID:              types.StringValue(ci.ID),
		OrganizationID:  types.StringValue(ci.OrganizationID),
		SpaceID:         types.StringValue(ci.SpaceID),
		Name:            types.StringValue(ci.Name),
		ProviderType:    types.StringValue(ci.Provider),
		Status:          types.StringValue(ci.Status),
		AutoAttachLabel: types.StringValue(ci.AutoAttachLabel),
		CreatedAt:       types.StringValue(ci.CreatedAt),
		UpdatedAt:       types.StringValue(ci.UpdatedAt),
	}

	if ci.AWS != nil {
		model.AWS = &cloudAWSConfigModel{
			RoleARN:          types.StringValue(ci.AWS.RoleARN),
			SessionDuration:  types.Int64Value(int64(ci.AWS.SessionDuration)),
			Region:           types.StringValue(ci.AWS.Region),
			GenerateOnWorker: types.BoolValue(ci.AWS.GenerateOnWorker),
		}
	}

	if ci.LastVerifiedAt != nil {
		model.LastVerifiedAt = types.StringValue(*ci.LastVerifiedAt)
	} else {
		model.LastVerifiedAt = types.StringNull()
	}

	return model
}
