// ABOUTME: Terraform state models for the zenfra_configuration_bundle resource.
// ABOUTME: Maps between API Bundle types and Terraform schema types including env vars, mounted files and hooks.
package bundle

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// BundleModel represents the Terraform state model for a Zenfra configuration bundle.
type BundleModel struct {
	ID                  types.String `tfsdk:"id"`
	OrganizationID      types.String `tfsdk:"organization_id"`
	SpaceID             types.String `tfsdk:"space_id"`
	Name                types.String `tfsdk:"name"`
	Slug                types.String `tfsdk:"slug"`
	Description         types.String `tfsdk:"description"`
	Labels              types.List   `tfsdk:"labels"`
	AutoAttachLabels    types.Set    `tfsdk:"auto_attach_labels"`
	Hooks               types.Object `tfsdk:"hooks"`
	ContentVersion      types.Int64  `tfsdk:"content_version"`
	AttachedStacksCount types.Int64  `tfsdk:"attached_stacks_count"`
	EnvironmentVariable types.Set    `tfsdk:"environment_variable"`
	MountedFile         types.Set    `tfsdk:"mounted_file"`
	CreatedAt           types.String `tfsdk:"created_at"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
}

// EnvVariableModel represents an environment variable block in the bundle.
type EnvVariableModel struct {
	Key         types.String `tfsdk:"key"`
	Value       types.String `tfsdk:"value"`
	Secret      types.Bool   `tfsdk:"secret"`
	Description types.String `tfsdk:"description"`
}

// MountedFileModel represents a mounted file block in the bundle.
type MountedFileModel struct {
	Path        types.String `tfsdk:"path"`
	Content     types.String `tfsdk:"content"`
	Secret      types.Bool   `tfsdk:"secret"`
	Description types.String `tfsdk:"description"`
}

// HooksModel is the bundle's per-phase hook commands.
type HooksModel struct {
	BeforeInit  types.List `tfsdk:"before_init"`
	AfterInit   types.List `tfsdk:"after_init"`
	BeforePlan  types.List `tfsdk:"before_plan"`
	AfterPlan   types.List `tfsdk:"after_plan"`
	BeforeApply types.List `tfsdk:"before_apply"`
	AfterApply  types.List `tfsdk:"after_apply"`
}

// hookPhases are the hooks attribute's phase names, in run order.
var hookPhases = []string{"before_init", "after_init", "before_plan", "after_plan", "before_apply", "after_apply"}

// hooksAttrTypes returns the attribute types for the hooks object.
func hooksAttrTypes() map[string]attr.Type {
	attrTypes := make(map[string]attr.Type, len(hookPhases))
	for _, phase := range hookPhases {
		attrTypes[phase] = types.ListType{ElemType: types.StringType}
	}
	return attrTypes
}

// commandsFromAPI maps one phase: no commands is null, since the API drops
// empty phases and the schema rejects an empty list.
func commandsFromAPI(commands []string) types.List {
	if len(commands) == 0 {
		return types.ListNull(types.StringType)
	}
	elems := make([]attr.Value, 0, len(commands))
	for _, c := range commands {
		elems = append(elems, types.StringValue(c))
	}
	return types.ListValueMust(types.StringType, elems)
}

// hooksFromAPI maps the API's hooks to state: nil or without a command is a
// null object.
func hooksFromAPI(ctx context.Context, h *zenfraclient.Hooks) (types.Object, diag.Diagnostics) {
	if h == nil || len(h.BeforeInit)+len(h.AfterInit)+len(h.BeforePlan)+len(h.AfterPlan)+len(h.BeforeApply)+len(h.AfterApply) == 0 {
		return types.ObjectNull(hooksAttrTypes()), nil
	}
	return types.ObjectValueFrom(ctx, hooksAttrTypes(), HooksModel{
		BeforeInit:  commandsFromAPI(h.BeforeInit),
		AfterInit:   commandsFromAPI(h.AfterInit),
		BeforePlan:  commandsFromAPI(h.BeforePlan),
		AfterPlan:   commandsFromAPI(h.AfterPlan),
		BeforeApply: commandsFromAPI(h.BeforeApply),
		AfterApply:  commandsFromAPI(h.AfterApply),
	})
}

// hooksToAPI maps planned hooks to the content write. Null maps to an empty
// Hooks, which marshals as {} and clears the bundle's hooks: Terraform owns
// them, and an absent member would keep whatever the bundle had.
func hooksToAPI(ctx context.Context, obj types.Object) (*zenfraclient.Hooks, diag.Diagnostics) {
	hooks := &zenfraclient.Hooks{}
	if obj.IsNull() || obj.IsUnknown() {
		return hooks, nil
	}
	var m HooksModel
	diags := obj.As(ctx, &m, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}
	for _, phase := range []struct {
		list types.List
		into *[]string
	}{
		{m.BeforeInit, &hooks.BeforeInit}, {m.AfterInit, &hooks.AfterInit},
		{m.BeforePlan, &hooks.BeforePlan}, {m.AfterPlan, &hooks.AfterPlan},
		{m.BeforeApply, &hooks.BeforeApply}, {m.AfterApply, &hooks.AfterApply},
	} {
		if phase.list.IsNull() || phase.list.IsUnknown() {
			continue
		}
		diags.Append(phase.list.ElementsAs(ctx, phase.into, false)...)
	}
	return hooks, diags
}

// mapBundleToState converts an API Bundle response to a BundleModel for Terraform state.
func mapBundleToState(bundle *zenfraclient.Bundle) BundleModel {
	model := BundleModel{
		ID:                  types.StringValue(bundle.ID),
		OrganizationID:      types.StringValue(bundle.OrganizationID),
		SpaceID:             types.StringValue(bundle.SpaceID),
		Name:                types.StringValue(bundle.Name),
		ContentVersion:      types.Int64Value(bundle.ContentVersion),
		AttachedStacksCount: types.Int64Value(bundle.AttachedStacksCount),
		CreatedAt:           types.StringValue(bundle.CreatedAt.Format("2006-01-02T15:04:05Z07:00")),
		UpdatedAt:           types.StringValue(bundle.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")),
	}

	if bundle.Slug != "" {
		model.Slug = types.StringValue(bundle.Slug)
	} else {
		model.Slug = types.StringNull()
	}

	if bundle.Description != "" {
		model.Description = types.StringValue(bundle.Description)
	} else {
		model.Description = types.StringNull()
	}

	return model
}
