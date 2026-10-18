// ABOUTME: Unit tests for zenfra_team_membership against an httptest API (zenfra-cloud#953).
// ABOUTME: Create, read through the member list, drift when the member or team is gone, import, the gate message.
package team_membership

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

const (
	teamID = "6630f0c2a1b2c3d4e5f60718"
	userID = "6630f0c2a1b2c3d4e5f60719"
)

// fakeMembersAPI models /api/v1/teams/:team_id/members for one team.
type fakeMembersAPI struct {
	mu       sync.Mutex
	team     bool            // the team exists
	members  map[string]bool // user ids
	inactive bool
}

func (f *fakeMembersAPI) handler() http.Handler {
	mux := http.NewServeMux()
	base := "/api/v1/teams/{team_id}/members"
	missing := func(w http.ResponseWriter, r *http.Request) bool {
		if f.team && r.PathValue("team_id") == teamID {
			return false
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found","message":"team not found"}`))
		return true
	}
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		var body zenfraclient.AddTeamMemberRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.inactive {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"access_boundaries_inactive","message":"Teams, grants and the Member role are not available yet"}`))
			return
		}
		if missing(w, r) {
			return
		}
		if f.members[body.UserID] {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"duplicate_membership","message":"the user is already a member of this team"}`))
			return
		}
		f.members[body.UserID] = true
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(zenfraclient.TeamMember{TeamID: teamID, UserID: body.UserID})
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if missing(w, r) {
			return
		}
		list := zenfraclient.TeamMemberList{Members: []zenfraclient.TeamMember{}}
		for u := range f.members {
			list.Members = append(list.Members, zenfraclient.TeamMember{TeamID: teamID, UserID: u})
		}
		_ = json.NewEncoder(w).Encode(list)
	})
	mux.HandleFunc("DELETE "+base+"/{user_id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if missing(w, r) {
			return
		}
		if !f.members[r.PathValue("user_id")] {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not_found","message":"team membership not found"}`))
			return
		}
		delete(f.members, r.PathValue("user_id"))
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

type harness struct {
	r      *TeamMembershipResource
	fake   *fakeMembersAPI
	schema resource.SchemaResponse
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	fake := &fakeMembersAPI{team: true, members: map[string]bool{}}
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)
	client, err := zenfraclient.NewClient(zenfraclient.ClientConfig{Endpoint: server.URL, APIToken: "test", MaxRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{r: &TeamMembershipResource{client: client}, fake: fake}
	h.r.Schema(context.Background(), resource.SchemaRequest{}, &h.schema)
	return h
}

func (h *harness) state(t *testing.T, m TeamMembershipModel) tfsdk.State {
	t.Helper()
	s := tfsdk.State{Schema: h.schema.Schema}
	if d := s.Set(context.Background(), m); d.HasError() {
		t.Fatal(d)
	}
	return s
}

func stored() TeamMembershipModel {
	return TeamMembershipModel{ID: types.StringValue(teamID + "/" + userID), TeamID: types.StringValue(teamID), UserID: types.StringValue(userID)}
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

func (h *harness) create(t *testing.T) resource.CreateResponse {
	t.Helper()
	planned := stored()
	planned.ID = types.StringUnknown()
	p := tfsdk.Plan{Schema: h.schema.Schema}
	if d := p.Set(context.Background(), planned); d.HasError() {
		t.Fatal(d)
	}
	resp := resource.CreateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	h.r.Create(context.Background(), resource.CreateRequest{Plan: p}, &resp)
	return resp
}

func TestTeamMembership_CreateReadDelete(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	resp := h.create(t)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: %v", resp.Diagnostics)
	}
	var got TeamMembershipModel
	resp.State.Get(context.Background(), &got)
	if got.ID.ValueString() != teamID+"/"+userID {
		t.Errorf("id = %v", got.ID)
	}
	if h.read(t, h.state(t, stored())).State.Raw.IsNull() {
		t.Fatal("a present member was removed from state")
	}

	// A second add of the same user surfaces the API's duplicate message.
	dup := h.create(t)
	if !dup.Diagnostics.HasError() || !strings.Contains(dup.Diagnostics.Errors()[0].Detail(), "already a member") {
		t.Errorf("duplicate: %v", dup.Diagnostics)
	}

	del := resource.DeleteResponse{}
	h.r.Delete(context.Background(), resource.DeleteRequest{State: h.state(t, stored())}, &del)
	if del.Diagnostics.HasError() || len(h.fake.members) != 0 {
		t.Fatalf("Delete: %v", del.Diagnostics)
	}
	h.r.Delete(context.Background(), resource.DeleteRequest{State: h.state(t, stored())}, &del)
	if del.Diagnostics.HasError() {
		t.Fatalf("second Delete: %v", del.Diagnostics)
	}
}

func TestTeamMembership_Drift(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if !h.read(t, h.state(t, stored())).State.Raw.IsNull() {
		t.Error("a member removed out of band stays in state")
	}
	h.fake.members[userID] = true
	h.fake.team = false
	if !h.read(t, h.state(t, stored())).State.Raw.IsNull() {
		t.Error("a membership of a deleted team stays in state")
	}
}

func TestTeamMembership_GateOffSurfacesTheAPIMessage(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.fake.inactive = true
	resp := h.create(t)
	if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "not available yet") {
		t.Fatalf("want the API message, got %v", resp.Diagnostics)
	}
}

func TestTeamMembership_Import(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.fake.members[userID] = true
	empty := tfsdk.State{Schema: h.schema.Schema, Raw: tftypes.NewValue(h.schema.Schema.Type().TerraformType(context.Background()), nil)}
	for _, bad := range []string{teamID, teamID + "/", "/" + userID, teamID + "/" + userID + "/x"} {
		imp := resource.ImportStateResponse{State: empty}
		h.r.ImportState(context.Background(), resource.ImportStateRequest{ID: bad}, &imp)
		if !imp.Diagnostics.HasError() {
			t.Errorf("import %q accepted", bad)
		}
	}
	imp := resource.ImportStateResponse{State: empty}
	h.r.ImportState(context.Background(), resource.ImportStateRequest{ID: teamID + "/" + userID}, &imp)
	if imp.Diagnostics.HasError() {
		t.Fatal(imp.Diagnostics)
	}
	if h.read(t, imp.State).State.Raw.IsNull() {
		t.Fatal("imported membership not found")
	}
}
