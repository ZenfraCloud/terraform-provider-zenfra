// ABOUTME: Plan-time validator requiring a configured set to hold at least one element.
// ABOUTME: Used by zenfra_webhook_endpoint.event_types, which the API refuses when empty.
package validate

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// NonEmptySet returns a set validator that refuses a known empty set. Null and
// unknown values pass: Required handles null, and an unknown set is checked
// again once it is known.
func NonEmptySet() validator.Set {
	return nonEmptySetValidator{}
}

type nonEmptySetValidator struct{}

func (nonEmptySetValidator) Description(_ context.Context) string {
	return "must contain at least one element"
}

func (v nonEmptySetValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (nonEmptySetValidator) ValidateSet(_ context.Context, req validator.SetRequest, resp *validator.SetResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if len(req.ConfigValue.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(req.Path, "Empty Set", "at least one element is required")
	}
}
