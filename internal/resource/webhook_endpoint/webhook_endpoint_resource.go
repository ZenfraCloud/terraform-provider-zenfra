// ABOUTME: Implements the zenfra_webhook_endpoint Terraform resource: outbound webhook endpoints (Admin role).
// ABOUTME: The secret is kept from the create response; enabled = true re-enables an auto-disabled endpoint on apply.
package webhook_endpoint

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/labelset"
	"github.com/zenfra/terraform-provider-zenfra/internal/validate"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

var (
	_ resource.Resource                = &WebhookEndpointResource{}
	_ resource.ResourceWithImportState = &WebhookEndpointResource{}
)

// NewWebhookEndpointResource is a constructor for the webhook endpoint resource.
func NewWebhookEndpointResource() resource.Resource {
	return &WebhookEndpointResource{}
}

// WebhookEndpointResource is the resource implementation.
type WebhookEndpointResource struct {
	client *zenfraclient.Client
}

func (r *WebhookEndpointResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook_endpoint"
}

func (r *WebhookEndpointResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an outbound webhook endpoint: Zenfra sends signed HTTP POST requests to `url` for the " +
			"subscribed run events. Requires an Admin token. The signing secret is returned only when the endpoint " +
			"is created and is kept in state from then on; an imported endpoint has no secret in state. Rotating the " +
			"secret is done in the dashboard or the API, not here. Zenfra disables an endpoint automatically after " +
			"repeated delivery failures; while `enabled = true` (the default), the next `terraform apply` shows " +
			"`enabled` changing from `false` to `true` and re-enables it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the webhook endpoint.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"url": schema.StringAttribute{
				Description: "The HTTPS URL deliveries are sent to. It must resolve to a public address; the API " +
					"refuses private, loopback and link-local addresses and a host that does not resolve.",
				Required: true,
			},
			"description": schema.StringAttribute{
				Description: "A description of the endpoint, at most 200 characters.",
				Optional:    true,
			},
			"event_types": schema.SetAttribute{
				Description: "The event types delivered to the endpoint, at least one: `run.<status>` for a run " +
					"status (for example `run.failed`, `run.finished`, `run.awaiting_approval`) and " +
					"`run.policy_reevaluated`. The API refuses an unknown type.",
				Required:    true,
				ElementType: types.StringType,
				Validators:  []validator.Set{validate.NonEmptySet()},
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether deliveries are sent. Defaults to `true`. An endpoint Zenfra disabled after " +
					"repeated delivery failures reads back as `false`, so an apply with `enabled = true` re-enables it.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"secret": schema.StringAttribute{
				Description: "The signing secret (`whsec_...`) used to verify deliveries. Set only when Terraform " +
					"creates the endpoint; null after an import. A rotation in the dashboard or API is not reflected here.",
				Computed:  true,
				Sensitive: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"disabled_reason": schema.StringAttribute{
				Description: "Why the endpoint is disabled: `manual` or `auto_failures`. Null when enabled.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					disabledReasonPlan{},
				},
			},
		},
	}
}

