// ABOUTME: Data source for listing all Zenfra worker pools in the organization.
// ABOUTME: Returns a list of worker pools with basic attributes.

package worker_pool

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

type workerPoolsDataSource struct {
	client *zenfraclient.Client
}

type workerPoolsDataSourceModel struct {
	Pools []workerPoolsListItemModel `tfsdk:"pools"`
}

type workerPoolsListItemModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Active         types.Bool   `tfsdk:"active"`
}

var _ datasource.DataSource = &workerPoolsDataSource{}
var _ datasource.DataSourceWithConfigure = &workerPoolsDataSource{}

func NewWorkerPoolsDataSource() datasource.DataSource {
	return &workerPoolsDataSource{}
}

func (d *workerPoolsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_worker_pools"
}

func (d *workerPoolsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all Zenfra worker pools in the organization.",
		Attributes: map[string]schema.Attribute{
			"pools": schema.ListNestedAttribute{
				MarkdownDescription: "List of worker pools.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the worker pool.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The name of the worker pool.",
							Computed:            true,
						},
						"organization_id": schema.StringAttribute{
							MarkdownDescription: "The organization ID that owns this worker pool.",
							Computed:            true,
						},
						"active": schema.BoolAttribute{
							MarkdownDescription: "Whether the worker pool is active.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *workerPoolsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *workerPoolsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data workerPoolsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pools, err := d.client.ListWorkerPools(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list worker pools, got error: %s", err))
		return
	}

	// Map results
	data.Pools = make([]workerPoolsListItemModel, 0, len(pools))
	for i := range pools {
		data.Pools = append(data.Pools, workerPoolsListItemModel{
			ID:             types.StringValue(pools[i].ID),
			Name:           types.StringValue(pools[i].Name),
			OrganizationID: types.StringValue(pools[i].OrganizationID),
			Active:         types.BoolValue(pools[i].Active),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
