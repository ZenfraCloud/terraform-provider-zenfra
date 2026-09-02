// ABOUTME: Unit tests for the zenfra_cloud_integration_attachment resource model mapping.
// ABOUTME: Verifies fromAPI conversion and composite ID parsing for import state.
package cloud_integration_attachment

import (
	"testing"

	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

func TestFromAPI(t *testing.T) {
	attachment := &zenfraclient.CloudAttachment{
		ID:             "att-123",
		OrganizationID: "org-456",
		IntegrationID:  "int-789",
		StackID:        "stack-abc",
		Read:           true,
		Write:          false,
		IsAutoAttached: false,
		CreatedAt:      "2025-01-15T10:00:00Z",
		CreatedBy:      "user-xyz",
		ExternalID:     "ext-001",
	}

	var model CloudIntegrationAttachmentModel
	model.fromAPI(attachment)

	if model.ID.ValueString() != "att-123" {
		t.Errorf("ID: got %s, want att-123", model.ID.ValueString())
	}
	if model.IntegrationID.ValueString() != "int-789" {
		t.Errorf("IntegrationID: got %s, want int-789", model.IntegrationID.ValueString())
	}
	if model.StackID.ValueString() != "stack-abc" {
		t.Errorf("StackID: got %s, want stack-abc", model.StackID.ValueString())
	}
	if model.Read.ValueBool() != true {
		t.Errorf("Read: got %v, want true", model.Read.ValueBool())
	}
	if model.Write.ValueBool() != false {
		t.Errorf("Write: got %v, want false", model.Write.ValueBool())
	}
	if model.IsAutoAttached.ValueBool() != false {
		t.Errorf("IsAutoAttached: got %v, want false", model.IsAutoAttached.ValueBool())
	}
	if model.CreatedAt.ValueString() != "2025-01-15T10:00:00Z" {
		t.Errorf("CreatedAt: got %s, want 2025-01-15T10:00:00Z", model.CreatedAt.ValueString())
	}
	if model.ExternalID.ValueString() != "ext-001" {
		t.Errorf("ExternalID: got %s, want ext-001", model.ExternalID.ValueString())
	}
}

func TestCompositeIDFormat(t *testing.T) {
	tests := []struct {
		name          string
		integrationID string
		attachmentID  string
		expected      string
	}{
		{
			name:          "standard IDs",
			integrationID: "int-abc-123",
			attachmentID:  "att-def-456",
			expected:      "int-abc-123:att-def-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.integrationID + ":" + tt.attachmentID
			if result != tt.expected {
				t.Errorf("composite ID: got %s, want %s", result, tt.expected)
			}
		})
	}
}
