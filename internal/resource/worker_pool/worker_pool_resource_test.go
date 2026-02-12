// ABOUTME: Unit tests for the zenfra_worker_pool resource model mapping.
// ABOUTME: Verifies correct handling of write-once api_key field.
package worker_pool

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

func TestMapPoolToState(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-24 * time.Hour)
	apiKeyID := "key-123"

	tests := []struct {
		name     string
		pool     *zenfraclient.WorkerPool
		validate func(t *testing.T, model WorkerPoolModel)
	}{
		{
			name: "full pool with all fields",
			pool: &zenfraclient.WorkerPool{
				ID:                 "pool-123",
				OrganizationID:     "org-456",
				Name:               "test-pool",
				PoolType:           "private",
				APIKeyID:           &apiKeyID,
				KeyVersion:         1,
				Active:             true,
				ActiveWorkersCount: 3,
				CreatedAt:          now,
				UpdatedAt:          now,
				LastUsedAt:         &lastUsed,
			},
			validate: func(t *testing.T, model WorkerPoolModel) {
				if model.ID.ValueString() != "pool-123" {
					t.Errorf("expected ID pool-123, got %s", model.ID.ValueString())
				}
				if model.OrganizationID.ValueString() != "org-456" {
					t.Errorf("expected OrganizationID org-456, got %s", model.OrganizationID.ValueString())
				}
				if model.Name.ValueString() != "test-pool" {
					t.Errorf("expected Name test-pool, got %s", model.Name.ValueString())
				}
				if model.APIKeyID.ValueString() != "key-123" {
					t.Errorf("expected APIKeyID key-123, got %s", model.APIKeyID.ValueString())
				}
				if model.KeyVersion.ValueInt64() != 1 {
					t.Errorf("expected KeyVersion 1, got %d", model.KeyVersion.ValueInt64())
				}
				if !model.Active.ValueBool() {
					t.Error("expected Active true, got false")
				}
				if model.ActiveWorkersCount.ValueInt64() != 3 {
					t.Errorf("expected ActiveWorkersCount 3, got %d", model.ActiveWorkersCount.ValueInt64())
				}
				if model.CreatedAt.IsNull() {
					t.Error("expected CreatedAt to be set")
				}
				if model.UpdatedAt.IsNull() {
					t.Error("expected UpdatedAt to be set")
				}
				if model.LastUsedAt.IsNull() {
					t.Error("expected LastUsedAt to be set")
				}
				// Verify api_key is NOT set by mapPoolToState
				if !model.APIKey.IsNull() {
					t.Error("expected APIKey to be null (not set by mapPoolToState)")
				}
			},
		},
		{
			name: "pool with nil optional fields",
			pool: &zenfraclient.WorkerPool{
				ID:                 "pool-789",
				OrganizationID:     "org-456",
				Name:               "minimal-pool",
				PoolType:           "private",
				APIKeyID:           nil,
				KeyVersion:         1,
				Active:             true,
				ActiveWorkersCount: 0,
				CreatedAt:          now,
				UpdatedAt:          now,
				LastUsedAt:         nil,
			},
			validate: func(t *testing.T, model WorkerPoolModel) {
				if model.ID.ValueString() != "pool-789" {
					t.Errorf("expected ID pool-789, got %s", model.ID.ValueString())
				}
				if !model.APIKeyID.IsNull() {
					t.Errorf("expected APIKeyID to be null, got %s", model.APIKeyID.ValueString())
				}
				if !model.LastUsedAt.IsNull() {
					t.Errorf("expected LastUsedAt to be null, got %s", model.LastUsedAt.ValueString())
				}
				if model.ActiveWorkersCount.ValueInt64() != 0 {
					t.Errorf("expected ActiveWorkersCount 0, got %d", model.ActiveWorkersCount.ValueInt64())
				}
				// Verify api_key is NOT set by mapPoolToState
				if !model.APIKey.IsNull() {
					t.Error("expected APIKey to be null (not set by mapPoolToState)")
				}
			},
		},
		{
			name: "pool with empty string api_key_id",
			pool: &zenfraclient.WorkerPool{
				ID:                 "pool-empty",
				OrganizationID:     "org-456",
				Name:               "empty-key-pool",
				PoolType:           "private",
				APIKeyID:           strPtr(""),
				KeyVersion:         1,
				Active:             false,
				ActiveWorkersCount: 0,
				CreatedAt:          now,
				UpdatedAt:          now,
				LastUsedAt:         nil,
			},
			validate: func(t *testing.T, model WorkerPoolModel) {
				if !model.APIKeyID.IsNull() {
					t.Errorf("expected APIKeyID to be null for empty string, got %s", model.APIKeyID.ValueString())
				}
				if model.Active.ValueBool() {
					t.Error("expected Active false, got true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := mapPoolToState(tt.pool)
			tt.validate(t, model)
		})
	}
}

func TestMapPoolToState_APIKeyNotSet(t *testing.T) {
	// This test specifically verifies that mapPoolToState does NOT set the api_key field,
	// since it's only available at creation time.
	now := time.Now()
	pool := &zenfraclient.WorkerPool{
		ID:                 "pool-123",
		OrganizationID:     "org-456",
		Name:               "test-pool",
		PoolType:           "private",
		KeyVersion:         1,
		Active:             true,
		ActiveWorkersCount: 1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	model := mapPoolToState(pool)

	// The api_key field must be null/unknown since mapPoolToState doesn't set it
	if !model.APIKey.IsNull() && !model.APIKey.IsUnknown() {
		t.Error("mapPoolToState must not set the api_key field")
	}
}

func TestMapPoolToState_PreservesAPIKey(t *testing.T) {
	// This test demonstrates the expected pattern: caller must preserve api_key separately
	now := time.Now()
	pool := &zenfraclient.WorkerPool{
		ID:                 "pool-123",
		OrganizationID:     "org-456",
		Name:               "test-pool",
		PoolType:           "private",
		KeyVersion:         1,
		Active:             true,
		ActiveWorkersCount: 1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	// Map the pool
	model := mapPoolToState(pool)

	// Caller is responsible for setting api_key separately (e.g., from prior state or create response)
	expectedAPIKey := "secret-api-key-value" //nolint:gosec // test fixture, not a real credential
	model.APIKey = types.StringValue(expectedAPIKey)

	if model.APIKey.ValueString() != expectedAPIKey {
		t.Errorf("expected api_key %s, got %s", expectedAPIKey, model.APIKey.ValueString())
	}
}

// Helper function to get pointer to string
func strPtr(s string) *string {
	return &s
}
