// ABOUTME: Implements the zenfra_team Terraform resource: a group of users that space grants are given to.
// ABOUTME: The slug forces replacement; writes need an Admin token and the access-boundary activation.
package team

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/validate"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

var (
	_ resource.Resource                = &TeamResource{}
	_ resource.ResourceWithImportState = &TeamResource{}
)

// NewTeamResource is a constructor for the team resource.
func NewTeamResource() resource.Resource {
	return &TeamResource{}
}

// TeamResource is the resource implementation.
type TeamResource struct {
	client *zenfraclient.Client
}

// TeamModel is the Terraform state of a team.
type TeamModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Slug        types.String `tfsdk:"slug"`
	Description types.String `tfsdk:"description"`
}

// toState converts an API team. The API answers "" for "no description";
// that maps to null unless prior is a known "", so an omitted description
// and an explicit "" both read back as written.
func toState(t *zenfraclient.Team, priorDescription types.String) TeamModel {
	m := TeamModel{
		ID:          types.StringValue(t.ID),
		Name:        types.StringValue(t.Name),
		Slug:        types.StringValue(t.Slug),
		Description: types.StringValue(t.Description),
	}
	if t.Description == "" && (priorDescription.IsNull() || priorDescription.IsUnknown() || priorDescription.ValueString() != "") {
		m.Description = types.StringNull()
	}
	return m
}

func (r *TeamResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

func (r *TeamResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a team: a group of users of the organization that space grants are given to " +
			"(`zenfra_space_grant`); members are `zenfra_team_membership`. Teams do not nest. Requires an Admin " +
			"token. Available after the organization's access-boundary activation; until then the API refuses " +
			"every team write with `access_boundaries_inactive`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the team.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The display name of the team: not blank, no leading or trailing whitespace, at most " +
					"128 characters.",
				Required:   true,
				Validators: []validator.String{validate.TeamName()},
			},
			"slug": schema.StringAttribute{
				Description: "The team's identifier in the organization: lowercase letters, numbers and hyphens, " +
					"starting and ending with a letter or number, at most 64 characters. Changing it replaces the team.",
				Required:   true,
				Validators: []validator.String{validate.TeamSlug()},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Description: "A description of the team, at most 2048 characters.",
				Optional:    true,
			},
		},
	}
}

func (r *TeamResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TeamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TeamModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	t, err := r.client.CreateTeam(ctx, zenfraclient.CreateTeamRequest{
		Name: plan.Name.ValueString(), Slug: plan.Slug.ValueString(), Description: plan.Description.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Team", fmt.Sprintf("Could not create team: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, toState(t, plan.Description))...)
}

func (r *TeamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TeamModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	t, err := r.client.GetTeam(ctx, state.ID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading Team",
			fmt.Sprintf("Could not read team ID %s: %s", state.ID.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, toState(t, state.Description))...)
}

func (r *TeamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state TeamModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var patch zenfraclient.UpdateTeamRequest
	if !plan.Name.Equal(state.Name) {
		n := plan.Name.ValueString()
		patch.Name = &n
	}
	if plan.Description.ValueString() != state.Description.ValueString() {
		// A null plan clears the description: the API stores "".
		d := plan.Description.ValueString()
		patch.Description = &d
	}
	id := state.ID.ValueString()
	var t *zenfraclient.Team
	var err error
	if patch.Name != nil || patch.Description != nil {
		t, err = r.client.UpdateTeam(ctx, id, patch)
	} else {
		t, err = r.client.GetTeam(ctx, id)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Team", fmt.Sprintf("Could not update team ID %s: %s", id, err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, toState(t, plan.Description))...)
}

func (r *TeamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state TeamModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteTeam(ctx, state.ID.ValueString()); err != nil && !zenfraclient.IsNotFound(err) {
		resp.Diagnostics.AddError("Error Deleting Team",
			fmt.Sprintf("Could not delete team ID %s: %s", state.ID.ValueString(), err))
	}
}

// ImportState imports by team ID.
func (r *TeamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
