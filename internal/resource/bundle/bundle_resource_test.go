// ABOUTME: Unit tests for the zenfra_configuration_bundle resource model mapping.
// ABOUTME: Verifies correct conversion between API Bundle types and Terraform state.
package bundle

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

func TestMapBundleToState(t *testing.T) {
	createdAt := time.Date(2026, 2, 11, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 2, 11, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    *zenfraclient.Bundle
		expected BundleModel
	}{
		{
			name: "bundle with all fields",
			input: &zenfraclient.Bundle{
				ID:                  "bundle-123",
				OrganizationID:      "org-456",
				SpaceID:             "space-789",
				Name:                "Production Config",
				Slug:                "production-config",
				Description:         "Production configuration bundle",
				ContentVersion:      3,
				AttachedStacksCount: 2,
				CreatedAt:           createdAt,
				UpdatedAt:           updatedAt,
			},
			expected: BundleModel{
				ID:                  types.StringValue("bundle-123"),
				OrganizationID:      types.StringValue("org-456"),
				SpaceID:             types.StringValue("space-789"),
				Name:                types.StringValue("Production Config"),
				Slug:                types.StringValue("production-config"),
				Description:         types.StringValue("Production configuration bundle"),
				ContentVersion:      types.Int64Value(3),
				AttachedStacksCount: types.Int64Value(2),
				CreatedAt:           types.StringValue("2026-02-11T10:00:00Z"),
				UpdatedAt:           types.StringValue("2026-02-11T12:00:00Z"),
			},
		},
		{
			name: "bundle without optional fields",
			input: &zenfraclient.Bundle{
				ID:             "bundle-456",
				OrganizationID: "org-789",
				SpaceID:        "space-123",
				Name:           "Dev Config",
				CreatedAt:      createdAt,
				UpdatedAt:      updatedAt,
			},
			expected: BundleModel{
				ID:                  types.StringValue("bundle-456"),
				OrganizationID:      types.StringValue("org-789"),
				SpaceID:             types.StringValue("space-123"),
				Name:                types.StringValue("Dev Config"),
				Slug:                types.StringNull(),
				Description:         types.StringNull(),
				ContentVersion:      types.Int64Value(0),
				AttachedStacksCount: types.Int64Value(0),
				CreatedAt:           types.StringValue("2026-02-11T10:00:00Z"),
				UpdatedAt:           types.StringValue("2026-02-11T12:00:00Z"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapBundleToState(tt.input)

			if !result.ID.Equal(tt.expected.ID) {
				t.Errorf("ID: got %v, want %v", result.ID, tt.expected.ID)
			}
			if !result.OrganizationID.Equal(tt.expected.OrganizationID) {
				t.Errorf("OrganizationID: got %v, want %v", result.OrganizationID, tt.expected.OrganizationID)
			}
			if !result.Name.Equal(tt.expected.Name) {
				t.Errorf("Name: got %v, want %v", result.Name, tt.expected.Name)
			}
			if !result.Slug.Equal(tt.expected.Slug) {
				t.Errorf("Slug: got %v, want %v", result.Slug, tt.expected.Slug)
			}
			if !result.Description.Equal(tt.expected.Description) {
				t.Errorf("Description: got %v, want %v", result.Description, tt.expected.Description)
			}
			if !result.ContentVersion.Equal(tt.expected.ContentVersion) {
				t.Errorf("ContentVersion: got %v, want %v", result.ContentVersion, tt.expected.ContentVersion)
			}
		})
	}
}
