// ABOUTME: Terraform state model for the zenfra_stack_variables resource.
// ABOUTME: Handles replace-all semantics and secret value masking ("****") from API.
package stack_variables

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// StackVariablesModel represents the Terraform state for all variables on a stack.
type StackVariablesModel struct {
	StackID  types.String `tfsdk:"stack_id"`
	Variable types.Set    `tfsdk:"variable"`
}

// VariableModel represents a single variable block.
type VariableModel struct {
	Key    types.String `tfsdk:"key"`
	Value  types.String `tfsdk:"value"`
	Secret types.Bool   `tfsdk:"secret"`
}
