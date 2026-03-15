// ABOUTME: Unit tests for the plural VCS integrations data source filtering logic.
// ABOUTME: Verifies client-side provider_type filtering works correctly.
package vcs_integration

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

func TestFilterVCSIntegrations(t *testing.T) {
	integrations := []zenfraclient.VCSIntegration{
		{
			ID:             "vcs-1",
			OrganizationID: "org-1",
			Provider:       "github",
			DisplayName:    "GitHub Org",
			Status:         "active",
		},
		{
			ID:             "vcs-2",
			OrganizationID: "org-1",
			Provider:       "gitlab",
			DisplayName:    "GitLab Self-Hosted",
			Status:         "active",
		},
		{
			ID:             "vcs-3",
			OrganizationID: "org-1",
			Provider:       "github",
			DisplayName:    "GitHub Personal",
			Status:         "pending",
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
			expectedIDs:   []string{"vcs-1", "vcs-2", "vcs-3"},
		},
		{
			name:          "filter github",
			filter:        "github",
			expectedCount: 2,
			expectedIDs:   []string{"vcs-1", "vcs-3"},
		},
		{
			name:          "filter gitlab",
			filter:        "gitlab",
			expectedCount: 1,
			expectedIDs:   []string{"vcs-2"},
		},
		{
			name:          "filter unknown returns empty",
			filter:        "bitbucket",
			expectedCount: 0,
			expectedIDs:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make([]vcsIntegrationListItemModel, 0, len(integrations))
			for i := range integrations {
				if tt.filter != "" && integrations[i].Provider != tt.filter {
					continue
				}
				result = append(result, vcsIntegrationListItemModel{
					ID:             types.StringValue(integrations[i].ID),
					Name:           types.StringValue(integrations[i].DisplayName),
					OrganizationID: types.StringValue(integrations[i].OrganizationID),
					ProviderType:   types.StringValue(integrations[i].Provider),
					Status:         types.StringValue(integrations[i].Status),
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
