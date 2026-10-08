// ABOUTME: Unit tests for zenfra_stack labels: state mapping and the requests Create and Update send.
// ABOUTME: Drives the resource against an httptest API so the wire body is asserted, not assumed.
package stack

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
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

func labelsFixture(labels []string) *zenfraclient.Stack {
	now := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	return &zenfraclient.Stack{
		ID:             "stack-1",
		OrganizationID: "org-1",
		SpaceID:        "space-1",
		Name:           "app",
		IAC:            zenfraclient.IACConfig{Engine: "terraform", Version: "1.9.0"},
		Source: zenfraclient.StackSource{
			Type: sourceTypeRawGit,
			RawGit: &zenfraclient.StackSourceRawGit{
				URL: "https://example.com/repo.git",
				Ref: zenfraclient.StackSourceRef{Type: "branch", Name: "main"},
			},
		},
		Labels:    labels,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func labelSet(values ...string) types.Set {
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	return types.SetValueMust(types.StringType, elems)
}

func TestMapStackToState_Labels(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name     string
		labels   []string
		prior    types.Set
		wantNull bool
		want     []string
	}{
		{name: "none read back as null", labels: []string{}, prior: types.SetNull(types.StringType), wantNull: true},
		{name: "none with an empty prior stay empty", labels: []string{}, prior: labelSet()},
		{name: "labels", labels: []string{"prod", "team.payments"}, prior: types.SetNull(types.StringType), want: []string{"prod", "team.payments"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			model, diags := mapStackToState(ctx, labelsFixture(tt.labels), tt.prior)
			if diags.HasError() {
				t.Fatalf("mapStackToState: %v", diags)
			}
			if model.Labels.IsNull() != tt.wantNull {
				t.Fatalf("labels null = %v, want %v", model.Labels.IsNull(), tt.wantNull)
			}
			if !tt.wantNull && !model.Labels.Equal(labelSet(tt.want...)) {
				t.Errorf("labels = %v, want %v", model.Labels, tt.want)
			}
		})
	}
}

// fakeStackAPI serves one stack and records the bodies of the writes.
type fakeStackAPI struct {
	mu     sync.Mutex
	stack  *zenfraclient.Stack
	writes []map[string]json.RawMessage
}

func (f *fakeStackAPI) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]json.RawMessage
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("request body: %v", err)
		}
		f.mu.Lock()
		f.writes = append(f.writes, body)
		if l, ok := body["labels"]; ok {
			var labels []string
			_ = json.Unmarshal(l, &labels)
			f.stack.Labels = append([]string{}, labels...)
		}
		f.mu.Unlock()
		f.get(w, r)
	}
	mux.HandleFunc("POST /api/v1/stacks", write)
	mux.HandleFunc("PUT /api/v1/stacks/stack-1", write)
	mux.HandleFunc("GET /api/v1/stacks/stack-1", f.get)
	return mux
}

func (f *fakeStackAPI) get(w http.ResponseWriter, _ *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(f.stack)
}

func newLabelsHarness(t *testing.T, apiLabels []string) (*StackResource, *fakeStackAPI, resource.SchemaResponse) {
	t.Helper()
	fake := &fakeStackAPI{stack: labelsFixture(apiLabels)}
	server := httptest.NewServer(fake.handler(t))
	t.Cleanup(server.Close)
	client, err := zenfraclient.NewClient(zenfraclient.ClientConfig{Endpoint: server.URL, APIToken: "test", MaxRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	r := &StackResource{client: client}
	var schemaResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	return r, fake, schemaResp
}

func modelWithLabels(t *testing.T, labels types.Set) *StackModel {
	t.Helper()
	model, diags := mapStackToState(context.Background(), labelsFixture(nil), types.SetNull(types.StringType))
	if diags.HasError() {
		t.Fatal(diags)
	}
	model.Labels = labels
	return model
}

func runUpdate(t *testing.T, r *StackResource, schemaResp resource.SchemaResponse, prior, planned *StackModel) *StackModel {
	t.Helper()
	ctx := context.Background()
	state := tfsdk.State{Schema: schemaResp.Schema}
	if d := state.Set(ctx, prior); d.HasError() {
		t.Fatal(d)
	}
	plan := tfsdk.Plan{Schema: schemaResp.Schema}
	if d := plan.Set(ctx, planned); d.HasError() {
		t.Fatal(d)
	}
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}
	var got StackModel
	if d := resp.State.Get(ctx, &got); d.HasError() {
		t.Fatal(d)
	}
	return &got
}

func TestUpdate_RemovingLabelsFromConfigSendsAnEmptyList(t *testing.T) {
	t.Parallel()

	r, fake, schemaResp := newLabelsHarness(t, []string{"prod"})
	got := runUpdate(t, r, schemaResp, modelWithLabels(t, labelSet("prod")), modelWithLabels(t, types.SetNull(types.StringType)))

	if len(fake.writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(fake.writes))
	}
	if string(fake.writes[0]["labels"]) != "[]" {
		t.Errorf("labels sent = %s, want [] (null or absent would leave the labels in place)", fake.writes[0]["labels"])
	}
	if !got.Labels.IsNull() {
		t.Errorf("state labels = %v, want null", got.Labels)
	}
}

func TestUpdate_ChangedLabelsReplaceTheSet(t *testing.T) {
	t.Parallel()

	r, fake, schemaResp := newLabelsHarness(t, []string{"prod"})
	got := runUpdate(t, r, schemaResp, modelWithLabels(t, labelSet("prod")), modelWithLabels(t, labelSet("prod", "eu")))

	if len(fake.writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(fake.writes))
	}
	var sent []string
	if err := json.Unmarshal(fake.writes[0]["labels"], &sent); err != nil || len(sent) != 2 {
		t.Errorf("labels sent = %s, want both labels", fake.writes[0]["labels"])
	}
	if !got.Labels.Equal(labelSet("prod", "eu")) {
		t.Errorf("state labels = %v", got.Labels)
	}
}

func TestUpdate_UnchangedLabelsAreNotSent(t *testing.T) {
	t.Parallel()

	r, fake, schemaResp := newLabelsHarness(t, []string{"prod"})
	prior := modelWithLabels(t, labelSet("prod"))
	planned := modelWithLabels(t, labelSet("prod"))
	planned.Name = types.StringValue("renamed")
	runUpdate(t, r, schemaResp, prior, planned)

	if len(fake.writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(fake.writes))
	}
	if _, ok := fake.writes[0]["labels"]; ok {
		t.Errorf("labels sent although unchanged: %s", fake.writes[0]["labels"])
	}
}

func TestCreate_SendsLabels(t *testing.T) {
	t.Parallel()

	r, fake, schemaResp := newLabelsHarness(t, nil)
	ctx := context.Background()
	planned := modelWithLabels(t, labelSet("prod"))
	plan := tfsdk.Plan{Schema: schemaResp.Schema}
	if d := plan.Set(ctx, planned); d.HasError() {
		t.Fatal(d)
	}
	resp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Create(ctx, resource.CreateRequest{Plan: plan}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: %v", resp.Diagnostics)
	}
	if len(fake.writes) != 1 || string(fake.writes[0]["labels"]) != `["prod"]` {
		t.Fatalf("create body labels = %v, want [\"prod\"]", fake.writes)
	}
	var got StackModel
	if d := resp.State.Get(ctx, &got); d.HasError() {
		t.Fatal(d)
	}
	if !got.Labels.Equal(labelSet("prod")) {
		t.Errorf("state labels = %v", got.Labels)
	}
}
