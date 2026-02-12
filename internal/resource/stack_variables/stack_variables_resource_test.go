// ABOUTME: Unit tests for the zenfra_stack_variables resource model.
// ABOUTME: Verifies variable attribute types and model structure.
package stack_variables

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestVariableAttrTypes(t *testing.T) {
	attrTypes := variableAttrTypes()

	expected := []string{"key", "value", "secret"}
	for _, key := range expected {
		if _, ok := attrTypes[key]; !ok {
			t.Errorf("missing expected attribute type: %s", key)
		}
	}

	if attrTypes["key"] != types.StringType {
		t.Errorf("key should be StringType, got %v", attrTypes["key"])
	}
	if attrTypes["value"] != types.StringType {
		t.Errorf("value should be StringType, got %v", attrTypes["value"])
	}
	if attrTypes["secret"] != types.BoolType {
		t.Errorf("secret should be BoolType, got %v", attrTypes["secret"])
	}
}
