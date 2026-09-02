// ABOUTME: Data source for listing all Zenfra VCS repositories for a given integration.
// ABOUTME: Requires integration_id and returns a flat list of repository summaries.
package vcs_repository

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

type vcsRepositoriesDataSource struct {
	client *zenfraclient.Client
}

var _ datasource.DataSource = &vcsRepositoriesDataSource{}
var _ datasource.DataSourceWithConfigure = &vcsRepositoriesDataSource{}

func NewVCSRepositoriesDataSource() datasource.DataSource {
	return &vcsRepositoriesDataSource{}
}

func (d *vcsRepositoriesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vcs_repositories"
}

func (d *vcsRepositoriesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all Zenfra VCS repositories for a given integration.",
		Attributes: map[string]schema.Attribute{
			"integration_id": schema.StringAttribute{
				MarkdownDescription: "The VCS integration ID to list repositories for.",
				Required:            true,
			},
			"repositories": schema.ListNestedAttribute{
				MarkdownDescription: "List of VCS repositories.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the VCS repository.",
							Computed:            true,
						},
						"integration_id": schema.StringAttribute{
							MarkdownDescription: "The VCS integration ID this repository belongs to.",
							Computed:            true,
						},
						"provider": schema.StringAttribute{
							MarkdownDescription: "The VCS provider type (github or gitlab).",
							Computed:            true,
						},
						"full_name": schema.StringAttribute{
							MarkdownDescription: "The full repository name (e.g. owner/repo).",
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
						"enabled": schema.BoolAttribute{
							MarkdownDescription: "Whether the repository is enabled in Zenfra.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *vcsRepositoriesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *vcsRepositoriesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data vcsRepositoriesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	repos, err := d.client.ListVCSRepositories(ctx, data.IntegrationID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to list VCS repositories, got error: %s", err))
		return
	}

	data.Repositories = make([]vcsRepositoryListItemModel, 0, len(repos))
	for i := range repos {
		data.Repositories = append(data.Repositories, vcsRepositoryListItemModel{
			ID:            types.StringValue(repos[i].ID),
			IntegrationID: types.StringValue(repos[i].IntegrationID),
			Provider:      types.StringValue(repos[i].Provider),
			FullName:      types.StringValue(repos[i].ProviderRepo.FullName),
			DefaultBranch: types.StringValue(repos[i].ProviderRepo.DefaultBranch),
			Visibility:    types.StringValue(repos[i].ProviderRepo.Visibility),
			Enabled:       types.BoolValue(repos[i].Enabled),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
