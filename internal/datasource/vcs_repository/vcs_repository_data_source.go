// ABOUTME: Data source for reading a single Zenfra VCS repository by ID or by integration_id + full_name.
// ABOUTME: Returns all repository attributes including provider-specific metadata.
package vcs_repository

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

type vcsRepositoryDataSource struct {
	client *zenfraclient.Client
}

var _ datasource.DataSource = &vcsRepositoryDataSource{}
var _ datasource.DataSourceWithConfigure = &vcsRepositoryDataSource{}

func NewVCSRepositoryDataSource() datasource.DataSource {
	return &vcsRepositoryDataSource{}
}

func (d *vcsRepositoryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vcs_repository"
}

func (d *vcsRepositoryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single Zenfra VCS repository by ID or by integration_id + full_name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the VCS repository. Either `id` or `integration_id` + `full_name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"integration_id": schema.StringAttribute{
				MarkdownDescription: "The VCS integration ID to search within. Required when looking up by `full_name`.",
				Optional:            true,
				Computed:            true,
			},
			"full_name": schema.StringAttribute{
				MarkdownDescription: "The full repository name (e.g. `owner/repo`). Used together with `integration_id` for name-based lookup.",
				Optional:            true,
				Computed:            true,
			},
			"provider_type": schema.StringAttribute{
				MarkdownDescription: "The VCS provider type (github or gitlab).",
				Computed:            true,
			},
			"web_url": schema.StringAttribute{
				MarkdownDescription: "The web URL of the repository.",
				Computed:            true,
			},
			"default_branch": schema.StringAttribute{
				MarkdownDescription: "The default branch of the repository.",
				Computed:            true,
			},
			"visibility": schema.StringAttribute{
				MarkdownDescription: "The visibility of the repository (e.g. public, private).",
				Computed:            true,
			},
			"archived": schema.BoolAttribute{
				MarkdownDescription: "Whether the repository is archived.",
				Computed:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the repository is enabled in Zenfra.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the repository was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the repository was last updated.",
				Computed:            true,
			},
		},
	}
}

func (d *vcsRepositoryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *vcsRepositoryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data vcsRepositoryDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasID := !data.ID.IsNull() && !data.ID.IsUnknown()
	hasIntegrationID := !data.IntegrationID.IsNull() && !data.IntegrationID.IsUnknown()
	hasFullName := !data.FullName.IsNull() && !data.FullName.IsUnknown()

	if !hasID && !(hasIntegrationID && hasFullName) {
		resp.Diagnostics.AddError("Missing Attributes",
			"Either `id` or both `integration_id` and `full_name` must be specified.")
		return
	}
	if hasID && (hasIntegrationID || hasFullName) {
		resp.Diagnostics.AddError("Conflicting Attributes",
			"Specify either `id` or `integration_id` + `full_name`, not both.")
		return
	}

	if hasID {
		repo, err := d.client.GetVCSRepository(ctx, data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error",
				fmt.Sprintf("Unable to read VCS repository, got error: %s", err))
			return
		}

		state := mapVCSRepositoryToDataSource(repo)
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	// Lookup by integration_id + full_name: list all repos for the integration and find match.
	repos, err := d.client.ListVCSRepositories(ctx, data.IntegrationID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to list VCS repositories, got error: %s", err))
		return
	}

	targetName := data.FullName.ValueString()
	var matched *zenfraclient.VCSRepository
	for i := range repos {
		if repos[i].ProviderRepo.FullName == targetName {
			if matched != nil {
				resp.Diagnostics.AddError("Multiple Matches",
					fmt.Sprintf("Found multiple VCS repositories with full_name %q. Use `id` instead.", targetName))
				return
			}
			matched = &repos[i]
		}
	}

	if matched == nil {
		resp.Diagnostics.AddError("Not Found",
			fmt.Sprintf("No VCS repository found with full_name %q.", targetName))
		return
	}

	state := mapVCSRepositoryToDataSource(matched)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
