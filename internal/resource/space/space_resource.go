// ABOUTME: Implements the zenfra_space Terraform resource with full CRUD lifecycle.
// ABOUTME: Manages Zenfra spaces for organizing stacks into logical groups.
package space

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &SpaceResource{}
	_ resource.ResourceWithImportState = &SpaceResource{}
)

// NewSpaceResource is a helper function to simplify the provider implementation.
func NewSpaceResource() resource.Resource {
	return &SpaceResource{}
}

// SpaceResource is the resource implementation.
type SpaceResource struct {
	client *zenfraclient.Client
}

// Metadata returns the resource type name.
func (r *SpaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_space"
}

// Schema defines the schema for the resource.
func (r *SpaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Zenfra space for organizing stacks into logical groups.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the space.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				Description: "The organization ID this space belongs to.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the space.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Optional description of the space.",
				Optional:    true,
			},
			"parent_space_id": schema.StringAttribute{
				Description: "Optional parent space ID for hierarchical organization.",
				Optional:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "Timestamp when the space was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Description: "Timestamp when the space was last updated.",
				Computed:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *SpaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*zenfraclient.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *zenfraclient.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

// Create creates the resource and sets the initial Terraform state.
func (r *SpaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SpaceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the create request
	createReq := zenfraclient.CreateSpaceRequest{
		Name: plan.Name.ValueString(),
		Slug: plan.Name.ValueString(), // Use name as slug for now
	}

	if !plan.Description.IsNull() {
		createReq.Description = plan.Description.ValueString()
	}

	if !plan.ParentSpaceID.IsNull() {
		parentID := plan.ParentSpaceID.ValueString()
		createReq.ParentID = &parentID
	}

	// Create the space
	space, err := r.client.CreateSpace(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Space",
			fmt.Sprintf("Could not create space: %s", err.Error()),
		)
		return
	}

	// Map response to state
	state := mapAPISpaceToModel(space)
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *SpaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SpaceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the space from the API
	space, err := r.client.GetSpace(ctx, state.ID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			// Space no longer exists, remove from state
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Space",
			fmt.Sprintf("Could not read space ID %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Map response to state
	newState := mapAPISpaceToModel(space)
	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *SpaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state SpaceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the update request with only changed fields
	updateReq := zenfraclient.UpdateSpaceRequest{}

	if !plan.Name.Equal(state.Name) {
		name := plan.Name.ValueString()
		updateReq.Name = &name
		slug := plan.Name.ValueString()
		updateReq.Slug = &slug
	}

	if !plan.Description.Equal(state.Description) {
		desc := plan.Description.ValueString()
		updateReq.Description = &desc
	}

	// Update the space
	space, err := r.client.UpdateSpace(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Space",
			fmt.Sprintf("Could not update space ID %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Map response to state
	newState := mapAPISpaceToModel(space)
	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *SpaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SpaceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the space
	err := r.client.DeleteSpace(ctx, state.ID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			// Space already deleted, consider it a success
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting Space",
			fmt.Sprintf("Could not delete space ID %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}
}

// ImportState imports the resource into Terraform state.
func (r *SpaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Use the ID from the import to set the state
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
