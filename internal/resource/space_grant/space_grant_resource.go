// ABOUTME: Implements the zenfra_space_grant Terraform resource: a team's read or write on a space and its subtree.
// ABOUTME: space_id and team_id force replacement, role changes in place; the id and import id are space_id/team_id.
package space_grant

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
	_ resource.Resource                = &SpaceGrantResource{}
	_ resource.ResourceWithImportState = &SpaceGrantResource{}
)

// NewSpaceGrantResource is a constructor for the space grant resource.
func NewSpaceGrantResource() resource.Resource {
	return &SpaceGrantResource{}
}

// SpaceGrantResource is the resource implementation.
type SpaceGrantResource struct {
	client *zenfraclient.Client
}

// SpaceGrantModel is the Terraform state of a grant.
type SpaceGrantModel struct {
	ID      types.String `tfsdk:"id"`
	SpaceID types.String `tfsdk:"space_id"`
	TeamID  types.String `tfsdk:"team_id"`
	Role    types.String `tfsdk:"role"`
}

func toState(g *zenfraclient.SpaceGrant) SpaceGrantModel {
	return SpaceGrantModel{
		ID:      types.StringValue(g.SpaceID + "/" + g.TeamID),
		SpaceID: types.StringValue(g.SpaceID),
		TeamID:  types.StringValue(g.TeamID),
		Role:    types.StringValue(g.Role),
	}
}

func (r *SpaceGrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_space_grant"
}

func (r *SpaceGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "Gives a team `read` or `write` on a space and every space below it. A grant on the root " +
			"space applies to the whole organization. Grants add up (the highest one wins) and there are no deny " +
			"rules. A `write` grant covers stacks, runs, bundles and spaces in the subtree, never organization " +
			"administration (users, teams, API keys of others, integrations, policies, webhooks, state rollback, " +
			"force-unlock). One grant per space and team. Requires an Admin token. Available after the " +
			"organization's access-boundary activation; until then the API refuses the write with " +
			"`access_boundaries_inactive`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite identifier in the format space_id/team_id.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"space_id": schema.StringAttribute{
				Description:   "The space. Changing it replaces the grant.",
				Required:      true,
				PlanModifiers: replace,
			},
			"team_id": schema.StringAttribute{
				Description:   "The team. Changing it replaces the grant.",
				Required:      true,
				PlanModifiers: replace,
			},
			"role": schema.StringAttribute{
				Description: "`read` or `write`. Changes in place.",
				Required:    true,
			},
		},
	}
}

func (r *SpaceGrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*zenfraclient.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *zenfraclient.Client, got: %T.", req.ProviderData))
		return
	}
	r.client = client
}

func (r *SpaceGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SpaceGrantModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.client.CreateSpaceGrant(ctx, plan.SpaceID.ValueString(), zenfraclient.CreateSpaceGrantRequest{
		TeamID: plan.TeamID.ValueString(), Role: plan.Role.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Space Grant", fmt.Sprintf("Could not create space grant: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, toState(g))...)
}

func (r *SpaceGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SpaceGrantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.client.GetSpaceGrant(ctx, state.SpaceID.ValueString(), state.TeamID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading Space Grant", fmt.Sprintf("Could not read space grant %s: %s", state.ID.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, toState(g))...)
}

func (r *SpaceGrantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SpaceGrantModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.client.UpdateSpaceGrant(ctx, plan.SpaceID.ValueString(), plan.TeamID.ValueString(), plan.Role.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Space Grant", fmt.Sprintf("Could not update space grant: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, toState(g))...)
}

func (r *SpaceGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SpaceGrantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteSpaceGrant(ctx, state.SpaceID.ValueString(), state.TeamID.ValueString())
	if err != nil && !zenfraclient.IsNotFound(err) {
		resp.Diagnostics.AddError("Error Deleting Space Grant", fmt.Sprintf("Could not delete space grant: %s", err))
	}
}

// ImportState imports by space_id/team_id.
func (r *SpaceGrantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid Import ID", fmt.Sprintf("Expected format: space_id/team_id, got: %s", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, SpaceGrantModel{
		ID: types.StringValue(req.ID), SpaceID: types.StringValue(parts[0]), TeamID: types.StringValue(parts[1]),
		Role: types.StringNull(),
	})...)
}
