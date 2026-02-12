// ABOUTME: Unit tests for the zenfra_vcs_integration resource model mapping.
// ABOUTME: Verifies correct conversion for both GitHub and GitLab provider types.
package vcs_integration

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

func TestMapVCSIntegrationToState(t *testing.T) {
	tests := []struct {
		name     string
		input    *zenfraclient.VCSIntegration
		expected VCSIntegrationModel
	}{
		{
			name: "github integration",
			input: &zenfraclient.VCSIntegration{
				ID:             "vcs-123",
				OrganizationID: "org-456",
				Provider:       "github",
				DisplayName:    "GitHub Org",
				Status:         "active",
				GitHub:         &zenfraclient.VCSGitHubConfig{InstallationID: 12345},
				CreatedAt:      "2026-02-11T10:00:00Z",
				UpdatedAt:      "2026-02-11T12:00:00Z",
			},
			expected: VCSIntegrationModel{
				ID:             types.StringValue("vcs-123"),
				OrganizationID: types.StringValue("org-456"),
				Name:           types.StringValue("GitHub Org"),
				ProviderType:   types.StringValue("github"),
				InstallationID: types.Int64Value(12345),
				APIURL:         types.StringNull(),
				Status:         types.StringValue("active"),
				CreatedAt:      types.StringValue("2026-02-11T10:00:00Z"),
				UpdatedAt:      types.StringValue("2026-02-11T12:00:00Z"),
			},
		},
		{
			name: "gitlab integration",
			input: &zenfraclient.VCSIntegration{
				ID:             "vcs-456",
				OrganizationID: "org-789",
				Provider:       "gitlab",
				DisplayName:    "GitLab Self-Hosted",
				Status:         "active",
				GitLab:         &zenfraclient.VCSGitLabConfig{BaseURL: "https://gitlab.example.com"},
				CreatedAt:      "2026-02-11T10:00:00Z",
				UpdatedAt:      "2026-02-11T12:00:00Z",
			},
			expected: VCSIntegrationModel{
				ID:             types.StringValue("vcs-456"),
				OrganizationID: types.StringValue("org-789"),
				Name:           types.StringValue("GitLab Self-Hosted"),
				ProviderType:   types.StringValue("gitlab"),
				InstallationID: types.Int64Null(),
				APIURL:         types.StringValue("https://gitlab.example.com"),
				Status:         types.StringValue("active"),
				CreatedAt:      types.StringValue("2026-02-11T10:00:00Z"),
				UpdatedAt:      types.StringValue("2026-02-11T12:00:00Z"),
			},
		},
		{
			name: "integration without provider-specific config",
			input: &zenfraclient.VCSIntegration{
				ID:             "vcs-789",
				OrganizationID: "org-123",
				Provider:       "github",
				DisplayName:    "Pending Setup",
				Status:         "pending",
				CreatedAt:      "2026-02-11T10:00:00Z",
				UpdatedAt:      "2026-02-11T10:00:00Z",
			},
			expected: VCSIntegrationModel{
				ID:             types.StringValue("vcs-789"),
				OrganizationID: types.StringValue("org-123"),
				Name:           types.StringValue("Pending Setup"),
				ProviderType:   types.StringValue("github"),
				InstallationID: types.Int64Null(),
				APIURL:         types.StringNull(),
				Status:         types.StringValue("pending"),
				CreatedAt:      types.StringValue("2026-02-11T10:00:00Z"),
				UpdatedAt:      types.StringValue("2026-02-11T10:00:00Z"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapVCSIntegrationToState(tt.input)

			if !result.ID.Equal(tt.expected.ID) {
				t.Errorf("ID: got %v, want %v", result.ID, tt.expected.ID)
			}
			if !result.OrganizationID.Equal(tt.expected.OrganizationID) {
				t.Errorf("OrganizationID: got %v, want %v", result.OrganizationID, tt.expected.OrganizationID)
			}
			if !result.Name.Equal(tt.expected.Name) {
				t.Errorf("Name: got %v, want %v", result.Name, tt.expected.Name)
			}
			if !result.ProviderType.Equal(tt.expected.ProviderType) {
				t.Errorf("ProviderType: got %v, want %v", result.ProviderType, tt.expected.ProviderType)
			}
			if !result.InstallationID.Equal(tt.expected.InstallationID) {
				t.Errorf("InstallationID: got %v, want %v", result.InstallationID, tt.expected.InstallationID)
			}
			if !result.APIURL.Equal(tt.expected.APIURL) {
				t.Errorf("APIURL: got %v, want %v", result.APIURL, tt.expected.APIURL)
			}
			if !result.Status.Equal(tt.expected.Status) {
				t.Errorf("Status: got %v, want %v", result.Status, tt.expected.Status)
			}
			// Verify PAT is not set by mapVCSIntegrationToState
			if !result.PersonalAccessToken.IsNull() {
				t.Errorf("PersonalAccessToken should be null from mapping, got %v", result.PersonalAccessToken)
			}
		})
	}
}
