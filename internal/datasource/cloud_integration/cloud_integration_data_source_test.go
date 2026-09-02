// ABOUTME: Unit tests for the singular cloud integration data source model mapping.
// ABOUTME: Verifies correct conversion for AWS integrations and integrations without provider-specific config.
package cloud_integration

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

func TestMapCloudIntegrationToDataSource(t *testing.T) {
	verifiedAt := "2026-01-15T10:00:00Z"

	tests := []struct {
		name     string
		input    *zenfraclient.CloudIntegration
		expected cloudIntegrationDataSourceModel
	}{
		{
			name: "aws integration with full config",
			input: &zenfraclient.CloudIntegration{
				ID:             "ci-123",
				OrganizationID: "org-456",
				SpaceID:        "sp-789",
				Name:           "AWS Production",
				Provider:       "aws",
				Status:         "active",
				AWS: &zenfraclient.CloudAWSConfig{
					RoleARN:          "arn:aws:iam::123456789:role/zenfra-exec",
					SessionDuration:  3600,
					Region:           "us-east-1",
					GenerateOnWorker: false,
				},
				AutoAttachLabel: "cloud:aws-prod",
				CreatedAt:       "2026-01-01T00:00:00Z",
				UpdatedAt:       "2026-01-01T12:00:00Z",
				LastVerifiedAt:  &verifiedAt,
			},
			expected: cloudIntegrationDataSourceModel{
				ID:             types.StringValue("ci-123"),
				OrganizationID: types.StringValue("org-456"),
				SpaceID:        types.StringValue("sp-789"),
				Name:           types.StringValue("AWS Production"),
				ProviderType:   types.StringValue("aws"),
				Status:         types.StringValue("active"),
				AWS: &cloudAWSConfigModel{
					RoleARN:          types.StringValue("arn:aws:iam::123456789:role/zenfra-exec"),
					SessionDuration:  types.Int64Value(3600),
					Region:           types.StringValue("us-east-1"),
					GenerateOnWorker: types.BoolValue(false),
				},
				AutoAttachLabel: types.StringValue("cloud:aws-prod"),
				CreatedAt:       types.StringValue("2026-01-01T00:00:00Z"),
				UpdatedAt:       types.StringValue("2026-01-01T12:00:00Z"),
				LastVerifiedAt:  types.StringValue("2026-01-15T10:00:00Z"),
			},
		},
		{
			name: "aws integration with generate_on_worker",
			input: &zenfraclient.CloudIntegration{
				ID:             "ci-456",
				OrganizationID: "org-789",
				SpaceID:        "sp-123",
				Name:           "AWS Staging",
				Provider:       "aws",
				Status:         "active",
				AWS: &zenfraclient.CloudAWSConfig{
					RoleARN:          "arn:aws:iam::987654321:role/zenfra-staging",
					SessionDuration:  7200,
					Region:           "eu-west-1",
					GenerateOnWorker: true,
				},
				CreatedAt: "2026-02-01T00:00:00Z",
				UpdatedAt: "2026-02-01T00:00:00Z",
			},
			expected: cloudIntegrationDataSourceModel{
				ID:             types.StringValue("ci-456"),
				OrganizationID: types.StringValue("org-789"),
				SpaceID:        types.StringValue("sp-123"),
				Name:           types.StringValue("AWS Staging"),
				ProviderType:   types.StringValue("aws"),
				Status:         types.StringValue("active"),
				AWS: &cloudAWSConfigModel{
					RoleARN:          types.StringValue("arn:aws:iam::987654321:role/zenfra-staging"),
					SessionDuration:  types.Int64Value(7200),
					Region:           types.StringValue("eu-west-1"),
					GenerateOnWorker: types.BoolValue(true),
				},
				AutoAttachLabel: types.StringValue(""),
				CreatedAt:       types.StringValue("2026-02-01T00:00:00Z"),
				UpdatedAt:       types.StringValue("2026-02-01T00:00:00Z"),
				LastVerifiedAt:  types.StringNull(),
			},
		},
		{
			name: "integration without provider-specific config",
			input: &zenfraclient.CloudIntegration{
				ID:             "ci-789",
				OrganizationID: "org-123",
				SpaceID:        "sp-456",
				Name:           "Pending Integration",
				Provider:       "aws",
				Status:         "pending",
				CreatedAt:      "2026-03-01T00:00:00Z",
				UpdatedAt:      "2026-03-01T00:00:00Z",
			},
			expected: cloudIntegrationDataSourceModel{
				ID:              types.StringValue("ci-789"),
				OrganizationID:  types.StringValue("org-123"),
				SpaceID:         types.StringValue("sp-456"),
				Name:            types.StringValue("Pending Integration"),
				ProviderType:    types.StringValue("aws"),
				Status:          types.StringValue("pending"),
				AWS:             nil,
				AutoAttachLabel: types.StringValue(""),
				CreatedAt:       types.StringValue("2026-03-01T00:00:00Z"),
				UpdatedAt:       types.StringValue("2026-03-01T00:00:00Z"),
				LastVerifiedAt:  types.StringNull(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapCloudIntegrationToDataSource(tt.input)

			if !result.ID.Equal(tt.expected.ID) {
				t.Errorf("ID: got %v, want %v", result.ID, tt.expected.ID)
			}
			if !result.OrganizationID.Equal(tt.expected.OrganizationID) {
				t.Errorf("OrganizationID: got %v, want %v", result.OrganizationID, tt.expected.OrganizationID)
			}
			if !result.SpaceID.Equal(tt.expected.SpaceID) {
				t.Errorf("SpaceID: got %v, want %v", result.SpaceID, tt.expected.SpaceID)
			}
			if !result.Name.Equal(tt.expected.Name) {
				t.Errorf("Name: got %v, want %v", result.Name, tt.expected.Name)
			}
			if !result.ProviderType.Equal(tt.expected.ProviderType) {
				t.Errorf("ProviderType: got %v, want %v", result.ProviderType, tt.expected.ProviderType)
			}
			if !result.Status.Equal(tt.expected.Status) {
				t.Errorf("Status: got %v, want %v", result.Status, tt.expected.Status)
			}
			if !result.AutoAttachLabel.Equal(tt.expected.AutoAttachLabel) {
				t.Errorf("AutoAttachLabel: got %v, want %v", result.AutoAttachLabel, tt.expected.AutoAttachLabel)
			}
			if !result.CreatedAt.Equal(tt.expected.CreatedAt) {
				t.Errorf("CreatedAt: got %v, want %v", result.CreatedAt, tt.expected.CreatedAt)
			}
			if !result.UpdatedAt.Equal(tt.expected.UpdatedAt) {
				t.Errorf("UpdatedAt: got %v, want %v", result.UpdatedAt, tt.expected.UpdatedAt)
			}
			if !result.LastVerifiedAt.Equal(tt.expected.LastVerifiedAt) {
				t.Errorf("LastVerifiedAt: got %v, want %v", result.LastVerifiedAt, tt.expected.LastVerifiedAt)
			}

			// Check AWS config.
			if tt.expected.AWS == nil {
				if result.AWS != nil {
					t.Errorf("AWS: expected nil, got %v", result.AWS)
				}
			} else {
				if result.AWS == nil {
					t.Fatalf("AWS: expected non-nil, got nil")
				}
				if !result.AWS.RoleARN.Equal(tt.expected.AWS.RoleARN) {
					t.Errorf("AWS.RoleARN: got %v, want %v", result.AWS.RoleARN, tt.expected.AWS.RoleARN)
				}
				if !result.AWS.SessionDuration.Equal(tt.expected.AWS.SessionDuration) {
					t.Errorf("AWS.SessionDuration: got %v, want %v", result.AWS.SessionDuration, tt.expected.AWS.SessionDuration)
				}
				if !result.AWS.Region.Equal(tt.expected.AWS.Region) {
					t.Errorf("AWS.Region: got %v, want %v", result.AWS.Region, tt.expected.AWS.Region)
				}
				if !result.AWS.GenerateOnWorker.Equal(tt.expected.AWS.GenerateOnWorker) {
					t.Errorf("AWS.GenerateOnWorker: got %v, want %v", result.AWS.GenerateOnWorker, tt.expected.AWS.GenerateOnWorker)
				}
			}
		})
	}
}
