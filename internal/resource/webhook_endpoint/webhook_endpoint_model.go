// ABOUTME: Terraform state model for the zenfra_webhook_endpoint resource and its API mapping.
// ABOUTME: The signing secret is never in an API read; the caller passes the value state already holds.
package webhook_endpoint

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/labelset"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// WebhookEndpointModel is the Terraform state of a webhook endpoint.
type WebhookEndpointModel struct {
	ID             types.String `tfsdk:"id"`
	URL            types.String `tfsdk:"url"`
	Description    types.String `tfsdk:"description"`
	EventTypes     types.Set    `tfsdk:"event_types"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	Secret         types.String `tfsdk:"secret"`
	DisabledReason types.String `tfsdk:"disabled_reason"`
}

// mapEndpointToState converts an API endpoint into state. The API answers ""
// for "no description"; that maps to null unless prior (the plan, or the
// state being refreshed) is a known "", so an omitted description and an
// explicit "" both read back as written. secret is the value to keep: the
// API never returns it after creation.
func mapEndpointToState(ep *zenfraclient.WebhookEndpoint, priorDescription, secret types.String) (WebhookEndpointModel, diag.Diagnostics) {
	eventTypes, diags := labelset.Computed(ep.EventTypes)
	m := WebhookEndpointModel{
		ID:             types.StringValue(ep.ID),
		URL:            types.StringValue(ep.URL),
		Description:    types.StringValue(ep.Description),
		EventTypes:     eventTypes,
		Enabled:        types.BoolValue(ep.Enabled),
		Secret:         secret,
		DisabledReason: types.StringNull(),
	}
	if ep.Description == "" && (priorDescription.IsNull() || priorDescription.IsUnknown() || priorDescription.ValueString() != "") {
		m.Description = types.StringNull()
	}
	if ep.DisabledReason != "" {
		m.DisabledReason = types.StringValue(ep.DisabledReason)
	}
	return m, diags
}
