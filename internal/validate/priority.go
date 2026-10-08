// ABOUTME: Plan-time validator for non-negative integers such as an attachment priority.
// ABOUTME: Mirrors the API, which refuses a negative priority.
package validate

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// NonNegative returns an int64 validator rejecting values below zero.
func NonNegative() validator.Int64 {
	return nonNegativeValidator{}
}

type nonNegativeValidator struct{}

func (nonNegativeValidator) Description(_ context.Context) string {
	return "must be zero or greater"
}

func (v nonNegativeValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (nonNegativeValidator) ValidateInt64(_ context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if v := req.ConfigValue.ValueInt64(); v < 0 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Value", fmt.Sprintf("must be zero or greater, got %d", v))
	}
}
