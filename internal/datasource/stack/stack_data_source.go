// ABOUTME: Data source for reading a single Zenfra stack by ID.
// ABOUTME: Returns all stack attributes including nested source, IAC, and trigger configuration.

package stack

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

type stackDataSource struct {
	client *zenfraclient.Client
}

type stackDataSourceModel struct {
	ID              types.String        `tfsdk:"id"`
	Name            types.String        `tfsdk:"name"`
	SpaceID         types.String        `tfsdk:"space_id"`
	OrganizationID  types.String        `tfsdk:"organization_id"`
	WorkerPoolID    types.String        `tfsdk:"worker_pool_id"`
	AllowPublicPool types.Bool          `tfsdk:"allow_public_pool"`
	IAC             *iacConfigModel     `tfsdk:"iac"`
	Source          *stackSourceModel   `tfsdk:"source"`
	Triggers        *stackTriggersModel `tfsdk:"triggers"`
	CreatedBy       types.String        `tfsdk:"created_by"`
	CreatedAt       types.String        `tfsdk:"created_at"`
	UpdatedAt       types.String        `tfsdk:"updated_at"`
	UpdatedBy       types.String        `tfsdk:"updated_by"`
}

type iacConfigModel struct {
	Engine  types.String `tfsdk:"engine"`
	Version types.String `tfsdk:"version"`
}

type stackSourceModel struct {
	Type   types.String            `tfsdk:"type"`
	RawGit *stackSourceRawGitModel `tfsdk:"raw_git"`
	VCS    *stackSourceVCSModel    `tfsdk:"vcs"`
}

type stackSourceRawGitModel struct {
	URL     types.String `tfsdk:"url"`
	RefType types.String `tfsdk:"ref_type"`
	RefName types.String `tfsdk:"ref_name"`
	Path    types.String `tfsdk:"path"`
}

type stackSourceVCSModel struct {
	Provider      types.String `tfsdk:"provider"`
	IntegrationID types.String `tfsdk:"integration_id"`
	RepositoryID  types.String `tfsdk:"repository_id"`
	RefType       types.String `tfsdk:"ref_type"`
	RefName       types.String `tfsdk:"ref_name"`
	Path          types.String `tfsdk:"path"`
}

type stackTriggersModel struct {
	OnPushEnabled types.Bool `tfsdk:"on_push_enabled"`
}

var _ datasource.DataSource = &stackDataSource{}
var _ datasource.DataSourceWithConfigure = &stackDataSource{}

func NewStackDataSource() datasource.DataSource {
	return &stackDataSource{}
}

func (d *stackDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stack"
}

