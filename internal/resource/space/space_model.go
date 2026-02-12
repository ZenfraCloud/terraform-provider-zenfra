// ABOUTME: Terraform state model for the zenfra_space resource.
// ABOUTME: Maps between API Space struct and Terraform schema types.
package space

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// SpaceModel represents the Terraform state model for a Zenfra space.
type SpaceModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	ParentSpaceID  types.String `tfsdk:"parent_space_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

// mapAPISpaceToModel converts an API Space response to a SpaceModel for Terraform state.
func mapAPISpaceToModel(space *zenfraclient.Space) SpaceModel {
	model := SpaceModel{
		ID:             types.StringValue(space.ID),
		OrganizationID: types.StringValue(space.OrganizationID),
		Name:           types.StringValue(space.Name),
		CreatedAt:      types.StringValue(space.CreatedAt.Format("2006-01-02T15:04:05Z07:00")),
		UpdatedAt:      types.StringValue(space.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")),
	}

	if space.Description != "" {
		model.Description = types.StringValue(space.Description)
	} else {
		model.Description = types.StringNull()
	}

	if space.ParentID != nil && *space.ParentID != "" {
		model.ParentSpaceID = types.StringValue(*space.ParentID)
	} else {
		model.ParentSpaceID = types.StringNull()
	}

	return model
}
