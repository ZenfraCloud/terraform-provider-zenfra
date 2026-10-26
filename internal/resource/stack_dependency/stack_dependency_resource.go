// ABOUTME: Implements the zenfra_stack_dependency Terraform resource: after the parent stack applies, start a run of the child.
// ABOUTME: Every call goes through the child stack; enabled is the only in-place change and a new edge is always created enabled.
package stack_dependency

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

var (
	_ resource.Resource                = &StackDependencyResource{}
	_ resource.ResourceWithImportState = &StackDependencyResource{}
	_ resource.ResourceWithModifyPlan  = &StackDependencyResource{}
)

// NewStackDependencyResource is a constructor for the stack dependency resource.
func NewStackDependencyResource() resource.Resource {
	return &StackDependencyResource{}
}

// StackDependencyResource is the resource implementation.
type StackDependencyResource struct {
	client *zenfraclient.Client
}

const disabledCreateDetail = "A new stack dependency is always created enabled. Create it with `enabled = true` " +
	"(or omit `enabled`) and disable it in a later apply."

func (r *StackDependencyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stack_dependency"
}

func (r *StackDependencyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a dependency between two stacks: after each successful tracked apply of the parent " +
			"stack, Zenfra starts a run of the child stack. Changing the parent or the child replaces the dependency. " +
			"The API refuses a dependency on the stack itself, a duplicate, one that would close a cycle, and one " +
			"past the depth or fan-out limits; the refusal is reported at apply. Import with " +
			"`<child_stack_id>/<dependency_id>`: a dependency is read through its child stack, so the child's id " +
			"is part of the import id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the dependency.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"parent_stack_id": schema.StringAttribute{
				Description: "The stack whose successful apply starts a run of the child. Changing it replaces the dependency.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"child_stack_id": schema.StringAttribute{
				Description: "The stack that runs after the parent applies. Changing it replaces the dependency.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the dependency starts runs of the child. Defaults to `true`. A new dependency " +
					"is always created enabled, so `false` is refused when the dependency is created or replaced: " +
					"create it enabled and set `enabled = false` in a later apply.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"activated_at": schema.StringAttribute{
				Description: "When the dependency was last enabled (RFC 3339), set at creation and at every enable.",
				Computed:    true,
			},
		},
	}
}

func (r *StackDependencyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ModifyPlan refuses a create or a replacement with enabled = false: the API
// creates enabled edges only, and creating then disabling would leave the
// edge live and enabled for an unbounded window. An update to false passes.
func (r *StackDependencyResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return // destroy, never refused
	}
	var plan StackDependencyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || plan.Enabled.IsUnknown() || plan.Enabled.IsNull() || plan.Enabled.ValueBool() {
		return
	}
	if !req.State.Raw.IsNull() {
		var state StackDependencyModel
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
		if resp.Diagnostics.HasError() {
			return
		}
		// Equal is false for an unknown planned value: a possible replacement.
		if plan.ParentStackID.Equal(state.ParentStackID) && plan.ChildStackID.Equal(state.ChildStackID) {
			return
		}
	}
	resp.Diagnostics.AddAttributeError(path.Root("enabled"), "Stack Dependency Cannot Be Created Disabled", disabledCreateDetail)
}

func (r *StackDependencyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan StackDependencyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// ModifyPlan cannot see a value known only at apply.
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() && !plan.Enabled.ValueBool() {
		resp.Diagnostics.AddAttributeError(path.Root("enabled"), "Stack Dependency Cannot Be Created Disabled", disabledCreateDetail)
		return
	}

	childID := plan.ChildStackID.ValueString()
	edge, err := r.client.CreateStackDependency(ctx, childID, plan.ParentStackID.ValueString())
	if err != nil {
		addWriteError(&resp.Diagnostics, "Error Creating Stack Dependency", "create", err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, toState(edge, plan.ParentStackID, childID))...)
}

func (r *StackDependencyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state StackDependencyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	childID, id := state.ChildStackID.ValueString(), state.ID.ValueString()
	list, err := r.client.ListStackDependencies(ctx, childID)
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading Stack Dependency",
			fmt.Sprintf("Could not read stack dependency ID %s of stack %s: %s", id, childID, err))
		return
	}
	edge := findUpstream(list, id)
	if edge == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if edge.Stack == nil && state.ParentStackID.IsNull() {
		// Only an import has no parent yet; a refresh keeps the one it has.
		resp.Diagnostics.AddError("Cannot Import Stack Dependency",
			fmt.Sprintf("The API token cannot read the parent stack of stack dependency %s, so its parent_stack_id "+
				"is unknown. Import it with a token that can read both stacks.", id))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, toState(edge, state.ParentStackID, childID))...)
}

// findUpstream returns the child's upstream edge with the given id, or nil.
// A downstream edge is never this resource: its stack is the child's child.
func findUpstream(list *zenfraclient.StackDependencyList, id string) *zenfraclient.StackDependency {
	for i := range list.Upstream {
		if list.Upstream[i].ID == id {
			return &list.Upstream[i]
		}
	}
	return nil
}

// Update changes enabled, the only in-place attribute. On an error the prior
// state stays (the framework pre-fills it), so a refused enable is offered
// again by the next plan.
func (r *StackDependencyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state StackDependencyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.Enabled.IsUnknown() || plan.Enabled.Equal(state.Enabled) {
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
		return
	}

	childID := state.ChildStackID.ValueString()
	edge, err := r.client.SetStackDependencyEnabled(ctx, childID, state.ID.ValueString(), plan.Enabled.ValueBool())
	if err != nil {
		addWriteError(&resp.Diagnostics, "Error Updating Stack Dependency", "update", err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, toState(edge, state.ParentStackID, childID))...)
}

// addWriteError reports a dependency refusal (a 409 with a code, such as
// dependency_cycle) as such and any other error generically.
func addWriteError(diags *diag.Diagnostics, summary, verb string, err error) {
	var ce *zenfraclient.ConflictError
	if errors.As(err, &ce) && ce.Code != "" {
		diags.AddError("Stack Dependency Refused", ce.Code+": "+ce.Message)
		return
	}
	diags.AddError(summary, fmt.Sprintf("Could not %s stack dependency: %s", verb, err))
}

func (r *StackDependencyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state StackDependencyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteStackDependency(ctx, state.ChildStackID.ValueString(), state.ID.ValueString())
	if err != nil && !zenfraclient.IsNotFound(err) {
		resp.Diagnostics.AddError("Error Deleting Stack Dependency",
			fmt.Sprintf("Could not delete stack dependency ID %s: %s", state.ID.ValueString(), err))
	}
}

// ImportState imports by child_stack_id/dependency_id, not by the bare id:
// there is no single-dependency read and an edge does not name its child, so
// the child comes with the import id. Read then fills the rest and refuses an
// edge whose parent the token cannot read.
func (r *StackDependencyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid Import ID",
			fmt.Sprintf("Expected format: child_stack_id/dependency_id, got: %s", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, StackDependencyModel{
		ID:            types.StringValue(parts[1]),
		ParentStackID: types.StringNull(),
		ChildStackID:  types.StringValue(parts[0]),
		Enabled:       types.BoolNull(),
		ActivatedAt:   types.StringNull(),
	})...)
}
