// ABOUTME: Unit tests for the client DTOs that need behaviour of their own.
// ABOUTME: Covers the stack state-ownership block and the VCS integration space_id patch and response.

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

func TestUpdateVCSIntegrationRequest_SpaceIDPatch(t *testing.T) {
	name := "Renamed"
	tests := []struct {
		name string
		req  zenfraclient.UpdateVCSIntegrationRequest
		want string
	}{
		{"unchanged omits the key", zenfraclient.UpdateVCSIntegrationRequest{DisplayName: &name}, `{"display_name":"Renamed"}`},
		{"set sends the id", zenfraclient.UpdateVCSIntegrationRequest{SpaceID: zenfraclient.StringPatch("6630f0c2a1b2c3d4e5f60718")}, `{"space_id":"6630f0c2a1b2c3d4e5f60718"}`},
		{"removed sends null", zenfraclient.UpdateVCSIntegrationRequest{SpaceID: zenfraclient.NullPatch()}, `{"space_id":null}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestCreateVCSIntegrationRequest_SpaceID(t *testing.T) {
	id := "6630f0c2a1b2c3d4e5f60718"
	for req, want := range map[*zenfraclient.CreateVCSIntegrationRequest]string{
		{Provider: "github"}:               `{"provider":"github"}`,
		{Provider: "github", SpaceID: &id}: `{"provider":"github","space_id":"6630f0c2a1b2c3d4e5f60718"}`,
	} {
		got, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("got %s, want %s", got, want)
		}
	}
}

func TestVCSIntegrationJSON_SpaceID(t *testing.T) {
	var orgWide zenfraclient.VCSIntegration
	if err := json.Unmarshal([]byte(`{"id":"v1","space_id":null}`), &orgWide); err != nil {
		t.Fatal(err)
	}
	if orgWide.SpaceID != nil {
		t.Errorf("null space_id = %q, want nil (organization-wide)", *orgWide.SpaceID)
	}

	var scoped zenfraclient.VCSIntegration
	if err := json.Unmarshal([]byte(`{"id":"v1","space_id":"6630f0c2a1b2c3d4e5f60718"}`), &scoped); err != nil {
		t.Fatal(err)
	}
	if scoped.SpaceID == nil || *scoped.SpaceID != "6630f0c2a1b2c3d4e5f60718" {
		t.Errorf("space_id = %v, want the hex id", scoped.SpaceID)
	}
}
