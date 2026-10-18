// ABOUTME: Plan-time validators for zenfra_team: the slug and name exactly as the API stores them (zenfra-cloud#953).
// ABOUTME: The API lowercases and trims; a value it would change is refused so state always matches config.
package validate

import (
	"context"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// teamSlugPattern is the API's slug rule (models.NormalizeTeamSlug).
var teamSlugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// TeamSlug refuses a slug the API would refuse or rewrite: lowercase letters,
// numbers and hyphens, starting and ending with a letter or number, at most
// 64 characters.
func TeamSlug() validator.String {
	return stringCheck{
		desc: "lowercase letters, numbers and hyphens, starting and ending with a letter or number, at most 64 characters",
		ok:   func(s string) bool { return len(s) <= 64 && teamSlugPattern.MatchString(s) },
	}
}

// TeamName refuses a name the API would refuse or rewrite: not blank, no
// leading or trailing whitespace, at most 128 characters.
func TeamName() validator.String {
	return stringCheck{
		desc: "not blank, no leading or trailing whitespace, at most 128 characters",
		ok:   func(s string) bool { return s != "" && strings.TrimSpace(s) == s && len(s) <= 128 },
	}
}

type stringCheck struct {
	desc string
	ok   func(string) bool
}

func (c stringCheck) Description(_ context.Context) string { return "must be " + c.desc }

func (c stringCheck) MarkdownDescription(ctx context.Context) string { return c.Description(ctx) }

func (c stringCheck) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if !c.ok(req.ConfigValue.ValueString()) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Value", "Must be "+c.desc+".")
	}
}
