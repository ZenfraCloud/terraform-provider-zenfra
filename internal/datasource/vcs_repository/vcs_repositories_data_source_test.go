// ABOUTME: Unit tests for the plural VCS repositories data source list mapping.
// ABOUTME: Verifies correct conversion of API repository list to data source models.
package vcs_repository

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

func TestMapVCSRepositoriesToListItems(t *testing.T) {
	repos := []zenfraclient.VCSRepository{
		{
			ID:            "repo-1",
			IntegrationID: "vcs-123",
			Provider:      "github",
			ProviderRepo: zenfraclient.VCSProviderRepo{
				FullName:      "org/repo-alpha",
				DefaultBranch: "main",
				Visibility:    "private",
			},
			Enabled: true,
		},
		{
			ID:            "repo-2",
			IntegrationID: "vcs-123",
			Provider:      "github",
			ProviderRepo: zenfraclient.VCSProviderRepo{
				FullName:      "org/repo-beta",
				DefaultBranch: "develop",
				Visibility:    "public",
			},
			Enabled: false,
		},
		{
			ID:            "repo-3",
			IntegrationID: "vcs-123",
			Provider:      "github",
			ProviderRepo: zenfraclient.VCSProviderRepo{
				FullName:      "org/repo-gamma",
				DefaultBranch: "main",
				Visibility:    "private",
			},
			Enabled: true,
		},
	}

	tests := []struct {
		name          string
		input         []zenfraclient.VCSRepository
		expectedCount int
		expectedIDs   []string
		expectedNames []string
	}{
		{
			name:          "maps all repositories",
			input:         repos,
			expectedCount: 3,
			expectedIDs:   []string{"repo-1", "repo-2", "repo-3"},
			expectedNames: []string{"org/repo-alpha", "org/repo-beta", "org/repo-gamma"},
		},
		{
			name:          "empty list",
			input:         []zenfraclient.VCSRepository{},
			expectedCount: 0,
			expectedIDs:   []string{},
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make([]vcsRepositoryListItemModel, 0, len(tt.input))
			for i := range tt.input {
				result = append(result, vcsRepositoryListItemModel{
					ID:            types.StringValue(tt.input[i].ID),
					IntegrationID: types.StringValue(tt.input[i].IntegrationID),
					Provider:      types.StringValue(tt.input[i].Provider),
					FullName:      types.StringValue(tt.input[i].ProviderRepo.FullName),
					DefaultBranch: types.StringValue(tt.input[i].ProviderRepo.DefaultBranch),
					Visibility:    types.StringValue(tt.input[i].ProviderRepo.Visibility),
					Enabled:       types.BoolValue(tt.input[i].Enabled),
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

			for i, expectedName := range tt.expectedNames {
				if i >= len(result) {
					break
				}
				if !result[i].FullName.Equal(types.StringValue(expectedName)) {
					t.Errorf("result[%d].FullName: got %v, want %v", i, result[i].FullName, expectedName)
				}
			}
		})
	}
}
