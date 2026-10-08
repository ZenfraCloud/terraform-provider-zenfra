// ABOUTME: Plan-time validators for hook command lists and the hooks object.
// ABOUTME: Mirror the API's hook caps and reject shapes the API would store differently.
package validate

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	// MaxHookCommandsPerPhase is the API's cap on one phase's command list.
	MaxHookCommandsPerPhase = 32
	// MaxHookCommandBytes is the API's cap on one command, in bytes.
	MaxHookCommandBytes = 4096
)

// HookCommands returns a list validator for one phase's commands: 1 to 32
// commands, each non-blank, at most 4096 bytes and free of NUL bytes. An
// empty list is rejected because the API drops empty phases, so [] would read
// back as absent and never converge; omit the phase instead.
func HookCommands() validator.List {
	return hookCommandsValidator{}
}

type hookCommandsValidator struct{}

func (hookCommandsValidator) Description(_ context.Context) string {
	return fmt.Sprintf("1 to %d commands, each non-blank and at most %d bytes", MaxHookCommandsPerPhase, MaxHookCommandBytes)
}

func (v hookCommandsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (hookCommandsValidator) ValidateList(_ context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	elements := req.ConfigValue.Elements()
	switch {
	case len(elements) == 0:
		resp.Diagnostics.AddAttributeError(req.Path, "Empty Hook Phase",
			"a phase must have at least one command; omit the phase to run nothing")
	case len(elements) > MaxHookCommandsPerPhase:
		resp.Diagnostics.AddAttributeError(req.Path, "Too Many Hook Commands",
			fmt.Sprintf("at most %d commands per phase, got %d", MaxHookCommandsPerPhase, len(elements)))
	}
	for i, el := range elements {
		s, ok := el.(types.String)
		if !ok || s.IsNull() || s.IsUnknown() {
			continue
		}
		if msg := HookCommandError(s.ValueString()); msg != "" {
			resp.Diagnostics.AddAttributeError(req.Path.AtListIndex(i), "Invalid Hook Command", msg)
		}
	}
}

// HookCommandError returns why cmd would be refused by the API, or "" when
// it is valid.
func HookCommandError(cmd string) string {
	switch {
	case strings.TrimSpace(cmd) == "":
		return "a command must not be blank"
	case len(cmd) > MaxHookCommandBytes:
		return fmt.Sprintf("a command must be at most %d bytes, got %d", MaxHookCommandBytes, len(cmd))
	case strings.ContainsRune(cmd, 0):
		return "a command must not contain a NUL byte"
	default:
		return ""
	}
}

// HooksNotEmpty returns an object validator requiring at least one phase in
// a configured hooks object. The API reads an object with no commands as "no
// hooks", so hooks = {} would read back as absent and never converge; omit
// the attribute instead.
func HooksNotEmpty() validator.Object {
	return hooksNotEmptyValidator{}
}

type hooksNotEmptyValidator struct{}

func (hooksNotEmptyValidator) Description(_ context.Context) string {
	return "at least one phase must be set"
}

func (v hooksNotEmptyValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (hooksNotEmptyValidator) ValidateObject(_ context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	for _, v := range req.ConfigValue.Attributes() {
		if !v.IsNull() {
			return
		}
	}
	resp.Diagnostics.AddAttributeError(req.Path, "Empty Hooks",
		"hooks must set at least one phase; omit hooks to run none")
}
