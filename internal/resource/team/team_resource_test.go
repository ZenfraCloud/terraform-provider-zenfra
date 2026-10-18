// ABOUTME: Unit tests for zenfra_team CRUD, import and drift against an httptest API (zenfra-cloud#953).
// ABOUTME: Covers leaf patches, the description round trip and the 409 access_boundaries_inactive message.
package team

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

const testID = "6630f0c2a1b2c3d4e5f60718"

// fakeTeamAPI models the handler's JSON for /api/v1/teams, and its
// normalization: the name is trimmed, the slug trimmed and lowercased.
type fakeTeamAPI struct {
	mu       sync.Mutex
	teams    map[string]*zenfraclient.Team
	patches  []string
	inactive bool // the access-boundary activation is absent
}

func (f *fakeTeamAPI) handler() http.Handler {
	mux := http.NewServeMux()
	refuse := func(w http.ResponseWriter) bool {
		if !f.inactive {
			return false
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"access_boundaries_inactive","message":"Teams, grants and the Member role are not available yet"}`))
		return true
	}
	notFound := func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found","message":"team not found"}`))
	}
	mux.HandleFunc("POST /api/v1/teams", func(w http.ResponseWriter, r *http.Request) {
		var body zenfraclient.CreateTeamRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		if refuse(w) {
			return
		}
		now := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
		t := &zenfraclient.Team{ID: testID, Name: strings.TrimSpace(body.Name), Slug: strings.ToLower(strings.TrimSpace(body.Slug)),
			Description: body.Description, CreatedAt: now, UpdatedAt: now}
		f.teams[testID] = t
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(t)
	})
	mux.HandleFunc("GET /api/v1/teams/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		t, ok := f.teams[r.PathValue("id")]
		if !ok {
			notFound(w)
			return
		}
		_ = json.NewEncoder(w).Encode(t)
	})
	mux.HandleFunc("PATCH /api/v1/teams/{id}", func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body zenfraclient.UpdateTeamRequest
		_ = json.Unmarshal(raw, &body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.patches = append(f.patches, string(raw))
		if refuse(w) {
			return
		}
		t, ok := f.teams[r.PathValue("id")]
		if !ok {
			notFound(w)
			return
		}
		if body.Name != nil {
			t.Name = strings.TrimSpace(*body.Name)
		}
		if body.Description != nil {
			t.Description = *body.Description
		}
		_ = json.NewEncoder(w).Encode(t)
	})
	mux.HandleFunc("DELETE /api/v1/teams/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if refuse(w) {
			return
		}
		if _, ok := f.teams[r.PathValue("id")]; !ok {
			notFound(w)
			return
		}
		delete(f.teams, r.PathValue("id"))
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

type harness struct {
	r      *TeamResource
	fake   *fakeTeamAPI
	schema resource.SchemaResponse
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	fake := &fakeTeamAPI{teams: map[string]*zenfraclient.Team{}}
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)
	client, err := zenfraclient.NewClient(zenfraclient.ClientConfig{Endpoint: server.URL, APIToken: "test", MaxRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{r: &TeamResource{client: client}, fake: fake}
	h.r.Schema(context.Background(), resource.SchemaRequest{}, &h.schema)
	return h
}

func (h *harness) state(t *testing.T, m TeamModel) tfsdk.State {
	t.Helper()
	s := tfsdk.State{Schema: h.schema.Schema}
	if d := s.Set(context.Background(), m); d.HasError() {
		t.Fatal(d)
	}
	return s
}

func (h *harness) plan(t *testing.T, m TeamModel) tfsdk.Plan {
	t.Helper()
	p := tfsdk.Plan{Schema: h.schema.Schema}
	if d := p.Set(context.Background(), m); d.HasError() {
		t.Fatal(d)
	}
	return p
}

func get(t *testing.T, s tfsdk.State) TeamModel {
	t.Helper()
	var m TeamModel
	if d := s.Get(context.Background(), &m); d.HasError() {
		t.Fatal(d)
	}
	return m
}

func stored() TeamModel {
	return TeamModel{ID: types.StringValue(testID), Name: types.StringValue("Platform"), Slug: types.StringValue("platform"), Description: types.StringNull()}
}

func (h *harness) create(t *testing.T, m TeamModel) resource.CreateResponse {
	t.Helper()
	m.ID = types.StringUnknown()
	resp := resource.CreateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	h.r.Create(context.Background(), resource.CreateRequest{Plan: h.plan(t, m)}, &resp)
	return resp
}

func TestTeam_CreateReadUpdateDelete(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	resp := h.create(t, stored())
	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: %v", resp.Diagnostics)
	}
	if got := get(t, resp.State); !got.ID.Equal(types.StringValue(testID)) || !got.Description.IsNull() {
		t.Fatalf("state = %+v", got)
	}

	read := resource.ReadResponse{State: h.state(t, stored())}
	h.r.Read(context.Background(), resource.ReadRequest{State: h.state(t, stored())}, &read)
	if read.Diagnostics.HasError() || !get(t, read.State).Name.Equal(types.StringValue("Platform")) {
		t.Fatalf("Read: %v %+v", read.Diagnostics, get(t, read.State))
	}

	planned := stored()
	planned.Description = types.StringValue("owns the base")
	upd := resource.UpdateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	h.r.Update(context.Background(), resource.UpdateRequest{Plan: h.plan(t, planned), State: h.state(t, stored())}, &upd)
	if upd.Diagnostics.HasError() {
		t.Fatalf("Update: %v", upd.Diagnostics)
	}
	if got := get(t, upd.State); got.Description.ValueString() != "owns the base" {
		t.Errorf("description = %v", got.Description)
	}
	if len(h.fake.patches) != 1 || strings.Contains(h.fake.patches[0], `"name"`) {
		t.Errorf("patches = %v, want only the changed description", h.fake.patches)
	}

	del := resource.DeleteResponse{}
	h.r.Delete(context.Background(), resource.DeleteRequest{State: h.state(t, planned)}, &del)
	if del.Diagnostics.HasError() || len(h.fake.teams) != 0 {
		t.Fatalf("Delete: %v, teams left %d", del.Diagnostics, len(h.fake.teams))
	}
	// Deleting what is already gone is not an error.
	h.r.Delete(context.Background(), resource.DeleteRequest{State: h.state(t, planned)}, &del)
	if del.Diagnostics.HasError() {
		t.Fatalf("second Delete: %v", del.Diagnostics)
	}
}

