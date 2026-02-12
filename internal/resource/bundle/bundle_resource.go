// ABOUTME: Implements the zenfra_configuration_bundle Terraform resource with full CRUD lifecycle.
// ABOUTME: Manages Zenfra configuration bundles including environment variables and mounted files with secret preservation.
package bundle

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

var (
	_ resource.Resource                = &BundleResource{}
	_ resource.ResourceWithImportState = &BundleResource{}
)

// NewBundleResource is a constructor for the bundle resource.
func NewBundleResource() resource.Resource {
	return &BundleResource{}
}

// BundleResource is the resource implementation.
type BundleResource struct {
	client *zenfraclient.Client
}

func (r *BundleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_configuration_bundle"
}

func (r *BundleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Zenfra configuration bundle containing environment variables and mounted files.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the bundle.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				Description: "The organization ID this bundle belongs to.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"space_id": schema.StringAttribute{
				Description: "The space ID this bundle is associated with.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the configuration bundle.",
				Required:    true,
			},
			"slug": schema.StringAttribute{
				Description: "URL-friendly identifier. Computed from name if not specified.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				Description: "Description of the configuration bundle.",
				Optional:    true,
			},
			"labels": schema.ListAttribute{
				Description: "Labels for categorizing the bundle.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"content_version": schema.Int64Attribute{
				Description: "The version number of the bundle content.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"attached_stacks_count": schema.Int64Attribute{
				Description: "Number of stacks this bundle is attached to.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "Timestamp when the bundle was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Description: "Timestamp when the bundle was last updated.",
				Computed:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"environment_variable": schema.SetNestedBlock{
				Description: "Environment variables included in the bundle.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Description: "The environment variable name.",
							Required:    true,
						},
						"value": schema.StringAttribute{
							Description: "The environment variable value.",
							Required:    true,
							Sensitive:   true,
						},
						"secret": schema.BoolAttribute{
							Description: "Whether this is a secret value. Secret values are write-only.",
							Optional:    true,
							Computed:    true,
							Default:     booldefault.StaticBool(false),
						},
						"description": schema.StringAttribute{
							Description: "Description of this environment variable.",
							Optional:    true,
						},
					},
				},
			},
			"mounted_file": schema.SetNestedBlock{
				Description: "Mounted files included in the bundle.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"path": schema.StringAttribute{
							Description: "The file path where the content will be mounted.",
							Required:    true,
						},
						"content": schema.StringAttribute{
							Description: "The file content.",
							Required:    true,
							Sensitive:   true,
						},
						"secret": schema.BoolAttribute{
							Description: "Whether this file is secret. Secret files are write-only.",
							Optional:    true,
							Computed:    true,
							Default:     booldefault.StaticBool(false),
						},
						"description": schema.StringAttribute{
							Description: "Description of this mounted file.",
							Optional:    true,
						},
					},
				},
			},
		},
	}
}

