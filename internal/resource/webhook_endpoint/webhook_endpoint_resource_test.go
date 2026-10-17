// ABOUTME: Unit tests for zenfra_webhook_endpoint CRUD, import and plan modifiers against an httptest API.
// ABOUTME: Covers the once-shown secret, leaf patches, the 409 webhooks_disabled message and re-enabling an auto-disabled endpoint.
package webhook_endpoint

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

const (
	testID = "6630f0c2a1b2c3d4e5f60718"
	// #nosec G101 -- test fixture: the API's published example secret, not a real credential
	testSecret = "whsec_AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="
	testURL    = "https://hooks.example.com/zenfra"
	newURL     = "https://hooks.example.com/zenfra-v2"
	runFailed  = "run.failed"
	runOK      = "run.finished"
)

// fakeWebhookAPI models one organization's endpoints with the handler's
// JSON: the secret only in the create response, event_types sorted and
// deduplicated, disabled_reason omitted while enabled, 404 not_found for a
// missing endpoint.
type fakeWebhookAPI struct {
	mu        sync.Mutex
	endpoints map[string]*zenfraclient.WebhookEndpoint
	creates   []string // raw create bodies
	patches   []string // raw patch bodies
	gets      int
	disabled  bool // the feature flag is off: create answers 409 webhooks_disabled
}

func normalize(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range in {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}

func (f *fakeWebhookAPI) handler() http.Handler {
	mux := http.NewServeMux()
	base := "/api/v1/webhook-endpoints"
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body zenfraclient.CreateWebhookEndpointRequest
		_ = json.Unmarshal(raw, &body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.creates = append(f.creates, string(raw))
		if f.disabled {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"webhooks_disabled","message":"webhooks are not available yet"}`))
			return
		}
		now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
		ep := &zenfraclient.WebhookEndpoint{
			ID: testID, URL: body.URL, Description: body.Description, EventTypes: normalize(body.EventTypes),
			Enabled: body.Enabled == nil || *body.Enabled, CreatedAt: now, UpdatedAt: now,
		}
		if !ep.Enabled {
			ep.DisabledReason, ep.DisabledAt = zenfraclient.WebhookDisabledManual, &now
		}
		f.endpoints[testID] = ep
		w.WriteHeader(http.StatusCreated)
		// #nosec G117 -- test fixture: the fake API returns the placeholder secret
		_ = json.NewEncoder(w).Encode(zenfraclient.CreateWebhookEndpointResponse{WebhookEndpoint: *ep, Secret: testSecret})
	})
	mux.HandleFunc("GET "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.gets++
		ep, ok := f.endpoints[r.PathValue("id")]
		if !ok {
			notFound(w)
			return
		}
		_ = json.NewEncoder(w).Encode(ep)
	})
	mux.HandleFunc("PATCH "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body zenfraclient.PatchWebhookEndpointRequest
		_ = json.Unmarshal(raw, &body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.patches = append(f.patches, string(raw))
		ep, ok := f.endpoints[r.PathValue("id")]
		if !ok {
			notFound(w)
			return
		}
		f.apply(ep, &body)
		_ = json.NewEncoder(w).Encode(ep)
	})
	mux.HandleFunc("DELETE "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if _, ok := f.endpoints[r.PathValue("id")]; !ok {
			notFound(w)
			return
		}
		delete(f.endpoints, r.PathValue("id"))
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func (f *fakeWebhookAPI) apply(ep *zenfraclient.WebhookEndpoint, p *zenfraclient.PatchWebhookEndpointRequest) {
	if p.URL != nil {
		ep.URL = *p.URL
	}
	if p.Description != nil {
		ep.Description = *p.Description
	}
	if p.EventTypes != nil {
		ep.EventTypes = normalize(*p.EventTypes)
	}
	if p.Enabled != nil && *p.Enabled != ep.Enabled {
		ep.Enabled = *p.Enabled
		if ep.Enabled {
			ep.DisabledReason, ep.DisabledAt = "", nil
		} else {
			now := time.Now().UTC()
			ep.DisabledReason, ep.DisabledAt = zenfraclient.WebhookDisabledManual, &now
		}
	}
}

func notFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"error":"not_found","message":"webhook endpoint or delivery not found"}`))
}

