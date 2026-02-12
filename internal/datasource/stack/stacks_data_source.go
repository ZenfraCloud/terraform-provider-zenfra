// ABOUTME: Data source for listing Zenfra stacks with optional space_id filter.
// ABOUTME: Returns a list of stacks matching the filter criteria.

package stack

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

type stacksDataSource struct {
	client *zenfraclient.Client
}

type stacksDataSourceModel struct {
	SpaceID types.String              `tfsdk:"space_id"`
	Stacks  []stacksListItemModel     `tfsdk:"stacks"`
}

type stacksListItemModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	SpaceID        types.String `tfsdk:"space_id"`
	OrganizationID types.String `tfsdk:"organization_id"`
}

var _ datasource.DataSource = &stacksDataSource{}
var _ datasource.DataSourceWithConfigure = &stacksDataSource{}

func NewStacksDataSource() datasource.DataSource {
	return &stacksDataSource{}
}

func (d *stacksDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stacks"
}

func (d *stacksDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists Zenfra stacks with optional filtering.",
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Optional space ID filter to list stacks in a specific space.",
				Optional:            true,
			},
			"stacks": schema.ListNestedAttribute{
				MarkdownDescription: "List of stacks matching the filter criteria.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the stack.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The name of the stack.",
							Computed:            true,
						},
						"space_id": schema.StringAttribute{
							MarkdownDescription: "The space ID containing this stack.",
							Computed:            true,
						},
						"organization_id": schema.StringAttribute{
							MarkdownDescription: "The organization ID that owns this stack.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *stacksDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *stacksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data stacksDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build options
	opts := &zenfraclient.ListStacksOptions{}
	if !data.SpaceID.IsNull() {
		spaceID := data.SpaceID.ValueString()
		opts.SpaceID = &spaceID
	}

	stacks, err := d.client.ListStacks(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list stacks, got error: %s", err))
		return
	}

	// Map results
	data.Stacks = make([]stacksListItemModel, 0, len(stacks))
	for i := range stacks {
		data.Stacks = append(data.Stacks, stacksListItemModel{
			ID:             types.StringValue(stacks[i].ID),
			Name:           types.StringValue(stacks[i].Name),
			SpaceID:        types.StringValue(stacks[i].SpaceID),
			OrganizationID: types.StringValue(stacks[i].OrganizationID),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
