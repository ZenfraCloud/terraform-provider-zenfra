// ABOUTME: Implements the zenfra_bundle_attachment Terraform resource for attaching bundles to stacks.
// ABOUTME: Uses composite ID "stack_id:bundle_id" and ForceNew semantics for both IDs.
package bundle_attachment

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

var (
	_ resource.Resource                = &BundleAttachmentResource{}
	_ resource.ResourceWithImportState = &BundleAttachmentResource{}
)

// NewBundleAttachmentResource is a constructor for the bundle attachment resource.
func NewBundleAttachmentResource() resource.Resource {
	return &BundleAttachmentResource{}
}

// BundleAttachmentResource is the resource implementation.
type BundleAttachmentResource struct {
	client *zenfraclient.Client
}

func (r *BundleAttachmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bundle_attachment"
}

func (r *BundleAttachmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Attaches a configuration bundle to a stack.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite identifier in the format stack_id:bundle_id.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"stack_id": schema.StringAttribute{
				Description: "The stack to attach the bundle to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"bundle_id": schema.StringAttribute{
				Description: "The bundle to attach.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *BundleAttachmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *BundleAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BundleAttachmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	stackID := plan.StackID.ValueString()
	bundleID := plan.BundleID.ValueString()

	err := r.client.AttachBundle(ctx, stackID, bundleID)
	if err != nil {
		resp.Diagnostics.AddError("Error Attaching Bundle", fmt.Sprintf("Could not attach bundle %s to stack %s: %s", bundleID, stackID, err))
		return
	}

	plan.ID = types.StringValue(stackID + ":" + bundleID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *BundleAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BundleAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	stackID := state.StackID.ValueString()
	bundleID := state.BundleID.ValueString()

	attachments, err := r.client.ListStackBundles(ctx, stackID)
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading Bundle Attachment", fmt.Sprintf("Could not list bundles for stack %s: %s", stackID, err))
		return
	}

	found := false
	for _, att := range attachments {
		if att.BundleID == bundleID {
			found = true
			break
		}
	}

	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *BundleAttachmentResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected Update", "Bundle attachment does not support in-place updates.")
}

func (r *BundleAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BundleAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DetachBundle(ctx, state.StackID.ValueString(), state.BundleID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error Detaching Bundle",
			fmt.Sprintf("Could not detach bundle %s from stack %s: %s", state.BundleID.ValueString(), state.StackID.ValueString(), err))
	}
}

func (r *BundleAttachmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected format: stack_id:bundle_id, got: %s", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, BundleAttachmentModel{
		ID:       types.StringValue(req.ID),
		StackID:  types.StringValue(parts[0]),
		BundleID: types.StringValue(parts[1]),
	})...)
}
