// ABOUTME: Unit tests for the label and hook validators.
// ABOUTME: Pins the API's label and hook contract as the provider enforces it at plan time.
package validate

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func stringSet(t *testing.T, values ...string) types.Set {
	t.Helper()
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	s, diags := types.SetValue(types.StringType, elems)
	if diags.HasError() {
		t.Fatalf("SetValue: %v", diags)
	}
	return s
}

func stringList(t *testing.T, values ...string) types.List {
	t.Helper()
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	l, diags := types.ListValue(types.StringType, elems)
	if diags.HasError() {
		t.Fatalf("ListValue: %v", diags)
	}
	return l
}

func TestLabelError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		label   string
		wantErr string
	}{
		{label: "prod"},
		{label: "team.payments"},
		{label: "eu-west_1"},
		{label: "0abc"},
		{label: "a" + strings.Repeat("b", 62)},
		{label: "a" + strings.Repeat("b", 63), wantErr: "1-63 characters"},
		{label: "Prod", wantErr: "must be lowercase"},
		{label: "PROD", wantErr: `use "prod"`},
		{label: "", wantErr: "blank"},
		{label: "   ", wantErr: "blank"},
		{label: " prod", wantErr: "surrounding whitespace"},
		{label: "prod ", wantErr: "surrounding whitespace"},
		{label: "-prod", wantErr: "starting with a letter or digit"},
		{label: ".prod", wantErr: "starting with a letter or digit"},
		{label: "pr od", wantErr: "1-63 characters"},
		{label: "prod/eu", wantErr: "1-63 characters"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.label), func(t *testing.T) {
			t.Parallel()
			got := LabelError(tt.label)
			if tt.wantErr == "" {
				if got != "" {
					t.Errorf("LabelError(%q) = %q, want valid", tt.label, got)
				}
				return
			}
			if !strings.Contains(got, tt.wantErr) {
				t.Errorf("LabelError(%q) = %q, want it to mention %q", tt.label, got, tt.wantErr)
			}
		})
	}
}

func TestLabels_Set(t *testing.T) {
	t.Parallel()

	many := make([]string, 0, MaxLabels+1)
	for i := range MaxLabels + 1 {
		many = append(many, fmt.Sprintf("l%d", i))
	}

	tests := []struct {
		name      string
		value     types.Set
		wantError bool
	}{
		{name: "null", value: types.SetNull(types.StringType)},
		{name: "unknown", value: types.SetUnknown(types.StringType)},
		{name: "empty", value: stringSet(t)},
		{name: "valid", value: stringSet(t, "prod", "team.payments")},
		{name: "exactly the cap", value: stringSet(t, many[:MaxLabels]...)},
		{name: "over the cap", value: stringSet(t, many...), wantError: true},
		{name: "uppercase", value: stringSet(t, "prod", "Team"), wantError: true},
		{name: "unknown element", value: types.SetValueMust(types.StringType, []attr.Value{types.StringUnknown()})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp := &validator.SetResponse{}
			Labels().ValidateSet(context.Background(), validator.SetRequest{
				Path:        path.Root("labels"),
				ConfigValue: tt.value,
			}, resp)
			if got := resp.Diagnostics.HasError(); got != tt.wantError {
				t.Errorf("HasError = %v, want %v: %v", got, tt.wantError, resp.Diagnostics)
			}
		})
	}
}

func TestHookCommandError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cmd     string
		wantErr string
	}{
		{name: "plain", cmd: "terraform fmt -check"},
		{name: "multi-line script", cmd: "set -e\necho hi"},
		{name: "exactly the byte cap", cmd: strings.Repeat("x", MaxHookCommandBytes)},
		{name: "over the byte cap", cmd: strings.Repeat("x", MaxHookCommandBytes+1), wantErr: "at most 4096 bytes"},
		{name: "multi-byte counted in bytes", cmd: strings.Repeat("é", MaxHookCommandBytes/2+1), wantErr: "at most 4096 bytes"},
		{name: "empty", cmd: "", wantErr: "blank"},
		{name: "whitespace only", cmd: " \t\n", wantErr: "blank"},
		{name: "NUL byte", cmd: "echo a\x00b", wantErr: "NUL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := HookCommandError(tt.cmd)
			if tt.wantErr == "" {
				if got != "" {
					t.Errorf("HookCommandError = %q, want valid", got)
				}
				return
			}
			if !strings.Contains(got, tt.wantErr) {
				t.Errorf("HookCommandError = %q, want it to mention %q", got, tt.wantErr)
			}
		})
	}
}