func (d *stackDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single Zenfra stack by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the stack.",
				Required:            true,
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
			"worker_pool_id": schema.StringAttribute{
				MarkdownDescription: "The worker pool ID to use for execution.",
				Computed:            true,
			},
			"allow_public_pool": schema.BoolAttribute{
				MarkdownDescription: "Whether to allow execution on public worker pools.",
				Computed:            true,
			},
			"iac": schema.SingleNestedAttribute{
				MarkdownDescription: "Infrastructure as Code engine configuration.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"engine": schema.StringAttribute{
						MarkdownDescription: "The IaC engine (terraform or opentofu).",
						Computed:            true,
					},
					"version": schema.StringAttribute{
						MarkdownDescription: "The IaC engine version.",
						Computed:            true,
					},
				},
			},
			"source": schema.SingleNestedAttribute{
				MarkdownDescription: "Source code configuration for the stack.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						MarkdownDescription: "Source type (raw_git or vcs).",
						Computed:            true,
					},
					"raw_git": schema.SingleNestedAttribute{
						MarkdownDescription: "Raw Git source configuration (only if type is raw_git).",
						Computed:            true,
						Attributes: map[string]schema.Attribute{
							"url": schema.StringAttribute{
								MarkdownDescription: "The HTTPS Git URL.",
								Computed:            true,
							},
							"ref_type": schema.StringAttribute{
								MarkdownDescription: "The Git ref type (branch, tag, commit).",
								Computed:            true,
							},
							"ref_name": schema.StringAttribute{
								MarkdownDescription: "The Git ref name (branch name, tag name, or commit SHA).",
								Computed:            true,
							},
							"path": schema.StringAttribute{
								MarkdownDescription: "The path within the repository.",
								Computed:            true,
							},
						},
					},
					"vcs": schema.SingleNestedAttribute{
						MarkdownDescription: "VCS integration source configuration (only if type is vcs).",
						Computed:            true,
						Attributes: map[string]schema.Attribute{
							"provider": schema.StringAttribute{
								MarkdownDescription: "The VCS provider (github or gitlab).",
								Computed:            true,
							},
							"integration_id": schema.StringAttribute{
								MarkdownDescription: "The VCS integration ID.",
								Computed:            true,
							},
							"repository_id": schema.StringAttribute{
								MarkdownDescription: "The repository ID in the VCS provider.",
								Computed:            true,
							},
							"ref_type": schema.StringAttribute{
								MarkdownDescription: "The Git ref type (branch, tag, commit).",
								Computed:            true,
							},
							"ref_name": schema.StringAttribute{
								MarkdownDescription: "The Git ref name (branch name, tag name, or commit SHA).",
								Computed:            true,
							},
							"path": schema.StringAttribute{
								MarkdownDescription: "The path within the repository.",
								Computed:            true,
							},
						},
					},
				},
			},
			"triggers": schema.SingleNestedAttribute{
				MarkdownDescription: "Automation trigger configuration.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"on_push_enabled": schema.BoolAttribute{
						MarkdownDescription: "Whether push-based automation triggers are enabled.",
						Computed:            true,
					},
				},
			},
			"created_by": schema.StringAttribute{
				MarkdownDescription: "The user ID who created this stack.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when the stack was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when the stack was last updated.",
				Computed:            true,
			},
			"updated_by": schema.StringAttribute{
				MarkdownDescription: "The user ID who last updated this stack.",
				Computed:            true,
			},
		},
	}
}

func (d *stackDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *stackDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data stackDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	stack, err := d.client.GetStack(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read stack, got error: %s", err))
		return
	}

	data.Name = types.StringValue(stack.Name)
	data.SpaceID = types.StringValue(stack.SpaceID)
	data.OrganizationID = types.StringValue(stack.OrganizationID)
	if stack.WorkerPoolID != nil {
		data.WorkerPoolID = types.StringValue(*stack.WorkerPoolID)
	} else {
		data.WorkerPoolID = types.StringNull()
	}
	data.AllowPublicPool = types.BoolValue(stack.AllowPublicPool)

	// Map IAC config
	data.IAC = &iacConfigModel{
		Engine:  types.StringValue(stack.IAC.Engine),
		Version: types.StringValue(stack.IAC.Version),
	}

	// Map Source
	data.Source = &stackSourceModel{
		Type: types.StringValue(stack.Source.Type),
	}
	if stack.Source.RawGit != nil {
		data.Source.RawGit = &stackSourceRawGitModel{
			URL:     types.StringValue(stack.Source.RawGit.URL),
			RefType: types.StringValue(stack.Source.RawGit.Ref.Type),
			RefName: types.StringValue(stack.Source.RawGit.Ref.Name),
			Path:    types.StringValue(stack.Source.RawGit.Path),
		}
	}
	if stack.Source.VCS != nil {
		data.Source.VCS = &stackSourceVCSModel{
			Provider:      types.StringValue(stack.Source.VCS.Provider),
			IntegrationID: types.StringValue(stack.Source.VCS.IntegrationID),
			RepositoryID:  types.StringValue(stack.Source.VCS.RepositoryID),
			RefType:       types.StringValue(stack.Source.VCS.Ref.Type),
			RefName:       types.StringValue(stack.Source.VCS.Ref.Name),
			Path:          types.StringValue(stack.Source.VCS.Path),
		}
	}

	// Map Triggers
	data.Triggers = &stackTriggersModel{
		OnPushEnabled: types.BoolValue(stack.Triggers.OnPush.Enabled),
	}

	data.CreatedBy = types.StringValue(stack.CreatedBy)
	data.CreatedAt = types.StringValue(stack.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	data.UpdatedAt = types.StringValue(stack.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
	data.UpdatedBy = types.StringValue(stack.UpdatedBy)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
