// ABOUTME: Unit tests for zenfra_configuration_bundle hooks (content) and auto_attach_labels (metadata).
// ABOUTME: Drives the resource against an httptest API so request bodies and read-back state are asserted.
package bundle

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

const testBundleID = "bundle-1"

// fakeBundleAPI serves one bundle with the API's write semantics: a metadata
// PUT answers a status message; a content PUT replaces env vars and files,
// keeps hooks when the member is absent and clears them on {}.
type fakeBundleAPI struct {
	mu            sync.Mutex
	bundle        zenfraclient.Bundle
	metadataBody  []map[string]json.RawMessage
	contentBodies []map[string]json.RawMessage
}

func newFakeBundleAPI() *fakeBundleAPI {
	now := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	return &fakeBundleAPI{bundle: zenfraclient.Bundle{
		ID: testBundleID, OrganizationID: "org-1", SpaceID: "space-1", Name: "shared", Slug: "shared",
		AutoAttachLabels: []string{}, ContentVersion: 1, CreatedAt: now, UpdatedAt: now,
	}}
}

func decodeBody(t *testing.T, r *http.Request) map[string]json.RawMessage {
	t.Helper()
	raw, _ := io.ReadAll(r.Body)
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Errorf("request body %s: %v", raw, err)
	}
	return body
}