func TestHookCommands_List(t *testing.T) {
	t.Parallel()

	tooMany := make([]string, MaxHookCommandsPerPhase+1)
	for i := range tooMany {
		tooMany[i] = "echo ok"
	}

	tests := []struct {
		name      string
		value     types.List
		wantError string
	}{
		{name: "null", value: types.ListNull(types.StringType)},
		{name: "unknown", value: types.ListUnknown(types.StringType)},
		{name: "one command", value: stringList(t, "make lint")},
		{name: "exactly the cap", value: stringList(t, tooMany[:MaxHookCommandsPerPhase]...)},
		{name: "empty list", value: stringList(t), wantError: "Empty Hook Phase"},
		{name: "over the cap", value: stringList(t, tooMany...), wantError: "Too Many Hook Commands"},
		{name: "blank command", value: stringList(t, "echo ok", "  "), wantError: "Invalid Hook Command"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp := &validator.ListResponse{}
			HookCommands().ValidateList(context.Background(), validator.ListRequest{
				Path:        path.Root("hooks").AtName("before_plan"),
				ConfigValue: tt.value,
			}, resp)
			if tt.wantError == "" {
				if resp.Diagnostics.HasError() {
					t.Errorf("unexpected error: %v", resp.Diagnostics)
				}
				return
			}
			if !resp.Diagnostics.HasError() || resp.Diagnostics.Errors()[0].Summary() != tt.wantError {
				t.Errorf("diagnostics = %v, want %q", resp.Diagnostics, tt.wantError)
			}
		})
	}
}

func TestHooksNotEmpty(t *testing.T) {
	t.Parallel()

	attrTypes := map[string]attr.Type{
		"before_plan": types.ListType{ElemType: types.StringType},
		"after_apply": types.ListType{ElemType: types.StringType},
	}
	allNull := types.ObjectValueMust(attrTypes, map[string]attr.Value{
		"before_plan": types.ListNull(types.StringType),
		"after_apply": types.ListNull(types.StringType),
	})
	onePhase := types.ObjectValueMust(attrTypes, map[string]attr.Value{
		"before_plan": stringList(t, "make lint"),
		"after_apply": types.ListNull(types.StringType),
	})
	unknownPhase := types.ObjectValueMust(attrTypes, map[string]attr.Value{
		"before_plan": types.ListUnknown(types.StringType),
		"after_apply": types.ListNull(types.StringType),
	})

	tests := []struct {
		name      string
		value     types.Object
		wantError bool
	}{
		{name: "null object", value: types.ObjectNull(attrTypes)},
		{name: "unknown object", value: types.ObjectUnknown(attrTypes)},
		{name: "one phase", value: onePhase},
		{name: "unknown phase", value: unknownPhase},
		{name: "no phase", value: allNull, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp := &validator.ObjectResponse{}
			HooksNotEmpty().ValidateObject(context.Background(), validator.ObjectRequest{
				Path:        path.Root("hooks"),
				ConfigValue: tt.value,
			}, resp)
			if got := resp.Diagnostics.HasError(); got != tt.wantError {
				t.Errorf("HasError = %v, want %v: %v", got, tt.wantError, resp.Diagnostics)
			}
		})
	}
}

func TestNonNegative(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     types.Int64
		wantError bool
	}{
		{name: "null", value: types.Int64Null()},
		{name: "unknown", value: types.Int64Unknown()},
		{name: "zero", value: types.Int64Value(0)},
		{name: "positive", value: types.Int64Value(7)},
		{name: "negative", value: types.Int64Value(-1), wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp := &validator.Int64Response{}
			NonNegative().ValidateInt64(context.Background(), validator.Int64Request{
				Path:        path.Root("priority"),
				ConfigValue: tt.value,
			}, resp)
			if got := resp.Diagnostics.HasError(); got != tt.wantError {
				t.Errorf("HasError = %v, want %v: %v", got, tt.wantError, resp.Diagnostics)
			}
		})
	}
}
