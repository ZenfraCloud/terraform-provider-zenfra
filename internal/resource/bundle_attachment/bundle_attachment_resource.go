// ABOUTME: Implements the zenfra_bundle_attachment Terraform resource for attaching bundles to stacks.
// ABOUTME: Composite ID "stack_id:bundle_id", ForceNew on both IDs; priority updates in place.
package bundle_attachment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/validate"
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
		Description: "Attaches a configuration bundle to a stack explicitly. A bundle can also reach a stack by label " +
			"(its auto_attach_labels); that needs no attachment resource, and this resource only manages the explicit one.",
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
			"priority": schema.Int64Attribute{
				Description: "Order in which the stack's explicitly attached bundles apply: ascending, ties by bundle ID, so " +
					"on a conflicting environment variable or file the higher priority wins. Zero or greater; a new " +
					"attachment starts at 0. Changed in place. Omitting it leaves the current priority unmanaged.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
				Validators: []validator.Int64{validate.NonNegative()},
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

	// The attachment exists from here on: record it before setting the
	// priority, so a failure below leaves a tainted resource rather than an
	// attachment Terraform does not know about.
	state := BundleAttachmentModel{
		ID:       types.StringValue(stackID + ":" + bundleID),
		StackID:  plan.StackID,
		BundleID: plan.BundleID,
		Priority: types.Int64Value(0),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A new attachment starts at priority 0, so only another value is sent.
	if !plan.Priority.IsUnknown() && !plan.Priority.IsNull() && plan.Priority.ValueInt64() != 0 {
		if err := r.setPriority(ctx, stackID, bundleID, plan.Priority.ValueInt64()); err != nil {
			resp.Diagnostics.AddError("Error Setting Bundle Attachment Priority", err.Error())
			return
		}
	}

	r.readInto(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *BundleAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BundleAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	stackID := state.StackID.ValueString()
	bundleID := state.BundleID.ValueString()

	list, err := r.client.ListStackBundles(ctx, stackID)
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading Bundle Attachment", fmt.Sprintf("Could not list bundles for stack %s: %s", stackID, err))
		return
	}

	// Only an explicit attachment is this resource's. A bundle that still
	// reaches the stack by label (auto_attached) is not: the attachment is gone.
	att := list.ExplicitAttachment(bundleID)
	if att == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	state.Priority = priorityOf(att)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update changes the priority in place; every other attribute forces a new
// attachment.
func (r *BundleAttachmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state BundleAttachmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	stackID, bundleID := state.StackID.ValueString(), state.BundleID.ValueString()
	if !plan.Priority.IsUnknown() && !plan.Priority.IsNull() && !plan.Priority.Equal(state.Priority) {
		if err := r.setPriority(ctx, stackID, bundleID, plan.Priority.ValueInt64()); err != nil {
			resp.Diagnostics.AddError("Error Updating Bundle Attachment Priority", err.Error())
			return
		}
	}

	r.readInto(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *BundleAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BundleAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DetachBundle(ctx, state.StackID.ValueString(), state.BundleID.ValueString())
	if err != nil {
		// 404: nothing attached. 409 auto_attached: the explicit attachment is
		// already gone and the bundle reaches the stack only by label, which
		// this resource never managed. Either way there is nothing to detach.
		if zenfraclient.IsNotFound(err) || zenfraclient.IsAutoAttached(err) {
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

// setPriority PATCHes the priority and explains the failures a user can act on.
func (r *BundleAttachmentResource) setPriority(ctx context.Context, stackID, bundleID string, priority int64) error {
	err := r.client.UpdateBundlePriority(ctx, stackID, bundleID, int(priority))
	switch {
	case err == nil:
		return nil
	case zenfraclient.IsAutoAttached(err):
		return fmt.Errorf("bundle %s has no explicit attachment to stack %s any more; it reaches the stack only by "+
			"label, and a label match has no priority of its own: %w", bundleID, stackID, err)
	case priority == 0 && isValidation(err):
		return fmt.Errorf("the API refused to set priority 0 on bundle %s for stack %s (an API older than this provider); "+
			"a new attachment starts at 0, so replace the attachment (terraform apply -replace) to return it to 0: %w", bundleID, stackID, err)
	default:
		return fmt.Errorf("could not set the priority of bundle %s on stack %s: %w", bundleID, stackID, err)
	}
}

// readInto refreshes state's priority from the stack's explicit attachments.
func (r *BundleAttachmentResource) readInto(ctx context.Context, state *BundleAttachmentModel, diags *diag.Diagnostics) {
	stackID, bundleID := state.StackID.ValueString(), state.BundleID.ValueString()
	list, err := r.client.ListStackBundles(ctx, stackID)
	if err != nil {
		diags.AddError("Error Reading Bundle Attachment", fmt.Sprintf("Could not list bundles for stack %s: %s", stackID, err))
		return
	}
	att := list.ExplicitAttachment(bundleID)
	if att == nil {
		diags.AddError("Bundle Attachment Missing",
			fmt.Sprintf("Bundle %s is not explicitly attached to stack %s after the write.", bundleID, stackID))
		return
	}
	state.Priority = priorityOf(att)
}

// priorityOf is an explicit attachment's priority; the API always sends one
// on an explicit row, and 0 is its default.
func priorityOf(att *zenfraclient.BundleAttachment) types.Int64 {
	if att.Priority == nil {
		return types.Int64Value(0)
	}
	return types.Int64Value(int64(*att.Priority))
}

func isValidation(err error) bool {
	var ve *zenfraclient.ValidationError
	return errors.As(err, &ve)
}
