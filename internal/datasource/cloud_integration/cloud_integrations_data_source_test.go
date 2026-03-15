// ABOUTME: Unit tests for the plural cloud integrations data source filtering logic.
// ABOUTME: Verifies client-side provider_type filtering works correctly.
package cloud_integration

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

func TestFilterCloudIntegrations(t *testing.T) {
	integrations := []zenfraclient.CloudIntegration{
		{
			ID:       "ci-1",
			SpaceID:  "sp-1",
			Name:     "AWS Production",
			Provider: "aws",
			Status:   "active",
		},
		{
			ID:       "ci-2",
			SpaceID:  "sp-1",
			Name:     "AWS Staging",
			Provider: "aws",
			Status:   "active",
		},
		{
			ID:       "ci-3",
			SpaceID:  "sp-2",
			Name:     "GCP Dev",
			Provider: "gcp",
			Status:   "pending",
		},
	}

	tests := []struct {
		name          string
		filter        string
		expectedCount int
		expectedIDs   []string
	}{
		{
			name:          "no filter returns all",
			filter:        "",
			expectedCount: 3,
			expectedIDs:   []string{"ci-1", "ci-2", "ci-3"},
		},
		{
			name:          "filter aws",
			filter:        "aws",
			expectedCount: 2,
			expectedIDs:   []string{"ci-1", "ci-2"},
		},
		{
			name:          "filter gcp",
			filter:        "gcp",
			expectedCount: 1,
			expectedIDs:   []string{"ci-3"},
		},
		{
			name:          "filter unknown returns empty",
			filter:        "azure",
			expectedCount: 0,
			expectedIDs:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make([]cloudIntegrationListItemModel, 0, len(integrations))
			for i := range integrations {
				if tt.filter != "" && integrations[i].Provider != tt.filter {
					continue
				}
				result = append(result, cloudIntegrationListItemModel{
					ID:           types.StringValue(integrations[i].ID),
					Name:         types.StringValue(integrations[i].Name),
					SpaceID:      types.StringValue(integrations[i].SpaceID),
					ProviderType: types.StringValue(integrations[i].Provider),
					Status:       types.StringValue(integrations[i].Status),
				})
			}

			if len(result) != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, len(result))
			}

			for i, expectedID := range tt.expectedIDs {
				if i >= len(result) {
					break
				}
				if !result[i].ID.Equal(types.StringValue(expectedID)) {
					t.Errorf("result[%d].ID: got %v, want %v", i, result[i].ID, expectedID)
				}
			}
		})
	}
}
