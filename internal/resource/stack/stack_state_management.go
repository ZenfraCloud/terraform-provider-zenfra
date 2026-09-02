// ABOUTME: The create-only state ownership attribute on zenfra_stack.
// ABOUTME: Refuses in-place mode changes rather than forcing a destructive replacement.
package stack

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// stateManagementGuard refuses an in-place change of the ownership mode.
// It is deliberately NOT stringplanmodifier.RequiresReplace: replacement would
// soft-delete the stack, and a managed stack's state cannot be exported first.
type stateManagementGuard struct{}

func (stateManagementGuard) Description(_ context.Context) string {
	return "State ownership is fixed at creation; changing it is refused rather than replacing the stack."
}

func (g stateManagementGuard) MarkdownDescription(ctx context.Context) string {
	return g.Description(ctx)
}

func (stateManagementGuard) PlanModifyString(
	_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse,
) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return // creation or destroy: nothing to compare
	}
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}
	// ConfigValue, not PlanValue: only an explicitly configured mode is a request
	// to change. An omitted attribute is filled from prior state by UseStateForUnknown.
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	prior, wanted := req.StateValue.ValueString(), req.ConfigValue.ValueString()
	if prior == wanted {
		return
	}
	resp.Diagnostics.AddAttributeError(req.Path,
		"State ownership cannot be changed",
		fmt.Sprintf(
			"This stack was created with state_management = %q and cannot be changed to %q in place.\n\n"+
				"Zenfra cannot migrate Terraform state between backends, and there is no self-service way "+
				"to export the state Zenfra holds for a managed stack.\n\n"+
				"To move a stack, destroy it and create a new one, handling the state yourself in between; "+
				"contact your Zenfra operator first if the existing stack is managed.",
			prior, wanted),
	)
}

// validateMode accepts exactly the two modes. Null and unknown are the schema's
// and Create's business, so they pass through untouched. The caller supplies the
// path: StringRequest.Path is documented as the one to use for response
// diagnostics (schema/validator/string.go:25-27), so it must not be hard-coded.
func validateMode(at path.Path, v types.String) diag.Diagnostics {
	var diags diag.Diagnostics
	if v.IsNull() || v.IsUnknown() {
		return diags
	}
	switch v.ValueString() {
	case zenfraclient.StateModeManaged, zenfraclient.StateModeExternal:
	default:
		diags.AddAttributeError(at,
			"Invalid state ownership mode",
			fmt.Sprintf("state_management must be %q or %q, got %q.",
				zenfraclient.StateModeManaged, zenfraclient.StateModeExternal, v.ValueString()))
	}
	return diags
}

// stateManagementValidator adapts validateMode to the framework interface.
type stateManagementValidator struct{}

func (stateManagementValidator) Description(_ context.Context) string {
	return `Must be "managed" or "external".`
}

func (v stateManagementValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (stateManagementValidator) ValidateString(
	_ context.Context, req validator.StringRequest, resp *validator.StringResponse,
) {
	resp.Diagnostics.Append(validateMode(req.Path, req.ConfigValue)...)
}
