// ABOUTME: Data source for reading a single Zenfra cloud integration by ID or name.
// ABOUTME: Returns all integration attributes including provider-specific AWS fields.
package cloud_integration

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

type cloudIntegrationDataSource struct {
	client *zenfraclient.Client
}

var _ datasource.DataSource = &cloudIntegrationDataSource{}
var _ datasource.DataSourceWithConfigure = &cloudIntegrationDataSource{}

func NewCloudIntegrationDataSource() datasource.DataSource {
	return &cloudIntegrationDataSource{}
}

func (d *cloudIntegrationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_integration"
}

func (d *cloudIntegrationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single Zenfra cloud integration by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the cloud integration. Exactly one of `id` or `name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The display name of the cloud integration. Exactly one of `id` or `name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID this integration belongs to.",
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
			"aws": schema.SingleNestedAttribute{
				MarkdownDescription: "AWS-specific configuration. Only set for AWS integrations.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"role_arn": schema.StringAttribute{
						MarkdownDescription: "The IAM role ARN used for assuming credentials.",
						Computed:            true,
					},
					"session_duration": schema.Int64Attribute{
						MarkdownDescription: "The session duration in seconds for assumed role credentials.",
						Computed:            true,
					},
					"region": schema.StringAttribute{
						MarkdownDescription: "The default AWS region for this integration.",
						Computed:            true,
					},
					"generate_on_worker": schema.BoolAttribute{
						MarkdownDescription: "Whether credentials are generated on the worker rather than the control plane.",
						Computed:            true,
					},
				},
			},
			"auto_attach_label": schema.StringAttribute{
				MarkdownDescription: "Label used for automatic attachment to stacks.",
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
			"last_verified_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the integration was last verified.",
				Computed:            true,
			},
		},
	}
}

func (d *cloudIntegrationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *cloudIntegrationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data cloudIntegrationDataSourceModel
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
		ci, err := d.client.GetCloudIntegration(ctx, data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read cloud integration, got error: %s", err))
			return
		}

		state := mapCloudIntegrationToDataSource(ci)
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	// Lookup by name: list all and find match.
	integrations, err := d.client.ListCloudIntegrations(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list cloud integrations, got error: %s", err))
		return
	}

	targetName := data.Name.ValueString()
	var matched *zenfraclient.CloudIntegration
	for i := range integrations {
		if integrations[i].Name == targetName {
			if matched != nil {
				resp.Diagnostics.AddError("Multiple Matches",
					fmt.Sprintf("Found multiple cloud integrations with name %q. Use `id` instead.", targetName))
				return
			}
			matched = &integrations[i]
		}
	}

	if matched == nil {
		resp.Diagnostics.AddError("Not Found",
			fmt.Sprintf("No cloud integration found with name %q.", targetName))
		return
	}

	state := mapCloudIntegrationToDataSource(matched)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
