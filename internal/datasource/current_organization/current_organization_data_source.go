// ABOUTME: Data source for reading the current authenticated organization.
// ABOUTME: Uses the API token's auth context to determine the organization.

package current_organization

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

type currentOrganizationDataSource struct {
	client *zenfraclient.Client
}

type currentOrganizationDataSourceModel struct {
	ID        types.String      `tfsdk:"id"`
	Name      types.String      `tfsdk:"name"`
	Slug      types.String      `tfsdk:"slug"`
	Settings  *orgSettingsModel `tfsdk:"settings"`
	Billing   *orgBillingModel  `tfsdk:"billing"`
	CreatedAt types.String      `tfsdk:"created_at"`
	UpdatedAt types.String      `tfsdk:"updated_at"`
}

type orgIACToolModel struct {
	Engine  types.String `tfsdk:"engine"`
	Version types.String `tfsdk:"version"`
}

type orgSettingsModel struct {
	DefaultIACTool     *orgIACToolModel `tfsdk:"default_iac_tool"`
	RunTimeoutMinutes  types.Int64      `tfsdk:"run_timeout_minutes"`
	PlanTimeoutMinutes types.Int64      `tfsdk:"plan_timeout_minutes"`
	AuditRetentionDays types.Int64      `tfsdk:"audit_retention_days"`
}

type orgBillingModel struct {
	Plan            types.String `tfsdk:"plan"`
	SlotLimit       types.Int64  `tfsdk:"slot_limit"`
	SlotsUsed       types.Int64  `tfsdk:"slots_used"`
	SlotsAvailable  types.Int64  `tfsdk:"slots_available"`
	EnforcementMode types.String `tfsdk:"enforcement_mode"`
}

var _ datasource.DataSource = &currentOrganizationDataSource{}
var _ datasource.DataSourceWithConfigure = &currentOrganizationDataSource{}

func NewCurrentOrganizationDataSource() datasource.DataSource {
	return &currentOrganizationDataSource{}
}

func (d *currentOrganizationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_current_organization"
}

func (d *currentOrganizationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the current authenticated organization based on the API token.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the organization.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the organization.",
				Computed:            true,
			},
			"slug": schema.StringAttribute{
				MarkdownDescription: "The URL-friendly slug for the organization.",
				Computed:            true,
			},
			"settings": schema.SingleNestedAttribute{
				MarkdownDescription: "Organization settings.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"default_iac_tool": schema.SingleNestedAttribute{
						MarkdownDescription: "Default IaC tool configuration.",
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
					"run_timeout_minutes": schema.Int64Attribute{
						MarkdownDescription: "Maximum run duration in minutes.",
						Computed:            true,
					},
					"plan_timeout_minutes": schema.Int64Attribute{
						MarkdownDescription: "Maximum plan duration in minutes.",
						Computed:            true,
					},
					"audit_retention_days": schema.Int64Attribute{
						MarkdownDescription: "Audit log retention in days.",
						Computed:            true,
					},
				},
			},
			"billing": schema.SingleNestedAttribute{
				MarkdownDescription: "Organization billing information.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"plan": schema.StringAttribute{
						MarkdownDescription: "The billing plan tier.",
						Computed:            true,
					},
					"slot_limit": schema.Int64Attribute{
						MarkdownDescription: "Maximum number of worker slots.",
						Computed:            true,
					},
					"slots_used": schema.Int64Attribute{
						MarkdownDescription: "Number of worker slots currently in use.",
						Computed:            true,
					},
					"slots_available": schema.Int64Attribute{
						MarkdownDescription: "Number of worker slots available.",
						Computed:            true,
					},
					"enforcement_mode": schema.StringAttribute{
						MarkdownDescription: "Billing enforcement mode (soft or hard).",
						Computed:            true,
					},
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when the organization was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when the organization was last updated.",
				Computed:            true,
			},
		},
	}
}

func (d *currentOrganizationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *currentOrganizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data currentOrganizationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	org, err := d.client.GetCurrentOrganization(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read current organization, got error: %s", err))
		return
	}

	data.ID = types.StringValue(org.ID)
	data.Name = types.StringValue(org.Name)
	data.Slug = types.StringValue(org.Slug)
	data.Settings = &orgSettingsModel{
		DefaultIACTool: &orgIACToolModel{
			Engine:  types.StringValue(org.Settings.DefaultIACTool.Engine),
			Version: types.StringValue(org.Settings.DefaultIACTool.Version),
		},
		RunTimeoutMinutes:  types.Int64Value(int64(org.Settings.RunTimeoutMinutes)),
		PlanTimeoutMinutes: types.Int64Value(int64(org.Settings.PlanTimeoutMinutes)),
		AuditRetentionDays: types.Int64Value(int64(org.Settings.AuditRetentionDays)),
	}
	if org.Billing != nil {
		data.Billing = &orgBillingModel{
			Plan:            types.StringValue(org.Billing.Plan),
			SlotLimit:       types.Int64Value(int64(org.Billing.SlotLimit)),
			SlotsUsed:       types.Int64Value(int64(org.Billing.SlotsUsed)),
			SlotsAvailable:  types.Int64Value(int64(org.Billing.SlotsAvailable)),
			EnforcementMode: types.StringValue(org.Billing.EnforcementMode),
		}
	}
	if org.CreatedAt != "" {
		data.CreatedAt = types.StringValue(org.CreatedAt)
	}
	if org.UpdatedAt != "" {
		data.UpdatedAt = types.StringValue(org.UpdatedAt)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
