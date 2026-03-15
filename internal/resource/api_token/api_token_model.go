// ABOUTME: Terraform state model for the zenfra_api_token resource.
// ABOUTME: Includes write-once token that is only populated on creation.
package api_token

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// APITokenModel represents the Terraform state model for a Zenfra API token.
type APITokenModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Role         types.String `tfsdk:"role"`
	ExpiresInDays types.Int64  `tfsdk:"expires_in_days"`
	Token        types.String `tfsdk:"token"`
	TokenPrefix  types.String `tfsdk:"token_prefix"`
	UsageCount   types.Int64  `tfsdk:"usage_count"`
	LastUsedAt   types.String `tfsdk:"last_used_at"`
	CreatedAt    types.String `tfsdk:"created_at"`
	ExpiresAt    types.String `tfsdk:"expires_at"`
	Active       types.Bool   `tfsdk:"active"`
}

// mapTokenToState converts an API Token response to an APITokenModel.
// Note: Does NOT set Token or ExpiresInDays fields - Token is only available at creation time,
// and ExpiresInDays is a write-only input parameter not returned by the API.
func mapTokenToState(token *zenfraclient.Token) APITokenModel {
	model := APITokenModel{
		ID:          types.StringValue(token.ID),
		Name:        types.StringValue(token.Name),
		Role:        types.StringValue(token.Role),
		TokenPrefix: types.StringValue(token.TokenPrefix),
		UsageCount:  types.Int64Value(token.UsageCount),
		Active:      types.BoolValue(token.Active),
		CreatedAt:   types.StringValue(token.CreatedAt.Format("2006-01-02T15:04:05Z07:00")),
		ExpiresAt:   types.StringValue(token.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")),
	}

	if token.Description != "" {
		model.Description = types.StringValue(token.Description)
	} else {
		model.Description = types.StringNull()
	}

	if token.LastUsedAt != nil {
		model.LastUsedAt = types.StringValue(token.LastUsedAt.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		model.LastUsedAt = types.StringNull()
	}

	return model
}
