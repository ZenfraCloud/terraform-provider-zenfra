// ABOUTME: Unit tests for zenfra_stack_dependency CRUD, import and plan modification against an httptest API.
// ABOUTME: Covers the kept parent on a null stack, refusal diagnostics, the child/edge import and the disabled-create guard.
package stack_dependency

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

const (
	childID   = "6630f0c2a1b2c3d4e5f60701"
	parentID  = "6630f0c2a1b2c3d4e5f60702"
	otherID   = "6630f0c2a1b2c3d4e5f60703"
	edgeID    = "6630f0c2a1b2c3d4e5f60710"
	created   = "2026-10-25T12:00:00.123Z"
	reEnabled = "2026-10-26T08:30:00.456Z"
)

// fakeDependencyAPI models one child stack's dependencies with the handler's
// JSON: every write goes through the child, stack is the parent (null when
// hidden), 404 for an unknown child or edge, 409 {"error","message"} for a
// refusal.
type fakeDependencyAPI struct {
	mu         sync.Mutex
	children   map[string]bool // stacks that exist
	upstream   []zenfraclient.StackDependency
	downstream []zenfraclient.StackDependency
	hideParent bool   // the token cannot read the parent: stack is null
	refuse     string // the next write answers 409 with this code
	calls      []string
}

