// ABOUTME: Terraform state model for the zenfra_api_token resource.
// ABOUTME: Includes write-once token that is only populated on creation.
package api_token

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// APITokenModel represents the Terraform state model for a Zenfra API token.
type APITokenModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Token       types.String `tfsdk:"token"`
	CreatedAt   types.String `tfsdk:"created_at"`
	ExpiresAt   types.String `tfsdk:"expires_at"`
	Active      types.Bool   `tfsdk:"active"`
}

// mapTokenToState converts an API Token response to an APITokenModel.
// Note: Does NOT set the Token field - it's only available at creation time.
func mapTokenToState(token *zenfraclient.Token) APITokenModel {
	model := APITokenModel{
		ID:        types.StringValue(token.ID),
		Name:      types.StringValue(token.Name),
		Active:    types.BoolValue(token.Active),
		CreatedAt: types.StringValue(token.CreatedAt.Format("2006-01-02T15:04:05Z07:00")),
		ExpiresAt: types.StringValue(token.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")),
	}

	if token.Description != "" {
		model.Description = types.StringValue(token.Description)
	} else {
		model.Description = types.StringNull()
	}

	return model
}
