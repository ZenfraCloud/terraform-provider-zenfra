// ABOUTME: Terraform state model for the zenfra_stack_dependency resource and its API mapping.
// ABOUTME: The API never returns raw stack ids; the parent comes from the edge's stack, else from prior state.
package stack_dependency

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// StackDependencyModel is the Terraform state of one dependency edge.
type StackDependencyModel struct {
	ID            types.String `tfsdk:"id"`
	ParentStackID types.String `tfsdk:"parent_stack_id"`
	ChildStackID  types.String `tfsdk:"child_stack_id"`
	Enabled       types.Bool   `tfsdk:"enabled"`
	ActivatedAt   types.String `tfsdk:"activated_at"`
}

// toState converts an edge read from the child's side into state. The edge's
// stack is its parent; when the token cannot read the parent the API sends
// null, and priorParent (the value state or the plan already holds) is kept.
// An edge is never edited, so the kept value cannot be stale.
func toState(edge *zenfraclient.StackDependency, priorParent types.String, childID string) StackDependencyModel {
	parent := priorParent
	if edge.Stack != nil {
		parent = types.StringValue(edge.Stack.ID)
	}
	return StackDependencyModel{
		ID:            types.StringValue(edge.ID),
		ParentStackID: parent,
		ChildStackID:  types.StringValue(childID),
		Enabled:       types.BoolValue(edge.Enabled),
		ActivatedAt:   types.StringValue(edge.ActivatedAt),
	}
}