func (f *fakeDependencyAPI) handler() http.Handler {
	mux := http.NewServeMux()
	base := "/api/v1/stacks/{child}/dependencies"
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		var body zenfraclient.CreateStackDependencyRequest
		f.lock(r, &body)
		defer f.mu.Unlock()
		if !f.children[r.PathValue("child")] {
			notFound(w)
			return
		}
		if f.refused(w) {
			return
		}
		edge := zenfraclient.StackDependency{
			ID: edgeID, Stack: &zenfraclient.DependencyStackRef{ID: body.ParentStackID, Name: "network"},
			Enabled: true, ActivatedAt: created, Revision: 1,
		}
		f.upstream = append(f.upstream, edge)
		w.WriteHeader(http.StatusCreated)
		f.write(w, edge)
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		f.lock(r, nil)
		defer f.mu.Unlock()
		if !f.children[r.PathValue("child")] {
			notFound(w)
			return
		}
		list := zenfraclient.StackDependencyList{Upstream: []zenfraclient.StackDependency{}, Downstream: f.downstream}
		for _, e := range f.upstream {
			list.Upstream = append(list.Upstream, f.view(e))
		}
		_ = json.NewEncoder(w).Encode(list)
	})
	mux.HandleFunc("PUT "+base+"/{edge}/enabled", func(w http.ResponseWriter, r *http.Request) {
		var body zenfraclient.SetStackDependencyEnabledRequest
		f.lock(r, &body)
		defer f.mu.Unlock()
		e := f.find(r.PathValue("edge"))
		if e == nil {
			notFound(w)
			return
		}
		if f.refused(w) {
			return
		}
		if body.Enabled && !e.Enabled {
			e.ActivatedAt = reEnabled
		}
		e.Enabled = body.Enabled
		f.write(w, *e)
	})
	mux.HandleFunc("DELETE "+base+"/{edge}", func(w http.ResponseWriter, r *http.Request) {
		f.lock(r, nil)
		defer f.mu.Unlock()
		for i := range f.upstream {
			if f.upstream[i].ID == r.PathValue("edge") {
				f.upstream = append(f.upstream[:i], f.upstream[i+1:]...)
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		notFound(w)
	})
	return mux
}

// lock records the call, decodes the body into dst and takes the lock.
func (f *fakeDependencyAPI) lock(r *http.Request, dst any) {
	raw, _ := io.ReadAll(r.Body)
	if dst != nil {
		_ = json.Unmarshal(raw, dst)
	}
	f.mu.Lock()
	f.calls = append(f.calls, strings.TrimSpace(r.Method+" "+r.URL.Path+" "+string(raw)))
}

func (f *fakeDependencyAPI) find(id string) *zenfraclient.StackDependency {
	for i := range f.upstream {
		if f.upstream[i].ID == id {
			return &f.upstream[i]
		}
	}
	return nil
}

func (f *fakeDependencyAPI) view(e zenfraclient.StackDependency) zenfraclient.StackDependency {
	if f.hideParent {
		e.Stack = nil
	}
	return e
}

func (f *fakeDependencyAPI) write(w http.ResponseWriter, e zenfraclient.StackDependency) {
	_ = json.NewEncoder(w).Encode(f.view(e))
}

func (f *fakeDependencyAPI) refused(w http.ResponseWriter) bool {
	if f.refuse == "" {
		return false
	}
	w.WriteHeader(http.StatusConflict)
	_, _ = fmt.Fprintf(w, `{"error":%q,"message":"refused: %s"}`, f.refuse, f.refuse)
	return true
}

func notFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"error":"not_found","message":"stack not found"}`))
}

type harness struct {
	r      *StackDependencyResource
	fake   *fakeDependencyAPI
	schema resource.SchemaResponse
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	fake := &fakeDependencyAPI{children: map[string]bool{childID: true}}
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)
	client, err := zenfraclient.NewClient(zenfraclient.ClientConfig{Endpoint: server.URL, APIToken: "test", MaxRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{r: &StackDependencyResource{client: client}, fake: fake}
	h.r.Schema(context.Background(), resource.SchemaRequest{}, &h.schema)
	return h
}

// seed stores an upstream edge of the child as the API holds it.
func (h *harness) seed(enabled bool) {
	h.fake.upstream = append(h.fake.upstream, zenfraclient.StackDependency{
		ID: edgeID, Stack: &zenfraclient.DependencyStackRef{ID: parentID, Name: "network"},
		Enabled: enabled, ActivatedAt: created, Revision: 1,
	})
}

// stored is the state of an enabled edge Terraform created.
func stored() StackDependencyModel {
	return StackDependencyModel{
		ID:            types.StringValue(edgeID),
		ParentStackID: types.StringValue(parentID),
		ChildStackID:  types.StringValue(childID),
		Enabled:       types.BoolValue(true),
		ActivatedAt:   types.StringValue(created),
	}
}

func planned(enabled types.Bool) StackDependencyModel {
	return StackDependencyModel{
		ID:            types.StringUnknown(),
		ParentStackID: types.StringValue(parentID),
		ChildStackID:  types.StringValue(childID),
		Enabled:       enabled,
		ActivatedAt:   types.StringUnknown(),
	}
}

func (h *harness) state(t *testing.T, m StackDependencyModel) tfsdk.State {
	t.Helper()
	s := tfsdk.State{Schema: h.schema.Schema}
	if d := s.Set(context.Background(), m); d.HasError() {
		t.Fatal(d)
	}
	return s
}

func (h *harness) plan(t *testing.T, m StackDependencyModel) tfsdk.Plan {
	t.Helper()
	p := tfsdk.Plan{Schema: h.schema.Schema}
	if d := p.Set(context.Background(), m); d.HasError() {
		t.Fatal(d)
	}
	return p
}

func (h *harness) null() tftypes.Value {
	return tftypes.NewValue(h.schema.Schema.Type().TerraformType(context.Background()), nil)
}

func get(t *testing.T, s tfsdk.State) StackDependencyModel {
	t.Helper()
	var m StackDependencyModel
	if d := s.Get(context.Background(), &m); d.HasError() {
		t.Fatal(d)
	}
	return m
}

func (h *harness) create(t *testing.T, m StackDependencyModel) resource.CreateResponse {
	t.Helper()
	resp := resource.CreateResponse{State: tfsdk.State{Schema: h.schema.Schema, Raw: h.null()}}
	h.r.Create(context.Background(), resource.CreateRequest{Plan: h.plan(t, m)}, &resp)
	return resp
}

func (h *harness) read(t *testing.T, prior tfsdk.State) resource.ReadResponse {
	t.Helper()
	resp := resource.ReadResponse{State: prior}
	h.r.Read(context.Background(), resource.ReadRequest{State: prior}, &resp)
	return resp
}

// update pre-fills the response with the prior state, as the framework does.
func (h *harness) update(t *testing.T, plan, prior StackDependencyModel) resource.UpdateResponse {
	t.Helper()
	resp := resource.UpdateResponse{State: h.state(t, prior)}
	h.r.Update(context.Background(), resource.UpdateRequest{Plan: h.plan(t, plan), State: h.state(t, prior)}, &resp)
	return resp
}

func (h *harness) importID(t *testing.T, id string) resource.ImportStateResponse {
	t.Helper()
	resp := resource.ImportStateResponse{State: tfsdk.State{Schema: h.schema.Schema, Raw: h.null()}}
	h.r.ImportState(context.Background(), resource.ImportStateRequest{ID: id}, &resp)
	return resp
}

func mustOK(t *testing.T, d interface{ HasError() bool }, what string) {
	t.Helper()
	if d.HasError() {
		t.Fatalf("%s: %v", what, d)
	}
}

func TestCreate_MapsTheEdge(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	resp := h.create(t, planned(types.BoolValue(true)))
	mustOK(t, resp.Diagnostics, "Create")
	if got := get(t, resp.State); got != stored() {
		t.Errorf("state = %+v, want %+v", got, stored())
	}
	want := `POST /api/v1/stacks/` + childID + `/dependencies {"parent_stack_id":"` + parentID + `"}`
	if len(h.fake.calls) != 1 || h.fake.calls[0] != want {
		t.Errorf("calls = %v, want exactly %s", h.fake.calls, want)
	}
}

func TestCreate_RefusesDisabledBeforeAnyRequest(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	resp := h.create(t, planned(types.BoolValue(false)))
	if !resp.Diagnostics.HasError() || resp.Diagnostics.Errors()[0].Summary() != "Stack Dependency Cannot Be Created Disabled" {
		t.Fatalf("diagnostics = %v, want the disabled-create error", resp.Diagnostics)
	}
	if len(h.fake.calls) != 0 {
		t.Errorf("calls = %v, want none", h.fake.calls)
	}
}

var refusalCodes = []string{
	"dependency_self", "dependency_exists", "dependency_cycle", "dependency_depth_reached", "dependency_fanout_reached",
}

func TestCreate_RefusalIsReportedWithItsCode(t *testing.T) {
	t.Parallel()

	for _, code := range refusalCodes {
		t.Run(code, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.fake.refuse = code
			resp := h.create(t, planned(types.BoolValue(true)))
			errs := resp.Diagnostics.Errors()
			if len(errs) != 1 || errs[0].Summary() != "Stack Dependency Refused" || errs[0].Detail() != code+": refused: "+code {
				t.Fatalf("diagnostics = %v, want Stack Dependency Refused / %s: refused: %s", resp.Diagnostics, code, code)
			}
			if !resp.State.Raw.IsNull() {
				t.Error("a refused create must leave no state")
			}
		})
	}
}

func TestCreate_OtherErrorIsGeneric(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	m := planned(types.BoolValue(true))
	m.ChildStackID = types.StringValue(otherID) // unknown child: 404
	resp := h.create(t, m)
	if errs := resp.Diagnostics.Errors(); len(errs) != 1 || errs[0].Summary() != "Error Creating Stack Dependency" {
		t.Fatalf("diagnostics = %v, want the generic create error", resp.Diagnostics)
	}
}

func TestRead_FindsTheEdgeUpstreamOnly(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.seed(false)
	// A downstream edge with the same id is the child's own child: never this resource.
	h.fake.downstream = []zenfraclient.StackDependency{{ID: edgeID, Stack: &zenfraclient.DependencyStackRef{ID: otherID}, Enabled: true}}
	resp := h.read(t, h.state(t, stored()))
	mustOK(t, resp.Diagnostics, "Read")
	want := stored()
	want.Enabled = types.BoolValue(false)
	if got := get(t, resp.State); got != want {
		t.Errorf("state = %+v, want %+v", got, want)
	}
}

func TestRead_DownstreamOnlyIsGone(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.fake.downstream = []zenfraclient.StackDependency{{ID: edgeID, Stack: &zenfraclient.DependencyStackRef{ID: otherID}, Enabled: true}}
	resp := h.read(t, h.state(t, stored()))
	mustOK(t, resp.Diagnostics, "Read")
	if !resp.State.Raw.IsNull() {
		t.Error("an edge found only downstream must be removed from state")
	}
}

func TestRead_GoneEdgeOrChildIsRemoved(t *testing.T) {
	t.Parallel()

	for name, prior := range map[string]func() StackDependencyModel{
		"absent edge": stored,
		"child 404": func() StackDependencyModel {
			m := stored()
			m.ChildStackID = types.StringValue(otherID)
			return m
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			resp := h.read(t, h.state(t, prior()))
			mustOK(t, resp.Diagnostics, "Read")
			if !resp.State.Raw.IsNull() {
				t.Error("the resource must be removed from state")
			}
		})
	}
}

func TestRead_NullStackKeepsThePriorParent(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.seed(true)
	h.fake.hideParent = true
	resp := h.read(t, h.state(t, stored()))
	mustOK(t, resp.Diagnostics, "Read")
	if got := get(t, resp.State); got != stored() {
		t.Errorf("state = %+v, want the parent kept: %+v", got, stored())
	}
}

func TestUpdate_DisableAndReEnable(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.seed(true)
	disable := stored()
	disable.Enabled, disable.ActivatedAt = types.BoolValue(false), types.StringUnknown()
	resp := h.update(t, disable, stored())
	mustOK(t, resp.Diagnostics, "Update disable")
	off := get(t, resp.State)
	if off.Enabled.ValueBool() || off.ActivatedAt.ValueString() != created || off.ID.ValueString() != edgeID {
		t.Errorf("after disable = %+v, want disabled with activated_at unchanged", off)
	}

	enable := off
	enable.Enabled, enable.ActivatedAt = types.BoolValue(true), types.StringUnknown()
	resp = h.update(t, enable, off)
	mustOK(t, resp.Diagnostics, "Update enable")
	on := get(t, resp.State)
	if !on.Enabled.ValueBool() || on.ActivatedAt.ValueString() != reEnabled {
		t.Errorf("after enable = %+v, want enabled with activated_at %s from the response", on, reEnabled)
	}

	base := "PUT /api/v1/stacks/" + childID + "/dependencies/" + edgeID + "/enabled "
	if want := []string{base + `{"enabled":false}`, base + `{"enabled":true}`}; strings.Join(h.fake.calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("calls = %v, want %v", h.fake.calls, want)
	}
}

func TestUpdate_NullStackOnADisableKeepsTheParent(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.seed(true)
	h.fake.hideParent = true
	disable := stored()
	disable.Enabled = types.BoolValue(false)
	resp := h.update(t, disable, stored())
	mustOK(t, resp.Diagnostics, "Update")
	if got := get(t, resp.State); got.ParentStackID.ValueString() != parentID {
		t.Errorf("parent_stack_id = %v, want %s kept", got.ParentStackID, parentID)
	}
}

func TestUpdate_NothingChangedMakesNoCall(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.seed(true)
	resp := h.update(t, stored(), stored())
	mustOK(t, resp.Diagnostics, "Update")
	if len(h.fake.calls) != 0 {
		t.Errorf("calls = %v, want none", h.fake.calls)
	}
	if got := get(t, resp.State); got != stored() {
		t.Errorf("state = %+v", got)
	}
}

func TestUpdate_EnableRefusalLeavesStateUnchanged(t *testing.T) {
	t.Parallel()

	for _, code := range refusalCodes {
		if code == "dependency_exists" {
			continue // does not apply at enable
		}
		t.Run(code, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.seed(false)
			h.fake.refuse = code
			prior := stored()
			prior.Enabled = types.BoolValue(false)
			enable := prior
			enable.Enabled, enable.ActivatedAt = types.BoolValue(true), types.StringUnknown()
			resp := h.update(t, enable, prior)
			errs := resp.Diagnostics.Errors()
			if len(errs) != 1 || errs[0].Summary() != "Stack Dependency Refused" || errs[0].Detail() != code+": refused: "+code {
				t.Fatalf("diagnostics = %v, want Stack Dependency Refused / %s", resp.Diagnostics, code)
			}
			if got := get(t, resp.State); got != prior {
				t.Errorf("state = %+v, want the prior state %+v", got, prior)
			}
		})
	}
}

func TestDelete_SendsDeleteAndToleratesAMissingEdge(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.seed(true)
	for i := range 2 { // the second finds the edge gone: 404 is success
		resp := resource.DeleteResponse{State: h.state(t, stored())}
		h.r.Delete(context.Background(), resource.DeleteRequest{State: h.state(t, stored())}, &resp)
		mustOK(t, resp.Diagnostics, fmt.Sprintf("Delete %d", i))
	}
	want := "DELETE /api/v1/stacks/" + childID + "/dependencies/" + edgeID
	if len(h.fake.calls) != 2 || h.fake.calls[0] != want || len(h.fake.upstream) != 0 {
		t.Errorf("calls = %v, upstream = %v; want %s twice and no edge left", h.fake.calls, h.fake.upstream, want)
	}
}

func TestImport_FillsEveryAttribute(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.seed(false)
	imp := h.importID(t, childID+"/"+edgeID)
	mustOK(t, imp.Diagnostics, "ImportState")
	resp := h.read(t, imp.State)
	mustOK(t, resp.Diagnostics, "Read")
	want := stored()
	want.Enabled = types.BoolValue(false)
	if got := get(t, resp.State); got != want {
		t.Errorf("state = %+v, want %+v", got, want)
	}
}

func TestImport_NullStackIsAnError(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.seed(true)
	h.fake.hideParent = true
	imp := h.importID(t, childID+"/"+edgeID)
	mustOK(t, imp.Diagnostics, "ImportState")
	resp := h.read(t, imp.State)
	if errs := resp.Diagnostics.Errors(); len(errs) != 1 || errs[0].Summary() != "Cannot Import Stack Dependency" {
		t.Fatalf("diagnostics = %v, want the unreadable-parent import error", resp.Diagnostics)
	}
}

func TestImport_MalformedID(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	for _, id := range []string{"x", "/e", "c/", "a/b/c", ""} {
		imp := h.importID(t, id)
		if errs := imp.Diagnostics.Errors(); len(errs) != 1 || errs[0].Summary() != "Invalid Import ID" {
			t.Errorf("import %q: diagnostics = %v, want Invalid Import ID", id, imp.Diagnostics)
		}
	}
}

// modifyPlan runs ModifyPlan; a nil prior is a create, a nil plan a destroy.
func (h *harness) modifyPlan(t *testing.T, plan, prior *StackDependencyModel) resource.ModifyPlanResponse {
	t.Helper()
	req := resource.ModifyPlanRequest{
		Plan:  tfsdk.Plan{Schema: h.schema.Schema, Raw: h.null()},
		State: tfsdk.State{Schema: h.schema.Schema, Raw: h.null()},
	}
	if plan != nil {
		req.Plan = h.plan(t, *plan)
	}
	if prior != nil {
		req.State = h.state(t, *prior)
	}
	resp := resource.ModifyPlanResponse{Plan: req.Plan}
	h.r.ModifyPlan(context.Background(), req, &resp)
	return resp
}

func TestModifyPlan_DisabledCreateOrReplacement(t *testing.T) {
	t.Parallel()

	disabled := stored()
	disabled.Enabled = types.BoolValue(false)
	newParent := disabled
	newParent.ParentStackID = types.StringValue(otherID)
	newChild := disabled
	newChild.ChildStackID = types.StringValue(otherID)
	unknownParent := disabled
	unknownParent.ParentStackID = types.StringUnknown()
	enabledPrior := stored()

	tests := []struct {
		name    string
		plan    *StackDependencyModel
		prior   *StackDependencyModel
		wantErr bool
	}{
		{"create disabled", ptr(planned(types.BoolValue(false))), nil, true},
		{"replace parent, disabled", &newParent, &enabledPrior, true},
		{"replace child, disabled", &newChild, &enabledPrior, true},
		{"unknown parent, disabled", &unknownParent, &enabledPrior, true},
		{"update to disabled", &disabled, &enabledPrior, false},
		{"create enabled", ptr(planned(types.BoolValue(true))), nil, false},
		{"create, enabled unknown", ptr(planned(types.BoolUnknown())), nil, false},
		{"destroy a disabled edge", nil, &disabled, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			resp := h.modifyPlan(t, tt.plan, tt.prior)
			if got := resp.Diagnostics.HasError(); got != tt.wantErr {
				t.Fatalf("HasError = %v, want %v (%v)", got, tt.wantErr, resp.Diagnostics)
			}
			if tt.wantErr {
				e := resp.Diagnostics.Errors()[0]
				withPath, ok := e.(interface{ Path() path.Path })
				if !ok || !withPath.Path().Equal(path.Root("enabled")) || !strings.Contains(e.Detail(), "always created enabled") {
					t.Errorf("diagnostic = %v, want it on enabled with the reason", e)
				}
			}
			if len(h.fake.calls) != 0 {
				t.Errorf("calls = %v, want none at plan", h.fake.calls)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestSchema_ParentAndChildRequireReplace(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	want := stringplanmodifier.RequiresReplace().Description(context.Background())
	for _, name := range []string{"parent_stack_id", "child_stack_id"} {
		attr := h.schema.Schema.Attributes[name].(schema.StringAttribute)
		if !attr.Required || !hasModifier(attr.PlanModifiers, want) {
			t.Errorf("%s: required %v, modifiers %v; want required with RequiresReplace", name, attr.Required, attr.PlanModifiers)
		}
	}
	activated := h.schema.Schema.Attributes["activated_at"].(schema.StringAttribute)
	if !activated.Computed || len(activated.PlanModifiers) != 0 {
		t.Errorf("activated_at: computed %v, modifiers %v; want plain computed", activated.Computed, activated.PlanModifiers)
	}
}

func hasModifier(mods []planmodifier.String, description string) bool {
	for _, m := range mods {
		if m.Description(context.Background()) == description {
			return true
		}
	}
	return false
}
