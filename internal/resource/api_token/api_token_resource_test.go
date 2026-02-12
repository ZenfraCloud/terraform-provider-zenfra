// ABOUTME: Unit tests for the zenfra_api_token resource model mapping.
// ABOUTME: Verifies correct conversion and write-once token handling.
package api_token

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

func TestMapTokenToState(t *testing.T) {
	createdAt := time.Date(2026, 2, 11, 10, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    *zenfraclient.Token
		expected APITokenModel
	}{
		{
			name: "token with all fields",
			input: &zenfraclient.Token{
				ID:          "token-123",
				Name:        "CI/CD Token",
				Description: "Token for CI pipeline",
				Active:      true,
				CreatedAt:   createdAt,
				ExpiresAt:   expiresAt,
			},
			expected: APITokenModel{
				ID:          types.StringValue("token-123"),
				Name:        types.StringValue("CI/CD Token"),
				Description: types.StringValue("Token for CI pipeline"),
				Active:      types.BoolValue(true),
				CreatedAt:   types.StringValue("2026-02-11T10:00:00Z"),
				ExpiresAt:   types.StringValue("2026-05-11T10:00:00Z"),
			},
		},
		{
			name: "token without description",
			input: &zenfraclient.Token{
				ID:        "token-456",
				Name:      "Deploy Token",
				Active:    false,
				CreatedAt: createdAt,
				ExpiresAt: expiresAt,
			},
			expected: APITokenModel{
				ID:          types.StringValue("token-456"),
				Name:        types.StringValue("Deploy Token"),
				Description: types.StringNull(),
				Active:      types.BoolValue(false),
				CreatedAt:   types.StringValue("2026-02-11T10:00:00Z"),
				ExpiresAt:   types.StringValue("2026-05-11T10:00:00Z"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapTokenToState(tt.input)

			if !result.ID.Equal(tt.expected.ID) {
				t.Errorf("ID: got %v, want %v", result.ID, tt.expected.ID)
			}
			if !result.Name.Equal(tt.expected.Name) {
				t.Errorf("Name: got %v, want %v", result.Name, tt.expected.Name)
			}
			if !result.Description.Equal(tt.expected.Description) {
				t.Errorf("Description: got %v, want %v", result.Description, tt.expected.Description)
			}
			if !result.Active.Equal(tt.expected.Active) {
				t.Errorf("Active: got %v, want %v", result.Active, tt.expected.Active)
			}
			if !result.Token.IsNull() {
				t.Errorf("Token should be null from mapTokenToState, got %v", result.Token)
			}
		})
	}
}
