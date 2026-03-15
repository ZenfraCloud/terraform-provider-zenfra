// ABOUTME: Terraform state model for the zenfra_cloud_integration_attachment resource.
// ABOUTME: Maps between the API CloudAttachment type and the Terraform state representation.
package cloud_integration_attachment

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// CloudIntegrationAttachmentModel represents the Terraform state model for a cloud integration attachment.
type CloudIntegrationAttachmentModel struct {
	ID             types.String `tfsdk:"id"`
	IntegrationID  types.String `tfsdk:"integration_id"`
	StackID        types.String `tfsdk:"stack_id"`
	Read           types.Bool   `tfsdk:"read"`
	Write          types.Bool   `tfsdk:"write"`
	IsAutoAttached types.Bool   `tfsdk:"is_auto_attached"`
	CreatedAt      types.String `tfsdk:"created_at"`
	ExternalID     types.String `tfsdk:"external_id"`
}

// fromAPI populates the model from an API CloudAttachment response.
func (m *CloudIntegrationAttachmentModel) fromAPI(a *zenfraclient.CloudAttachment) {
	m.ID = types.StringValue(a.ID)
	m.IntegrationID = types.StringValue(a.IntegrationID)
	m.StackID = types.StringValue(a.StackID)
	m.Read = types.BoolValue(a.Read)
	m.Write = types.BoolValue(a.Write)
	m.IsAutoAttached = types.BoolValue(a.IsAutoAttached)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	m.ExternalID = types.StringValue(a.ExternalID)
}
