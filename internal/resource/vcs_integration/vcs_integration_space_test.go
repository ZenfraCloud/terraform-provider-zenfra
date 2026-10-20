// ABOUTME: Unit tests for the space_id scope of zenfra_vcs_integration against an httptest API (zenfra-cloud#661).
// ABOUTME: Create scoped, change scope in place, remove to go organization-wide, unchanged scope not sent, import keeps it.
package vcs_integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
	integrationID = "6630f0c2a1b2c3d4e5f60700"
	spaceA        = "6630f0c2a1b2c3d4e5f60718"
	spaceB        = "6630f0c2a1b2c3d4e5f60719"
)

// fakeIntegrationsAPI models /api/v1/vcs/integrations with the handler's JSON:
// space_id is always present in a response, null when organization-wide, and
// a PATCH tells an absent space_id (unchanged) from null (unscope).
type fakeIntegrationsAPI struct {
	mu          sync.Mutex
	integration *zenfraclient.VCSIntegration
	lastBody    map[string]json.RawMessage // the last POST or PATCH body, by key
}

func (f *fakeIntegrationsAPI) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	base := "/api/v1/vcs/integrations"
	decode := func(r *http.Request) ([]byte, map[string]json.RawMessage) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		body := map[string]json.RawMessage{}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("decode body %s: %v", raw, err)
		}
		return raw, body
	}
	spaceOf := func(raw json.RawMessage) *string {
		var id *string
		if err := json.Unmarshal(raw, &id); err != nil {
			t.Errorf("space_id %s: %v", raw, err)
		}
		return id
	}
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		raw, body := decode(r)
		var req zenfraclient.CreateVCSIntegrationRequest
		_ = json.Unmarshal(raw, &req)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.lastBody = body
		f.integration = &zenfraclient.VCSIntegration{
			ID: integrationID, OrganizationID: "org-1", Provider: req.Provider, Status: "active",
			DisplayName: req.DisplayName, SpaceID: req.SpaceID,
			GitHub: &zenfraclient.VCSGitHubConfig{InstallationID: req.GitHub.InstallationID},
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(f.integration)
	})
	mux.HandleFunc("GET "+base+"/{id}", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(f.integration)
	})
	mux.HandleFunc("PATCH "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, body := decode(r)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.lastBody = body
		if raw, ok := body["display_name"]; ok {
			_ = json.Unmarshal(raw, &f.integration.DisplayName)
		}
		if raw, ok := body["space_id"]; ok {
			f.integration.SpaceID = spaceOf(raw)
		}
		_ = json.NewEncoder(w).Encode(f.integration)
	})
	return mux
}