func (r *WebhookEndpointResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*zenfraclient.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *zenfraclient.Client, got: %T.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *WebhookEndpointResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan WebhookEndpointModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	eventTypes, diags := labelset.ToAPI(ctx, plan.EventTypes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	createReq := zenfraclient.CreateWebhookEndpointRequest{
		URL:         plan.URL.ValueString(),
		Description: plan.Description.ValueString(),
		EventTypes:  eventTypes,
	}
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() {
		enabled := plan.Enabled.ValueBool()
		createReq.Enabled = &enabled
	}

	created, err := r.client.CreateWebhookEndpoint(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Webhook Endpoint", fmt.Sprintf("Could not create webhook endpoint: %s", err))
		return
	}

	state, diags := mapEndpointToState(&created.WebhookEndpoint, plan.Description, types.StringValue(created.Secret))
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *WebhookEndpointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state WebhookEndpointModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ep, err := r.client.GetWebhookEndpoint(ctx, state.ID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading Webhook Endpoint",
			fmt.Sprintf("Could not read webhook endpoint ID %s: %s", state.ID.ValueString(), err))
		return
	}

	newState, diags := mapEndpointToState(ep, state.Description, state.Secret)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *WebhookEndpointResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state WebhookEndpointModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	patch, changed, diags := buildPatch(ctx, plan, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	var ep *zenfraclient.WebhookEndpoint
	var err error
	if changed {
		ep, err = r.client.PatchWebhookEndpoint(ctx, id, patch)
	} else {
		ep, err = r.client.GetWebhookEndpoint(ctx, id)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Webhook Endpoint",
			fmt.Sprintf("Could not update webhook endpoint ID %s: %s", id, err))
		return
	}

	newState, diags := mapEndpointToState(ep, plan.Description, state.Secret)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

// buildPatch returns the leaf patch for the fields the plan changes and
// whether there is any. Unchanged fields stay out of it: a resent URL would
// be checked (resolved) by the API again for nothing.
func buildPatch(ctx context.Context, plan, state WebhookEndpointModel) (zenfraclient.PatchWebhookEndpointRequest, bool, diag.Diagnostics) {
	var patch zenfraclient.PatchWebhookEndpointRequest
	changed := false
	if !plan.URL.Equal(state.URL) {
		u := plan.URL.ValueString()
		patch.URL, changed = &u, true
	}
	if plan.Description.ValueString() != state.Description.ValueString() {
		// A null plan clears the description: the API stores "".
		d := plan.Description.ValueString()
		patch.Description, changed = &d, true
	}
	if !plan.EventTypes.Equal(state.EventTypes) {
		eventTypes, diags := labelset.ToAPI(ctx, plan.EventTypes)
		if diags.HasError() {
			return patch, false, diags
		}
		patch.EventTypes, changed = &eventTypes, true
	}
	if !plan.Enabled.IsUnknown() && !plan.Enabled.Equal(state.Enabled) {
		// This is also how an endpoint auto-disabled after delivery failures is
		// re-enabled: refresh reads enabled = false and the config says true.
		e := plan.Enabled.ValueBool()
		patch.Enabled, changed = &e, true
	}
	return patch, changed, nil
}

func (r *WebhookEndpointResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state WebhookEndpointModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteWebhookEndpoint(ctx, state.ID.ValueString()); err != nil {
		if zenfraclient.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error Deleting Webhook Endpoint",
			fmt.Sprintf("Could not delete webhook endpoint ID %s: %s", state.ID.ValueString(), err))
	}
}

// ImportState imports by endpoint ID. The secret is not readable after
// creation, so an imported endpoint's secret stays null.
func (r *WebhookEndpointResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// disabledReasonPlan predicts disabled_reason from the planned enablement, so
// an update that does not touch enabled does not show it as unknown:
// enabling clears it (the API omits it on an enabled endpoint), an endpoint
// that stays disabled keeps its reason, and disabling an enabled endpoint
// leaves it unknown for the API to decide.
type disabledReasonPlan struct{}

func (disabledReasonPlan) Description(_ context.Context) string {
	return "Null while enabled; unchanged while the endpoint stays disabled."
}

func (p disabledReasonPlan) MarkdownDescription(ctx context.Context) string {
	return p.Description(ctx)
}

func (disabledReasonPlan) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.Plan.Raw.IsNull() {
		return // destroy
	}
	var planned types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("enabled"), &planned)...)
	if resp.Diagnostics.HasError() || planned.IsNull() || planned.IsUnknown() {
		return
	}
	if planned.ValueBool() {
		resp.PlanValue = types.StringNull()
		return
	}
	if req.State.Raw.IsNull() {
		return // a disabled create: the API sets the reason
	}
	var prior types.Bool
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("enabled"), &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !prior.IsNull() && !prior.IsUnknown() && !prior.ValueBool() {
		resp.PlanValue = req.StateValue
	}
}
