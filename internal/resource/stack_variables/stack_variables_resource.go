// ABOUTME: Implements the zenfra_stack_variables Terraform resource with replace-all semantics.
// ABOUTME: Includes import safety guard to prevent accidental variable deletion and secret value preservation.
package stack_variables

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	_ resource.Resource                = &StackVariablesResource{}
	_ resource.ResourceWithImportState = &StackVariablesResource{}
	_ resource.ResourceWithModifyPlan  = &StackVariablesResource{}
)

// NewStackVariablesResource is a constructor for the stack variables resource.
func NewStackVariablesResource() resource.Resource {
	return &StackVariablesResource{}
}

// StackVariablesResource is the resource implementation.
type StackVariablesResource struct {
	client *zenfraclient.Client
}

func (r *StackVariablesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stack_variables"
}

func (r *StackVariablesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the complete set of variables on a Zenfra stack. Uses replace-all semantics: variables not in the configuration will be deleted.",
		Attributes: map[string]schema.Attribute{
			"stack_id": schema.StringAttribute{
				Description: "The stack ID to manage variables for.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"variable": schema.SetNestedBlock{
				Description: "A variable to set on the stack.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Description: "The variable name.",
							Required:    true,
						},
						"value": schema.StringAttribute{
							Description: "The variable value.",
							Required:    true,
							Sensitive:   true,
						},
						"secret": schema.BoolAttribute{
							Description: "Whether this is a secret variable. Secret values are write-only.",
							Optional:    true,
							Computed:    true,
							Default:     booldefault.StaticBool(false),
						},
					},
				},
			},
		},
	}
}

func (r *StackVariablesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ModifyPlan implements the import safety guard. When a stack has variables on the remote
// that are NOT in the config, this emits an error to prevent accidental deletion.
func (r *StackVariablesResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	if r.client == nil {
		return
	}

	var plan StackVariablesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	stackID := plan.StackID.ValueString()
	if stackID == "" {
		return
	}

	remoteVars, err := r.client.GetStackVariables(ctx, stackID)
	if err != nil {
		return
	}

	configKeys := make(map[string]bool)
	if !plan.Variable.IsNull() {
		var vars []VariableModel
		resp.Diagnostics.Append(plan.Variable.ElementsAs(ctx, &vars, false)...)
		for _, v := range vars {
			configKeys[v.Key.ValueString()] = true
		}
	}

	var missing []string
	for _, rv := range remoteVars {
		if !configKeys[rv.Key] {
			missing = append(missing, rv.Key)
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		resp.Diagnostics.AddError(
			"Stack Has Variables Not In Configuration",
			fmt.Sprintf(
				"Stack %s has variables not in your configuration that will be deleted: [%s]. "+
					"Add all variables to your configuration or they will be removed.",
				stackID, strings.Join(missing, ", "),
			),
		)
	}
}

func (r *StackVariablesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan StackVariablesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiVars := planToAPIVars(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.SetStackVariables(ctx, plan.StackID.ValueString(), apiVars)
	if err != nil {
		resp.Diagnostics.AddError("Error Setting Stack Variables", fmt.Sprintf("Could not set variables: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

//nolint:gocognit // Terraform CRUD with secret value preservation
func (r *StackVariablesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state StackVariablesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	remoteVars, err := r.client.GetStackVariables(ctx, state.StackID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading Stack Variables",
			fmt.Sprintf("Could not read variables for stack %s: %s", state.StackID.ValueString(), err))
		return
	}

	// Build prior secret values from state
	priorSecrets := make(map[string]string)
	if !state.Variable.IsNull() {
		var stateVars []VariableModel
		resp.Diagnostics.Append(state.Variable.ElementsAs(ctx, &stateVars, false)...)
		for _, sv := range stateVars {
			if sv.Secret.ValueBool() {
				priorSecrets[sv.Key.ValueString()] = sv.Value.ValueString()
			}
		}
	}

	varObjType := types.ObjectType{AttrTypes: variableAttrTypes()}
	if len(remoteVars) > 0 {
		var varObjects []attr.Value
		for _, rv := range remoteVars {
			value := rv.Value
			if rv.Secret && value == "****" {
				if prior, ok := priorSecrets[rv.Key]; ok {
					value = prior
				}
			}
			obj, diags := types.ObjectValue(variableAttrTypes(), map[string]attr.Value{
				"key":    types.StringValue(rv.Key),
				"value":  types.StringValue(value),
				"secret": types.BoolValue(rv.Secret),
			})
			resp.Diagnostics.Append(diags...)
			varObjects = append(varObjects, obj)
		}
		varSet, diags := types.SetValue(varObjType, varObjects)
		resp.Diagnostics.Append(diags...)
		state.Variable = varSet
	} else {
		state.Variable = types.SetNull(varObjType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *StackVariablesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan StackVariablesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiVars := planToAPIVars(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.SetStackVariables(ctx, plan.StackID.ValueString(), apiVars)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Stack Variables", fmt.Sprintf("Could not update variables: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *StackVariablesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state StackVariablesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.SetStackVariables(ctx, state.StackID.ValueString(), []zenfraclient.StackVariable{})
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error Deleting Stack Variables",
			fmt.Sprintf("Could not clear variables for stack %s: %s", state.StackID.ValueString(), err))
	}
}

func (r *StackVariablesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("stack_id"), req, resp)
}

// variableAttrTypes returns the attribute types for a variable object.
func variableAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"key":    types.StringType,
		"value":  types.StringType,
		"secret": types.BoolType,
	}
}

// planToAPIVars converts plan variable blocks into API StackVariable structs.
func planToAPIVars(ctx context.Context, plan StackVariablesModel, diags *diag.Diagnostics) []zenfraclient.StackVariable {
	var result []zenfraclient.StackVariable
	if plan.Variable.IsNull() {
		return result
	}

	var vars []VariableModel
	diags.Append(plan.Variable.ElementsAs(ctx, &vars, false)...)
	for _, v := range vars {
		result = append(result, zenfraclient.StackVariable{
			Key:    v.Key.ValueString(),
			Value:  v.Value.ValueString(),
			Secret: v.Secret.ValueBool(),
		})
	}
	return result
}
