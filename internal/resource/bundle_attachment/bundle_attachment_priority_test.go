// ABOUTME: Unit tests for zenfra_bundle_attachment priority, read and delete against an httptest API.
// ABOUTME: Covers in-place priority updates, auto-attached-only reads and the 409 auto_attached delete.
package bundle_attachment

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

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

const (
	testStackID  = "stack-1"
	testBundleID = "bundle-1"
)

// fakeAttachmentAPI models one stack's attachments with the API's rules:
// PATCH refuses priority 0 (the binding requires a non-zero value) and both
// PATCH and DELETE answer 409 auto_attached for a bundle that reaches the
// stack only by label.
type fakeAttachmentAPI struct {
	mu        sync.Mutex
	explicit  map[string]int // bundle ID -> priority
	auto      map[string]bool
	patches   []string
	detachErr int // status DELETE answers with, 0 = normal behaviour
	patchErr  int // status PATCH answers with, 0 = normal behaviour
}

func (f *fakeAttachmentAPI) handler() http.Handler {
	mux := http.NewServeMux()
	base := "/api/v1/stacks/" + testStackID + "/bundles"
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		var body zenfraclient.AttachBundleRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.explicit[body.BundleID] = 0
		f.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"message":"bundle attached successfully"}`))
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		resp := zenfraclient.ListAttachmentsResponse{Attachments: []zenfraclient.BundleAttachment{}, AutoAttached: []zenfraclient.BundleAttachment{}}
		for id, p := range f.explicit {
			resp.Attachments = append(resp.Attachments, zenfraclient.BundleAttachment{
				ID: "att-" + id, StackID: testStackID, BundleID: id, Source: zenfraclient.AttachmentSourceExplicit, Priority: &p,
			})
		}
		for id := range f.auto {
			resp.AutoAttached = append(resp.AutoAttached, zenfraclient.BundleAttachment{
				StackID: testStackID, BundleID: id, Source: zenfraclient.AttachmentSourceAuto, BundleSlug: "slug-" + id,
			})
		}
		resp.Total, resp.AutoMatchCount = len(resp.Attachments), len(resp.AutoAttached)
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("PATCH "+base+"/{bundle}", func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body struct {
			Priority int `json:"priority"`
		}
		_ = json.Unmarshal(raw, &body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.patches = append(f.patches, string(raw))
		if f.patchErr != 0 {
			w.WriteHeader(f.patchErr)
			_, _ = w.Write([]byte(`{"code":"internal","message":"patch refused"}`))
			return
		}
		id := r.PathValue("bundle")
		if _, ok := f.explicit[id]; !ok {
			f.notAttached(w, id)
			return
		}
		if body.Priority == 0 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"code":"invalid_argument","message":"Key: 'UpdateBundlePriorityRequest.Priority' Error:Field validation for 'Priority' failed on the 'required' tag"}`))
			return
		}
		f.explicit[id] = body.Priority
		_, _ = w.Write([]byte(`{"message":"priority updated successfully"}`))
	})
	mux.HandleFunc("DELETE "+base+"/{bundle}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.detachErr != 0 {
			w.WriteHeader(f.detachErr)
			_, _ = w.Write([]byte(`{"code":"internal","message":"boom"}`))
			return
		}
		id := r.PathValue("bundle")
		if _, ok := f.explicit[id]; !ok {
			f.notAttached(w, id)
			return
		}
		delete(f.explicit, id)
		_, _ = w.Write([]byte(`{"message":"bundle detached successfully"}`))
	})
	return mux
}

func (f *fakeAttachmentAPI) notAttached(w http.ResponseWriter, id string) {
	if f.auto[id] {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"auto_attached","message":"bundle is auto-attached by label"}`))
		return
	}
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"code":"not_found","message":"bundle attachment not found"}`))
}

type attachmentHarness struct {
	r      *BundleAttachmentResource
	fake   *fakeAttachmentAPI
	schema resource.SchemaResponse
}

