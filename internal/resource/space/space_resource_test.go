// ABOUTME: Unit tests for the zenfra_space resource model mapping.
// ABOUTME: Verifies correct conversion between API types and Terraform state.
package space

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

func TestMapAPISpaceToModel(t *testing.T) {
	createdAt := time.Date(2026, 2, 11, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 2, 11, 12, 0, 0, 0, time.UTC)
	parentID := "parent-space-123"

	tests := []struct {
		name     string
		input    *zenfraclient.Space
		expected SpaceModel
	}{
		{
			name: "space with all fields",
			input: &zenfraclient.Space{
				ID:             "space-123",
				OrganizationID: "org-456",
				Name:           "Production",
				Slug:           "production",
				Description:    "Production environment space",
				ParentID:       &parentID,
				Depth:          1,
				InheritBundles: true,
				ChildCount:     5,
				StackCount:     10,
				CreatedBy:      "user-789",
				CreatedAt:      createdAt,
				UpdatedAt:      updatedAt,
				UpdatedBy:      "user-789",
			},
			expected: SpaceModel{
				ID:             types.StringValue("space-123"),
				OrganizationID: types.StringValue("org-456"),
				Name:           types.StringValue("Production"),
				Description:    types.StringValue("Production environment space"),
				ParentSpaceID:  types.StringValue("parent-space-123"),
				CreatedAt:      types.StringValue("2026-02-11T10:00:00Z"),
				UpdatedAt:      types.StringValue("2026-02-11T12:00:00Z"),
			},
		},
		{
			name: "space without optional fields",
			input: &zenfraclient.Space{
				ID:             "space-456",
				OrganizationID: "org-789",
				Name:           "Development",
				Slug:           "development",
				Description:    "",
				ParentID:       nil,
				Depth:          0,
				InheritBundles: false,
				ChildCount:     0,
				StackCount:     3,
				CreatedBy:      "user-123",
				CreatedAt:      createdAt,
				UpdatedAt:      updatedAt,
				UpdatedBy:      "user-123",
			},
			expected: SpaceModel{
				ID:             types.StringValue("space-456"),
				OrganizationID: types.StringValue("org-789"),
				Name:           types.StringValue("Development"),
				Description:    types.StringNull(),
				ParentSpaceID:  types.StringNull(),
				CreatedAt:      types.StringValue("2026-02-11T10:00:00Z"),
				UpdatedAt:      types.StringValue("2026-02-11T12:00:00Z"),
			},
		},
		{
			name: "space with empty parent ID pointer",
			input: &zenfraclient.Space{
				ID:             "space-789",
				OrganizationID: "org-123",
				Name:           "Staging",
				Slug:           "staging",
				Description:    "Staging environment",
				ParentID:       stringPtr(""),
				Depth:          0,
				InheritBundles: true,
				ChildCount:     2,
				StackCount:     5,
				CreatedBy:      "user-456",
				CreatedAt:      createdAt,
				UpdatedAt:      updatedAt,
				UpdatedBy:      "user-456",
			},
			expected: SpaceModel{
				ID:             types.StringValue("space-789"),
				OrganizationID: types.StringValue("org-123"),
				Name:           types.StringValue("Staging"),
				Description:    types.StringValue("Staging environment"),
				ParentSpaceID:  types.StringNull(),
				CreatedAt:      types.StringValue("2026-02-11T10:00:00Z"),
				UpdatedAt:      types.StringValue("2026-02-11T12:00:00Z"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapAPISpaceToModel(tt.input)

			// Compare each field
			if !result.ID.Equal(tt.expected.ID) {
				t.Errorf("ID mismatch: got %v, want %v", result.ID, tt.expected.ID)
			}
			if !result.OrganizationID.Equal(tt.expected.OrganizationID) {
				t.Errorf("OrganizationID mismatch: got %v, want %v", result.OrganizationID, tt.expected.OrganizationID)
			}
			if !result.Name.Equal(tt.expected.Name) {
				t.Errorf("Name mismatch: got %v, want %v", result.Name, tt.expected.Name)
			}
			if !result.Description.Equal(tt.expected.Description) {
				t.Errorf("Description mismatch: got %v, want %v", result.Description, tt.expected.Description)
			}
			if !result.ParentSpaceID.Equal(tt.expected.ParentSpaceID) {
				t.Errorf("ParentSpaceID mismatch: got %v, want %v", result.ParentSpaceID, tt.expected.ParentSpaceID)
			}
			if !result.CreatedAt.Equal(tt.expected.CreatedAt) {
				t.Errorf("CreatedAt mismatch: got %v, want %v", result.CreatedAt, tt.expected.CreatedAt)
			}
			if !result.UpdatedAt.Equal(tt.expected.UpdatedAt) {
				t.Errorf("UpdatedAt mismatch: got %v, want %v", result.UpdatedAt, tt.expected.UpdatedAt)
			}
		})
	}
}

// stringPtr is a helper function to create a pointer to a string.
func stringPtr(s string) *string {
	return &s
}
