// ABOUTME: Unit tests for the label set conversions.
// ABOUTME: Pins null-versus-empty handling in both directions.
package labelset

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFromAPI(t *testing.T) {
	t.Parallel()

	empty := types.SetValueMust(types.StringType, []attr.Value{})
	one := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("prod")})

	tests := []struct {
		name      string
		labels    []string
		prior     types.Set
		wantNull  bool
		wantCount int
	}{
		{name: "none, prior null", labels: nil, prior: types.SetNull(types.StringType), wantNull: true},
		{name: "[] from the API, prior null", labels: []string{}, prior: types.SetNull(types.StringType), wantNull: true},
		{name: "none, prior unknown", labels: nil, prior: types.SetUnknown(types.StringType), wantNull: true},
		{name: "none, prior empty set", labels: []string{}, prior: empty, wantCount: 0},
		{name: "none, prior had labels (cleared out of band)", labels: nil, prior: one, wantNull: true},
		{name: "labels, prior null (set out of band)", labels: []string{"prod", "eu"}, prior: types.SetNull(types.StringType), wantCount: 2},
		{name: "labels, prior empty", labels: []string{"prod"}, prior: empty, wantCount: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, diags := FromAPI(tt.labels, tt.prior)
			if diags.HasError() {
				t.Fatalf("diags: %v", diags)
			}
			if got.IsNull() != tt.wantNull {
				t.Fatalf("IsNull = %v, want %v (%v)", got.IsNull(), tt.wantNull, got)
			}
			if !tt.wantNull && len(got.Elements()) != tt.wantCount {
				t.Errorf("elements = %d, want %d", len(got.Elements()), tt.wantCount)
			}
		})
	}
}

func TestComputed_NeverNull(t *testing.T) {
	t.Parallel()

	got, diags := Computed(nil)
	if diags.HasError() || got.IsNull() || len(got.Elements()) != 0 {
		t.Errorf("Computed(nil) = %v (%v), want an empty set", got, diags)
	}
}

func TestToAPI_ClearIsAnEmptyArrayNeverNull(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	for name, set := range map[string]types.Set{
		"null":    types.SetNull(types.StringType),
		"unknown": types.SetUnknown(types.StringType),
		"empty":   types.SetValueMust(types.StringType, []attr.Value{}),
	} {
		got, diags := ToAPI(ctx, set)
		if diags.HasError() {
			t.Fatalf("%s: %v", name, diags)
		}
		raw, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != "[]" {
			t.Errorf("%s: marshals as %s, want [] (null would leave the labels unchanged)", name, raw)
		}
	}

	got, diags := ToAPI(ctx, types.SetValueMust(types.StringType, []attr.Value{types.StringValue("prod")}))
	if diags.HasError() || len(got) != 1 || got[0] != "prod" {
		t.Errorf("ToAPI = %v (%v), want [prod]", got, diags)
	}
}
