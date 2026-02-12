// ABOUTME: Terraform state model for the zenfra_worker_pool resource.
// ABOUTME: Includes write-once api_key that is only populated on creation.
package worker_pool

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// WorkerPoolModel represents the Terraform state model for a Zenfra worker pool.
type WorkerPoolModel struct {
	ID                 types.String `tfsdk:"id"`
	OrganizationID     types.String `tfsdk:"organization_id"`
	Name               types.String `tfsdk:"name"`
	APIKey             types.String `tfsdk:"api_key"`
	APIKeyID           types.String `tfsdk:"api_key_id"`
	KeyVersion         types.Int64  `tfsdk:"key_version"`
	Active             types.Bool   `tfsdk:"active"`
	ActiveWorkersCount types.Int64  `tfsdk:"active_workers_count"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
	LastUsedAt         types.String `tfsdk:"last_used_at"`
}

// mapPoolToState converts an API WorkerPool response to a WorkerPoolModel for Terraform state.
// Note: This does NOT set the api_key field - caller must handle that separately since
// it's only available at creation time.
func mapPoolToState(pool *zenfraclient.WorkerPool) WorkerPoolModel {
	model := WorkerPoolModel{
		ID:                 types.StringValue(pool.ID),
		OrganizationID:     types.StringValue(pool.OrganizationID),
		Name:               types.StringValue(pool.Name),
		KeyVersion:         types.Int64Value(int64(pool.KeyVersion)),
		Active:             types.BoolValue(pool.Active),
		ActiveWorkersCount: types.Int64Value(pool.ActiveWorkersCount),
		CreatedAt:          types.StringValue(pool.CreatedAt.Format("2006-01-02T15:04:05Z07:00")),
		UpdatedAt:          types.StringValue(pool.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")),
	}

	if pool.APIKeyID != nil && *pool.APIKeyID != "" {
		model.APIKeyID = types.StringValue(*pool.APIKeyID)
	} else {
		model.APIKeyID = types.StringNull()
	}

	if pool.LastUsedAt != nil {
		model.LastUsedAt = types.StringValue(pool.LastUsedAt.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		model.LastUsedAt = types.StringNull()
	}

	return model
}
