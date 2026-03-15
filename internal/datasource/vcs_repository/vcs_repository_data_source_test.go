// ABOUTME: Unit tests for the singular VCS repository data source model mapping.
// ABOUTME: Verifies correct conversion for GitHub and GitLab repositories.
package vcs_repository

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

func TestMapVCSRepositoryToDataSource(t *testing.T) {
	tests := []struct {
		name     string
		input    *zenfraclient.VCSRepository
		expected vcsRepositoryDataSourceModel
	}{
		{
			name: "github repository",
			input: &zenfraclient.VCSRepository{
				ID:            "repo-abc",
				IntegrationID: "vcs-123",
				Provider:      "github",
				ProviderRepo: zenfraclient.VCSProviderRepo{
					ID:            "12345",
					FullName:      "ndemeshchenko/tf-base",
					WebURL:        "https://github.com/ndemeshchenko/tf-base",
					DefaultBranch: "main",
					Visibility:    "private",
					Archived:      false,
				},
				Enabled:   true,
				CreatedAt: "2026-01-01T00:00:00Z",
				UpdatedAt: "2026-01-01T00:00:00Z",
			},
			expected: vcsRepositoryDataSourceModel{
				ID:            types.StringValue("repo-abc"),
				IntegrationID: types.StringValue("vcs-123"),
				ProviderType:      types.StringValue("github"),
				FullName:      types.StringValue("ndemeshchenko/tf-base"),
				WebURL:        types.StringValue("https://github.com/ndemeshchenko/tf-base"),
				DefaultBranch: types.StringValue("main"),
				Visibility:    types.StringValue("private"),
				Archived:      types.BoolValue(false),
				Enabled:       types.BoolValue(true),
				CreatedAt:     types.StringValue("2026-01-01T00:00:00Z"),
				UpdatedAt:     types.StringValue("2026-01-01T00:00:00Z"),
			},
		},
		{
			name: "gitlab repository",
			input: &zenfraclient.VCSRepository{
				ID:            "repo-def",
				IntegrationID: "vcs-456",
				Provider:      "gitlab",
				ProviderRepo: zenfraclient.VCSProviderRepo{
					ID:            "67890",
					FullName:      "group/infra-modules",
					WebURL:        "https://gitlab.example.com/group/infra-modules",
					DefaultBranch: "develop",
					Visibility:    "internal",
					Archived:      true,
				},
				Enabled:   false,
				CreatedAt: "2026-02-15T10:30:00Z",
				UpdatedAt: "2026-02-15T12:00:00Z",
			},
			expected: vcsRepositoryDataSourceModel{
				ID:            types.StringValue("repo-def"),
				IntegrationID: types.StringValue("vcs-456"),
				ProviderType:      types.StringValue("gitlab"),
				FullName:      types.StringValue("group/infra-modules"),
				WebURL:        types.StringValue("https://gitlab.example.com/group/infra-modules"),
				DefaultBranch: types.StringValue("develop"),
				Visibility:    types.StringValue("internal"),
				Archived:      types.BoolValue(true),
				Enabled:       types.BoolValue(false),
				CreatedAt:     types.StringValue("2026-02-15T10:30:00Z"),
				UpdatedAt:     types.StringValue("2026-02-15T12:00:00Z"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapVCSRepositoryToDataSource(tt.input)

			if !result.ID.Equal(tt.expected.ID) {
				t.Errorf("ID: got %v, want %v", result.ID, tt.expected.ID)
			}
			if !result.IntegrationID.Equal(tt.expected.IntegrationID) {
				t.Errorf("IntegrationID: got %v, want %v", result.IntegrationID, tt.expected.IntegrationID)
			}
			if !result.ProviderType.Equal(tt.expected.ProviderType) {
				t.Errorf("ProviderType: got %v, want %v", result.ProviderType, tt.expected.ProviderType)
			}
			if !result.FullName.Equal(tt.expected.FullName) {
				t.Errorf("FullName: got %v, want %v", result.FullName, tt.expected.FullName)
			}
			if !result.WebURL.Equal(tt.expected.WebURL) {
				t.Errorf("WebURL: got %v, want %v", result.WebURL, tt.expected.WebURL)
			}
			if !result.DefaultBranch.Equal(tt.expected.DefaultBranch) {
				t.Errorf("DefaultBranch: got %v, want %v", result.DefaultBranch, tt.expected.DefaultBranch)
			}
			if !result.Visibility.Equal(tt.expected.Visibility) {
				t.Errorf("Visibility: got %v, want %v", result.Visibility, tt.expected.Visibility)
			}
			if !result.Archived.Equal(tt.expected.Archived) {
				t.Errorf("Archived: got %v, want %v", result.Archived, tt.expected.Archived)
			}
			if !result.Enabled.Equal(tt.expected.Enabled) {
				t.Errorf("Enabled: got %v, want %v", result.Enabled, tt.expected.Enabled)
			}
			if !result.CreatedAt.Equal(tt.expected.CreatedAt) {
				t.Errorf("CreatedAt: got %v, want %v", result.CreatedAt, tt.expected.CreatedAt)
			}
			if !result.UpdatedAt.Equal(tt.expected.UpdatedAt) {
				t.Errorf("UpdatedAt: got %v, want %v", result.UpdatedAt, tt.expected.UpdatedAt)
			}
		})
	}
}
