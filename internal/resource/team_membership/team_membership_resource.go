// ABOUTME: Implements the zenfra_team_membership Terraform resource: one user in one team.
// ABOUTME: Both ids force replacement; the id and the import id are team_id/user_id.
package team_membership

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
	_ resource.Resource                = &TeamMembershipResource{}
	_ resource.ResourceWithImportState = &TeamMembershipResource{}
)

// NewTeamMembershipResource is a constructor for the team membership resource.
func NewTeamMembershipResource() resource.Resource {
	return &TeamMembershipResource{}
}

// TeamMembershipResource is the resource implementation.
type TeamMembershipResource struct {
	client *zenfraclient.Client
}

// TeamMembershipModel is the Terraform state of a membership.
type TeamMembershipModel struct {
	ID     types.String `tfsdk:"id"`
	TeamID types.String `tfsdk:"team_id"`
	UserID types.String `tfsdk:"user_id"`
}

func membershipID(teamID, userID string) string { return teamID + "/" + userID }

func (r *TeamMembershipResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_membership"
}

func (r *TeamMembershipResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "Adds a user of the organization to a team. The user must be an active member of the " +
			"organization; a user can belong to several teams. Requires an Admin token. Available after the " +
			"organization's access-boundary activation; until then the API refuses the write with " +
			"`access_boundaries_inactive`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite identifier in the format team_id/user_id.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"team_id": schema.StringAttribute{
				Description:   "The team. Changing it replaces the membership.",
				Required:      true,
				PlanModifiers: replace,
			},
			"user_id": schema.StringAttribute{
				Description:   "The user. Changing it replaces the membership.",
				Required:      true,
				PlanModifiers: replace,
			},
		},
	}
}

func (r *TeamMembershipResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TeamMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TeamMembershipModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	m, err := r.client.AddTeamMember(ctx, plan.TeamID.ValueString(), plan.UserID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Team Membership", fmt.Sprintf("Could not add the user to the team: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, TeamMembershipModel{
		ID: types.StringValue(membershipID(m.TeamID, m.UserID)), TeamID: types.StringValue(m.TeamID), UserID: types.StringValue(m.UserID),
	})...)
}

// Read finds the user in the team's member list; a missing team or member
// removes the resource from state.
func (r *TeamMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TeamMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	members, err := r.client.ListTeamMembers(ctx, state.TeamID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading Team Membership",
			fmt.Sprintf("Could not read the members of team ID %s: %s", state.TeamID.ValueString(), err))
		return
	}
	for _, m := range members {
		if m.UserID == state.UserID.ValueString() {
			state.ID = types.StringValue(membershipID(m.TeamID, m.UserID))
			resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

// Update is never called with a change: both attributes force replacement.
func (r *TeamMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan TeamMembershipModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *TeamMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state TeamMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.RemoveTeamMember(ctx, state.TeamID.ValueString(), state.UserID.ValueString())
	if err != nil && !zenfraclient.IsNotFound(err) {
		resp.Diagnostics.AddError("Error Deleting Team Membership",
			fmt.Sprintf("Could not remove the user from the team: %s", err))
	}
}

// ImportState imports by team_id/user_id.
func (r *TeamMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid Import ID", fmt.Sprintf("Expected format: team_id/user_id, got: %s", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, TeamMembershipModel{
		ID: types.StringValue(req.ID), TeamID: types.StringValue(parts[0]), UserID: types.StringValue(parts[1]),
	})...)
}
