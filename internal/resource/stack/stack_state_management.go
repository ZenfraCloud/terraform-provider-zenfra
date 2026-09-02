// ABOUTME: The create-only state ownership attribute on zenfra_stack.
// ABOUTME: Refuses in-place mode changes rather than forcing a destructive replacement.
package stack

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
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
	if req.StateValue.IsUnknown() {
		return
	}
	// ConfigValue, not PlanValue: only an explicitly configured mode is a request
	// to change. An omitted attribute is filled from prior state by the
	// UseNonNullStateForUnknown modifier that runs before this one.
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	// A null prior value is not "no opinion": state written before this attribute
	// existed decodes as null against the current schema, and such a stack is
	// managed, because external did not exist yet. That is the same contract as
	// zenfraclient.EffectiveStateMode, which reads an absent block as managed.
	prior := zenfraclient.StateModeManaged
	if !req.StateValue.IsNull() {
		prior = req.StateValue.ValueString()
	}
	wanted := req.ConfigValue.ValueString()
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

// stateManagementForCreate turns the configured attribute into a request block,
// or reports why it cannot. Null means the practitioner did not choose, which the
// API reads as managed, so send nothing rather than assert a default the server
// owns. Unknown is NOT the same thing: it means they wrote something that did not
// resolve at plan time, and quietly sending nothing would turn that into managed.
func stateManagementForCreate(
	at path.Path, configured types.String,
) (*zenfraclient.StateManagement, diag.Diagnostics) {
	var diags diag.Diagnostics
	if configured.IsUnknown() {
		diags.AddAttributeError(at, "State ownership must be known at plan time",
			"state_management could not be resolved during planning, and it cannot be chosen after "+
				"the stack exists. Use a literal or a value that is known before apply.")
		return nil, diags
	}
	if configured.IsNull() {
		return nil, diags
	}
	return &zenfraclient.StateManagement{Mode: configured.ValueString()}, diags
}

// requestedEffectiveMode is what the practitioner asked for, in effective terms.
// Only reached once the value is known, so unknown does not appear here.
func requestedEffectiveMode(configured types.String) string {
	if configured.IsNull() {
		return zenfraclient.StateModeManaged
	}
	return configured.ValueString()
}

// cleanupOutcome says what happened to the stack that should not exist, and is
// what Create branches on. Never branch on diagnostic text: wording is not a
// control-flow contract, and a reworded error must not silently invert whether
// state is kept.
type cleanupOutcome int

const (
	cleanupNotNeeded cleanupOutcome = iota // the mode was right; nothing was created wrongly
	cleanupSucceeded                       // the orphan is gone; Create must set NO state
	cleanupFailed                          // the orphan survives; Create MUST record it
)

// verifyCreatedMode compares what the server returned against what was asked
// for and, on a mismatch, deletes the stack that was just created. Returning a
// bare error would orphan it: the POST already succeeded.
func verifyCreatedMode(
	ctx context.Context, client stackClient, id, requested string, returned *zenfraclient.StateManagement,
) (cleanupOutcome, diag.Diagnostics) {
	var diags diag.Diagnostics
	if zenfraclient.EffectiveStateMode(returned) == requested {
		return cleanupNotNeeded, diags
	}

	if returned == nil {
		diags.AddError("Zenfra API does not support state ownership",
			fmt.Sprintf("Requested state_management = %q, but the API returned a stack with no "+
				"state_management field at all, so the request was silently dropped and the stack "+
				"is Zenfra-managed.\n\nUpgrade the Zenfra control plane to a version that supports "+
				"external state, then retry.", requested))
	} else {
		diags.AddError("Zenfra API returned a different state ownership mode",
			fmt.Sprintf("Requested state_management = %q but the created stack reports %q.",
				requested, returned.Mode))
	}

	if err := client.DeleteStack(ctx, id); err != nil {
		diags.AddError("Could not clean up the incorrectly created stack",
			fmt.Sprintf("Stack %s was created with the wrong state ownership and could not be "+
				"deleted: %s\n\nIt has been recorded in Terraform state so that you can destroy it.",
				id, err))
		return cleanupFailed, diags
	}
	return cleanupSucceeded, diags
}

// handleCreatedMode runs verifyCreatedMode and applies its outcome to the create
// response, reporting whether Create must stop. The two cleanup outcomes take
// opposite state handling and it is easy to implement backwards, so they live
// together here rather than inline in Create.
func handleCreatedMode(
	ctx context.Context,
	client stackClient,
	stack *zenfraclient.Stack,
	requested string,
	resp *resource.CreateResponse,
) (stop bool) {
	outcome, diags := verifyCreatedMode(ctx, client, stack.ID, requested, stack.StateManagement)
	resp.Diagnostics.Append(diags...)
	switch outcome {
	case cleanupSucceeded:
		return true // the stack is gone; leave state null so Terraform tracks nothing
	case cleanupFailed:
		// The orphan survives, so it MUST be recorded or it is untracked forever.
		// mapStackToState is fed the server's stack, so the recorded mode is the
		// real one, not the requested one. Its diagnostics are appended, never
		// dropped: losing them here would hide why the orphan went unrecorded.
		state, mapDiags := mapStackToState(ctx, stack)
		resp.Diagnostics.Append(mapDiags...)
		if !mapDiags.HasError() {
			resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
		}
		return true
	case cleanupNotNeeded:
	}
	return false
}
