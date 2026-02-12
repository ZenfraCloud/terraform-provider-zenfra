// ABOUTME: Implements the zenfra_vcs_integration Terraform resource with full CRUD lifecycle.
// ABOUTME: Supports GitHub (app installation) and GitLab (personal access token) VCS providers.
package vcs_integration

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

var (
	_ resource.Resource                = &VCSIntegrationResource{}
	_ resource.ResourceWithImportState = &VCSIntegrationResource{}
)

// NewVCSIntegrationResource is a constructor for the VCS integration resource.
func NewVCSIntegrationResource() resource.Resource {
	return &VCSIntegrationResource{}
}

// VCSIntegrationResource is the resource implementation.
type VCSIntegrationResource struct {
	client *zenfraclient.Client
}

func (r *VCSIntegrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vcs_integration"
}

func (r *VCSIntegrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Zenfra VCS integration for GitHub or GitLab.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the VCS integration.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				Description: "The organization ID this integration belongs to.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The display name of the VCS integration.",
				Required:    true,
			},
			"provider_type": schema.StringAttribute{
				Description: "The VCS provider type. Must be 'github' or 'gitlab'.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"personal_access_token": schema.StringAttribute{
				Description: "Personal access token for GitLab integration. Only used when provider_type is 'gitlab'.",
				Optional:    true,
				Sensitive:   true,
			},
			"api_url": schema.StringAttribute{
				Description: "API URL for GitLab integration (e.g., 'https://gitlab.com'). Only used when provider_type is 'gitlab'.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"installation_id": schema.Int64Attribute{
				Description: "GitHub App installation ID. Only used when provider_type is 'github'.",
				Optional:    true,
				Computed:    true,
			},
			"status": schema.StringAttribute{
				Description: "The current status of the integration.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "Timestamp when the integration was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Description: "Timestamp when the integration was last updated.",
				Computed:    true,
			},
		},
	}
}

func (r *VCSIntegrationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*zenfraclient.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *zenfraclient.Client, got: %T.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *VCSIntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan VCSIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := zenfraclient.CreateVCSIntegrationRequest{
		Provider:    plan.ProviderType.ValueString(),
		DisplayName: plan.Name.ValueString(),
	}

	switch plan.ProviderType.ValueString() {
	case "github":
		if plan.InstallationID.IsNull() || plan.InstallationID.IsUnknown() {
			resp.Diagnostics.AddError("Missing Installation ID",
				"installation_id is required for GitHub integrations.")
			return
		}
		createReq.GitHub = &zenfraclient.CreateVCSGitHubRequest{
			InstallationID: plan.InstallationID.ValueInt64(),
		}
	case "gitlab":
		if plan.PersonalAccessToken.IsNull() {
			resp.Diagnostics.AddError("Missing Personal Access Token",
				"personal_access_token is required for GitLab integrations.")
			return
		}
		gitlabReq := &zenfraclient.CreateVCSGitLabRequest{
			AccessToken: plan.PersonalAccessToken.ValueString(),
		}
		if !plan.APIURL.IsNull() && !plan.APIURL.IsUnknown() {
			gitlabReq.BaseURL = plan.APIURL.ValueString()
		}
		createReq.GitLab = gitlabReq
	default:
		resp.Diagnostics.AddError("Invalid Provider Type",
			fmt.Sprintf("provider_type must be 'github' or 'gitlab', got: %s", plan.ProviderType.ValueString()))
		return
	}

	vcs, err := r.client.CreateVCSIntegration(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating VCS Integration",
			fmt.Sprintf("Could not create VCS integration: %s", err))
		return
	}

	state := mapVCSIntegrationToState(vcs)
	// Preserve the sensitive PAT from plan (API won't return it)
	state.PersonalAccessToken = plan.PersonalAccessToken

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *VCSIntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state VCSIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vcs, err := r.client.GetVCSIntegration(ctx, state.ID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading VCS Integration",
			fmt.Sprintf("Could not read VCS integration ID %s: %s", state.ID.ValueString(), err))
		return
	}

	newState := mapVCSIntegrationToState(vcs)
	// Preserve the sensitive PAT from current state (API won't return it)
	newState.PersonalAccessToken = state.PersonalAccessToken

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *VCSIntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state VCSIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := zenfraclient.UpdateVCSIntegrationRequest{}
	if !plan.Name.Equal(state.Name) {
		name := plan.Name.ValueString()
		updateReq.DisplayName = &name
	}

	vcs, err := r.client.UpdateVCSIntegration(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating VCS Integration",
			fmt.Sprintf("Could not update VCS integration: %s", err))
		return
	}

	newState := mapVCSIntegrationToState(vcs)
	newState.PersonalAccessToken = plan.PersonalAccessToken

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *VCSIntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state VCSIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteVCSIntegration(ctx, state.ID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error Deleting VCS Integration",
			fmt.Sprintf("Could not delete VCS integration ID %s: %s", state.ID.ValueString(), err))
	}
}

func (r *VCSIntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