func newAttachmentHarness(t *testing.T) *attachmentHarness {
	t.Helper()
	fake := &fakeAttachmentAPI{explicit: map[string]int{}, auto: map[string]bool{}}
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)
	client, err := zenfraclient.NewClient(zenfraclient.ClientConfig{Endpoint: server.URL, APIToken: "test", MaxRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	h := &attachmentHarness{r: &BundleAttachmentResource{client: client}, fake: fake}
	h.r.Schema(context.Background(), resource.SchemaRequest{}, &h.schema)
	return h
}

func model(priority types.Int64) BundleAttachmentModel {
	return BundleAttachmentModel{
		ID:       types.StringValue(testStackID + ":" + testBundleID),
		StackID:  types.StringValue(testStackID),
		BundleID: types.StringValue(testBundleID),
		Priority: priority,
	}
}

func (h *attachmentHarness) state(t *testing.T, m BundleAttachmentModel) tfsdk.State {
	t.Helper()
	s := tfsdk.State{Schema: h.schema.Schema}
	if d := s.Set(context.Background(), m); d.HasError() {
		t.Fatal(d)
	}
	return s
}

func (h *attachmentHarness) plan(t *testing.T, m BundleAttachmentModel) tfsdk.Plan {
	t.Helper()
	p := tfsdk.Plan{Schema: h.schema.Schema}
	if d := p.Set(context.Background(), m); d.HasError() {
		t.Fatal(d)
	}
	return p
}

func TestCreate_PriorityIsPatchedOnlyWhenNotZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		priority    types.Int64
		wantPatches int
		want        int64
	}{
		{name: "omitted", priority: types.Int64Unknown(), wantPatches: 0, want: 0},
		{name: "zero", priority: types.Int64Value(0), wantPatches: 0, want: 0},
		{name: "five", priority: types.Int64Value(5), wantPatches: 1, want: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newAttachmentHarness(t)
			planned := model(tt.priority)
			planned.ID = types.StringUnknown()
			resp := resource.CreateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
			h.r.Create(context.Background(), resource.CreateRequest{Plan: h.plan(t, planned)}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("Create: %v", resp.Diagnostics)
			}
			if len(h.fake.patches) != tt.wantPatches {
				t.Errorf("patches = %v, want %d", h.fake.patches, tt.wantPatches)
			}
			var got BundleAttachmentModel
			if d := resp.State.Get(context.Background(), &got); d.HasError() {
				t.Fatal(d)
			}
			if got.Priority.ValueInt64() != tt.want || got.ID.ValueString() != testStackID+":"+testBundleID {
				t.Errorf("state = %+v, want priority %d", got, tt.want)
			}
		})
	}
}

func TestUpdate_PriorityChangesInPlace(t *testing.T) {
	t.Parallel()

	h := newAttachmentHarness(t)
	h.fake.explicit[testBundleID] = 5
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	h.r.Update(context.Background(), resource.UpdateRequest{
		Plan:  h.plan(t, model(types.Int64Value(2))),
		State: h.state(t, model(types.Int64Value(5))),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}
	if len(h.fake.patches) != 1 || h.fake.patches[0] != `{"priority":2}` {
		t.Errorf("patches = %v, want one {\"priority\":2}", h.fake.patches)
	}
	var got BundleAttachmentModel
	if d := resp.State.Get(context.Background(), &got); d.HasError() {
		t.Fatal(d)
	}
	if got.Priority.ValueInt64() != 2 {
		t.Errorf("priority = %v, want 2", got.Priority)
	}
}

func TestUpdate_BackToZeroExplainsTheAPIRefusal(t *testing.T) {
	t.Parallel()

	h := newAttachmentHarness(t)
	h.fake.explicit[testBundleID] = 5
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	h.r.Update(context.Background(), resource.UpdateRequest{
		Plan:  h.plan(t, model(types.Int64Value(0))),
		State: h.state(t, model(types.Int64Value(5))),
	}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error: the API refuses priority 0 on PATCH")
	}
	if detail := resp.Diagnostics.Errors()[0].Detail(); !strings.Contains(detail, "-replace") {
		t.Errorf("detail = %q, want it to point at replacing the attachment", detail)
	}
}

func TestUpdate_AutoAttachedOnlyIsAnError(t *testing.T) {
	t.Parallel()

	h := newAttachmentHarness(t)
	h.fake.auto[testBundleID] = true
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	h.r.Update(context.Background(), resource.UpdateRequest{
		Plan:  h.plan(t, model(types.Int64Value(3))),
		State: h.state(t, model(types.Int64Value(1))),
	}, &resp)
	if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "only by label") {
		t.Errorf("diagnostics = %v, want an error saying the bundle reaches the stack only by label", resp.Diagnostics)
	}
}

func TestRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		explicit    map[string]int
		auto        map[string]bool
		wantRemoved bool
		want        int64
	}{
		{name: "explicit refreshes the priority", explicit: map[string]int{testBundleID: 7}, want: 7},
		{name: "explicit and auto-attached is still explicit", explicit: map[string]int{testBundleID: 1}, auto: map[string]bool{testBundleID: true}, want: 1},
		{name: "auto-attached only is gone", auto: map[string]bool{testBundleID: true}, wantRemoved: true},
		{name: "not attached at all is gone", wantRemoved: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newAttachmentHarness(t)
			for k, v := range tt.explicit {
				h.fake.explicit[k] = v
			}
			for k, v := range tt.auto {
				h.fake.auto[k] = v
			}
			st := h.state(t, model(types.Int64Value(0)))
			resp := resource.ReadResponse{State: st}
			h.r.Read(context.Background(), resource.ReadRequest{State: st}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("Read: %v", resp.Diagnostics)
			}
			if resp.State.Raw.IsNull() != tt.wantRemoved {
				t.Fatalf("removed = %v, want %v", resp.State.Raw.IsNull(), tt.wantRemoved)
			}
			if tt.wantRemoved {
				return
			}
			var got BundleAttachmentModel
			if d := resp.State.Get(context.Background(), &got); d.HasError() {
				t.Fatal(d)
			}
			if got.Priority.ValueInt64() != tt.want {
				t.Errorf("priority = %v, want %d", got.Priority, tt.want)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		explicit  bool
		auto      bool
		detachErr int
		wantError bool
	}{
		{name: "explicit is detached", explicit: true},
		{name: "409 auto_attached means already gone", auto: true},
		{name: "404 means already gone"},
		{name: "another failure is an error", explicit: true, detachErr: http.StatusInternalServerError, wantError: true},
		{name: "another conflict is an error", explicit: true, detachErr: http.StatusConflict, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newAttachmentHarness(t)
			if tt.explicit {
				h.fake.explicit[testBundleID] = 0
			}
			h.fake.auto[testBundleID] = tt.auto
			h.fake.detachErr = tt.detachErr
			resp := resource.DeleteResponse{State: h.state(t, model(types.Int64Value(0)))}
			h.r.Delete(context.Background(), resource.DeleteRequest{State: h.state(t, model(types.Int64Value(0)))}, &resp)
			if got := resp.Diagnostics.HasError(); got != tt.wantError {
				t.Errorf("HasError = %v, want %v: %v", got, tt.wantError, resp.Diagnostics)
			}
			if !tt.wantError {
				if _, still := h.fake.explicit[testBundleID]; still {
					t.Error("explicit attachment still present")
				}
			}
		})
	}
}

func TestImportState_LeavesPriorityToRead(t *testing.T) {
	t.Parallel()

	h := newAttachmentHarness(t)
	h.fake.explicit[testBundleID] = 4
	ctx := context.Background()
	importResp := resource.ImportStateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	importResp.State.Raw = h.state(t, model(types.Int64Null())).Raw
	h.r.ImportState(ctx, resource.ImportStateRequest{ID: fmt.Sprintf("%s:%s", testStackID, testBundleID)}, &importResp)
	if importResp.Diagnostics.HasError() {
		t.Fatal(importResp.Diagnostics)
	}
	readResp := resource.ReadResponse{State: importResp.State}
	h.r.Read(ctx, resource.ReadRequest{State: importResp.State}, &readResp)
	var got BundleAttachmentModel
	if d := readResp.State.Get(ctx, &got); d.HasError() {
		t.Fatal(d)
	}
	if got.Priority.ValueInt64() != 4 {
		t.Errorf("priority after import = %v, want 4", got.Priority)
	}
}

// A priority PATCH that fails after the attach must leave the attachment in
// state (tainted, at the priority it has), not untracked.
func TestCreate_PriorityFailureKeepsTheAttachmentTracked(t *testing.T) {
	t.Parallel()

	h := newAttachmentHarness(t)
	h.fake.patchErr = http.StatusInternalServerError
	planned := model(types.Int64Value(5))
	planned.ID = types.StringUnknown()
	resp := resource.CreateResponse{State: tfsdk.State{Schema: h.schema.Schema}}
	h.r.Create(context.Background(), resource.CreateRequest{Plan: h.plan(t, planned)}, &resp)

	if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "patch refused") {
		t.Fatalf("diagnostics = %v, want the PATCH failure", resp.Diagnostics)
	}
	if resp.State.Raw.IsNull() {
		t.Fatal("state is empty: the attachment is not tracked")
	}
	var got BundleAttachmentModel
	if d := resp.State.Get(context.Background(), &got); d.HasError() {
		t.Fatal(d)
	}
	if got.ID.ValueString() != testStackID+":"+testBundleID || got.Priority.ValueInt64() != 0 {
		t.Errorf("state = %+v, want the attachment at priority 0", got)
	}
}
