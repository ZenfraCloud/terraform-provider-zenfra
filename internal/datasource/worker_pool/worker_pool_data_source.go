// ABOUTME: Data source for reading a single Zenfra worker pool by ID.
// ABOUTME: Returns all pool attributes except the sensitive api_key.

package worker_pool

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

type workerPoolDataSource struct {
	client *zenfraclient.Client
}

type workerPoolDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	OrganizationID     types.String `tfsdk:"organization_id"`
	PoolType           types.String `tfsdk:"pool_type"`
	APIKeyID           types.String `tfsdk:"api_key_id"`
	KeyVersion         types.Int64  `tfsdk:"key_version"`
	Active             types.Bool   `tfsdk:"active"`
	ActiveWorkersCount types.Int64  `tfsdk:"active_workers_count"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
	LastUsedAt         types.String `tfsdk:"last_used_at"`
}

var _ datasource.DataSource = &workerPoolDataSource{}
var _ datasource.DataSourceWithConfigure = &workerPoolDataSource{}

func NewWorkerPoolDataSource() datasource.DataSource {
	return &workerPoolDataSource{}
}

func (d *workerPoolDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_worker_pool"
}

func (d *workerPoolDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single Zenfra worker pool by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the worker pool.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the worker pool.",
				Computed:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID that owns this worker pool.",
				Computed:            true,
			},
			"pool_type": schema.StringAttribute{
				MarkdownDescription: "The type of worker pool (private or public).",
				Computed:            true,
			},
			"api_key_id": schema.StringAttribute{
				MarkdownDescription: "The API key ID associated with this pool.",
				Computed:            true,
			},
			"key_version": schema.Int64Attribute{
				MarkdownDescription: "The version of the API key.",
				Computed:            true,
			},
			"active": schema.BoolAttribute{
				MarkdownDescription: "Whether the worker pool is active.",
				Computed:            true,
			},
			"active_workers_count": schema.Int64Attribute{
				MarkdownDescription: "The number of currently active workers in this pool.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when the worker pool was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when the worker pool was last updated.",
				Computed:            true,
			},
			"last_used_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when the worker pool was last used.",
				Computed:            true,
			},
		},
	}
}

func (d *workerPoolDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*zenfraclient.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *zenfraclient.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *workerPoolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data workerPoolDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pool, err := d.client.GetWorkerPool(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read worker pool, got error: %s", err))
		return
	}

	data.Name = types.StringValue(pool.Name)
	data.OrganizationID = types.StringValue(pool.OrganizationID)
	data.PoolType = types.StringValue(pool.PoolType)
	if pool.APIKeyID != nil {
		data.APIKeyID = types.StringValue(*pool.APIKeyID)
	} else {
		data.APIKeyID = types.StringNull()
	}
	data.KeyVersion = types.Int64Value(int64(pool.KeyVersion))
	data.Active = types.BoolValue(pool.Active)
	data.ActiveWorkersCount = types.Int64Value(pool.ActiveWorkersCount)
	data.CreatedAt = types.StringValue(pool.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	data.UpdatedAt = types.StringValue(pool.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
	if pool.LastUsedAt != nil {
		data.LastUsedAt = types.StringValue(pool.LastUsedAt.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		data.LastUsedAt = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
