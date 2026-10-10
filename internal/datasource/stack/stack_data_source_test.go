// ABOUTME: Unit tests for the zenfra_stack and zenfra_stacks data sources: labels and state ownership.
// ABOUTME: Labels are never null; a control plane older than state ownership reads as managed.
package stack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

func TestStackDataSource_Labels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		wantCount int
	}{
		{name: "labels", body: `{"id":"s1","labels":["prod","eu"]}`, wantCount: 2},
		{name: "empty", body: `{"id":"s1","labels":[]}`, wantCount: 0},
		{name: "null from an older API", body: `{"id":"s1"}`, wantCount: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			client, err := zenfraclient.NewClient(zenfraclient.ClientConfig{Endpoint: server.URL, APIToken: "test", MaxRetries: 1})
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			d := &stackDataSource{client: client}
			var schemaResp datasource.SchemaResponse
			d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

			// Build the config through a State, which can set one attribute.
			raw := tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil)}
			if diags := raw.SetAttribute(ctx, path.Root("id"), types.StringValue("s1")); diags.HasError() {
				t.Fatal(diags)
			}
			config := tfsdk.Config{Schema: schemaResp.Schema, Raw: raw.Raw}
			resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
			d.Read(ctx, datasource.ReadRequest{Config: config}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("Read: %v", resp.Diagnostics)
			}
			var got stackDataSourceModel
			if diags := resp.State.Get(ctx, &got); diags.HasError() {
				t.Fatal(diags)
			}
			if got.Labels.IsNull() || len(got.Labels.Elements()) != tt.wantCount {
				t.Errorf("labels = %v, want %d labels and never null", got.Labels, tt.wantCount)
			}
		})
	}
}

func TestDataSource_StateManagement(t *testing.T) {
	// A control plane older than the feature omits the block; it reads as managed.
	if got, _ := mapStackToDataSource(&zenfraclient.Stack{ID: "s1"}); got.StateManagement.ValueString() != zenfraclient.StateModeManaged {
		t.Errorf("nil block = %q, want managed", got.StateManagement.ValueString())
	}
	external := &zenfraclient.Stack{ID: "s1", StateManagement: &zenfraclient.StateManagement{Mode: zenfraclient.StateModeExternal}}
	if got, _ := mapStackToDataSource(external); got.StateManagement.ValueString() != zenfraclient.StateModeExternal {
		t.Error("an external stack must read as external")
	}
}

func TestListItem_StateManagement(t *testing.T) {
	if got, _ := mapStackToListItem(&zenfraclient.Stack{ID: "s1"}); got.StateManagement.ValueString() != zenfraclient.StateModeManaged {
		t.Errorf("nil block = %q, want managed", got.StateManagement.ValueString())
	}
	external := &zenfraclient.Stack{ID: "s1", StateManagement: &zenfraclient.StateManagement{Mode: zenfraclient.StateModeExternal}}
	if got, _ := mapStackToListItem(external); got.StateManagement.ValueString() != zenfraclient.StateModeExternal {
		t.Error("an external stack must read as external in the list")
	}
}
