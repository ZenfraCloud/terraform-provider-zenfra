// ABOUTME: Unit tests for the create-only state_management attribute on zenfra_stack.
// ABOUTME: Covers the transition guard, the mode validator and Create's mismatch compensation.

package stack

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
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

// State written before this attribute existed decodes with a null value against
// the current schema, while the resource state itself is non-null. That stack is
// managed - external did not exist yet - so an explicit external must still be
// refused, not waved through because the prior attribute happens to be null.
func TestStateManagementGuard_RefusesExternalAgainstPreFeatureState(t *testing.T) {
	resp := runGuard(t, nonNullRaw(), nonNullRaw(), types.StringNull(), types.StringValue("external"))
	if !resp.Diagnostics.HasError() {
		t.Fatal("a pre-feature stack is managed and must not be changed to external in place")
	}
	detail := resp.Diagnostics.Errors()[0].Detail()
	for _, want := range []string{"managed", "external"} {
		if !strings.Contains(detail, want) {
			t.Errorf("error must mention %q, got: %s", want, detail)
		}
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
		// A pre-feature stack is managed, so asking for managed is not a change.
		"pre-feature state, managed configured": {
			nonNullRaw(), nonNullRaw(), types.StringNull(), types.StringValue("managed"),
		},
		"pre-feature state, omitted config": {
			nonNullRaw(), nonNullRaw(), types.StringNull(), types.StringNull(),
		},
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

// --- Task 3: Create ---------------------------------------------------------

// fakeStackClient is a stackClient that records what Create did. Any call a test
// did not arrange for panics, so an unexpected API call cannot pass silently.
type fakeStackClient struct {
	created    *zenfraclient.Stack
	createErr  error
	createReqs []zenfraclient.CreateStackRequest
	deleted    string
	deleteErr  error
}

func (f *fakeStackClient) CreateStack(
	_ context.Context, req zenfraclient.CreateStackRequest,
) (*zenfraclient.Stack, error) {
	f.createReqs = append(f.createReqs, req)
	return f.created, f.createErr
}

func (f *fakeStackClient) DeleteStack(_ context.Context, id string) error {
	f.deleted = id
	return f.deleteErr
}

func (f *fakeStackClient) GetStack(context.Context, string) (*zenfraclient.Stack, error) {
	panic("GetStack: unexpected call")
}

func (f *fakeStackClient) UpdateStack(
	context.Context, string, zenfraclient.UpdateStackRequest,
) (*zenfraclient.Stack, error) {
	panic("UpdateStack: unexpected call")
}

func (f *fakeStackClient) SetStackSource(context.Context, string, zenfraclient.StackSource) error {
	panic("SetStackSource: unexpected call")
}

func (f *fakeStackClient) SetStackTriggers(context.Context, string, zenfraclient.StackTriggers) error {
	panic("SetStackTriggers: unexpected call")
}

// createdStack is what the API really returns: mapStackToState refuses a stack
// with no source, so a bare &Stack{ID: "s1"} would not exercise state retention.
func createdStack(mode string) *zenfraclient.Stack {
	return &zenfraclient.Stack{
		ID:              "s1",
		OrganizationID:  "org-1",
		SpaceID:         "space-1",
		Name:            "my-stack",
		AllowPublicPool: true,
		StateManagement: &zenfraclient.StateManagement{Mode: mode},
		IAC:             zenfraclient.IACConfig{Engine: "terraform", Version: "1.9.0"},
		Source: zenfraclient.StackSource{
			Type: "raw_git",
			RawGit: &zenfraclient.StackSourceRawGit{
				URL:  "https://example.com/repo.git",
				Ref:  zenfraclient.StackSourceRef{Type: "branch", Name: "main"},
				Path: ".",
			},
		},
	}
}

func TestCreate_RequestedMode(t *testing.T) {
	at := path.Root("state_management")
	for name, tc := range map[string]struct {
		config types.String
		want   *zenfraclient.StateManagement
	}{
		// Omitted config is unknown in the create PLAN, so Create reads the CONFIG,
		// where an omitted attribute is null and means "let the server default".
		"omitted":  {types.StringNull(), nil},
		"managed":  {types.StringValue("managed"), &zenfraclient.StateManagement{Mode: "managed"}},
		"external": {types.StringValue("external"), &zenfraclient.StateManagement{Mode: "external"}},
	} {
		t.Run(name, func(t *testing.T) {
			got, diags := stateManagementForCreate(at, tc.config)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			switch {
			case tc.want == nil && got != nil:
				t.Errorf("got %+v, want no block", got)
			case tc.want != nil && got == nil:
				t.Errorf("got no block, want %+v", tc.want)
			case tc.want != nil && got.Mode != tc.want.Mode:
				t.Errorf("mode = %q, want %q", got.Mode, tc.want.Mode)
			}
		})
	}
}

// An unknown mode must NOT collapse into null. Folding it in would silently
// create a managed stack for someone who asked for something unresolvable.
func TestCreate_UnknownModeIsRefusedBeforeAnyCall(t *testing.T) {
	block, diags := stateManagementForCreate(path.Root("state_management"), types.StringUnknown())
	if !diags.HasError() {
		t.Fatal("an unknown mode must be refused, not treated as omitted")
	}
	if block != nil {
		t.Error("no request block may be built from an unknown value")
	}
}

func TestCreate_OldControlPlaneOmittingTheField(t *testing.T) {
	// External requested, server returns no block at all: the field was dropped
	// by binding on a control plane older than the feature.
	fake := &fakeStackClient{created: &zenfraclient.Stack{ID: "s1"}}
	outcome, diags := verifyCreatedMode(context.Background(), fake, "s1", "external", nil)

	if outcome != cleanupSucceeded {
		t.Errorf("outcome = %v, want cleanupSucceeded", outcome)
	}
	if !diags.HasError() {
		t.Fatal("a silently downgraded external stack must fail loudly")
	}
	detail := strings.ToLower(diags.Errors()[0].Detail())
	if !strings.Contains(detail, "upgrade") {
		t.Errorf("error must point at the control plane version, got: %s", detail)
	}
	if fake.deleted != "s1" {
		t.Error("the orphaned stack must be deleted")
	}
}

func TestCreate_WrongModeReturned(t *testing.T) {
	fake := &fakeStackClient{}
	outcome, diags := verifyCreatedMode(context.Background(), fake, "s1", "external",
		&zenfraclient.StateManagement{Mode: "managed"})

	if outcome != cleanupSucceeded {
		t.Errorf("outcome = %v, want cleanupSucceeded", outcome)
	}
	if !diags.HasError() {
		t.Fatal("a returned mode that differs from the requested one must fail")
	}
	if strings.Contains(strings.ToLower(diags.Errors()[0].Detail()), "upgrade") {
		t.Error("a present-but-wrong mode is not the old-API case and must not say upgrade")
	}
	if fake.deleted != "s1" {
		t.Error("the orphaned stack must be deleted")
	}
}

func TestCreate_OmittedAndManagedAgainstOldControlPlane(t *testing.T) {
	// nil means managed under the effective-mode contract, so this is not an error.
	fake := &fakeStackClient{}
	outcome, diags := verifyCreatedMode(context.Background(), fake, "s1", "managed", nil)
	if diags.HasError() {
		t.Errorf("managed requested and nil returned is consistent: %v", diags)
	}
	if outcome != cleanupNotNeeded {
		t.Errorf("outcome = %v, want cleanupNotNeeded", outcome)
	}
	if fake.deleted != "" {
		t.Error("nothing to clean up")
	}
}

func TestCreate_CleanupFailureKeepsState(t *testing.T) {
	fake := &fakeStackClient{deleteErr: errors.New("boom")}
	outcome, diags := verifyCreatedMode(context.Background(), fake, "s1", "external",
		&zenfraclient.StateManagement{Mode: "managed"})

	if outcome != cleanupFailed {
		t.Fatalf("outcome = %v, want cleanupFailed; Create keys state retention off this", outcome)
	}
	if len(diags.Errors()) < 2 {
		t.Error("both the contract failure and the cleanup failure must be reported")
	}
	if !strings.Contains(diags.Errors()[1].Detail(), "boom") {
		t.Error("the cleanup failure must name the underlying error")
	}
}

// --- Create end to end, where the two cleanup outcomes take opposite state
// handling and it is easy to implement backwards ------------------------------

// stackTestSchema is the real resource schema, so the plan and config values the
// tests build are exactly the ones the framework would hand Create.
func stackTestSchema(t *testing.T) schema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	(&StackResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", resp.Diagnostics)
	}
	return resp.Schema
}

// stackCreateValues builds a minimal but complete plan and config for a create.
// The plan carries an unknown state_management, which is what the framework
// produces for an omitted Optional+Computed attribute; the config carries what
// the practitioner actually wrote.
func stackCreateValues(t *testing.T, configured types.String) (tfsdk.Plan, tfsdk.Config) {
	t.Helper()
	ctx := context.Background()
	s := stackTestSchema(t)

	iac, d := types.ObjectValueFrom(ctx, IACModelAttrTypes,
		&IACModel{Engine: types.StringValue("terraform"), Version: types.StringValue("1.9.0")})
	if d.HasError() {
		t.Fatalf("iac: %v", d)
	}
	ref, d := types.ObjectValueFrom(ctx, RefModelAttrTypes,
		&RefModel{Type: types.StringValue("branch"), Name: types.StringValue("main")})
	if d.HasError() {
		t.Fatalf("ref: %v", d)
	}
	rawGit, d := types.ObjectValueFrom(ctx, RawGitModelAttrTypes, &RawGitModel{
		URL: types.StringValue("https://example.com/repo.git"), Ref: ref, Path: types.StringValue("."),
	})
	if d.HasError() {
		t.Fatalf("raw_git: %v", d)
	}
	source, d := types.ObjectValueFrom(ctx, SourceModelAttrTypes, &SourceModel{
		Type:   types.StringValue("raw_git"),
		RawGit: rawGit,
		VCS:    types.ObjectNull(VCSModelAttrTypes),
	})
	if d.HasError() {
		t.Fatalf("source: %v", d)
	}

	model := StackModel{
		ID:              types.StringUnknown(),
		OrganizationID:  types.StringUnknown(),
		SpaceID:         types.StringValue("space-1"),
		Name:            types.StringValue("my-stack"),
		WorkerPoolID:    types.StringNull(),
		AllowPublicPool: types.BoolValue(true),
		StateManagement: types.StringUnknown(),
		IAC:             iac,
		Source:          source,
		Triggers:        types.ObjectNull(TriggersModelAttrTypes),
		CreatedAt:       types.StringUnknown(),
		UpdatedAt:       types.StringUnknown(),
		CreatedBy:       types.StringUnknown(),
		UpdatedBy:       types.StringUnknown(),
	}

	plan := tfsdk.Plan{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	if d := plan.Set(ctx, model); d.HasError() {
		t.Fatalf("plan: %v", d)
	}

	model.StateManagement = configured
	cfg := tfsdk.Config{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	cfgPlan := tfsdk.Plan{Schema: s, Raw: cfg.Raw}
	if d := cfgPlan.Set(ctx, model); d.HasError() {
		t.Fatalf("config: %v", d)
	}
	cfg.Raw = cfgPlan.Raw

	return plan, cfg
}

func runCreate(t *testing.T, client stackClient, configured types.String) resource.CreateResponse {
	t.Helper()
	ctx := context.Background()
	s := stackTestSchema(t)
	plan, cfg := stackCreateValues(t, configured)

	r := &StackResource{client: client}
	resp := resource.CreateResponse{
		State: tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)},
	}
	r.Create(ctx, resource.CreateRequest{Plan: plan, Config: cfg}, &resp)
	return resp
}

// The unknown-mode refusal at the Create level: nothing may reach the server.
func TestCreate_UnknownModeCreatesNothing(t *testing.T) {
	fake := &fakeStackClient{}
	resp := runCreate(t, fake, types.StringUnknown())

	if !resp.Diagnostics.HasError() {
		t.Fatal("an unknown state_management must fail the create")
	}
	if len(fake.createReqs) != 0 {
		t.Errorf("CreateStack was called %d times; nothing may be created", len(fake.createReqs))
	}
	if !resp.State.Raw.IsNull() {
		t.Error("no state may be set when nothing was created")
	}
}

func TestCreate_CleanupSuccessSetsNoState(t *testing.T) {
	fake := &fakeStackClient{created: createdStack("managed")}
	resp := runCreate(t, fake, types.StringValue("external"))

	if !resp.Diagnostics.HasError() {
		t.Fatal("a wrong returned mode must fail the create")
	}
	if fake.deleted != "s1" {
		t.Errorf("deleted = %q, want s1", fake.deleted)
	}
	if !resp.State.Raw.IsNull() {
		t.Error("the stack is gone, so Terraform must track nothing")
	}
}

func TestCreate_CleanupFailureSetsReturnedMode(t *testing.T) {
	fake := &fakeStackClient{created: createdStack("managed"), deleteErr: errors.New("boom")}
	resp := runCreate(t, fake, types.StringValue("external"))

	if !resp.Diagnostics.HasError() {
		t.Fatal("a wrong returned mode must fail the create")
	}
	if resp.State.Raw.IsNull() {
		t.Fatal("the orphan survives, so it MUST be recorded or it is untracked forever")
	}
	var got StackModel
	if d := resp.State.Get(context.Background(), &got); d.HasError() {
		t.Fatalf("state: %v", d)
	}
	if got.ID.ValueString() != "s1" {
		t.Errorf("id = %q, want s1 so the orphan can be destroyed", got.ID.ValueString())
	}
	if got.StateManagement.ValueString() != "managed" {
		t.Errorf("state_management = %q, want the mode the server actually returned",
			got.StateManagement.ValueString())
	}
}

// The requested mode really does reach the wire.
func TestCreate_SendsRequestedModeAndKeepsState(t *testing.T) {
	fake := &fakeStackClient{created: createdStack("external")}
	resp := runCreate(t, fake, types.StringValue("external"))

	if resp.Diagnostics.HasError() {
		t.Fatalf("a matching mode must succeed: %v", resp.Diagnostics)
	}
	if len(fake.createReqs) != 1 {
		t.Fatalf("CreateStack called %d times, want 1", len(fake.createReqs))
	}
	sm := fake.createReqs[0].StateManagement
	if sm == nil || sm.Mode != "external" {
		t.Errorf("request carried %+v, want mode external", sm)
	}
	if fake.deleted != "" {
		t.Errorf("nothing to clean up, but deleted %q", fake.deleted)
	}
}
