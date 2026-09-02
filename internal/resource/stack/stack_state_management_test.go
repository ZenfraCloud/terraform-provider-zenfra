// ABOUTME: Unit tests for the create-only state_management attribute on zenfra_stack.
// ABOUTME: Covers the transition guard, the mode validator and Create's mismatch compensation.

package stack

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// nonNullRaw is any non-null object; the guard only checks null-ness of the raw
// plan and state to tell creation and destroy apart.
func nonNullRaw() tftypes.Value {
	return tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{"id": tftypes.String}},
		map[string]tftypes.Value{"id": tftypes.NewValue(tftypes.String, "s1")})
}

func nullRaw() tftypes.Value {
	return tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{"id": tftypes.String}}, nil)
}

func runGuard(t *testing.T, rawState, rawPlan tftypes.Value, state, config types.String) planmodifier.StringResponse {
	t.Helper()
	req := planmodifier.StringRequest{
		Path:        path.Root("state_management"),
		State:       tfsdk.State{Raw: rawState},
		Plan:        tfsdk.Plan{Raw: rawPlan},
		StateValue:  state,
		ConfigValue: config,
	}
	resp := planmodifier.StringResponse{PlanValue: config}
	stateManagementGuard{}.PlanModifyString(context.Background(), req, &resp)
	return resp
}

func TestStateManagementGuard_RefusesBothTransitions(t *testing.T) {
	for name, tc := range map[string]struct{ from, to string }{
		"managed to external": {"managed", "external"},
		"external to managed": {"external", "managed"},
	} {
		t.Run(name, func(t *testing.T) {
			resp := runGuard(t, nonNullRaw(), nonNullRaw(),
				types.StringValue(tc.from), types.StringValue(tc.to))
			if !resp.Diagnostics.HasError() {
				t.Fatalf("%s must be refused", name)
			}
			detail := resp.Diagnostics.Errors()[0].Detail()
			for _, want := range []string{"destroy", tc.from, tc.to} {
				if !strings.Contains(detail, want) {
					t.Errorf("error must mention %q, got: %s", want, detail)
				}
			}
		})
	}
}

func TestStateManagementGuard_AllowsEverythingElse(t *testing.T) {
	cases := map[string]struct {
		rawState, rawPlan tftypes.Value
		state, config     types.String
	}{
		// Creation: no prior state to compare against, and taint reaches us this way too.
		"create":         {nullRaw(), nonNullRaw(), types.StringNull(), types.StringValue("external")},
		"destroy":        {nonNullRaw(), nullRaw(), types.StringValue("external"), types.StringNull()},
		"unchanged":      {nonNullRaw(), nonNullRaw(), types.StringValue("external"), types.StringValue("external")},
		"omitted config": {nonNullRaw(), nonNullRaw(), types.StringValue("external"), types.StringNull()},
		"unknown config": {nonNullRaw(), nonNullRaw(), types.StringValue("external"), types.StringUnknown()},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if resp := runGuard(t, tc.rawState, tc.rawPlan, tc.state, tc.config); resp.Diagnostics.HasError() {
				t.Errorf("%s must be allowed: %v", name, resp.Diagnostics)
			}
		})
	}
}

func TestStateManagementValidator(t *testing.T) {
	at := path.Root("state_management")
	for _, mode := range []string{"managed", "external"} {
		if diags := validateMode(at, types.StringValue(mode)); diags.HasError() {
			t.Errorf("%q must be accepted", mode)
		}
	}
	for _, mode := range []string{"", "Managed", "MANAGED", "manged", "s3"} {
		if diags := validateMode(at, types.StringValue(mode)); !diags.HasError() {
			t.Errorf("%q must be rejected", mode)
		}
	}
	// Null and unknown are for the schema and Create to judge, not the validator.
	for _, v := range []types.String{types.StringNull(), types.StringUnknown()} {
		if diags := validateMode(at, v); diags.HasError() {
			t.Errorf("%v must pass the validator untouched", v)
		}
	}
}
