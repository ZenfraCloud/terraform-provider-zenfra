// ABOUTME: Tests for refusing an empty secret (zenfra-cloud#865): at plan through ValidateConfig, and at apply
// ABOUTME: before any request, so a value that resolves empty leaves no bundle and writes nothing.
package bundle

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func entrySet(t *testing.T, attrTypes map[string]attr.Type, idAttr, valueAttr string, entries ...[3]attr.Value) types.Set {
	t.Helper()
	elems := make([]attr.Value, 0, len(entries))
	for _, e := range entries {
		obj, d := types.ObjectValue(attrTypes, map[string]attr.Value{
			idAttr: e[0], valueAttr: e[1], "secret": e[2], "description": types.StringNull(),
		})
		if d.HasError() {
			t.Fatal(d)
		}
		elems = append(elems, obj)
	}
	set, d := types.SetValue(types.ObjectType{AttrTypes: attrTypes}, elems)
	if d.HasError() {
		t.Fatal(d)
	}
	return set
}

func envVars(t *testing.T, entries ...[3]attr.Value) types.Set {
	t.Helper()
	return entrySet(t, envVarAttrTypes(), "key", "value", entries...)
}

func files(t *testing.T, entries ...[3]attr.Value) types.Set {
	t.Helper()
	return entrySet(t, mountedFileAttrTypes(), "path", "content", entries...)
}

func entry(id string, value, secret attr.Value) [3]attr.Value {
	return [3]attr.Value{types.StringValue(id), value, secret}
}

var (
	str   = types.StringValue
	yes   = types.BoolValue(true)
	no    = types.BoolValue(false)
	unset = types.BoolNull() // secret not written in the configuration
)

func validateConfig(t *testing.T, h *bundleHarness, model BundleModel) []string {
	t.Helper()
	ctx := context.Background()
	plan := tfsdk.Plan{Schema: h.schema.Schema}
	if d := plan.Set(ctx, model); d.HasError() {
		t.Fatal(d)
	}
	resp := resource.ValidateConfigResponse{}
	h.r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: h.schema.Schema, Raw: plan.Raw}}, &resp)
	var details []string
	for _, d := range resp.Diagnostics.Errors() {
		details = append(details, d.Detail())
	}
	return details
}

func TestValidateConfig_EmptySecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		env   func(t *testing.T) types.Set
		files func(t *testing.T) types.Set
		want  string // "" = valid
	}{
		{name: "secret variable with an empty value", env: func(t *testing.T) types.Set { return envVars(t, entry("TOKEN", str(""), yes)) }, want: `environment_variable "TOKEN" is secret and its value is empty`},
		{name: "secret file with empty content", files: func(t *testing.T) types.Set { return files(t, entry("/etc/key.pem", str(""), yes)) }, want: `mounted_file "/etc/key.pem" is secret and its content is empty`},
		{name: "plain empty variable", env: func(t *testing.T) types.Set { return envVars(t, entry("EMPTY", str(""), no)) }},
		{name: "secret flag left out", env: func(t *testing.T) types.Set { return envVars(t, entry("EMPTY", str(""), unset)) }},
		{name: "whitespace is a value", env: func(t *testing.T) types.Set { return envVars(t, entry("TOKEN", str("  "), yes)) }},
		{name: "unknown value is deferred", env: func(t *testing.T) types.Set { return envVars(t, entry("TOKEN", types.StringUnknown(), yes)) }},
		{name: "unknown flag is deferred", files: func(t *testing.T) types.Set { return files(t, entry("/k", str(""), types.BoolUnknown())) }},
		{name: "unknown collection is deferred", env: func(*testing.T) types.Set { return types.SetUnknown(types.ObjectType{AttrTypes: envVarAttrTypes()}) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newBundleHarness(t)
			model := baseModel()
			if tt.env != nil {
				model.EnvironmentVariable = tt.env(t)
			}
			if tt.files != nil {
				model.MountedFile = tt.files(t)
			}
			details := validateConfig(t, h, model)
			if tt.want == "" {
				if len(details) != 0 {
					t.Fatalf("unexpected errors: %v", details)
				}
				return
			}
			if len(details) != 1 || !strings.Contains(details[0], tt.want) {
				t.Fatalf("errors = %v, want one containing %q", details, tt.want)
			}
		})
	}
}