type harness struct {
	r      *WebhookEndpointResource
	fake   *fakeWebhookAPI
	schema resource.SchemaResponse
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	fake := &fakeWebhookAPI{endpoints: map[string]*zenfraclient.WebhookEndpoint{}}
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)
	client, err := zenfraclient.NewClient(zenfraclient.ClientConfig{Endpoint: server.URL, APIToken: "test", MaxRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{r: &WebhookEndpointResource{client: client}, fake: fake}
	h.r.Schema(context.Background(), resource.SchemaRequest{}, &h.schema)
	return h
}

// seed stores an endpoint as the API holds it.
func (h *harness) seed(ep zenfraclient.WebhookEndpoint) {
	h.fake.mu.Lock()
	defer h.fake.mu.Unlock()
	h.fake.endpoints[ep.ID] = &ep
}

func eventTypes(values ...string) types.Set {
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	return types.SetValueMust(types.StringType, elems)
}

// stored is the state of an enabled endpoint Terraform created.
func stored() WebhookEndpointModel {
	return WebhookEndpointModel{
		ID:             types.StringValue(testID),
		URL:            types.StringValue(testURL),
		Description:    types.StringNull(),
		EventTypes:     eventTypes(runFailed),
		Enabled:        types.BoolValue(true),
		Secret:         types.StringValue(testSecret),
		DisabledReason: types.StringNull(),
	}
}

func apiEndpoint() zenfraclient.WebhookEndpoint {
	return zenfraclient.WebhookEndpoint{ID: testID, URL: testURL, EventTypes: []string{runFailed}, Enabled: true}
}

func (h *harness) state(t *testing.T, m WebhookEndpointModel) tfsdk.State {
	t.Helper()
	s := tfsdk.State{Schema: h.schema.Schema}
	if d := s.Set(context.Background(), m); d.HasError() {
		t.Fatal(d)
	}
	return s
}

func (h *harness) plan(t *testing.T, m WebhookEndpointModel) tfsdk.Plan {
	t.Helper()
	p := tfsdk.Plan{Schema: h.schema.Schema}
	if d := p.Set(context.Background(), m); d.HasError() {
		t.Fatal(d)
	}
	return p
}

func get(t *testing.T, s tfsdk.State) WebhookEndpointModel {
	t.Helper()
	var m WebhookEndpointModel
	if d := s.Get(context.Background(), &m); d.HasError() {
		t.Fatal(d)
	}
	return m
}

func (h *harness) read(t *testing.T, prior tfsdk.State) resource.ReadResponse {
	t.Helper()
	resp := resource.ReadResponse{State: prior}
	h.r.Read(context.Background(), resource.ReadRequest{State: prior}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}
	return resp
}

func (h *harness) update(t *testing.T, plan, prior WebhookEndpointModel) WebhookEndpointModel {
	t.Helper()
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	h.r.Update(context.Background(), resource.UpdateRequest{Plan: h.plan(t, plan), State: h.state(t, prior)}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}
	return get(t, resp.State)
}

func TestCreate_KeepsTheSecretFromTheCreateResponse(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	planned := stored()
	planned.ID, planned.Secret, planned.DisabledReason = types.StringUnknown(), types.StringUnknown(), types.StringNull()
	planned.EventTypes = eventTypes(runOK, runFailed)
	resp := resource.CreateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	h.r.Create(context.Background(), resource.CreateRequest{Plan: h.plan(t, planned)}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: %v", resp.Diagnostics)
	}

	got := get(t, resp.State)
	if got.Secret.ValueString() != testSecret || got.ID.ValueString() != testID {
		t.Errorf("state = %+v, want the create response's id and secret", got)
	}
	if !got.Enabled.ValueBool() || !got.DisabledReason.IsNull() || !got.Description.IsNull() {
		t.Errorf("state = %+v, want enabled, no reason, null description", got)
	}
	if !got.EventTypes.Equal(eventTypes(runFailed, runOK)) {
		t.Errorf("event_types = %v", got.EventTypes)
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(h.fake.creates[0]), &body); err != nil {
		t.Fatal(err)
	}
	if body["url"] != testURL || body["enabled"] != true || len(body["event_types"].([]any)) != 2 {
		t.Errorf("create body = %s", h.fake.creates[0])
	}
	if _, ok := body["description"]; ok {
		t.Errorf("create body = %s, want no description for a null one", h.fake.creates[0])
	}
}

func TestCreate_FeatureOffSurfacesTheAPIMessage(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.fake.disabled = true
	planned := stored()
	planned.ID, planned.Secret = types.StringUnknown(), types.StringUnknown()
	resp := resource.CreateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	h.r.Create(context.Background(), resource.CreateRequest{Plan: h.plan(t, planned)}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error with the feature flag off")
	}
	if detail := resp.Diagnostics.Errors()[0].Detail(); !strings.Contains(detail, "webhooks are not available yet") {
		t.Errorf("detail = %q, want the API message", detail)
	}
}

func TestRead_KeepsTheSecretTheAPINeverReturns(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.seed(apiEndpoint())
	got := get(t, h.read(t, h.state(t, stored())).State)
	if got.Secret.ValueString() != testSecret {
		t.Errorf("secret = %v, want it kept from state", got.Secret)
	}
}

func TestImport_LeavesTheSecretNull(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	ep := apiEndpoint()
	ep.Description = "on-call"
	h.seed(ep)

	empty := tfsdk.State{Schema: h.schema.Schema, Raw: tftypes.NewValue(h.schema.Schema.Type().TerraformType(context.Background()), nil)}
	imp := resource.ImportStateResponse{State: empty}
	h.r.ImportState(context.Background(), resource.ImportStateRequest{ID: testID}, &imp)
	if imp.Diagnostics.HasError() {
		t.Fatalf("ImportState: %v", imp.Diagnostics)
	}
	got := get(t, h.read(t, imp.State).State)
	if !got.Secret.IsNull() {
		t.Errorf("secret = %v, want null after import", got.Secret)
	}
	if got.URL.ValueString() != testURL || got.Description.ValueString() != "on-call" || !got.Enabled.ValueBool() {
		t.Errorf("state = %+v, want the API's endpoint", got)
	}
}

func TestRead_MissingEndpointIsRemovedFromState(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	resp := h.read(t, h.state(t, stored()))
	if !resp.State.Raw.IsNull() {
		t.Error("a 404 must remove the resource from state")
	}
}

func TestUpdate_URLIsALeafPatch(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.seed(apiEndpoint())
	planned := stored()
	planned.URL = types.StringValue(newURL)
	got := h.update(t, planned, stored())

	if len(h.fake.patches) != 1 || h.fake.patches[0] != `{"url":"`+newURL+`"}` {
		t.Errorf("patches = %v, want exactly the url", h.fake.patches)
	}
	if got.URL.ValueString() != newURL || got.Secret.ValueString() != testSecret {
		t.Errorf("state = %+v, want the new url and the kept secret", got)
	}
}

func TestUpdate_RemovingTheDescriptionClearsIt(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	ep := apiEndpoint()
	ep.Description = "old"
	h.seed(ep)
	prior := stored()
	prior.Description = types.StringValue("old")
	got := h.update(t, stored(), prior)

	if len(h.fake.patches) != 1 || h.fake.patches[0] != `{"description":""}` {
		t.Errorf("patches = %v, want an empty description", h.fake.patches)
	}
	if !got.Description.IsNull() {
		t.Errorf("description = %v, want null", got.Description)
	}
}

func TestUpdate_EventTypesReplaceTheSet(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.seed(apiEndpoint())
	planned := stored()
	planned.EventTypes = eventTypes(runOK, runFailed)
	got := h.update(t, planned, stored())

	if len(h.fake.patches) != 1 || h.fake.patches[0] != `{"event_types":["`+runFailed+`","`+runOK+`"]}` &&
		h.fake.patches[0] != `{"event_types":["`+runOK+`","`+runFailed+`"]}` {
		t.Errorf("patches = %v, want exactly the event types", h.fake.patches)
	}
	if !got.EventTypes.Equal(eventTypes(runFailed, runOK)) {
		t.Errorf("event_types = %v", got.EventTypes)
	}
}

func TestUpdate_NothingChangedSendsNoPatch(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.seed(apiEndpoint())
	got := h.update(t, stored(), stored())
	if len(h.fake.patches) != 0 || h.fake.gets != 1 {
		t.Errorf("patches = %v, gets = %d, want no patch and one read", h.fake.patches, h.fake.gets)
	}
	if got.Secret.ValueString() != testSecret {
		t.Errorf("secret = %v", got.Secret)
	}
}

// An endpoint Zenfra disabled after delivery failures: refresh reads
// enabled = false with reason auto_failures, the config (default) says true,
// so the plan carries the change and Update sends exactly enabled = true.
func TestAutoDisabledEndpointIsReEnabledOnApply(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	ep := apiEndpoint()
	ep.Enabled, ep.DisabledReason = false, zenfraclient.WebhookDisabledAutoFailures
	h.seed(ep)

	refreshed := get(t, h.read(t, h.state(t, stored())).State)
	if refreshed.Enabled.ValueBool() || refreshed.DisabledReason.ValueString() != zenfraclient.WebhookDisabledAutoFailures {
		t.Fatalf("refreshed = %+v, want enabled false with reason auto_failures", refreshed)
	}

	planned := refreshed
	planned.Enabled = types.BoolValue(true)
	planned.DisabledReason = h.planDisabledReason(t, planned, refreshed)
	if !planned.DisabledReason.IsNull() {
		t.Errorf("planned disabled_reason = %v, want null once re-enabled", planned.DisabledReason)
	}

	got := h.update(t, planned, refreshed)
	if len(h.fake.patches) != 1 || h.fake.patches[0] != `{"enabled":true}` {
		t.Errorf("patches = %v, want exactly enabled true", h.fake.patches)
	}
	if !got.Enabled.ValueBool() || !got.DisabledReason.IsNull() || got.Secret.ValueString() != testSecret {
		t.Errorf("state = %+v, want enabled, no reason, the kept secret", got)
	}
}

func TestDelete_ToleratesAMissingEndpoint(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	resp := resource.DeleteResponse{}
	h.r.Delete(context.Background(), resource.DeleteRequest{State: h.state(t, stored())}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete: %v", resp.Diagnostics)
	}
}

// planDisabledReason runs the disabled_reason plan modifier with the
// attribute unknown, as Terraform plans a computed attribute of a changed
// resource.
func (h *harness) planDisabledReason(t *testing.T, planned, prior WebhookEndpointModel) types.String {
	t.Helper()
	planned.DisabledReason = types.StringUnknown()
	req := planmodifier.StringRequest{
		Path:       path.Root("disabled_reason"),
		Plan:       h.plan(t, planned),
		PlanValue:  types.StringUnknown(),
		State:      h.state(t, prior),
		StateValue: prior.DisabledReason,
	}
	resp := planmodifier.StringResponse{PlanValue: req.PlanValue}
	disabledReasonPlan{}.PlanModifyString(context.Background(), req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	return resp.PlanValue
}

func TestDisabledReasonPlan(t *testing.T) {
	t.Parallel()

	reason := func(r string) types.String {
		if r == "" {
			return types.StringNull()
		}
		return types.StringValue(r)
	}
	tests := []struct {
		name         string
		priorEnabled bool
		priorReason  string
		planEnabled  bool
		wantUnknown  bool
		want         string
	}{
		{name: "stays enabled", priorEnabled: true, planEnabled: true},
		{name: "re-enabled", priorEnabled: false, priorReason: zenfraclient.WebhookDisabledAutoFailures, planEnabled: true},
		{name: "stays disabled", priorEnabled: false, priorReason: zenfraclient.WebhookDisabledAutoFailures, want: zenfraclient.WebhookDisabledAutoFailures},
		{name: "disabled now", priorEnabled: true, planEnabled: false, wantUnknown: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			prior := stored()
			prior.Enabled, prior.DisabledReason = types.BoolValue(tt.priorEnabled), reason(tt.priorReason)
			planned := prior
			planned.Enabled = types.BoolValue(tt.planEnabled)
			got := h.planDisabledReason(t, planned, prior)
			switch {
			case tt.wantUnknown:
				if !got.IsUnknown() {
					t.Errorf("got %v, want unknown", got)
				}
			case !got.Equal(reason(tt.want)):
				t.Errorf("got %v, want %v", got, reason(tt.want))
			}
		})
	}
}

func TestMapEndpointToState_Description(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		api   string
		prior types.String
		want  types.String
	}{
		{name: "empty, unset", api: "", prior: types.StringNull(), want: types.StringNull()},
		{name: "empty, explicit empty", api: "", prior: types.StringValue(""), want: types.StringValue("")},
		{name: "empty, was set", api: "", prior: types.StringValue("old"), want: types.StringNull()},
		{name: "set", api: "on-call", prior: types.StringNull(), want: types.StringValue("on-call")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ep := apiEndpoint()
			ep.Description = tt.api
			got, diags := mapEndpointToState(&ep, tt.prior, types.StringNull())
			if diags.HasError() {
				t.Fatal(diags)
			}
			if !got.Description.Equal(tt.want) {
				t.Errorf("description = %v, want %v", got.Description, tt.want)
			}
		})
	}
}
