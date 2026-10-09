// ABOUTME: Implements the zenfra_configuration_bundle Terraform resource with full CRUD lifecycle.
// ABOUTME: Manages bundles' env vars, mounted files and hooks (content) and auto-attach labels (metadata), preserving secrets.
package bundle

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/labelset"
	"github.com/zenfra/terraform-provider-zenfra/internal/validate"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

var (
	_ resource.Resource                = &BundleResource{}
	_ resource.ResourceWithImportState = &BundleResource{}
	_ resource.ResourceWithModifyPlan  = &BundleResource{}

	_ resource.ResourceWithValidateConfig = &BundleResource{}
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
		Description: "Manages a Zenfra configuration bundle containing environment variables, mounted files and hooks.",
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
			"auto_attach_labels": schema.SetAttribute{
				Description: "Stack labels this bundle attaches itself to: every stack carrying at least one of them gets the " +
					"bundle on its runs without a zenfra_bundle_attachment, provided the bundle could be attached to it by " +
					"hand (the stack's own space, or an ancestor space it inherits bundles from). Changes apply to runs " +
					"created afterwards. Lowercase a-z, 0-9, '.', '_' and '-', 1-63 " +
					"characters starting with a letter or digit, at most 20. A selector, not content: changing it does not " +
					"change content_version. Terraform owns the whole set: omitting the attribute means none.",
				Optional:    true,
				ElementType: types.StringType,
				Validators:  []validator.Set{validate.Labels()},
			},
			"hooks": schema.SingleNestedAttribute{
				Description: "Shell commands run before and after init, plan and apply on every run the bundle attaches to. " +
					"Each phase is an ordered list of 1-32 commands of at most 4096 bytes, run with `sh -c`; the first " +
					"non-zero exit stops the list. Hooks are content: changing them writes new content (fenced by " +
					"content_version) and a run whose bundles change after it was created stops as stale. Terraform owns " +
					"the hooks: omitting the attribute clears them. Only workers that run hooks claim such runs.",
				Optional:   true,
				Validators: []validator.Object{validate.HooksNotEmpty()},
				Attributes: hookPhaseAttributes(),
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
							Description: "The environment variable value. A secret's value must not be empty: it is refused at plan, or at apply when the value is only known then.",
							Required:    true,
							Sensitive:   true,
						},
						"secret": schema.BoolAttribute{
							Description: "Whether this is a secret value. Secret values are write-only: Zenfra never returns them, so the configuration always carries the value.",
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
							Description: "The file content. A secret file's content must not be empty: it is refused at plan, or at apply when the content is only known then.",
							Required:    true,
							Sensitive:   true,
						},
						"secret": schema.BoolAttribute{
							Description: "Whether this file is secret. Secret files are write-only: Zenfra never returns their content, so the configuration always carries it.",
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
	// Everything that can refuse the content is checked before the first
	// request: a value that resolved empty at apply leaves no bundle behind.
	resp.Diagnostics.Append(emptySecretDiags(plan.EnvironmentVariable, plan.MountedFile)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq, diags := buildCreateRequest(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A bundle with content gets its auto-attach selector only after the
	// content: writing content makes the runs that already hold the bundle
	// stale, so it must not attach by label while it is still empty.
	hasEnvVars := !plan.EnvironmentVariable.IsNull() && len(plan.EnvironmentVariable.Elements()) > 0
	hasFiles := !plan.MountedFile.IsNull() && len(plan.MountedFile.Elements()) > 0
	hasContent := hasEnvVars || hasFiles || !plan.Hooks.IsNull()
	var contentReq zenfraclient.UpdateBundleContentRequest
	if hasContent {
		contentReq = buildContentRequest(ctx, plan, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	selector := createReq.AutoAttachLabels
	if hasContent {
		createReq.AutoAttachLabels = nil
	}

	bundle, err := r.client.CreateBundle(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Bundle", fmt.Sprintf("Could not create bundle: %s", err))
		return
	}

	// The bundle exists from here on. Record it as created, with no content,
	// before the writes below: if one fails, Terraform keeps it as tainted and
	// replaces it, instead of losing track of a bundle that holds its slug.
	created := createdState(ctx, plan, bundle, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, created)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if hasContent {
		bundle = r.writeContentThenSelector(ctx, contentReq, bundle, selector, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	state := mapBundleToState(bundle)
	// Preserve plan values for content blocks - API masks secret values
	state.EnvironmentVariable = plan.EnvironmentVariable
	state.MountedFile = plan.MountedFile
	state.Labels = plan.Labels
	resp.Diagnostics.Append(mapSelectorAndHooks(ctx, &state, bundle, plan.AutoAttachLabels)...)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// createdState is the state of a bundle just created, before any content
// write: metadata from the API, no content, and the selector and hooks the
// API holds.
func createdState(ctx context.Context, plan BundleModel, bundle *zenfraclient.Bundle, diags *diag.Diagnostics) BundleModel {
	state := mapBundleToState(bundle)
	state.Labels = plan.Labels
	state.EnvironmentVariable = types.SetNull(types.ObjectType{AttrTypes: envVarAttrTypes()})
	state.MountedFile = types.SetNull(types.ObjectType{AttrTypes: mountedFileAttrTypes()})
	diags.Append(mapSelectorAndHooks(ctx, &state, bundle, plan.AutoAttachLabels)...)
	return state
}

// writeContentThenSelector writes a new bundle's content, then its
// auto-attach selector, and returns the bundle as it stands afterwards.
func (r *BundleResource) writeContentThenSelector(
	ctx context.Context, contentReq zenfraclient.UpdateBundleContentRequest, bundle *zenfraclient.Bundle, selector []string, diags *diag.Diagnostics,
) *zenfraclient.Bundle {
	// A new bundle is at content_version 0; the API fences this first write on
	// it too (see UpdateBundleContentRequest).
	contentReq.ExpectedVersion = bundle.ContentVersion
	contentResp, err := r.client.UpdateBundleContent(ctx, bundle.ID, contentReq)
	if err != nil {
		diags.AddError("Error Setting Bundle Content", fmt.Sprintf("Could not set bundle content: %s", err))
		return nil
	}
	if len(selector) == 0 {
		return &contentResp.Bundle
	}
	if err := r.client.UpdateBundle(ctx, bundle.ID, zenfraclient.UpdateBundleRequest{AutoAttachLabels: &selector}); err != nil {
		diags.AddError("Error Setting Bundle Auto-Attach Labels", fmt.Sprintf("Could not set auto_attach_labels: %s", err))
		return nil
	}
	updated, err := r.client.GetBundle(ctx, bundle.ID)
	if err != nil {
		diags.AddError("Error Reading Bundle", fmt.Sprintf("Could not read bundle: %s", err))
		return nil
	}
	return updated
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

	resp.Diagnostics.Append(mapSelectorAndHooks(ctx, &newState, bundle, state.AutoAttachLabels)...)

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
	// Before any request: neither the content nor the metadata is written
	// when a secret resolved empty at apply.
	resp.Diagnostics.Append(emptySecretDiags(plan.EnvironmentVariable, plan.MountedFile)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var bundle *zenfraclient.Bundle

	// Update content if env vars, mounted files or hooks changed
	if contentChanged(plan, state) {
		contentReq := buildContentRequest(ctx, plan, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		// Fenced on the version Terraform last saw, 0 included while the
		// bundle's content was never written (see UpdateBundleContentRequest).
		contentReq.ExpectedVersion = state.ContentVersion.ValueInt64()

		contentResp, err := r.client.UpdateBundleContent(ctx, state.ID.ValueString(), contentReq)
		if err != nil {
			resp.Diagnostics.AddError("Error Updating Bundle Content", fmt.Sprintf("Could not update bundle content: %s", err))
			return
		}
		bundle = &contentResp.Bundle
	}

	// Update metadata if changed. After the content: a selector added in the
	// same apply must not attach the bundle by label before its new content
	// is written, or the runs that pick it up meanwhile go stale.
	metadataChanged := !plan.Name.Equal(state.Name) ||
		!plan.Description.Equal(state.Description) ||
		!plan.Labels.Equal(state.Labels) ||
		!plan.AutoAttachLabels.Equal(state.AutoAttachLabels) ||
		!plan.SpaceID.Equal(state.SpaceID)

	if metadataChanged {
		updateReq := zenfraclient.UpdateBundleRequest{}
		if !plan.Description.Equal(state.Description) {
			desc := plan.Description.ValueString()
			updateReq.Description = &desc
		}
		if !plan.Labels.Equal(state.Labels) {
			// Non-nil even when the plan is null: a nil slice would send
			// null, which the API reads as "unchanged", not as a clear.
			labels := []string{}
			resp.Diagnostics.Append(plan.Labels.ElementsAs(ctx, &labels, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			if labels == nil {
				labels = []string{}
			}
			updateReq.Labels = &labels
		}
		if !plan.AutoAttachLabels.Equal(state.AutoAttachLabels) {
			autoAttach, diags := labelset.ToAPI(ctx, plan.AutoAttachLabels)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			updateReq.AutoAttachLabels = &autoAttach
		}
		if !plan.SpaceID.Equal(state.SpaceID) {
			spaceID := plan.SpaceID.ValueString()
			updateReq.SpaceID = &spaceID
		}

		// The API answers a metadata update with a status message, not the
		// bundle; the bundle is read back below.
		if err := r.client.UpdateBundle(ctx, state.ID.ValueString(), updateReq); err != nil {
			resp.Diagnostics.AddError("Error Updating Bundle", fmt.Sprintf("Could not update bundle: %s", err))
			return
		}
		bundle = nil
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
	resp.Diagnostics.Append(mapSelectorAndHooks(ctx, &newState, bundle, plan.AutoAttachLabels)...)

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

// ValidateConfig refuses a secret whose value is known to be empty, at plan
// time. Unknown values are left to the same check at apply.
func (r *BundleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var envVars, files types.Set
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("environment_variable"), &envVars)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("mounted_file"), &files)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(emptySecretDiags(envVars, files)...)
}

// emptySecretDiags reports each secret = true entry whose value or content is
// known and empty (zero characters; whitespace is a value). The API never
// returns a secret's value and keeps a stored one only when it is sent
// empty on purpose; this provider always sends the configured value, so an
// empty one is a mistake. Anything unknown is skipped. A diagnostic names the
// key or path, never a value.
func emptySecretDiags(envVars, files types.Set) diag.Diagnostics {
	var diags diag.Diagnostics
	check := func(set types.Set, block, idAttr, valueAttr string) {
		if set.IsNull() || set.IsUnknown() {
			return
		}
		for _, elem := range set.Elements() {
			obj, ok := elem.(types.Object)
			if !ok || obj.IsNull() || obj.IsUnknown() {
				continue
			}
			attrs := obj.Attributes()
			secret, _ := attrs["secret"].(types.Bool)
			value, _ := attrs[valueAttr].(types.String)
			if secret.IsUnknown() || !secret.ValueBool() || value.IsNull() || value.IsUnknown() || value.ValueString() != "" {
				continue
			}
			name := "with an unknown " + idAttr
			if id, _ := attrs[idAttr].(types.String); !id.IsNull() && !id.IsUnknown() {
				name = strconv.Quote(id.ValueString())
			}
			diags.AddAttributeError(path.Root(block), "Secret Without a Value",
				fmt.Sprintf("The %s %s is secret and its %s is empty. A secret must carry its %s.", block, name, valueAttr, valueAttr))
		}
	}
	check(envVars, "environment_variable", "key", "value")
	check(files, "mounted_file", "path", "content")
	return diags
}

// ModifyPlan marks content_version unknown when the plan writes new content:
// the write bumps it, and a planned value carried over from state would make
// the applied result inconsistent with the plan.
func (r *BundleResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var plan, state BundleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if contentChanged(plan, state) {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("content_version"), types.Int64Unknown())...)
	}
}

// contentChanged reports whether the plan changes the bundle's content: its
// environment variables, mounted files or hooks.
func contentChanged(plan, state BundleModel) bool {
	return !plan.EnvironmentVariable.Equal(state.EnvironmentVariable) ||
		!plan.MountedFile.Equal(state.MountedFile) ||
		!plan.Hooks.Equal(state.Hooks)
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

// buildContentRequest extracts env vars, mounted files and hooks from the
// plan into an API content request. Hooks are always sent: the planned hooks,
// or {} to clear them when the plan has none.
func buildContentRequest(ctx context.Context, plan BundleModel, diags *diag.Diagnostics) zenfraclient.UpdateBundleContentRequest {
	content := zenfraclient.BundleContent{}

	hooks, d := hooksToAPI(ctx, plan.Hooks)
	diags.Append(d...)
	content.Hooks = hooks

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

// hookPhaseAttributes returns the hooks object's six phase attributes.
func hookPhaseAttributes() map[string]schema.Attribute {
	descriptions := map[string]string{
		"before_init":  "Commands run before terraform init.",
		"after_init":   "Commands run after terraform init.",
		"before_plan":  "Commands run before terraform plan.",
		"after_plan":   "Commands run after terraform plan.",
		"before_apply": "Commands run before terraform apply.",
		"after_apply":  "Commands run after terraform apply.",
	}
	attrs := make(map[string]schema.Attribute, len(hookPhases))
	for _, phase := range hookPhases {
		attrs[phase] = schema.ListAttribute{
			Description: descriptions[phase],
			Optional:    true,
			ElementType: types.StringType,
			Validators:  []validator.List{validate.HookCommands()},
		}
	}
	return attrs
}

// mapSelectorAndHooks sets the auto-attach selector and the hooks from the
// API. priorAutoAttach is the plan (after a write) or the state being
// refreshed; it only decides whether "no labels" reads back as null or empty.
func mapSelectorAndHooks(ctx context.Context, model *BundleModel, bundle *zenfraclient.Bundle, priorAutoAttach types.Set) diag.Diagnostics {
	autoAttach, diags := labelset.FromAPI(bundle.AutoAttachLabels, priorAutoAttach)
	model.AutoAttachLabels = autoAttach
	hooks, d := hooksFromAPI(ctx, bundle.Hooks)
	diags.Append(d...)
	model.Hooks = hooks
	return diags
}

// buildCreateRequest maps the plan's metadata, including the auto-attach
// selector, into a create request. Content is written separately.
func buildCreateRequest(ctx context.Context, plan BundleModel) (zenfraclient.CreateBundleRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
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
		diags.Append(plan.Labels.ElementsAs(ctx, &labels, false)...)
		createReq.Labels = labels
	}
	autoAttach, d := labelset.ToAPI(ctx, plan.AutoAttachLabels)
	diags.Append(d...)
	if len(autoAttach) > 0 {
		createReq.AutoAttachLabels = autoAttach
	}
	return createReq, diags
}