type harness struct {
	r      *VCSIntegrationResource
	fake   *fakeIntegrationsAPI
	schema resource.SchemaResponse
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	fake := &fakeIntegrationsAPI{}
	server := httptest.NewServer(fake.handler(t))
	t.Cleanup(server.Close)
	client, err := zenfraclient.NewClient(zenfraclient.ClientConfig{Endpoint: server.URL, APIToken: "test", MaxRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{r: &VCSIntegrationResource{client: client}, fake: fake}
	h.r.Schema(context.Background(), resource.SchemaRequest{}, &h.schema)
	return h
}

func (h *harness) state(t *testing.T, m VCSIntegrationModel) tfsdk.State {
	t.Helper()
	s := tfsdk.State{Schema: h.schema.Schema}
	if d := s.Set(context.Background(), m); d.HasError() {
		t.Fatal(d)
	}
	return s
}

func (h *harness) plan(t *testing.T, m VCSIntegrationModel) tfsdk.Plan {
	t.Helper()
	p := tfsdk.Plan{Schema: h.schema.Schema}
	if d := p.Set(context.Background(), m); d.HasError() {
		t.Fatal(d)
	}
	return p
}

func get(t *testing.T, s tfsdk.State) VCSIntegrationModel {
	t.Helper()
	var m VCSIntegrationModel
	if d := s.Get(context.Background(), &m); d.HasError() {
		t.Fatal(d)
	}
	return m
}

// stored is the state of the GitHub integration with the given name and scope
// (an empty space is organization-wide).
func stored(name, space string) VCSIntegrationModel {
	m := VCSIntegrationModel{
		ID: types.StringValue(integrationID), OrganizationID: types.StringValue("org-1"),
		Name: types.StringValue(name), ProviderType: types.StringValue("github"),
		PersonalAccessToken: types.StringNull(), APIURL: types.StringNull(),
		InstallationID: types.Int64Value(42), SpaceID: types.StringNull(),
		Status: types.StringValue("active"), CreatedAt: types.StringValue(""), UpdatedAt: types.StringValue(""),
	}
	if space != "" {
		m.SpaceID = types.StringValue(space)
	}
	return m
}

func (h *harness) create(t *testing.T, space string) VCSIntegrationModel {
	t.Helper()
	planned := stored("GitHub Org", space)
	planned.ID, planned.OrganizationID, planned.APIURL = types.StringUnknown(), types.StringUnknown(), types.StringUnknown()
	planned.Status, planned.CreatedAt, planned.UpdatedAt = types.StringUnknown(), types.StringUnknown(), types.StringUnknown()
	resp := resource.CreateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	h.r.Create(context.Background(), resource.CreateRequest{Plan: h.plan(t, planned)}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: %v", resp.Diagnostics)
	}
	return get(t, resp.State)
}

func (h *harness) update(t *testing.T, from, to VCSIntegrationModel) VCSIntegrationModel {
	t.Helper()
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	h.r.Update(context.Background(), resource.UpdateRequest{Plan: h.plan(t, to), State: h.state(t, from)}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}
	return get(t, resp.State)
}

// sent returns a key of the last POST or PATCH body as raw JSON, and whether
// the body carried it.
func (h *harness) sent(key string) (string, bool) {
	h.fake.mu.Lock()
	defer h.fake.mu.Unlock()
	raw, ok := h.fake.lastBody[key]
	return string(raw), ok
}

func (h *harness) storedSpace() *string {
	h.fake.mu.Lock()
	defer h.fake.mu.Unlock()
	return h.fake.integration.SpaceID
}

func TestVCSIntegration_CreateScoped(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	got := h.create(t, spaceA)
	if sent, ok := h.sent("space_id"); !ok || sent != `"`+spaceA+`"` {
		t.Errorf("POST space_id = %s (present %v), want %q", sent, ok, spaceA)
	}
	if got.SpaceID.ValueString() != spaceA {
		t.Errorf("state space_id = %v, want %s", got.SpaceID, spaceA)
	}
}

func TestVCSIntegration_CreateOrganizationWide(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	got := h.create(t, "")
	if sent, ok := h.sent("space_id"); ok {
		t.Errorf("POST carried space_id %s for an organization-wide integration", sent)
	}
	if !got.SpaceID.IsNull() {
		t.Errorf("state space_id = %v, want null", got.SpaceID)
	}
}

func TestVCSIntegration_ScopeChangesInPlace(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.create(t, spaceA)

	got := h.update(t, stored("GitHub Org", spaceA), stored("GitHub Org", spaceB))
	if sent, ok := h.sent("space_id"); !ok || sent != `"`+spaceB+`"` {
		t.Errorf("PATCH space_id = %s (present %v), want %q", sent, ok, spaceB)
	}
	if got.SpaceID.ValueString() != spaceB || got.ID.ValueString() != integrationID {
		t.Errorf("state = %+v, want the same integration scoped to %s", got, spaceB)
	}
}

func TestVCSIntegration_RemovedScopeGoesOrganizationWide(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.create(t, spaceA)

	got := h.update(t, stored("GitHub Org", spaceA), stored("GitHub Org", ""))
	if sent, ok := h.sent("space_id"); !ok || sent != "null" {
		t.Errorf("PATCH space_id = %s (present %v), want null", sent, ok)
	}
	if stored := h.storedSpace(); !got.SpaceID.IsNull() || stored != nil {
		t.Errorf("state space_id = %v, stored %v, want both null", got.SpaceID, stored)
	}
}

func TestVCSIntegration_UnchangedScopeNotSent(t *testing.T) {
	t.Parallel()
	for name, space := range map[string]string{"scoped": spaceA, "organization-wide": ""} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.create(t, space)

			got := h.update(t, stored("GitHub Org", space), stored("Renamed", space))
			if sent, ok := h.sent("space_id"); ok {
				t.Errorf("PATCH carried space_id %s although it did not change", sent)
			}
			if _, ok := h.sent("display_name"); !ok {
				t.Error("PATCH did not carry the changed display_name")
			}
			if got.Name.ValueString() != "Renamed" || got.SpaceID.ValueString() != space {
				t.Errorf("state = %+v", got)
			}
		})
	}
}

func TestVCSIntegration_ImportKeepsScope(t *testing.T) {
	t.Parallel()
	for name, space := range map[string]*string{"scoped": strPtr(spaceA), "organization-wide": nil} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.fake.integration = &zenfraclient.VCSIntegration{
				ID: integrationID, OrganizationID: "org-1", Provider: "github", Status: "active",
				DisplayName: "GitHub Org", SpaceID: space, GitHub: &zenfraclient.VCSGitHubConfig{InstallationID: 42},
			}
			empty := tfsdk.State{Schema: h.schema.Schema, Raw: tftypes.NewValue(h.schema.Schema.Type().TerraformType(context.Background()), nil)}
			imp := resource.ImportStateResponse{State: empty}
			h.r.ImportState(context.Background(), resource.ImportStateRequest{ID: integrationID}, &imp)
			if imp.Diagnostics.HasError() {
				t.Fatal(imp.Diagnostics)
			}
			read := resource.ReadResponse{State: imp.State}
			h.r.Read(context.Background(), resource.ReadRequest{State: imp.State}, &read)
			if read.Diagnostics.HasError() {
				t.Fatalf("Read: %v", read.Diagnostics)
			}
			got := get(t, read.State).SpaceID
			if !got.Equal(types.StringPointerValue(space)) {
				t.Errorf("imported space_id = %v, want %v", got, types.StringPointerValue(space))
			}
		})
	}
}

// space_id must be Optional without Computed: a Computed attribute removed
// from the configuration plans unknown instead of null, and the integration
// would never be made organization-wide. No plan modifier: it is changed in
// place, never by replacement.
func TestVCSIntegration_SpaceIDSchema(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	a, ok := h.schema.Schema.Attributes["space_id"].(schema.StringAttribute)
	if !ok {
		t.Fatal("space_id is not a string attribute")
	}
	if !a.Optional || a.Computed || a.Required || len(a.PlanModifiers) != 0 {
		t.Errorf("space_id = optional %v, computed %v, required %v, plan modifiers %v; want optional only",
			a.Optional, a.Computed, a.Required, a.PlanModifiers)
	}
}
