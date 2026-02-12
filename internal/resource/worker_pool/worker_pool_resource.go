// ABOUTME: Implements the zenfra_worker_pool Terraform resource with full CRUD lifecycle.
// ABOUTME: Handles the write-once api_key that is only available at creation time.
package worker_pool

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &WorkerPoolResource{}
	_ resource.ResourceWithImportState = &WorkerPoolResource{}
)

// NewWorkerPoolResource is a helper function to simplify the provider implementation.
func NewWorkerPoolResource() resource.Resource {
	return &WorkerPoolResource{}
}

// WorkerPoolResource is the resource implementation.
type WorkerPoolResource struct {
	client *zenfraclient.Client
}

// Metadata returns the resource type name.
func (r *WorkerPoolResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_worker_pool"
}

// Schema defines the schema for the resource.
func (r *WorkerPoolResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Zenfra worker pool for executing infrastructure operations.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the worker pool.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				Description: "The organization ID this worker pool belongs to.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the worker pool.",
				Required:    true,
			},
			"api_key": schema.StringAttribute{
				Description: "The API key for workers to authenticate with this pool. This value is only available at creation time and cannot be retrieved later.",
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"api_key_id": schema.StringAttribute{
				Description: "The ID of the API key associated with this worker pool.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key_version": schema.Int64Attribute{
				Description: "The version of the API key.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"active": schema.BoolAttribute{
				Description: "Whether the worker pool is active.",
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"active_workers_count": schema.Int64Attribute{
				Description: "The number of active workers in the pool.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "Timestamp when the worker pool was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Description: "Timestamp when the worker pool was last updated.",
				Computed:    true,
			},
			"last_used_at": schema.StringAttribute{
				Description: "Timestamp when the worker pool was last used.",
				Computed:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *WorkerPoolResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *WorkerPoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan WorkerPoolModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the create request
	createReq := zenfraclient.CreateWorkerPoolRequest{
		Name: plan.Name.ValueString(),
	}

	// Create the worker pool
	createResp, err := r.client.CreateWorkerPool(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Worker Pool",
			fmt.Sprintf("Could not create worker pool: %s", err.Error()),
		)
		return
	}

	// Map response to state
	state := mapPoolToState(&createResp.Pool)

	// CRITICAL: Set the api_key from the create response - this is the ONLY time it's available
	state.APIKey = types.StringValue(createResp.APIKey)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *WorkerPoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state WorkerPoolModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the worker pool from the API
	pool, err := r.client.GetWorkerPool(ctx, state.ID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			// Worker pool no longer exists, remove from state
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Worker Pool",
			fmt.Sprintf("Could not read worker pool ID %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Map response to new state
	newState := mapPoolToState(pool)

	// CRITICAL: Preserve api_key from prior state since it's not returned by Read
	var existingAPIKey types.String
	diags = req.State.GetAttribute(ctx, path.Root("api_key"), &existingAPIKey)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	newState.APIKey = existingAPIKey

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *WorkerPoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state WorkerPoolModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the update request with only changed fields
	updateReq := zenfraclient.UpdateWorkerPoolRequest{}

	if !plan.Name.Equal(state.Name) {
		name := plan.Name.ValueString()
		updateReq.Name = &name
	}

	// Update the worker pool
	pool, err := r.client.UpdateWorkerPool(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Worker Pool",
			fmt.Sprintf("Could not update worker pool ID %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Map response to new state
	newState := mapPoolToState(pool)

	// CRITICAL: Preserve api_key from prior state
	newState.APIKey = state.APIKey

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *WorkerPoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state WorkerPoolModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the worker pool
	err := r.client.DeleteWorkerPool(ctx, state.ID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			// Worker pool already deleted, consider it a success
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting Worker Pool",
			fmt.Sprintf("Could not delete worker pool ID %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}
}

// ImportState imports the resource into Terraform state.
func (r *WorkerPoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Use the ID from the import to set the state
	// Note: api_key will be unknown/null after import since it's only available at creation
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
