// ABOUTME: Plan-time validator for label sets: stack labels and bundle auto_attach_labels.
// ABOUTME: Mirrors the API's label contract so a bad label fails at plan, not at apply.
package validate

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// MaxLabels is the API's cap on a label list (ZenfraCloud/zenfra-cloud#736).
const MaxLabels = 20

// labelPattern is the API's label pattern: lowercase letters, digits, '.',
// '_' and '-', 1-63 characters, starting with a letter or digit.
var labelPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

// Labels returns a set validator enforcing the API's label contract. It is
// stricter than the API in one respect: the API trims surrounding whitespace,
// which would store a label different from the configured one and leave a
// permanent diff, so a label with surrounding whitespace is rejected here.
// Uppercase is rejected, never folded, exactly as the API does.
func Labels() validator.Set {
	return labelsValidator{}
}

type labelsValidator struct{}

func (labelsValidator) Description(_ context.Context) string {
	return fmt.Sprintf("at most %d labels, each 1-63 characters of a-z, 0-9, '.', '_' or '-', starting with a letter or digit", MaxLabels)
}

func (v labelsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (labelsValidator) ValidateSet(_ context.Context, req validator.SetRequest, resp *validator.SetResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	elements := req.ConfigValue.Elements()
	for _, el := range elements {
		s, ok := el.(types.String)
		if !ok || s.IsNull() || s.IsUnknown() {
			continue
		}
		if msg := LabelError(s.ValueString()); msg != "" {
			resp.Diagnostics.AddAttributeError(req.Path, "Invalid Label", msg)
		}
	}
	if len(elements) > MaxLabels {
		resp.Diagnostics.AddAttributeError(req.Path, "Too Many Labels",
			fmt.Sprintf("at most %d labels are allowed, got %d", MaxLabels, len(elements)))
	}
}

// LabelError returns why label breaks the API's label contract, or "" when
// it is valid.
func LabelError(label string) string {
	switch {
	case strings.TrimSpace(label) == "":
		return "a label must not be blank"
	case strings.TrimSpace(label) != label:
		return fmt.Sprintf("label %q has surrounding whitespace; remove it", label)
	case labelPattern.MatchString(label):
		return ""
	case labelPattern.MatchString(strings.ToLower(label)):
		return fmt.Sprintf("label %q must be lowercase (uppercase is rejected, not folded): use %q", label, strings.ToLower(label))
	default:
		return fmt.Sprintf("label %q must be 1-63 characters of a-z, 0-9, '.', '_' or '-', starting with a letter or digit", label)
	}
}
