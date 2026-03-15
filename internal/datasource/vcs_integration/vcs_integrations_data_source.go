// ABOUTME: Data source for listing all Zenfra VCS integrations in the organization.
// ABOUTME: Supports optional filtering by provider_type (github or gitlab).
package vcs_integration

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

type vcsIntegrationsDataSource struct {
	client *zenfraclient.Client
}

var _ datasource.DataSource = &vcsIntegrationsDataSource{}
var _ datasource.DataSourceWithConfigure = &vcsIntegrationsDataSource{}

func NewVCSIntegrationsDataSource() datasource.DataSource {
	return &vcsIntegrationsDataSource{}
}

func (d *vcsIntegrationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vcs_integrations"
}

func (d *vcsIntegrationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all Zenfra VCS integrations in the organization.",
		Attributes: map[string]schema.Attribute{
			"provider_type": schema.StringAttribute{
				MarkdownDescription: "Optional filter by VCS provider type (github or gitlab).",
				Optional:            true,
			},
			"integrations": schema.ListNestedAttribute{
				MarkdownDescription: "List of VCS integrations.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the VCS integration.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The display name of the VCS integration.",
							Computed:            true,
						},
						"organization_id": schema.StringAttribute{
							MarkdownDescription: "The organization ID this integration belongs to.",
							Computed:            true,
						},
						"provider_type": schema.StringAttribute{
							MarkdownDescription: "The VCS provider type (github or gitlab).",
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

func (d *vcsIntegrationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *vcsIntegrationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data vcsIntegrationsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	integrations, err := d.client.ListVCSIntegrations(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list VCS integrations, got error: %s", err))
		return
	}

	filterProvider := ""
	if !data.ProviderType.IsNull() && !data.ProviderType.IsUnknown() {
		filterProvider = data.ProviderType.ValueString()
	}

	data.Integrations = make([]vcsIntegrationListItemModel, 0, len(integrations))
	for i := range integrations {
		if filterProvider != "" && integrations[i].Provider != filterProvider {
			continue
		}
		data.Integrations = append(data.Integrations, vcsIntegrationListItemModel{
			ID:             types.StringValue(integrations[i].ID),
			Name:           types.StringValue(integrations[i].DisplayName),
			OrganizationID: types.StringValue(integrations[i].OrganizationID),
			ProviderType:   types.StringValue(integrations[i].Provider),
			Status:         types.StringValue(integrations[i].Status),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
