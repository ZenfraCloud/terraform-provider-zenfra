// ABOUTME: Data source for reading a single Zenfra space by ID.
// ABOUTME: Returns all space attributes including name, description, and hierarchy info.

package space

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

type spaceDataSource struct {
	client *zenfraclient.Client
}

type spaceDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Slug           types.String `tfsdk:"slug"`
	Description    types.String `tfsdk:"description"`
	OrganizationID types.String `tfsdk:"organization_id"`
	ParentID       types.String `tfsdk:"parent_id"`
	Depth          types.Int64  `tfsdk:"depth"`
	InheritBundles types.Bool   `tfsdk:"inherit_bundles"`
	ChildCount     types.Int64  `tfsdk:"child_count"`
	StackCount     types.Int64  `tfsdk:"stack_count"`
	CreatedBy      types.String `tfsdk:"created_by"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
	UpdatedBy      types.String `tfsdk:"updated_by"`
}

var _ datasource.DataSource = &spaceDataSource{}
var _ datasource.DataSourceWithConfigure = &spaceDataSource{}

func NewSpaceDataSource() datasource.DataSource {
	return &spaceDataSource{}
}

func (d *spaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_space"
}

func (d *spaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single Zenfra space by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the space.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the space.",
				Computed:            true,
			},
			"slug": schema.StringAttribute{
				MarkdownDescription: "The URL-friendly slug for the space.",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the space.",
				Computed:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID that owns this space.",
				Computed:            true,
			},
			"parent_id": schema.StringAttribute{
				MarkdownDescription: "The parent space ID if this is a nested space.",
				Computed:            true,
			},
			"depth": schema.Int64Attribute{
				MarkdownDescription: "The nesting depth of this space in the hierarchy.",
				Computed:            true,
			},
			"inherit_bundles": schema.BoolAttribute{
				MarkdownDescription: "Whether stacks in this space inherit bundles from parent spaces.",
				Computed:            true,
			},
			"child_count": schema.Int64Attribute{
				MarkdownDescription: "Number of child spaces.",
				Computed:            true,
			},
			"stack_count": schema.Int64Attribute{
				MarkdownDescription: "Number of stacks in this space.",
				Computed:            true,
			},
			"created_by": schema.StringAttribute{
				MarkdownDescription: "The user ID who created this space.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when the space was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when the space was last updated.",
				Computed:            true,
			},
			"updated_by": schema.StringAttribute{
				MarkdownDescription: "The user ID who last updated this space.",
				Computed:            true,
			},
		},
	}
}

func (d *spaceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *spaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data spaceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	space, err := d.client.GetSpace(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read space, got error: %s", err))
		return
	}

	data.Name = types.StringValue(space.Name)
	data.Slug = types.StringValue(space.Slug)
	data.Description = types.StringValue(space.Description)
	data.OrganizationID = types.StringValue(space.OrganizationID)
	if space.ParentID != nil {
		data.ParentID = types.StringValue(*space.ParentID)
	} else {
		data.ParentID = types.StringNull()
	}
	data.Depth = types.Int64Value(int64(space.Depth))
	data.InheritBundles = types.BoolValue(space.InheritBundles)
	data.ChildCount = types.Int64Value(int64(space.ChildCount))
	data.StackCount = types.Int64Value(int64(space.StackCount))
	data.CreatedBy = types.StringValue(space.CreatedBy)
	data.CreatedAt = types.StringValue(space.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	data.UpdatedAt = types.StringValue(space.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
	data.UpdatedBy = types.StringValue(space.UpdatedBy)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
