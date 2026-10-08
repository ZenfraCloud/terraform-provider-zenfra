// ABOUTME: Converts label lists between the API's []string and Terraform set values.
// ABOUTME: Keeps an unset label attribute null and a configured empty set empty, so neither diffs forever.
package labelset

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// FromAPI converts labels read from the API into a state value for an
// Optional set attribute. The API answers [] for "no labels"; that maps to
// null unless prior (the plan, or the state being refreshed) is a known empty
// set, in which case it stays empty. Either way an unset attribute and an
// explicit `= []` both read back as written.
func FromAPI(labels []string, prior types.Set) (types.Set, diag.Diagnostics) {
	if len(labels) == 0 {
		if !prior.IsNull() && !prior.IsUnknown() && len(prior.Elements()) == 0 {
			return types.SetValueMust(types.StringType, []attr.Value{}), nil
		}
		return types.SetNull(types.StringType), nil
	}
	return Computed(labels)
}

// Computed converts labels read from the API into a set for a Computed-only
// attribute, such as a data source's: never null, empty when there are none.
func Computed(labels []string) (types.Set, diag.Diagnostics) {
	elems := make([]attr.Value, 0, len(labels))
	for _, l := range labels {
		elems = append(elems, types.StringValue(l))
	}
	return types.SetValue(types.StringType, elems)
}

// ToAPI converts a planned set into the request list. A null or empty set
// yields a non-nil empty slice, which marshals as [] (the API's "clear"),
// never as null (the API's "unchanged").
func ToAPI(ctx context.Context, set types.Set) ([]string, diag.Diagnostics) {
	labels := []string{}
	if set.IsNull() || set.IsUnknown() {
		return labels, nil
	}
	diags := set.ElementsAs(ctx, &labels, false)
	if labels == nil {
		labels = []string{}
	}
	return labels, diags
}
