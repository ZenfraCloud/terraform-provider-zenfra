// ABOUTME: Implements the zenfra_stack Terraform resource with full CRUD lifecycle.
// ABOUTME: Manages stacks with nested source (raw_git/vcs), IAC config, and trigger configuration.
package stack

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &StackResource{}
	_ resource.ResourceWithImportState = &StackResource{}
)

// NewStackResource is a helper function to simplify the provider implementation.
func NewStackResource() resource.Resource {
	return &StackResource{}
}

// StackResource is the resource implementation.
type StackResource struct {
	client *zenfraclient.Client
}

// Metadata returns the resource type name.
func (r *StackResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stack"
}

// Schema defines the schema for the resource.
func (r *StackResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Zenfra stack with IaC configuration, source, and triggers.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the stack.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				Description: "The organization ID this stack belongs to.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"space_id": schema.StringAttribute{
				Description: "The space ID this stack belongs to.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the stack.",
				Required:    true,
			},
			"worker_pool_id": schema.StringAttribute{
				Description: "Optional worker pool ID for executing runs.",
				Optional:    true,
			},
			"allow_public_pool": schema.BoolAttribute{
				Description: "Whether to allow using the public worker pool. Defaults to false.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"iac": schema.SingleNestedAttribute{
				Description: "Infrastructure as Code configuration.",
				Required:    true,
				Attributes: map[string]schema.Attribute{
					"engine": schema.StringAttribute{
						Description: "IaC engine (e.g., 'terraform', 'opentofu').",
						Required:    true,
					},
					"version": schema.StringAttribute{
						Description: "IaC engine version.",
						Required:    true,
					},
				},
			},
			"source": schema.SingleNestedAttribute{
				Description: "Stack source configuration (raw_git or vcs).",
				Required:    true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Description: "Source type: 'raw_git' or 'vcs'.",
						Required:    true,
					},
					"raw_git": schema.SingleNestedAttribute{
						Description: "Raw HTTPS git source configuration.",
						Optional:    true,
						Attributes: map[string]schema.Attribute{
							"url": schema.StringAttribute{
								Description: "Git repository HTTPS URL.",
								Required:    true,
							},
							"ref": schema.SingleNestedAttribute{
								Description: "Git reference (branch, tag, or commit).",
								Required:    true,
								Attributes: map[string]schema.Attribute{
									"type": schema.StringAttribute{
										Description: "Reference type: 'branch', 'tag', or 'commit'.",
										Required:    true,
									},
									"name": schema.StringAttribute{
										Description: "Reference name (branch name, tag name, or commit SHA).",
										Required:    true,
									},
								},
							},
							"path": schema.StringAttribute{
								Description: "Optional path within the repository.",
								Optional:    true,
							},
						},
					},
					"vcs": schema.SingleNestedAttribute{
						Description: "VCS integration-backed source configuration.",
						Optional:    true,
						Attributes: map[string]schema.Attribute{
							"provider": schema.StringAttribute{
								Description: "VCS provider: 'github' or 'gitlab'.",
								Required:    true,
							},
							"integration_id": schema.StringAttribute{
								Description: "VCS integration ID.",
								Required:    true,
							},
							"repository_id": schema.StringAttribute{
								Description: "Repository identifier.",
								Required:    true,
							},
							"ref": schema.SingleNestedAttribute{
								Description: "Git reference (branch, tag, or commit).",
								Required:    true,
								Attributes: map[string]schema.Attribute{
									"type": schema.StringAttribute{
										Description: "Reference type: 'branch', 'tag', or 'commit'.",
										Required:    true,
									},
									"name": schema.StringAttribute{
										Description: "Reference name (branch name, tag name, or commit SHA).",
										Required:    true,
									},
								},
							},
							"path": schema.StringAttribute{
								Description: "Optional path within the repository.",
								Optional:    true,
							},
						},
					},
				},
			},
			"triggers": schema.SingleNestedAttribute{
				Description: "Stack trigger configuration.",
				Optional:    true,
				Attributes: map[string]schema.Attribute{
					"on_push": schema.SingleNestedAttribute{
						Description: "Push-based trigger configuration.",
						Optional:    true,
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Description: "Whether push triggers are enabled.",
								Optional:    true,
							},
							"paths": schema.ListAttribute{
								Description: "Optional list of paths to watch for changes.",
								Optional:    true,
								ElementType: types.StringType,
							},
						},
					},
				},
			},
			"created_at": schema.StringAttribute{
				Description: "Timestamp when the stack was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Description: "Timestamp when the stack was last updated.",
				Computed:    true,
			},
			"created_by": schema.StringAttribute{
				Description: "User who created the stack.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_by": schema.StringAttribute{
				Description: "User who last updated the stack.",
				Computed:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *StackResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *StackResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan StackModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Extract IAC configuration
	var iacModel IACModel
	diags = plan.IAC.As(ctx, &iacModel, basetypes.ObjectAsOptions{})
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Extract source configuration
	var sourceModel SourceModel
	diags = plan.Source.As(ctx, &sourceModel, basetypes.ObjectAsOptions{})
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	source, diags := buildSourceFromModel(ctx, &sourceModel)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build create request
	createReq := zenfraclient.CreateStackRequest{
		SpaceID:         plan.SpaceID.ValueString(),
		Name:            plan.Name.ValueString(),
		AllowPublicPool: plan.AllowPublicPool.ValueBool(),
		IAC: zenfraclient.IACConfig{
			Engine:  iacModel.Engine.ValueString(),
			Version: iacModel.Version.ValueString(),
		},
		Source: *source,
	}

	if !plan.WorkerPoolID.IsNull() {
		poolID := plan.WorkerPoolID.ValueString()
		createReq.WorkerPoolID = &poolID
	}

	// Create the stack
	stack, err := r.client.CreateStack(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Stack",
			fmt.Sprintf("Could not create stack: %s", err.Error()),
		)
		return
	}

	// Set triggers if provided
	if !plan.Triggers.IsNull() {
		var triggersModel TriggersModel
		diags = plan.Triggers.As(ctx, &triggersModel, basetypes.ObjectAsOptions{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		triggers, diags := buildTriggersFromModel(ctx, &triggersModel)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		err = r.client.SetStackTriggers(ctx, stack.ID, *triggers)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Setting Stack Triggers",
				fmt.Sprintf("Could not set triggers for stack: %s", err.Error()),
			)
			return
		}
	}

	// Map response to state
	state, diags := mapStackToState(ctx, stack)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *StackResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state StackModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the stack from the API
	stack, err := r.client.GetStack(ctx, state.ID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			// Stack no longer exists, remove from state
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Stack",
			fmt.Sprintf("Could not read stack ID %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Map response to state
	newState, diags := mapStackToState(ctx, stack)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
//
//nolint:gocognit,gocyclo // Terraform CRUD with source, triggers, and base field updates
func (r *StackResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state StackModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check for source changes
	if !plan.Source.Equal(state.Source) {
		var sourceModel SourceModel
		diags = plan.Source.As(ctx, &sourceModel, basetypes.ObjectAsOptions{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		source, diags := buildSourceFromModel(ctx, &sourceModel)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		err := r.client.SetStackSource(ctx, state.ID.ValueString(), *source)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Stack Source",
				fmt.Sprintf("Could not update stack source: %s", err.Error()),
			)
			return
		}
	}

	// Check for trigger changes
	if !plan.Triggers.Equal(state.Triggers) {
		var triggersModel TriggersModel
		diags = plan.Triggers.As(ctx, &triggersModel, basetypes.ObjectAsOptions{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		triggers, diags := buildTriggersFromModel(ctx, &triggersModel)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		err := r.client.SetStackTriggers(ctx, state.ID.ValueString(), *triggers)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Stack Triggers",
				fmt.Sprintf("Could not update stack triggers: %s", err.Error()),
			)
			return
		}
	}

	// Build update request for changed base fields
	updateReq := zenfraclient.UpdateStackRequest{}
	hasChanges := false

	if !plan.Name.Equal(state.Name) {
		name := plan.Name.ValueString()
		updateReq.Name = &name
		hasChanges = true
	}

	if !plan.WorkerPoolID.Equal(state.WorkerPoolID) {
		if !plan.WorkerPoolID.IsNull() {
			poolID := plan.WorkerPoolID.ValueString()
			updateReq.WorkerPoolID = &poolID
		} else {
			// Set to nil to clear
			updateReq.WorkerPoolID = nil
		}
		hasChanges = true
	}

	if !plan.AllowPublicPool.Equal(state.AllowPublicPool) {
		allowPublic := plan.AllowPublicPool.ValueBool()
		updateReq.AllowPublicPool = &allowPublic
		hasChanges = true
	}

	if !plan.IAC.Equal(state.IAC) {
		var iacModel IACModel
		diags = plan.IAC.As(ctx, &iacModel, basetypes.ObjectAsOptions{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		iacConfig := zenfraclient.IACConfig{
			Engine:  iacModel.Engine.ValueString(),
			Version: iacModel.Version.ValueString(),
		}
		updateReq.IAC = &iacConfig
		hasChanges = true
	}

	// Update the stack if there are changes
	if hasChanges {
		_, err := r.client.UpdateStack(ctx, state.ID.ValueString(), updateReq)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Stack",
				fmt.Sprintf("Could not update stack ID %s: %s", state.ID.ValueString(), err.Error()),
			)
			return
		}
	}

	// Read back the updated stack
	stack, err := r.client.GetStack(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Updated Stack",
			fmt.Sprintf("Could not read stack ID %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Map response to state
	newState, diags := mapStackToState(ctx, stack)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *StackResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state StackModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the stack
	err := r.client.DeleteStack(ctx, state.ID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			// Stack already deleted, consider it a success
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting Stack",
			fmt.Sprintf("Could not delete stack ID %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}
}

// ImportState imports the resource into Terraform state.
func (r *StackResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Use the ID from the import to set the state
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapStackToState converts an API Stack response to a StackModel for Terraform state.
func mapStackToState(ctx context.Context, stack *zenfraclient.Stack) (*StackModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Map IAC config
	iacObj, d := types.ObjectValueFrom(ctx, IACModelAttrTypes, &IACModel{
		Engine:  types.StringValue(stack.IAC.Engine),
		Version: types.StringValue(stack.IAC.Version),
	})
	diags.Append(d...)

	// Map source reference helper
	mapRef := func(ref zenfraclient.StackSourceRef) (types.Object, diag.Diagnostics) {
		return types.ObjectValueFrom(ctx, RefModelAttrTypes, &RefModel{
			Type: types.StringValue(ref.Type),
			Name: types.StringValue(ref.Name),
		})
	}

	// Map source
	var sourceObj types.Object
	if stack.Source.Type == "raw_git" && stack.Source.RawGit != nil {
		refObj, d := mapRef(stack.Source.RawGit.Ref)
		diags.Append(d...)

		rawGitObj, d := types.ObjectValueFrom(ctx, RawGitModelAttrTypes, &RawGitModel{
			URL:  types.StringValue(stack.Source.RawGit.URL),
			Ref:  refObj,
			Path: types.StringValue(stack.Source.RawGit.Path),
		})
		diags.Append(d...)

		sourceObj, d = types.ObjectValueFrom(ctx, SourceModelAttrTypes, &SourceModel{
			Type:   types.StringValue("raw_git"),
			RawGit: rawGitObj,
			VCS:    types.ObjectNull(VCSModelAttrTypes),
		})
		diags.Append(d...)
	} else if stack.Source.Type == "vcs" && stack.Source.VCS != nil {
		refObj, d := mapRef(stack.Source.VCS.Ref)
		diags.Append(d...)

		vcsObj, d := types.ObjectValueFrom(ctx, VCSModelAttrTypes, &VCSModel{
			Provider:      types.StringValue(stack.Source.VCS.Provider),
			IntegrationID: types.StringValue(stack.Source.VCS.IntegrationID),
			RepositoryID:  types.StringValue(stack.Source.VCS.RepositoryID),
			Ref:           refObj,
			Path:          types.StringValue(stack.Source.VCS.Path),
		})
		diags.Append(d...)

		sourceObj, d = types.ObjectValueFrom(ctx, SourceModelAttrTypes, &SourceModel{
			Type:   types.StringValue("vcs"),
			RawGit: types.ObjectNull(RawGitModelAttrTypes),
			VCS:    vcsObj,
		})
		diags.Append(d...)
	}

	// Map triggers
	paths, d := types.ListValueFrom(ctx, types.StringType, stack.Triggers.OnPush.Paths)
	diags.Append(d...)

	onPushObj, d := types.ObjectValueFrom(ctx, OnPushModelAttrTypes, &OnPushModel{
		Enabled: types.BoolValue(stack.Triggers.OnPush.Enabled),
		Paths:   paths,
	})
	diags.Append(d...)

	triggersObj, d := types.ObjectValueFrom(ctx, TriggersModelAttrTypes, &TriggersModel{
		OnPush: onPushObj,
	})
	diags.Append(d...)

	model := &StackModel{
		ID:              types.StringValue(stack.ID),
		OrganizationID:  types.StringValue(stack.OrganizationID),
		SpaceID:         types.StringValue(stack.SpaceID),
		Name:            types.StringValue(stack.Name),
		AllowPublicPool: types.BoolValue(stack.AllowPublicPool),
		IAC:             iacObj,
		Source:          sourceObj,
		Triggers:        triggersObj,
		CreatedAt:       types.StringValue(stack.CreatedAt.Format("2006-01-02T15:04:05Z07:00")),
		UpdatedAt:       types.StringValue(stack.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")),
		CreatedBy:       types.StringValue(stack.CreatedBy),
		UpdatedBy:       types.StringValue(stack.UpdatedBy),
	}

	if stack.WorkerPoolID != nil {
		model.WorkerPoolID = types.StringValue(*stack.WorkerPoolID)
	} else {
		model.WorkerPoolID = types.StringNull()
	}

	return model, diags
}

// buildSourceFromModel extracts source configuration from Terraform model.
func buildSourceFromModel(ctx context.Context, model *SourceModel) (*zenfraclient.StackSource, diag.Diagnostics) {
	var diags diag.Diagnostics

	source := &zenfraclient.StackSource{
		Type: model.Type.ValueString(),
	}

	if model.Type.ValueString() == "raw_git" && !model.RawGit.IsNull() {
		var rawGitModel RawGitModel
		d := model.RawGit.As(ctx, &rawGitModel, basetypes.ObjectAsOptions{})
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}

		var refModel RefModel
		d = rawGitModel.Ref.As(ctx, &refModel, basetypes.ObjectAsOptions{})
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}

		source.RawGit = &zenfraclient.StackSourceRawGit{
			URL: rawGitModel.URL.ValueString(),
			Ref: zenfraclient.StackSourceRef{
				Type: refModel.Type.ValueString(),
				Name: refModel.Name.ValueString(),
			},
			Path: rawGitModel.Path.ValueString(),
		}
	} else if model.Type.ValueString() == "vcs" && !model.VCS.IsNull() {
		var vcsModel VCSModel
		d := model.VCS.As(ctx, &vcsModel, basetypes.ObjectAsOptions{})
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}

		var refModel RefModel
		d = vcsModel.Ref.As(ctx, &refModel, basetypes.ObjectAsOptions{})
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}

		source.VCS = &zenfraclient.StackSourceVCS{
			Provider:      vcsModel.Provider.ValueString(),
			IntegrationID: vcsModel.IntegrationID.ValueString(),
			RepositoryID:  vcsModel.RepositoryID.ValueString(),
			Ref: zenfraclient.StackSourceRef{
				Type: refModel.Type.ValueString(),
				Name: refModel.Name.ValueString(),
			},
			Path: vcsModel.Path.ValueString(),
		}
	}

	return source, diags
}

// buildTriggersFromModel extracts triggers configuration from Terraform model.
func buildTriggersFromModel(ctx context.Context, model *TriggersModel) (*zenfraclient.StackTriggers, diag.Diagnostics) {
	var diags diag.Diagnostics

	triggers := &zenfraclient.StackTriggers{}

	if !model.OnPush.IsNull() {
		var onPushModel OnPushModel
		d := model.OnPush.As(ctx, &onPushModel, basetypes.ObjectAsOptions{})
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}

		var paths []string
		d = onPushModel.Paths.ElementsAs(ctx, &paths, false)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}

		triggers.OnPush = zenfraclient.StackTriggerOnPush{
			Enabled: onPushModel.Enabled.ValueBool(),
			Paths:   paths,
		}
	}

	return triggers, diags
}