// A value unknown at plan that resolves empty is refused at apply before
// the bundle is created: no request at all.
func TestCreate_EmptySecretAtApplyMakesNoRequest(t *testing.T) {
	t.Parallel()

	h := newBundleHarness(t)
	planned := baseModel()
	planned.ID = types.StringUnknown()
	planned.EnvironmentVariable = envVars(t, entry("REGION", str("eu"), no), entry("TOKEN", str(""), yes))

	ctx := context.Background()
	plan := tfsdk.Plan{Schema: h.schema.Schema}
	if d := plan.Set(ctx, planned); d.HasError() {
		t.Fatal(d)
	}
	resp := resource.CreateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	h.r.Create(ctx, resource.CreateRequest{Plan: plan}, &resp)

	if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), `"TOKEN"`) {
		t.Fatalf("diagnostics = %v, want the TOKEN refusal", resp.Diagnostics)
	}
	if len(h.fake.order) != 0 {
		t.Errorf("requests = %v, want none", h.fake.order)
	}
	if !resp.State.Raw.IsNull() {
		t.Error("state was written for a bundle that was never created")
	}
}

// A combined metadata and content update with a secret that resolved empty
// writes neither.
func TestUpdate_EmptySecretAtApplyMakesNoRequest(t *testing.T) {
	t.Parallel()

	h := newBundleHarness(t)
	prior := baseModel()
	prior.Description = types.StringValue("before")
	prior.MountedFile = files(t, entry("/etc/key.pem", str("pem"), yes))
	planned := prior
	planned.Description = types.StringValue("after")
	planned.MountedFile = files(t, entry("/etc/key.pem", str(""), yes))

	ctx := context.Background()
	state := tfsdk.State{Schema: h.schema.Schema}
	if d := state.Set(ctx, prior); d.HasError() {
		t.Fatal(d)
	}
	plan := tfsdk.Plan{Schema: h.schema.Schema}
	if d := plan.Set(ctx, planned); d.HasError() {
		t.Fatal(d)
	}
	resp := resource.UpdateResponse{State: state}
	h.r.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, &resp)

	if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), `"/etc/key.pem"`) {
		t.Fatalf("diagnostics = %v, want the /etc/key.pem refusal", resp.Diagnostics)
	}
	if len(h.fake.order) != 0 {
		t.Errorf("requests = %v, want none", h.fake.order)
	}
}

// An unchanged secret is re-sent with its value from the configuration when
// another entry changes, as before.
func TestUpdate_UnchangedSecretIsResent(t *testing.T) {
	t.Parallel()

	h := newBundleHarness(t)
	prior := baseModel()
	prior.EnvironmentVariable = envVars(t, entry("REGION", str("eu"), no), entry("TOKEN", str("fixture-token"), yes))
	planned := prior
	planned.EnvironmentVariable = envVars(t, entry("REGION", str("us"), no), entry("TOKEN", str("fixture-token"), yes))
	h.update(t, prior, planned)

	if len(h.fake.contentBodies) != 1 {
		t.Fatalf("content writes = %d, want 1", len(h.fake.contentBodies))
	}
	var content struct {
		EnvironmentVariables []struct {
			Key    string `json:"key"`
			Value  string `json:"value"`
			Secret bool   `json:"secret"`
		} `json:"environment_variables"`
	}
	if err := json.Unmarshal(h.fake.contentBodies[0]["content"], &content); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range content.EnvironmentVariables {
		if ev.Key != "TOKEN" {
			continue
		}
		found = true
		if !ev.Secret || ev.Value != "fixture-token" {
			t.Errorf("TOKEN sent as secret=%v without its configured value", ev.Secret)
		}
	}
	if !found {
		t.Error("TOKEN missing from the content write")
	}
}