func (r *BundleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *BundleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BundleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := zenfraclient.CreateBundleRequest{
		Name:    plan.Name.ValueString(),
		SpaceID: plan.SpaceID.ValueString(),
	}
	if !plan.Slug.IsNull() && !plan.Slug.IsUnknown() {
		createReq.Slug = plan.Slug.ValueString()
	} else {
		createReq.Slug = plan.Name.ValueString()
	}
	if !plan.Description.IsNull() {
		createReq.Description = plan.Description.ValueString()
	}
	if !plan.Labels.IsNull() {
		var labels []string
		resp.Diagnostics.Append(plan.Labels.ElementsAs(ctx, &labels, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Labels = labels
	}

	bundle, err := r.client.CreateBundle(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Bundle", fmt.Sprintf("Could not create bundle: %s", err))
		return
	}

	// If content blocks are specified, update content after creating metadata
	hasEnvVars := !plan.EnvironmentVariable.IsNull() && len(plan.EnvironmentVariable.Elements()) > 0
	hasFiles := !plan.MountedFile.IsNull() && len(plan.MountedFile.Elements()) > 0
	if hasEnvVars || hasFiles {
		contentReq := buildContentRequest(ctx, plan, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		contentReq.ExpectedVersion = bundle.ContentVersion

		contentResp, err := r.client.UpdateBundleContent(ctx, bundle.ID, contentReq)
		if err != nil {
			resp.Diagnostics.AddError("Error Setting Bundle Content", fmt.Sprintf("Could not set bundle content: %s", err))
			return
		}
		bundle = &contentResp.Bundle
	}

	state := mapBundleToState(bundle)
	// Preserve plan values for content blocks - API masks secret values
	state.EnvironmentVariable = plan.EnvironmentVariable
	state.MountedFile = plan.MountedFile
	state.Labels = plan.Labels

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

//nolint:gocognit,gocyclo // Terraform CRUD with secret preservation requires complex state management
func (r *BundleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BundleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bundle, err := r.client.GetBundle(ctx, state.ID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading Bundle", fmt.Sprintf("Could not read bundle ID %s: %s", state.ID.ValueString(), err))
		return
	}

	// Build prior secret maps from current state for secret value preservation
	priorEnvVars := make(map[string]string)
	priorFiles := make(map[string]string)
	if !state.EnvironmentVariable.IsNull() {
		var envVars []EnvVariableModel
		resp.Diagnostics.Append(state.EnvironmentVariable.ElementsAs(ctx, &envVars, false)...)
		for _, ev := range envVars {
			if ev.Secret.ValueBool() {
				priorEnvVars[ev.Key.ValueString()] = ev.Value.ValueString()
			}
		}
	}
	if !state.MountedFile.IsNull() {
		var files []MountedFileModel
		resp.Diagnostics.Append(state.MountedFile.ElementsAs(ctx, &files, false)...)
		for _, f := range files {
			if f.Secret.ValueBool() {
				priorFiles[f.Path.ValueString()] = f.Content.ValueString()
			}
		}
	}

	newState := mapBundleToState(bundle)

	// Rebuild env vars from API, preserving secret values from prior state
	envVarObjType := types.ObjectType{AttrTypes: envVarAttrTypes()}
	if len(bundle.EnvironmentVariables) > 0 {
		var envVarObjects []attr.Value
		for _, ev := range bundle.EnvironmentVariables {
			value := ev.Value
			if ev.Secret && value == "" {
				if prior, ok := priorEnvVars[ev.Key]; ok {
					value = prior
				}
			}
			desc := types.StringNull()
			if ev.Description != "" {
				desc = types.StringValue(ev.Description)
			}
			obj, diags := types.ObjectValue(envVarAttrTypes(), map[string]attr.Value{
				"key":         types.StringValue(ev.Key),
				"value":       types.StringValue(value),
				"secret":      types.BoolValue(ev.Secret),
				"description": desc,
			})
			resp.Diagnostics.Append(diags...)
			envVarObjects = append(envVarObjects, obj)
		}
		evSet, diags := types.SetValue(envVarObjType, envVarObjects)
		resp.Diagnostics.Append(diags...)
		newState.EnvironmentVariable = evSet
	} else {
		newState.EnvironmentVariable = types.SetNull(envVarObjType)
	}

	// Rebuild mounted files from API, preserving secret values from prior state
	fileObjType := types.ObjectType{AttrTypes: mountedFileAttrTypes()}
	if len(bundle.MountedFiles) > 0 {
		var fileObjects []attr.Value
		for _, f := range bundle.MountedFiles {
			content := f.Content
			if f.Secret && content == "" {
				if prior, ok := priorFiles[f.Path]; ok {
					content = prior
				}
			}
			desc := types.StringNull()
			if f.Description != "" {
				desc = types.StringValue(f.Description)
			}
			obj, diags := types.ObjectValue(mountedFileAttrTypes(), map[string]attr.Value{
				"path":        types.StringValue(f.Path),
				"content":     types.StringValue(content),
				"secret":      types.BoolValue(f.Secret),
				"description": desc,
			})
			resp.Diagnostics.Append(diags...)
			fileObjects = append(fileObjects, obj)
		}
		mfSet, diags := types.SetValue(fileObjType, fileObjects)
		resp.Diagnostics.Append(diags...)
		newState.MountedFile = mfSet
	} else {
		newState.MountedFile = types.SetNull(fileObjType)
	}

	// Rebuild labels
	if len(bundle.Labels) > 0 {
		var labelValues []attr.Value
		for _, l := range bundle.Labels {
			labelValues = append(labelValues, types.StringValue(l))
		}
		labelsList, diags := types.ListValue(types.StringType, labelValues)
		resp.Diagnostics.Append(diags...)
		newState.Labels = labelsList
	} else {
		newState.Labels = types.ListNull(types.StringType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

//nolint:gocognit,gocyclo // Terraform CRUD with metadata+content split update
func (r *BundleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state BundleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var bundle *zenfraclient.Bundle

	// Update metadata if changed
	metadataChanged := !plan.Name.Equal(state.Name) ||
		!plan.Description.Equal(state.Description) ||
		!plan.Labels.Equal(state.Labels) ||
		!plan.SpaceID.Equal(state.SpaceID)

	if metadataChanged {
		updateReq := zenfraclient.UpdateBundleRequest{}
		if !plan.Description.Equal(state.Description) {
			desc := plan.Description.ValueString()
			updateReq.Description = &desc
		}
		if !plan.Labels.Equal(state.Labels) {
			var labels []string
			resp.Diagnostics.Append(plan.Labels.ElementsAs(ctx, &labels, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			updateReq.Labels = &labels
		}
		if !plan.SpaceID.Equal(state.SpaceID) {
			spaceID := plan.SpaceID.ValueString()
			updateReq.SpaceID = &spaceID
		}

		var err error
		bundle, err = r.client.UpdateBundle(ctx, state.ID.ValueString(), updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Error Updating Bundle", fmt.Sprintf("Could not update bundle: %s", err))
			return
		}
	}

	// Update content if env vars or mounted files changed
	contentChanged := !plan.EnvironmentVariable.Equal(state.EnvironmentVariable) ||
		!plan.MountedFile.Equal(state.MountedFile)

	if contentChanged {
		contentReq := buildContentRequest(ctx, plan, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		contentReq.ExpectedVersion = state.ContentVersion.ValueInt64()

		contentResp, err := r.client.UpdateBundleContent(ctx, state.ID.ValueString(), contentReq)
		if err != nil {
			resp.Diagnostics.AddError("Error Updating Bundle Content", fmt.Sprintf("Could not update bundle content: %s", err))
			return
		}
		bundle = &contentResp.Bundle
	}

	if bundle == nil {
		var err error
		bundle, err = r.client.GetBundle(ctx, state.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error Reading Bundle", fmt.Sprintf("Could not read bundle: %s", err))
			return
		}
	}

	newState := mapBundleToState(bundle)
	newState.EnvironmentVariable = plan.EnvironmentVariable
	newState.MountedFile = plan.MountedFile
	newState.Labels = plan.Labels

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *BundleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BundleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteBundle(ctx, state.ID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error Deleting Bundle", fmt.Sprintf("Could not delete bundle ID %s: %s", state.ID.ValueString(), err))
	}
}

func (r *BundleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// envVarAttrTypes returns the attribute types for an environment variable object.
func envVarAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"key":         types.StringType,
		"value":       types.StringType,
		"secret":      types.BoolType,
		"description": types.StringType,
	}
}

// mountedFileAttrTypes returns the attribute types for a mounted file object.
func mountedFileAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"path":        types.StringType,
		"content":     types.StringType,
		"secret":      types.BoolType,
		"description": types.StringType,
	}
}

// buildContentRequest extracts env vars and mounted files from the plan into an API content request.
func buildContentRequest(ctx context.Context, plan BundleModel, diags *diag.Diagnostics) zenfraclient.UpdateBundleContentRequest {
	type bundleContent struct {
		EnvironmentVariables []zenfraclient.EnvVariable `json:"environment_variables"`
		MountedFiles         []zenfraclient.MountedFile `json:"mounted_files"`
	}
	content := bundleContent{}

	if !plan.EnvironmentVariable.IsNull() {
		var envVars []EnvVariableModel
		diags.Append(plan.EnvironmentVariable.ElementsAs(ctx, &envVars, false)...)
		for _, ev := range envVars {
			apiVar := zenfraclient.EnvVariable{
				Key:    ev.Key.ValueString(),
				Value:  ev.Value.ValueString(),
				Secret: ev.Secret.ValueBool(),
			}
			if !ev.Description.IsNull() {
				apiVar.Description = ev.Description.ValueString()
			}
			content.EnvironmentVariables = append(content.EnvironmentVariables, apiVar)
		}
	}

	if !plan.MountedFile.IsNull() {
		var files []MountedFileModel
		diags.Append(plan.MountedFile.ElementsAs(ctx, &files, false)...)
		for _, f := range files {
			apiFile := zenfraclient.MountedFile{
				Path:    f.Path.ValueString(),
				Content: f.Content.ValueString(),
				Secret:  f.Secret.ValueBool(),
			}
			if !f.Description.IsNull() {
				apiFile.Description = f.Description.ValueString()
			}
			content.MountedFiles = append(content.MountedFiles, apiFile)
		}
	}

	return zenfraclient.UpdateBundleContentRequest{
		Content: content,
	}
}
