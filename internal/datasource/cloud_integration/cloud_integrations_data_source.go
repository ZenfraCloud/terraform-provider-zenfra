// ABOUTME: Data source for listing all Zenfra cloud integrations in the organization.
// ABOUTME: Supports optional filtering by space_id and provider_type.
package cloud_integration

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

type cloudIntegrationsDataSource struct {
	client *zenfraclient.Client
}

var _ datasource.DataSource = &cloudIntegrationsDataSource{}
var _ datasource.DataSourceWithConfigure = &cloudIntegrationsDataSource{}

func NewCloudIntegrationsDataSource() datasource.DataSource {
	return &cloudIntegrationsDataSource{}
}

func (d *cloudIntegrationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_integrations"
}

func (d *cloudIntegrationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all Zenfra cloud integrations in the organization.",
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Optional filter by space ID.",
				Optional:            true,
			},
			"provider_type": schema.StringAttribute{
				MarkdownDescription: "Optional filter by cloud provider type (e.g., aws).",
				Optional:            true,
			},
			"integrations": schema.ListNestedAttribute{
				MarkdownDescription: "List of cloud integrations.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the cloud integration.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The display name of the cloud integration.",
							Computed:            true,
						},
						"space_id": schema.StringAttribute{
							MarkdownDescription: "The space ID this integration is scoped to.",
							Computed:            true,
						},
						"provider_type": schema.StringAttribute{
							MarkdownDescription: "The cloud provider type (e.g., aws).",
							Computed:            true,
						},
						"status": schema.StringAttribute{
							MarkdownDescription: "The current status of the integration.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *cloudIntegrationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *cloudIntegrationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudIntegrationsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build API options from space_id filter (server-side).
	var opts *zenfraclient.ListCloudIntegrationsOptions
	if !data.SpaceID.IsNull() && !data.SpaceID.IsUnknown() {
		spaceID := data.SpaceID.ValueString()
		opts = &zenfraclient.ListCloudIntegrationsOptions{
			SpaceID: &spaceID,
		}
	}

	integrations, err := d.client.ListCloudIntegrations(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list cloud integrations, got error: %s", err))
		return
	}

	// Client-side filter by provider_type.
	filterProvider := ""
	if !data.ProviderType.IsNull() && !data.ProviderType.IsUnknown() {
		filterProvider = data.ProviderType.ValueString()
	}

	data.Integrations = make([]cloudIntegrationListItemModel, 0, len(integrations))
	for i := range integrations {
		if filterProvider != "" && integrations[i].Provider != filterProvider {
			continue
		}
		data.Integrations = append(data.Integrations, cloudIntegrationListItemModel{
			ID:           types.StringValue(integrations[i].ID),
			Name:         types.StringValue(integrations[i].Name),
			SpaceID:      types.StringValue(integrations[i].SpaceID),
			ProviderType: types.StringValue(integrations[i].Provider),
			Status:       types.StringValue(integrations[i].Status),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