func (f *fakeBundleAPI) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/bundles", func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(t, r)
		f.mu.Lock()
		if l, ok := body["auto_attach_labels"]; ok {
			_ = json.Unmarshal(l, &f.bundle.AutoAttachLabels)
		}
		f.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		f.writeBundle(w)
	})
	mux.HandleFunc("GET /api/v1/bundles/"+testBundleID, func(w http.ResponseWriter, _ *http.Request) {
		f.writeBundle(w)
	})
	mux.HandleFunc("PUT /api/v1/bundles/"+testBundleID, func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(t, r)
		f.mu.Lock()
		f.metadataBody = append(f.metadataBody, body)
		if l, ok := body["auto_attach_labels"]; ok && string(l) != "null" {
			f.bundle.AutoAttachLabels = []string{}
			_ = json.Unmarshal(l, &f.bundle.AutoAttachLabels)
		}
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{"message":"bundle updated successfully"}`))
	})
	mux.HandleFunc("PUT /api/v1/bundles/"+testBundleID+"/content", func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(t, r)
		var content map[string]json.RawMessage
		_ = json.Unmarshal(body["content"], &content)
		f.mu.Lock()
		f.contentBodies = append(f.contentBodies, body)
		if h, ok := content["hooks"]; ok && string(h) != "null" {
			var hooks zenfraclient.Hooks
			_ = json.Unmarshal(h, &hooks)
			f.bundle.Hooks = &hooks
			if string(h) == "{}" {
				f.bundle.Hooks = nil
			}
		}
		f.bundle.ContentVersion++
		resp := zenfraclient.UpdateBundleContentResponse{Bundle: f.bundle}
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(resp)
	})
	return mux
}

func (f *fakeBundleAPI) writeBundle(w http.ResponseWriter) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(f.bundle)
}

type bundleHarness struct {
	r      *BundleResource
	fake   *fakeBundleAPI
	schema resource.SchemaResponse
}

func newBundleHarness(t *testing.T) *bundleHarness {
	t.Helper()
	fake := newFakeBundleAPI()
	server := httptest.NewServer(fake.handler(t))
	t.Cleanup(server.Close)
	client, err := zenfraclient.NewClient(zenfraclient.ClientConfig{Endpoint: server.URL, APIToken: "test", MaxRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	h := &bundleHarness{r: &BundleResource{client: client}, fake: fake}
	h.r.Schema(context.Background(), resource.SchemaRequest{}, &h.schema)
	return h
}

// baseModel is a bundle as state holds it: no content, no selector.
func baseModel() BundleModel {
	m := mapBundleToState(&newFakeBundleAPI().bundle)
	m.Labels = types.ListNull(types.StringType)
	m.AutoAttachLabels = types.SetNull(types.StringType)
	m.Hooks = types.ObjectNull(hooksAttrTypes())
	m.EnvironmentVariable = types.SetNull(types.ObjectType{AttrTypes: envVarAttrTypes()})
	m.MountedFile = types.SetNull(types.ObjectType{AttrTypes: mountedFileAttrTypes()})
	return m
}

func cmds(values ...string) types.List {
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	return types.ListValueMust(types.StringType, elems)
}

func labels(values ...string) types.Set {
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	return types.SetValueMust(types.StringType, elems)
}

func hooksObject(t *testing.T, beforePlan, afterApply types.List) types.Object {
	t.Helper()
	obj, diags := types.ObjectValue(hooksAttrTypes(), map[string]attr.Value{
		"before_init":  types.ListNull(types.StringType),
		"after_init":   types.ListNull(types.StringType),
		"before_plan":  beforePlan,
		"after_plan":   types.ListNull(types.StringType),
		"before_apply": types.ListNull(types.StringType),
		"after_apply":  afterApply,
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	return obj
}

func (h *bundleHarness) create(t *testing.T, planned BundleModel) BundleModel {
	t.Helper()
	ctx := context.Background()
	plan := tfsdk.Plan{Schema: h.schema.Schema}
	if d := plan.Set(ctx, planned); d.HasError() {
		t.Fatal(d)
	}
	resp := resource.CreateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	h.r.Create(ctx, resource.CreateRequest{Plan: plan}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: %v", resp.Diagnostics)
	}
	var got BundleModel
	if d := resp.State.Get(ctx, &got); d.HasError() {
		t.Fatal(d)
	}
	return got
}

func (h *bundleHarness) update(t *testing.T, prior, planned BundleModel) BundleModel {
	t.Helper()
	ctx := context.Background()
	state := tfsdk.State{Schema: h.schema.Schema}
	if d := state.Set(ctx, prior); d.HasError() {
		t.Fatal(d)
	}
	plan := tfsdk.Plan{Schema: h.schema.Schema}
	if d := plan.Set(ctx, planned); d.HasError() {
		t.Fatal(d)
	}
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	h.r.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}
	var got BundleModel
	if d := resp.State.Get(ctx, &got); d.HasError() {
		t.Fatal(d)
	}
	return got
}

func (h *bundleHarness) read(t *testing.T, prior BundleModel) BundleModel {
	t.Helper()
	ctx := context.Background()
	state := tfsdk.State{Schema: h.schema.Schema}
	if d := state.Set(ctx, prior); d.HasError() {
		t.Fatal(d)
	}
	resp := resource.ReadResponse{State: state}
	h.r.Read(ctx, resource.ReadRequest{State: state}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}
	var got BundleModel
	if d := resp.State.Get(ctx, &got); d.HasError() {
		t.Fatal(d)
	}
	return got
}

func TestCreate_HooksAloneWriteContent(t *testing.T) {
	t.Parallel()

	h := newBundleHarness(t)
	planned := baseModel()
	planned.ID = types.StringUnknown()
	planned.Hooks = hooksObject(t, cmds("make lint"), types.ListNull(types.StringType))

	got := h.create(t, planned)

	if len(h.fake.contentBodies) != 1 {
		t.Fatalf("content writes = %d, want 1: hooks are content", len(h.fake.contentBodies))
	}
	var content map[string]json.RawMessage
	_ = json.Unmarshal(h.fake.contentBodies[0]["content"], &content)
	if string(content["hooks"]) != `{"before_plan":["make lint"]}` {
		t.Errorf("hooks sent = %s", content["hooks"])
	}
	if string(h.fake.contentBodies[0]["expected_version"]) != "1" {
		t.Errorf("expected_version = %s, want the created bundle's version 1", h.fake.contentBodies[0]["expected_version"])
	}
	if !got.Hooks.Equal(planned.Hooks) {
		t.Errorf("state hooks = %v, want %v", got.Hooks, planned.Hooks)
	}
	if got.ContentVersion.ValueInt64() != 2 {
		t.Errorf("content_version = %v, want 2", got.ContentVersion)
	}
}

func TestCreate_NoContentNoContentWrite(t *testing.T) {
	t.Parallel()

	h := newBundleHarness(t)
	planned := baseModel()
	planned.AutoAttachLabels = labels("prod")

	got := h.create(t, planned)

	if len(h.fake.contentBodies) != 0 {
		t.Errorf("content writes = %d, want 0", len(h.fake.contentBodies))
	}
	if !got.AutoAttachLabels.Equal(labels("prod")) {
		t.Errorf("auto_attach_labels = %v, want [prod]", got.AutoAttachLabels)
	}
	if !got.Hooks.IsNull() {
		t.Errorf("hooks = %v, want null", got.Hooks)
	}
}

func TestUpdate_RemovingHooksSendsAnEmptyObject(t *testing.T) {
	t.Parallel()

	h := newBundleHarness(t)
	h.fake.bundle.Hooks = &zenfraclient.Hooks{BeforePlan: []string{"make lint"}}
	h.fake.bundle.ContentVersion = 4
	prior := baseModel()
	prior.ContentVersion = types.Int64Value(4)
	prior.Hooks = hooksObject(t, cmds("make lint"), types.ListNull(types.StringType))
	planned := prior
	planned.Hooks = types.ObjectNull(hooksAttrTypes())

	got := h.update(t, prior, planned)

	if len(h.fake.contentBodies) != 1 {
		t.Fatalf("content writes = %d, want 1", len(h.fake.contentBodies))
	}
	var content map[string]json.RawMessage
	_ = json.Unmarshal(h.fake.contentBodies[0]["content"], &content)
	if string(content["hooks"]) != "{}" {
		t.Errorf("hooks sent = %s, want {} (absent or null would keep the hooks)", content["hooks"])
	}
	if string(h.fake.contentBodies[0]["expected_version"]) != "4" {
		t.Errorf("expected_version = %s, want 4: hooks are fenced like env vars and files", h.fake.contentBodies[0]["expected_version"])
	}
	if len(h.fake.metadataBody) != 0 {
		t.Errorf("metadata writes = %d, want 0", len(h.fake.metadataBody))
	}
	if !got.Hooks.IsNull() {
		t.Errorf("state hooks = %v, want null", got.Hooks)
	}
}

func TestUpdate_ChangingHooksReplacesThem(t *testing.T) {
	t.Parallel()

	h := newBundleHarness(t)
	h.fake.bundle.Hooks = &zenfraclient.Hooks{BeforePlan: []string{"make lint"}}
	prior := baseModel()
	prior.Hooks = hooksObject(t, cmds("make lint"), types.ListNull(types.StringType))
	planned := prior
	planned.Hooks = hooksObject(t, cmds("make lint"), cmds("./notify.sh", "echo done"))

	got := h.update(t, prior, planned)

	var content map[string]json.RawMessage
	_ = json.Unmarshal(h.fake.contentBodies[0]["content"], &content)
	if string(content["hooks"]) != `{"before_plan":["make lint"],"after_apply":["./notify.sh","echo done"]}` {
		t.Errorf("hooks sent = %s", content["hooks"])
	}
	if !got.Hooks.Equal(planned.Hooks) {
		t.Errorf("state hooks = %v, want %v", got.Hooks, planned.Hooks)
	}
}

func TestUpdate_AutoAttachLabelsAloneAreAMetadataWrite(t *testing.T) {
	t.Parallel()

	h := newBundleHarness(t)
	prior := baseModel()
	planned := prior
	planned.AutoAttachLabels = labels("prod", "eu")

	got := h.update(t, prior, planned)

	if len(h.fake.contentBodies) != 0 {
		t.Errorf("content writes = %d, want 0: the selector is metadata", len(h.fake.contentBodies))
	}
	if len(h.fake.metadataBody) != 1 {
		t.Fatalf("metadata writes = %d, want 1", len(h.fake.metadataBody))
	}
	body := h.fake.metadataBody[0]
	if len(body) != 1 || body["auto_attach_labels"] == nil {
		t.Errorf("metadata body = %v, want only auto_attach_labels", body)
	}
	// The metadata PUT answers a status message; the state must come from the
	// bundle read back, not from that message.
	if got.ID.ValueString() != testBundleID || got.Name.ValueString() != "shared" {
		t.Errorf("state id/name = %v/%v, want the bundle read back", got.ID, got.Name)
	}
	if !got.AutoAttachLabels.Equal(labels("prod", "eu")) {
		t.Errorf("auto_attach_labels = %v", got.AutoAttachLabels)
	}
	if got.ContentVersion.ValueInt64() != 1 {
		t.Errorf("content_version = %v, want 1 (unchanged)", got.ContentVersion)
	}
}

func TestUpdate_RemovingAutoAttachLabelsSendsAnEmptyList(t *testing.T) {
	t.Parallel()

	h := newBundleHarness(t)
	h.fake.bundle.AutoAttachLabels = []string{"prod"}
	prior := baseModel()
	prior.AutoAttachLabels = labels("prod")
	planned := prior
	planned.AutoAttachLabels = types.SetNull(types.StringType)

	got := h.update(t, prior, planned)

	if len(h.fake.metadataBody) != 1 || string(h.fake.metadataBody[0]["auto_attach_labels"]) != "[]" {
		t.Fatalf("metadata bodies = %v, want auto_attach_labels [] (null would leave the selector)", h.fake.metadataBody)
	}
	if !got.AutoAttachLabels.IsNull() {
		t.Errorf("auto_attach_labels = %v, want null", got.AutoAttachLabels)
	}
}

func TestRead_MapsHooksAndSelector(t *testing.T) {
	t.Parallel()

	h := newBundleHarness(t)
	h.fake.bundle.AutoAttachLabels = []string{"prod"}
	h.fake.bundle.Hooks = &zenfraclient.Hooks{AfterApply: []string{"./notify.sh"}}

	got := h.read(t, baseModel())

	if !got.AutoAttachLabels.Equal(labels("prod")) {
		t.Errorf("auto_attach_labels = %v", got.AutoAttachLabels)
	}
	want := hooksObject(t, types.ListNull(types.StringType), cmds("./notify.sh"))
	if !got.Hooks.Equal(want) {
		t.Errorf("hooks = %v, want %v", got.Hooks, want)
	}

	// Cleared outside Terraform: [] reads back as null for an unset attribute
	// and as an empty set for a configured `= []`.
	h.fake.bundle.AutoAttachLabels = []string{}
	h.fake.bundle.Hooks = nil
	got = h.read(t, got)
	if !got.AutoAttachLabels.IsNull() || !got.Hooks.IsNull() {
		t.Errorf("after clear: auto_attach_labels = %v, hooks = %v, want both null", got.AutoAttachLabels, got.Hooks)
	}
	prior := baseModel()
	prior.AutoAttachLabels = labels()
	got = h.read(t, prior)
	if got.AutoAttachLabels.IsNull() || len(got.AutoAttachLabels.Elements()) != 0 {
		t.Errorf("configured [] read back as %v, want an empty set", got.AutoAttachLabels)
	}
}

func TestModifyPlan_ContentChangeMakesContentVersionUnknown(t *testing.T) {
	t.Parallel()

	h := newBundleHarness(t)
	ctx := context.Background()
	prior := baseModel()

	tests := []struct {
		name        string
		mutate      func(*BundleModel)
		wantUnknown bool
	}{
		{name: "hooks", mutate: func(m *BundleModel) {
			m.Hooks = hooksObject(t, cmds("make lint"), types.ListNull(types.StringType))
		}, wantUnknown: true},
		{name: "selector only", mutate: func(m *BundleModel) { m.AutoAttachLabels = labels("prod") }},
		{name: "nothing", mutate: func(*BundleModel) {}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planned := prior
			tt.mutate(&planned)
			state := tfsdk.State{Schema: h.schema.Schema}
			plan := tfsdk.Plan{Schema: h.schema.Schema}
			if d := state.Set(ctx, prior); d.HasError() {
				t.Fatal(d)
			}
			if d := plan.Set(ctx, planned); d.HasError() {
				t.Fatal(d)
			}
			resp := resource.ModifyPlanResponse{Plan: plan}
			h.r.ModifyPlan(ctx, resource.ModifyPlanRequest{Plan: plan, State: state}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			var version types.Int64
			if d := resp.Plan.GetAttribute(ctx, path.Root("content_version"), &version); d.HasError() {
				t.Fatal(d)
			}
			if version.IsUnknown() != tt.wantUnknown {
				t.Errorf("content_version unknown = %v, want %v", version.IsUnknown(), tt.wantUnknown)
			}
		})
	}
}
