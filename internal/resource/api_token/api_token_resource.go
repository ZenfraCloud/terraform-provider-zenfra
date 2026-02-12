// ABOUTME: Implements the zenfra_api_token Terraform resource with create-and-delete lifecycle.
// ABOUTME: Token value is write-once and only returned on creation; never re-populated on Read.
package api_token

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

var (
	_ resource.Resource                = &APITokenResource{}
	_ resource.ResourceWithImportState = &APITokenResource{}
)

// NewAPITokenResource is a constructor for the API token resource.
func NewAPITokenResource() resource.Resource {
	return &APITokenResource{}
}

// APITokenResource is the resource implementation.
type APITokenResource struct {
	client *zenfraclient.Client
}

func (r *APITokenResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_token"
}

func (r *APITokenResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Zenfra API token. The token value is only available at creation time.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the token.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the API token.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the API token.",
				Optional:    true,
			},
			"token": schema.StringAttribute{
				Description: "The API token value. Only available after creation.",
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "Timestamp when the token was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"expires_at": schema.StringAttribute{
				Description: "Timestamp when the token expires.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"active": schema.BoolAttribute{
				Description: "Whether the token is currently active.",
				Computed:    true,
			},
		},
	}
}

func (r *APITokenResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *APITokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan APITokenModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := zenfraclient.CreateTokenRequest{
		Name: plan.Name.ValueString(),
	}
	if !plan.Description.IsNull() {
		createReq.Description = plan.Description.ValueString()
	}

	createResp, err := r.client.CreateToken(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating API Token", fmt.Sprintf("Could not create token: %s", err))
		return
	}

	state := mapTokenToState(&createResp.TokenObj)
	state.Token = types.StringValue(createResp.Token)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *APITokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state APITokenModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token, err := r.client.GetToken(ctx, state.ID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading API Token",
			fmt.Sprintf("Could not read token ID %s: %s", state.ID.ValueString(), err))
		return
	}

	newState := mapTokenToState(token)
	// Preserve the write-once token value from current state
	newState.Token = state.Token

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *APITokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state APITokenModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token, err := r.client.GetToken(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading API Token", fmt.Sprintf("Could not read token: %s", err))
		return
	}

	newState := mapTokenToState(token)
	newState.Token = state.Token
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *APITokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state APITokenModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteToken(ctx, state.ID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error Deleting API Token",
			fmt.Sprintf("Could not delete token ID %s: %s", state.ID.ValueString(), err))
	}
}

func (r *APITokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
