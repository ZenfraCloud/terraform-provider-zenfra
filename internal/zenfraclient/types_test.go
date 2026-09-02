// ABOUTME: Unit tests for the client DTOs that need behaviour of their own.
// ABOUTME: Covers the stack state-ownership block and its effective-mode helper.

package zenfraclient_test

import (
	"encoding/json"
	"testing"

	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

func TestEffectiveStateMode(t *testing.T) {
	if got := zenfraclient.EffectiveStateMode(nil); got != "managed" {
		t.Errorf("nil = %q, want managed (a server older than the feature omits the field)", got)
	}
	for _, mode := range []string{"managed", "external"} {
		got := zenfraclient.EffectiveStateMode(&zenfraclient.StateManagement{Mode: mode})
		if got != mode {
			t.Errorf("%q = %q, want %q", mode, got, mode)
		}
	}
}

func TestStackJSON_StateManagementPresence(t *testing.T) {
	var absent zenfraclient.Stack
	if err := json.Unmarshal([]byte(`{"id":"s1"}`), &absent); err != nil {
		t.Fatal(err)
	}
	if absent.StateManagement != nil {
		t.Error("an absent field must stay nil so Create can detect an old control plane")
	}

	var present zenfraclient.Stack
	if err := json.Unmarshal([]byte(`{"id":"s1","state_management":{"mode":"external"}}`), &present); err != nil {
		t.Fatal(err)
	}
	if present.StateManagement == nil || present.StateManagement.Mode != "external" {
		t.Errorf("present = %+v, want mode external", present.StateManagement)
	}
}