func TestTeam_GateOffSurfacesTheAPIMessage(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.fake.inactive = true
	resp := h.create(t, stored())
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error while the activation is absent")
	}
	if d := resp.Diagnostics.Errors()[0].Detail(); !strings.Contains(d, "not available yet") || !strings.Contains(d, "409") {
		t.Errorf("detail = %q, want the 409 and the API message", d)
	}
}

func TestTeam_RemovedOutOfBandLeavesState(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	read := resource.ReadResponse{State: h.state(t, stored())}
	h.r.Read(context.Background(), resource.ReadRequest{State: h.state(t, stored())}, &read)
	if read.Diagnostics.HasError() || !read.State.Raw.IsNull() {
		t.Fatalf("want the resource removed from state, got %v %v", read.Diagnostics, read.State.Raw)
	}
}

func TestTeam_Import(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.fake.teams[testID] = &zenfraclient.Team{ID: testID, Name: "Platform", Slug: "platform", Description: "d"}
	empty := tfsdk.State{Schema: h.schema.Schema, Raw: tftypes.NewValue(h.schema.Schema.Type().TerraformType(context.Background()), nil)}
	imp := resource.ImportStateResponse{State: empty}
	h.r.ImportState(context.Background(), resource.ImportStateRequest{ID: testID}, &imp)
	if imp.Diagnostics.HasError() {
		t.Fatal(imp.Diagnostics)
	}
	read := resource.ReadResponse{State: imp.State}
	h.r.Read(context.Background(), resource.ReadRequest{State: imp.State}, &read)
	got := get(t, read.State)
	if got.Slug.ValueString() != "platform" || got.Description.ValueString() != "d" {
		t.Errorf("imported state = %+v", got)
	}
}

func TestTeam_SlugForcesReplacement(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	slug, ok := h.schema.Schema.Attributes["slug"].(schema.StringAttribute)
	if !ok || len(slug.PlanModifiers) != 1 ||
		!strings.Contains(slug.PlanModifiers[0].Description(context.Background()), "destroy and recreate") {
		t.Fatalf("slug plan modifiers = %v, want RequiresReplace", slug.PlanModifiers)
	}
}

// TestTeam_NonCanonicalConfigIsRefusedAtPlan: the API lowercases the slug and
// trims the name, so a config it would rewrite is refused at plan time;
// otherwise the applied state would differ from the config. A canonical
// config round-trips through the normalizing API unchanged.
func TestTeam_NonCanonicalConfigIsRefusedAtPlan(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	refused := func(attr, value string) bool {
		a := h.schema.Schema.Attributes[attr].(schema.StringAttribute)
		for _, v := range a.Validators {
			resp := validator.StringResponse{}
			v.ValidateString(context.Background(), validator.StringRequest{Path: path.Root(attr), ConfigValue: types.StringValue(value)}, &resp)
			if resp.Diagnostics.HasError() {
				return true
			}
		}
		return false
	}
	for attr, values := range map[string][]string{"slug": {"Platform", " platform", "platform "}, "name": {" Platform ", "Platform\t"}} {
		for _, v := range values {
			if !refused(attr, v) {
				t.Errorf("%s %q passes validation; the API would store a different value", attr, v)
			}
		}
		if refused(attr, map[string]string{"slug": "platform", "name": "Platform Ops"}[attr]) {
			t.Errorf("%s: a canonical value is refused", attr)
		}
	}

	planned := stored()
	planned.Name = types.StringValue("Platform Ops")
	resp := h.create(t, planned)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if got := get(t, resp.State); !got.Name.Equal(planned.Name) || !got.Slug.Equal(planned.Slug) {
		t.Errorf("state %+v differs from the canonical plan %+v", got, planned)
	}
}
