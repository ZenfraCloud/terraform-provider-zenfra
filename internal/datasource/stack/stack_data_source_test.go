// ABOUTME: Unit tests for the zenfra_stack and zenfra_stacks data source mappings.
// ABOUTME: Covers state ownership, which a control plane older than the feature omits.

package stack

import (
	"testing"

	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

func TestDataSource_StateManagement(t *testing.T) {
	// A control plane older than the feature omits the block; it reads as managed.
	if got := mapStackToDataSource(&zenfraclient.Stack{ID: "s1"}); got.StateManagement.ValueString() != "managed" {
		t.Errorf("nil block = %q, want managed", got.StateManagement.ValueString())
	}
	external := &zenfraclient.Stack{ID: "s1", StateManagement: &zenfraclient.StateManagement{Mode: "external"}}
	if got := mapStackToDataSource(external); got.StateManagement.ValueString() != "external" {
		t.Error("an external stack must read as external")
	}
}

func TestListItem_StateManagement(t *testing.T) {
	if got := mapStackToListItem(&zenfraclient.Stack{ID: "s1"}); got.StateManagement.ValueString() != "managed" {
		t.Errorf("nil block = %q, want managed", got.StateManagement.ValueString())
	}
	external := &zenfraclient.Stack{ID: "s1", StateManagement: &zenfraclient.StateManagement{Mode: "external"}}
	if got := mapStackToListItem(external); got.StateManagement.ValueString() != "external" {
		t.Error("an external stack must read as external in the list")
	}
}
