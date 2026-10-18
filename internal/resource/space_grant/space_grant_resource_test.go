// ABOUTME: Unit tests for zenfra_space_grant against an httptest API (zenfra-cloud#953).
// ABOUTME: Create, role change in place, drift, import by space_id/team_id, the gate and role messages.
package space_grant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

const (
	spaceID   = "6630f0c2a1b2c3d4e5f60718"
	teamID    = "6630f0c2a1b2c3d4e5f60719"
	roleRead  = "read"
	roleWrite = "write"
)

// fakeGrantsAPI models /api/v1/spaces/:space_id/grants with the handler's JSON.
type fakeGrantsAPI struct {
	mu       sync.Mutex
	grants   map[string]*zenfraclient.SpaceGrant // by team id
	patches  int
	inactive bool
}

func (f *fakeGrantsAPI) handler() http.Handler {
	mux := http.NewServeMux()
	base := "/api/v1/spaces/" + spaceID + "/grants"
	write := func(w http.ResponseWriter, code int, body string) {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}
	refused := func(w http.ResponseWriter, role string) bool {
		if f.inactive {
			write(w, http.StatusConflict, `{"error":"access_boundaries_inactive","message":"Teams, grants and the Member role are not available yet"}`)
			return true
		}
		if role != roleRead && role != roleWrite {
			write(w, http.StatusBadRequest, `{"error":"invalid_role","message":"a grant role is read or write"}`)
			return true
		}
		return false
	}
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		var body zenfraclient.CreateSpaceGrantRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		if refused(w, body.Role) {
			return
		}
		g := &zenfraclient.SpaceGrant{SpaceID: spaceID, TeamID: body.TeamID, Role: body.Role}
		f.grants[body.TeamID] = g
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(g)
	})
	mux.HandleFunc("GET "+base+"/{team_id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		g, ok := f.grants[r.PathValue("team_id")]
		if !ok {
			write(w, http.StatusNotFound, `{"error":"not_found","message":"space grant not found"}`)
			return
		}
		_ = json.NewEncoder(w).Encode(g)
	})
	mux.HandleFunc("PATCH "+base+"/{team_id}", func(w http.ResponseWriter, r *http.Request) {
		var body zenfraclient.UpdateSpaceGrantRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.patches++
		if refused(w, body.Role) {
			return
		}
		g, ok := f.grants[r.PathValue("team_id")]
		if !ok {
			write(w, http.StatusNotFound, `{"error":"not_found","message":"space grant not found"}`)
			return
		}
		g.Role = body.Role
		_ = json.NewEncoder(w).Encode(g)
	})
	mux.HandleFunc("DELETE "+base+"/{team_id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if _, ok := f.grants[r.PathValue("team_id")]; !ok {
			write(w, http.StatusNotFound, `{"error":"not_found","message":"space grant not found"}`)
			return
		}
		delete(f.grants, r.PathValue("team_id"))
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

