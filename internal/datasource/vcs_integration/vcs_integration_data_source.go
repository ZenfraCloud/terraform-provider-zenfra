// ABOUTME: Data source for reading a single Zenfra VCS integration by ID or name.
// ABOUTME: Returns all integration attributes including provider-specific fields.
package vcs_integration

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

type vcsIntegrationDataSource struct {
	client *zenfraclient.Client
}

var _ datasource.DataSource = &vcsIntegrationDataSource{}
var _ datasource.DataSourceWithConfigure = &vcsIntegrationDataSource{}

func NewVCSIntegrationDataSource() datasource.DataSource {
	return &vcsIntegrationDataSource{}
}

func (d *vcsIntegrationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vcs_integration"
}

func (d *vcsIntegrationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single Zenfra VCS integration by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the VCS integration. Exactly one of `id` or `name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The display name of the VCS integration. Exactly one of `id` or `name` must be specified.",
				Optional:            true,
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
			"installation_id": schema.Int64Attribute{
				MarkdownDescription: "GitHub App installation ID. Only set for GitHub integrations.",
				Computed:            true,
			},
			"api_url": schema.StringAttribute{
				MarkdownDescription: "API URL for GitLab integration. Only set for GitLab integrations.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the integration was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the integration was last updated.",
				Computed:            true,
			},
		},
	}
}

func (d *vcsIntegrationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *vcsIntegrationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data vcsIntegrationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasID := !data.ID.IsNull() && !data.ID.IsUnknown()
	hasName := !data.Name.IsNull() && !data.Name.IsUnknown()

	if !hasID && !hasName {
		resp.Diagnostics.AddError("Missing Attribute", "Exactly one of `id` or `name` must be specified.")
		return
	}
	if hasID && hasName {
		resp.Diagnostics.AddError("Conflicting Attributes", "Only one of `id` or `name` may be specified, not both.")
		return
	}

	if hasID {
		vcs, err := d.client.GetVCSIntegration(ctx, data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read VCS integration, got error: %s", err))
			return
		}

		state := mapVCSIntegrationToDataSource(vcs)
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	// Lookup by name: list all and find match.
	integrations, err := d.client.ListVCSIntegrations(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list VCS integrations, got error: %s", err))
		return
	}

	targetName := data.Name.ValueString()
	var matched *zenfraclient.VCSIntegration
	for i := range integrations {
		if integrations[i].DisplayName == targetName {
			if matched != nil {
				resp.Diagnostics.AddError("Multiple Matches",
					fmt.Sprintf("Found multiple VCS integrations with name %q. Use `id` instead.", targetName))
				return
			}
			matched = &integrations[i]
		}
	}

	if matched == nil {
		resp.Diagnostics.AddError("Not Found",
			fmt.Sprintf("No VCS integration found with name %q.", targetName))
		return
	}

	state := mapVCSIntegrationToDataSource(matched)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
