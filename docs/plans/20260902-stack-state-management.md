# `zenfra_stack` state ownership attribute — implementation plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Expose the create-only state-ownership mode on `zenfra_stack`, so a practitioner can declare a stack that keeps its own Terraform backend, without the provider ever making an unsupported and destructive mode conversion look routine.

**Architecture:** One string attribute, `Optional + Computed`, carried through the client as a pointer so an absent field is distinguishable from an explicit value. A custom plan modifier refuses any in-place mode change instead of forcing replacement. Create derives the requested mode from configuration rather than the plan, compares it against what the server actually returned, and compensates by deleting the stack it just made when they disagree.

**Tech Stack:** Go 1.25, terraform-plugin-framework v1.19.0. One new dependency, `terraform-plugin-testing`, and only for Task 5; the runtime code adds none.

**Upstream:** This is Task 8 of `zenfra-cloud/docs/plans/completed/20260902-external-state-backend.md` (issue #684). The API half is PR #686. Read D4, D7 and Task 12 of that plan before starting; this plan assumes them.

---

## Context — verified in the provider at `f9b1fc5`

- `StackModel` has no state field (`internal/resource/stack/stack_model.go:12-24`), and neither does `zenfraclient.Stack` or `CreateStackRequest` (`internal/zenfraclient/types.go:104-131`).
- `Create` POSTs and maps the response straight to state (`internal/resource/stack/stack_resource.go:217-282`); there is no post-create verification of anything.
- `ImportState` is ID passthrough (`internal/resource/stack/stack_resource.go:458-461`), so import is served by `Read`.
- The single-stack data source maps a detailed stack (`internal/datasource/stack/stack_data_source.go:20-33`); the list data source deliberately returns only id, name, space and org (`internal/datasource/stack/stacks_data_source.go:25-30`).
- No validator module is a dependency (`go.mod:5` has only the framework), and no resource uses `Validators` today.
- `make docs` regenerates `providers-schema.json` and then runs tfplugindocs (`GNUmakefile:30-37`), but CI only fails on a stale `docs` directory (`.github/workflows/check-documentation.yml:74-81`). The root schema file is not independently gated.

**The API guarantees a non-nil block on every stack response.** Every serialization site canonicalizes a legacy row to `{"mode":"managed"}` (`zenfra-cloud/zenfra-api/internal/handler/stack.go:84-98`). A nil therefore means a control plane older than the feature, not a legacy row.

## Decisions

**P1 — No `RequiresReplace`; refuse the transition instead.**
`RequiresReplace` is exactly the wrong tool. Its own source skips creation and destroy and only marks a changed attribute for replacement (`stringplanmodifier/requires_replace_if.go:53-59`), which is the destructive half we are trying not to offer. Replacing a managed stack soft-deletes it (`zenfra-cloud/zenfra-api/internal/service/stack.go:342-346`) and its managed state becomes inaccessible, with no self-service export. Replacing an external stack is no better: it creates an empty managed lineage for infrastructure that still exists. So a custom plan modifier raises an attribute error when prior state and configured mode are both known and differ, and the practitioner performs an explicit destroy and a separate create around an operator-assisted migration.

**P2 — `Optional + Computed` with `UseStateForUnknown`, and Create reads the configuration.**
`Computed` is what lets the provider supply the canonical value the API always returns; without it an omitted attribute is a permanent diff. `UseStateForUnknown` preserves a known prior value on update, which is what stops an omitted configuration from trying to drive an imported external stack toward managed. But it is documented and implemented as a no-op when prior state is null (`stringplanmodifier/use_state_for_unknown.go:44-46`), so **on create the attribute is unknown in the plan**. Create must read `req.Config`, treat null as `managed`, and **refuse a still-unknown value rather than folding it into null**. Unknown means the practitioner wrote something the plan could not resolve; sending no block would silently turn that intent into `managed`. A static schema default would be wrong for the same reason: it would drive an imported external stack toward managed.

**P3 — A pointer in the client, so presence and effective mode stay separate concepts.**
`Stack.StateManagement` and `CreateStackRequest.StateManagement` are pointers. `EffectiveStateMode(nil)` is `managed`, used by Read, import and both data sources. Create keeps the raw pointer so it can tell a control plane that dropped the field from one that honestly returned a different mode. A pointer collapses JSON `absent` and JSON `null`, which is acceptable: the API guarantees a non-nil object, so either raw form is the same old-server signal.

**P4 — A create-time mismatch is compensated, and the two outcomes take opposite state handling.**
The mismatch is only visible after the POST has already created a stack, so returning a bare diagnostic orphans a server object. Attempt `DeleteStack` immediately. On cleanup success set **no** state, or Terraform tracks something that no longer exists. On cleanup failure **do** set state, including the ID and the mode the server actually returned, and report both failures; the framework forwards `CreateResponse.State` even when diagnostics contain errors (`internal/fwserver/server_createresource.go:131` precedes the `HasError` check at `:158`). The retained state must record the returned mode and the returned ID, never the requested mode, so it does not lie about the remote object. The branch is driven by an explicit cleanup outcome value, never by matching diagnostic text: wording is not a control-flow contract, and a reworded error must not silently invert whether state is kept.

**P5 — Both data sources carry the mode.**
Not for resource/data-source parity, which is not an existing contract in this provider. State ownership is a material inventory and safety property, and the API list canonicalizes every entry, so adding a computed string to the summary is a small change that avoids an immediate follow-up.

**P6 — Documentation names the destructive boundary, and names the real bypass.**
`terraform apply -replace` does **not** get past the mode guard. Core calls `PlanResourceChange` with the real prior state first and aborts on provider error diagnostics before `forceReplace` is ever consulted; the null-prior second call is unreachable (verified in Terraform v1.14.2: first call `node_resource_abstract_instance.go:979`, error return `:996`, `getAction` with `forceReplace` `:1131`, second call `:1181`). The real bypass is `terraform taint`, because Core presents a typed null prior for a tainted object before its first plan call (`:884-890`), which our modifier correctly reads as a creation. Documentation must therefore say three separate things: mode changes are refused; anything that actually deletes and recreates a managed stack destroys its state lineage with no export; and taint can defeat the plan-time comparison and is not a migration mechanism.

⚠️ The `-replace` behaviour is asserted from Core source, not from a test. `terraform-plugin-testing` cannot pass the flag: `PlanOptions` exposes only `AllowDeferral` and `NoRefresh` (`helper/resource/additional_cli_options.go:23-29`). Building a bespoke Terraform CLI harness to assert one flag is not worth it, because the guard compares mode values only and a plain unchanged-mode apply already proves it stays quiet. The taint path **is** testable through `TestStep.Taint` (`helper/resource/testing.go:513-520`) and is covered in Task 5, which matters more: it is the bypass the documentation claims exists.

**P7 — A string attribute, not a single-field block.**
The API nests the mode in an object because a second, informational field was once considered and then cut. A single-value nested block is awkward HCL, so the attribute is a plain string named `state_management` holding `managed` or `external`, mapped to and from the API object in the client layer. If the API ever adds an authoritative second field, it arrives as its own attribute.

---

## Task 1: Client types and the effective-mode helper

**Files:**
- Modify: `internal/zenfraclient/types.go:104-131`
- Test: `internal/zenfraclient/types_test.go` (create if absent)

**Step 1: Write the failing test**

```go
func TestEffectiveStateMode(t *testing.T) {
	if got := zenfraclient.EffectiveStateMode(nil); got != "managed" {
		t.Errorf("nil = %q, want managed (a server older than the feature omits the field)", got)
	}
	for _, mode := range []string{"managed", "external"} {
		got := zenfraclient.EffectiveStateMode(&zenfraclient.StateManagement{Mode: mode})
		if got != mode {
			t.Errorf("%q = %q, want %q", mode, got, mode)
		}
	}
}

func TestStackJSON_StateManagementPresence(t *testing.T) {
	var absent zenfraclient.Stack
	if err := json.Unmarshal([]byte(`{"id":"s1"}`), &absent); err != nil {
		t.Fatal(err)
	}
	if absent.StateManagement != nil {
		t.Error("an absent field must stay nil so Create can detect an old control plane")
	}

	var present zenfraclient.Stack
	if err := json.Unmarshal([]byte(`{"id":"s1","state_management":{"mode":"external"}}`), &present); err != nil {
		t.Fatal(err)
	}
	if present.StateManagement == nil || present.StateManagement.Mode != "external" {
		t.Errorf("present = %+v, want mode external", present.StateManagement)
	}
}
```

**Step 2: Run it and watch it fail**

Run: `cd /Users/mykytademeshchenko/projects/zenfra/terraform-provider-zenfra && go test ./internal/zenfraclient/ -run 'TestEffectiveStateMode|TestStackJSON_StateManagementPresence' -v`
Expected: build failure, `undefined: zenfraclient.StateManagement`.

**Step 3: Write the minimal implementation**

In `internal/zenfraclient/types.go`, beside the other stack types:

```go
// StateMode values for StateManagement.Mode.
const (
	StateModeManaged  = "managed"
	StateModeExternal = "external"
)

// StateManagement is who owns the stack's Terraform state. Create-only.
type StateManagement struct {
	Mode string `json:"mode"`
}

// EffectiveStateMode reports the mode a stack behaves as. A nil block means a
// control plane older than this feature, which only ever managed state itself.
func EffectiveStateMode(sm *StateManagement) string {
	if sm == nil {
		return StateModeManaged
	}
	return sm.Mode
}
```

Add the field to both structs, as a pointer:

```go
// In Stack, after AllowPublicPool:
	StateManagement *StateManagement `json:"state_management,omitempty"`

// In CreateStackRequest, after AllowPublicPool:
	StateManagement *StateManagement `json:"state_management,omitempty"`
```

⚠️ Do **not** add it to `UpdateStackRequest`. The mode is create-only, and the API's update path ignores it anyway.

**Step 4: Run the tests and watch them pass**

Run: `go test ./internal/zenfraclient/ -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/zenfraclient/types.go internal/zenfraclient/types_test.go
git commit -m "feat(client): carry stack state ownership as a pointer"
```

---

## Task 2: The attribute, its validator and the transition guard

**Files:**
- Create: `internal/resource/stack/stack_state_management.go`
- Create: `internal/resource/stack/stack_state_management_test.go`
- Modify: `internal/resource/stack/stack_model.go:12-24`
- Modify: `internal/resource/stack/stack_resource.go:43-60` (schema)

**Step 1: Write the failing tests**

The guard is a `planmodifier.String`, so the test drives it directly. `tftypes` values stand in for the raw plan and state.

```go
package stack

import (
	"context"
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
		"create":              {nullRaw(), nonNullRaw(), types.StringNull(), types.StringValue("external")},
		"destroy":             {nonNullRaw(), nullRaw(), types.StringValue("external"), types.StringNull()},
		"unchanged":           {nonNullRaw(), nonNullRaw(), types.StringValue("external"), types.StringValue("external")},
		"omitted config":      {nonNullRaw(), nonNullRaw(), types.StringValue("external"), types.StringNull()},
		"unknown config":      {nonNullRaw(), nonNullRaw(), types.StringValue("external"), types.StringUnknown()},
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
```

**Step 2: Run them and watch them fail**

Run: `go test ./internal/resource/stack/ -run TestStateManagement -v`
Expected: build failure, `undefined: stateManagementGuard`.

**Step 3: Write the minimal implementation**

`internal/resource/stack/stack_state_management.go`:

```go
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
```

⚠️ No new module. `stringvalidator.OneOf` lives in `terraform-plugin-framework-validators`, which this provider does not depend on; the twenty lines above are cheaper than a dependency.

Add to `StackModel` (`stack_model.go`), after `AllowPublicPool`:

```go
	StateManagement types.String `tfsdk:"state_management"`
```

Add to the resource schema (`stack_resource.go`), beside `allow_public_pool`:

```go
			"state_management": schema.StringAttribute{
				Description: "Who owns this stack's Terraform state: \"managed\" (default, Zenfra " +
					"stores it) or \"external\" (your own backend block in the source). Set at " +
					"creation only; changing it is refused. Requires a worker advertising the " +
					"external-state-v1 capability.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{stateManagementValidator{}},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stateManagementGuard{},
				},
			},
```

**Step 4: Run the tests and watch them pass**

Run: `go test ./internal/resource/stack/ -run TestStateManagement -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/resource/stack/stack_state_management.go \
        internal/resource/stack/stack_state_management_test.go \
        internal/resource/stack/stack_model.go internal/resource/stack/stack_resource.go
git commit -m "feat(stack): add create-only state_management with a transition guard"
```

---

## Task 3: Create — requested mode, mismatch detection, compensating delete

**Files:**
- Modify: `internal/resource/stack/stack_resource.go:217-282` (Create), `:465` (mapStackToState)
- Test: `internal/resource/stack/stack_state_management_test.go`

**Step 1: Write the failing tests**

These need a client seam. `StackResource.client` is a concrete `*zenfraclient.Client` (`stack_resource.go:33-35`), so introduce an interface in this package covering the resource's whole client surface, and have the resource hold that instead, so a fake can drive both cleanup branches:

```go
// stackClient is every API call the resource makes, so Create's mismatch and
// cleanup branches are testable without a live server.
type stackClient interface {
	CreateStack(ctx context.Context, req zenfraclient.CreateStackRequest) (*zenfraclient.Stack, error)
	GetStack(ctx context.Context, id string) (*zenfraclient.Stack, error)
	UpdateStack(ctx context.Context, id string, req zenfraclient.UpdateStackRequest) (*zenfraclient.Stack, error)
	SetStackSource(ctx context.Context, id string, source zenfraclient.StackSource) error
	DeleteStack(ctx context.Context, id string) error
}
```

⚠️ It must carry **all five** methods, not just the two Create uses. The same field serves `GetStack` (`stack_resource.go:295`, `:414`), `SetStackSource` (`:348`), `UpdateStack` (`:403`) and `DeleteStack` (`:444`); a two-method interface does not compile. `Configure` assigns the concrete client, which satisfies this interface unchanged. The test fake should fail loudly on any call a test did not expect. Do not introduce interfaces for the other resources: this one has a branch a live server cannot reproduce on demand.

```go
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
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
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

// The same property at the Create level: nothing may reach the server.
func TestCreate_UnknownModeCreatesNothing(t *testing.T) {
	// Drive Create with an unknown state_management in config and assert
	// fakeStackClient recorded zero CreateStack calls.
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
	detail := diags.Errors()[0].Detail()
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
	if strings.Contains(diags.Errors()[0].Detail(), "upgrade") {
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
```

⚠️ The two cleanup outcomes take **opposite** state handling, and it is easy to implement backwards. Cover them as separate tests in `Create` itself:

```go
func TestCreate_CleanupSuccessSetsNoState(t *testing.T) { /* resp.State stays null */ }

func TestCreate_CleanupFailureSetsReturnedMode(t *testing.T) {
	// State must carry the mode the server ACTUALLY returned, plus the ID, so
	// Terraform can destroy the orphan and the state does not lie about it.
}
```

**Step 2: Run them and watch them fail**

Run: `go test ./internal/resource/stack/ -run TestCreate_ -v`
Expected: build failure, `undefined: stateManagementForCreate`.

**Step 3: Write the minimal implementation**

In `stack_state_management.go`:

```go
// stateManagementForCreate turns the configured attribute into a request block,
// or reports why it cannot. Null means the practitioner did not choose, which the
// API reads as managed, so send nothing rather than assert a default the server
// owns. Unknown is NOT the same thing: it means they wrote something that did not
// resolve at plan time, and quietly sending nothing would turn that into managed.
func stateManagementForCreate(at path.Path, configured types.String) (*zenfraclient.StateManagement, diag.Diagnostics) {
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

// verifyCreatedMode compares what the server returned against what was asked
// for and, on a mismatch, deletes the stack that was just created. Returning a
// bare error would orphan it: the POST already succeeded.
// cleanupOutcome says what happened to the stack that should not exist, and is
// what Create branches on. Never branch on diagnostic text.
type cleanupOutcome int

const (
	cleanupNotNeeded cleanupOutcome = iota // the mode was right; nothing was created wrongly
	cleanupSucceeded                       // the orphan is gone; Create must set NO state
	cleanupFailed                          // the orphan survives; Create MUST record it
)

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
```

In `Create`, read the configuration for the requested mode, populate the request, and verify after the POST:

```go
	// The plan value is unknown for an omitted Optional+Computed attribute on
	// create, so the configuration is the only honest source of intent.
	var config StackModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	stateBlock, stateDiags := stateManagementForCreate(path.Root("state_management"), config.StateManagement)
	resp.Diagnostics.Append(stateDiags...)
	if resp.Diagnostics.HasError() {
		return // refuse before the POST: nothing must be created
	}
	createReq.StateManagement = stateBlock
	requested := requestedEffectiveMode(config.StateManagement)

	stack, err := r.client.CreateStack(ctx, createReq)
	// ... existing error handling ...

	outcome, verifyDiags := verifyCreatedMode(ctx, r.client, stack.ID, requested, stack.StateManagement)
	resp.Diagnostics.Append(verifyDiags...)
	switch outcome {
	case cleanupSucceeded:
		return // the stack is gone; leave state null so Terraform tracks nothing
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
		return
	}
```

Extend `mapStackToState` to set the attribute from the effective mode:

```go
	model.StateManagement = types.StringValue(zenfraclient.EffectiveStateMode(stack.StateManagement))
```

**Step 4: Run the tests and watch them pass**

Run: `go test ./internal/resource/stack/ -v && go vet ./...`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/resource/stack/
git commit -m "feat(stack): verify the created state mode and compensate on mismatch"
```

---

## Task 4: Read, import and both data sources

**Files:**
- Modify: `internal/datasource/stack/stack_data_source.go:20-33` and its mapping near `:235`
- Modify: `internal/datasource/stack/stacks_data_source.go:25-30` and its mapping
- Test: alongside each

⚠️ Both data sources map inline today, with no helper to test: the single-stack one assigns field by field (`stack_data_source.go:227-269`) and the list builds its items in a loop (`stacks_data_source.go:116-124`). **Step 1 is a pure refactor with no behaviour change:** extract `mapStackToDataSource(*zenfraclient.Stack) stackDataSourceModel` and `mapStackToListItem(*zenfraclient.Stack) stacksListItemModel`, have `Read` call them, and confirm the existing tests still pass. Only then write the tests below, which depend on those helpers existing.

**Step 1: Write the failing tests (after the extraction)**

```go
func TestDataSource_StateManagement(t *testing.T) {
	// A control plane older than the feature omits the block; it reads as managed.
	if got := mapStackToDataSource(&zenfraclient.Stack{ID: "s1"}); got.StateManagement.ValueString() != "managed" {
		t.Errorf("nil block = %q, want managed", got.StateManagement.ValueString())
	}
	external := &zenfraclient.Stack{ID: "s1", StateManagement: &zenfraclient.StateManagement{Mode: "external"}}
	if got := mapStackToDataSource(external); got.StateManagement.ValueString() != "external" {
		t.Error("an external stack must read as external")
	}
}
```

Mirror it for the list item model.

**Step 2: Run and watch fail.** `go test ./internal/datasource/stack/ -v`

**Step 3: Implement.** Add `StateManagement types.String` to both models, a computed `state_management` string attribute to both schemas (description: "Who owns this stack's Terraform state."), and set it from `zenfraclient.EffectiveStateMode` in both mappings.

⚠️ Use the same helper as the resource. Read and import go through `mapStackToState`, which Task 3 already covered, so there is nothing extra to do for import beyond confirming it in Task 5.

**Step 4: Run and watch pass.** `go test ./internal/datasource/... -v`

**Step 5: Commit**

```bash
git add internal/datasource/stack/
git commit -m "feat(datasource): expose stack state ownership"
```

---

## Task 5: Acceptance tests

**Files:**
- Create: `internal/resource/stack/stack_resource_acc_test.go`

⚠️ **This repository has no acceptance harness at all**, so this task builds one, and it is the only task that adds a dependency:

```bash
go get github.com/hashicorp/terraform-plugin-testing@v1.16.0
```

Establish, in this order, because nothing below runs without them:

1. `testAccProtoV6ProviderFactories`, a `map[string]func() (tfprotov6.ProviderServer, error)` wrapping this provider through `providerserver.NewProtocol6WithError`.
2. `testAccPreCheck(t)` asserting every required variable with `t.Fatal`, **not** `t.Skip`. `resource.Test` already skips the whole test when `TF_ACC` is unset, so once the operator has asked for acceptance tests a missing endpoint, token or space ID is a misconfiguration. Skipping there lets `make testacc` report green while exercising nothing.
3. Environment: `TF_ACC=1`, the provider endpoint and API token, and `ZENFRA_ACC_SPACE_ID`. A stack cannot be created without a space and a valid source, so the test configuration needs a real space ID and a reachable public git source. Reuse the same public fixture the platform E2E uses, `github.com/ZenfraCloud/zenfra-tf-min-stack-public` at `simple`.
4. `CheckDestroy` asserting the stack is gone from the API, so a failed run does not leave stacks behind.
5. A control plane carrying PR #686. Against an older one every external case fails by design, which is Task 3's contract, not a test bug.

- [ ] create with `state_management = "external"`, assert the attribute survives a refresh
- [ ] create with the attribute omitted, assert it reads back `managed` and a second plan is **empty** — this is the regression that `Optional + Computed` exists to prevent
- [ ] create with `managed`, then change to `external`: `terraform plan` must fail with the guard's error, using `ExpectError`
- [ ] the reverse change, `external` to `managed`, must fail the same way
- [ ] ⚠️ **Taint crosses the guard on a transition that would otherwise be refused.** Step one creates a `managed` stack. Step two sets `Taint` for that address **and** changes the configuration to `external` in the same step; `Taint` is applied "prior to the execution of the step" (`helper/resource/testing.go:513-520`), so the changed configuration belongs there. Assert the apply succeeds, the state reads `external`, and the ID changed.

  ⚠️ Do **not** write this as a tainted external stack recreated as external. That version is vacuous: prior and configured modes are equal, so the guard would stay quiet even if Terraform supplied the real prior state, and the test would pass without proving anything. The point is that a refused transition succeeds *because* the tainted prior is null. `managed` to `external` is the direction to pin, since it is the one the warning is about.
- [ ] ⚠️ **Do not attempt an acceptance test for `-replace`.** The framework cannot pass the flag: `PlanOptions` carries only `AllowDeferral` and `NoRefresh` (`helper/resource/additional_cli_options.go:23-29`). `ConfigPlanChecks` observes a plan, it does not drive one, so claiming it exercises `-replace` would be false. That behaviour is settled from Core source in P6 and needs no test: the guard compares mode values only, and the unchanged-mode cases above already prove it stays silent.
- [ ] import an existing external stack and assert `ImportStateVerify` passes with the attribute populated
- [ ] destroy an external stack, which the guard must allow

**Commit**

```bash
git add internal/resource/stack/stack_resource_acc_test.go
git commit -m "test(stack): acceptance coverage for state ownership"
```

---

## Task 6: Documentation and generated artifacts

**Files:**
- Modify: `examples/resources/zenfra_stack/resource.tf` (add an external example)
- Regenerate: `docs/`, `providers-schema.json`

**Step 1: Write the documentation**

The generated page carries the attribute description from the schema. Add the destructive boundary as prose in the resource description or an example comment, saying all three things from P6 and no more:

1. State ownership is chosen at creation. Changing it on an existing stack is refused; there is no in-place migration.
2. Any operation that actually deletes and recreates a managed stack — an explicit destroy, `-replace`, or a tainted resource — soft-deletes the Zenfra stack, and the Terraform state Zenfra held for it becomes inaccessible. There is no self-service export.
3. `terraform taint` can defeat the plan-time comparison, because Terraform presents a tainted object as a creation. It is not a migration mechanism and must not be used as one.

⚠️ Do **not** write that `-replace` bypasses the mode guard. It does not: Terraform Core calls the provider with the real prior state first and aborts on the error before forced replacement is considered.

Also state that an external stack only runs on a worker advertising `external-state-v1`.

⚠️ **Do not link to `zenfra-cloud` at all.** The repository is private (verified: `gh repo view ZenfraCloud/zenfra-cloud` reports `PRIVATE`), so a GitHub URL is as dead for a registry reader as a filesystem path. Inline the contract compactly instead, and keep it to what a practitioner needs at the point of use:

- external mode accepts exactly the `s3` backend, and a run whose configuration resolves to anything else fails before plan or apply;
- the worker owns `TF_DATA_DIR`, `HOME` and `TF_CLI_CONFIG_FILE`, and refuses those plus `TF_PLUGIN_CACHE_DIR`, `TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE`, `TF_CLI_ARGS`, `TF_CLI_ARGS_<subcommand>`, `TERRAFORM_CONFIG` and `TF_REATTACH_PROVIDERS` if supplied through stack, bundle or run variables;
- credentials and locking for the backend are the customer's, and Zenfra never reads the state;
- the backend block is source code, so the destination can change in a later commit, but **migrating between backends is the customer's job and happens out of band**: every run starts in a clean workspace with a non-interactive init, so Zenfra cannot move state for you.

⚠️ The refusal list must be complete. `TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE` is easy to drop because it is long, but the worker really does refuse it (`zenfra-worker/internal/worker/env_policy.go:46`), and understating a hard refusal in public documentation is worse than omitting the list entirely.

Replace the inline copy with a link only once a public documentation site exists.

**Step 2: Regenerate**

Run: `make docs`
Expected: changes in `docs/resources/stack.md`, `docs/data-sources/stack.md`, `docs/data-sources/stacks.md` and `providers-schema.json`.

⚠️ Commit **both**. CI only fails on a stale `docs` directory, so a stale `providers-schema.json` passes review unnoticed.

**Step 3: Commit**

```bash
git add docs/ providers-schema.json examples/
git commit -m "docs: state ownership attribute and the destructive boundary"
```

---

## Task 7: Verify acceptance criteria

- [ ] `make test` green
- [ ] `make lint` reports 0 issues
- [ ] `make testacc` green against a control plane carrying PR #686
- [ ] `make docs` produces no further diff
- [ ] an omitted attribute produces an empty second plan, for both a managed and an imported external stack
- [ ] both mode transitions fail at plan time with an actionable error
- [ ] an unknown `state_management` is refused before any API call, not folded into `managed`
- [ ] a create against a control plane without #686 fails loudly rather than silently producing a managed stack
- [ ] the two cleanup outcomes are driven by `cleanupOutcome`, and no branch anywhere keys off diagnostic wording
- [ ] a tainted stack is replaced without the guard firing, matching what the documentation claims

## Rollout

This provider release must not ship before the API carrying PR #686 is deployed. A new provider against an old control plane is exactly the silent-downgrade case Task 3 turns into a loud failure, so the failure is safe, but it is still a broken release for anyone who upgrades in the wrong order.

## Out of scope

- `ExternalBackendType`. The API deliberately does not accept it, so the provider cannot offer it.
- Any migration tooling. There is no state export, and inventing a partial one here would imply a path that does not exist.
- The frontend toggle, which is Task 9 of the upstream plan and lives in `zenfra-fe`.