type harness struct {
	r      *SpaceGrantResource
	fake   *fakeGrantsAPI
	schema resource.SchemaResponse
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	fake := &fakeGrantsAPI{grants: map[string]*zenfraclient.SpaceGrant{}}
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)
	client, err := zenfraclient.NewClient(zenfraclient.ClientConfig{Endpoint: server.URL, APIToken: "test", MaxRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{r: &SpaceGrantResource{client: client}, fake: fake}
	h.r.Schema(context.Background(), resource.SchemaRequest{}, &h.schema)
	return h
}

func (h *harness) state(t *testing.T, m SpaceGrantModel) tfsdk.State {
	t.Helper()
	s := tfsdk.State{Schema: h.schema.Schema}
	if d := s.Set(context.Background(), m); d.HasError() {
		t.Fatal(d)
	}
	return s
}

func (h *harness) plan(t *testing.T, m SpaceGrantModel) tfsdk.Plan {
	t.Helper()
	p := tfsdk.Plan{Schema: h.schema.Schema}
	if d := p.Set(context.Background(), m); d.HasError() {
		t.Fatal(d)
	}
	return p
}

func get(t *testing.T, s tfsdk.State) SpaceGrantModel {
	t.Helper()
	var m SpaceGrantModel
	if d := s.Get(context.Background(), &m); d.HasError() {
		t.Fatal(d)
	}
	return m
}

func stored(role string) SpaceGrantModel {
	return SpaceGrantModel{ID: types.StringValue(spaceID + "/" + teamID), SpaceID: types.StringValue(spaceID),
		TeamID: types.StringValue(teamID), Role: types.StringValue(role)}
}

func (h *harness) create(t *testing.T, role string) resource.CreateResponse {
	t.Helper()
	planned := stored(role)
	planned.ID = types.StringUnknown()
	resp := resource.CreateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	h.r.Create(context.Background(), resource.CreateRequest{Plan: h.plan(t, planned)}, &resp)
	return resp
}

func (h *harness) read(t *testing.T, s tfsdk.State) resource.ReadResponse {
	t.Helper()
	resp := resource.ReadResponse{State: s}
	h.r.Read(context.Background(), resource.ReadRequest{State: s}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}
	return resp
}

func TestSpaceGrant_CreateUpdateDelete(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	resp := h.create(t, roleRead)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: %v", resp.Diagnostics)
	}
	if got := get(t, resp.State); got.ID.ValueString() != spaceID+"/"+teamID || got.Role.ValueString() != roleRead {
		t.Fatalf("state = %+v", got)
	}

	upd := resource.UpdateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	h.r.Update(context.Background(), resource.UpdateRequest{Plan: h.plan(t, stored(roleWrite)), State: h.state(t, stored(roleRead))}, &upd)
	if upd.Diagnostics.HasError() || get(t, upd.State).Role.ValueString() != roleWrite || h.fake.patches != 1 {
		t.Fatalf("Update: %v %+v patches=%d", upd.Diagnostics, get(t, upd.State), h.fake.patches)
	}
	if got := get(t, h.read(t, h.state(t, stored(roleWrite))).State); got.Role.ValueString() != roleWrite {
		t.Errorf("read role = %v", got.Role)
	}

	del := resource.DeleteResponse{}
	h.r.Delete(context.Background(), resource.DeleteRequest{State: h.state(t, stored(roleWrite))}, &del)
	if del.Diagnostics.HasError() || len(h.fake.grants) != 0 {
		t.Fatalf("Delete: %v", del.Diagnostics)
	}
	h.r.Delete(context.Background(), resource.DeleteRequest{State: h.state(t, stored(roleWrite))}, &del)
	if del.Diagnostics.HasError() {
		t.Fatalf("second Delete: %v", del.Diagnostics)
	}
}

func TestSpaceGrant_APIRefusals(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if resp := h.create(t, "admin"); !resp.Diagnostics.HasError() ||
		!strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "read or write") {
		t.Errorf("admin role: %v", resp.Diagnostics)
	}
	h.fake.inactive = true
	if resp := h.create(t, roleRead); !resp.Diagnostics.HasError() ||
		!strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "not available yet") {
		t.Errorf("gate off: %v", resp.Diagnostics)
	}
}

func TestSpaceGrant_RemovedOutOfBandLeavesState(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if !h.read(t, h.state(t, stored(roleRead))).State.Raw.IsNull() {
		t.Error("a grant removed out of band stays in state")
	}
}

func TestSpaceGrant_Import(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.fake.grants[teamID] = &zenfraclient.SpaceGrant{SpaceID: spaceID, TeamID: teamID, Role: roleWrite}
	empty := tfsdk.State{Schema: h.schema.Schema, Raw: tftypes.NewValue(h.schema.Schema.Type().TerraformType(context.Background()), nil)}
	imp := resource.ImportStateResponse{State: empty}
	h.r.ImportState(context.Background(), resource.ImportStateRequest{ID: "no-slash"}, &imp)
	if !imp.Diagnostics.HasError() {
		t.Error("a malformed import id was accepted")
	}
	imp = resource.ImportStateResponse{State: empty}
	h.r.ImportState(context.Background(), resource.ImportStateRequest{ID: spaceID + "/" + teamID}, &imp)
	if imp.Diagnostics.HasError() {
		t.Fatal(imp.Diagnostics)
	}
	if got := get(t, h.read(t, imp.State).State); got.Role.ValueString() != roleWrite {
		t.Errorf("imported role = %v", got.Role)
	}
}

func TestSpaceGrant_KeysForceReplacementRoleDoesNot(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	for name, want := range map[string]int{"space_id": 1, "team_id": 1, "role": 0} {
		a, ok := h.schema.Schema.Attributes[name].(schema.StringAttribute)
		if !ok || len(a.PlanModifiers) != want {
			t.Errorf("%s plan modifiers = %v, want %d", name, a.PlanModifiers, want)
		}
	}
}
