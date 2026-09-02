// ABOUTME: Implements the zenfra_cloud_integration_attachment Terraform resource for attaching cloud integrations to stacks.
// ABOUTME: Uses composite ID "integration_id:attachment_id" and ForceNew semantics for all user-configurable fields.
package cloud_integration_attachment

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

var (
	_ resource.Resource                = &CloudIntegrationAttachmentResource{}
	_ resource.ResourceWithImportState = &CloudIntegrationAttachmentResource{}
)

// NewCloudIntegrationAttachmentResource is a constructor for the cloud integration attachment resource.
func NewCloudIntegrationAttachmentResource() resource.Resource {
	return &CloudIntegrationAttachmentResource{}
}

// CloudIntegrationAttachmentResource is the resource implementation.
type CloudIntegrationAttachmentResource struct {
	client *zenfraclient.Client
}

func (r *CloudIntegrationAttachmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_integration_attachment"
}

func (r *CloudIntegrationAttachmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Attaches a cloud integration to a stack, granting the stack access to cloud credentials.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The attachment identifier.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"integration_id": schema.StringAttribute{
				Description: "The cloud integration to attach.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"stack_id": schema.StringAttribute{
				Description: "The stack to attach the cloud integration to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"read": schema.BoolAttribute{
				Description: "Whether the attachment grants read access. Defaults to true.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"write": schema.BoolAttribute{
				Description: "Whether the attachment grants write access. Defaults to true.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"is_auto_attached": schema.BoolAttribute{
				Description: "Whether this attachment was automatically created.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "Timestamp when the attachment was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"external_id": schema.StringAttribute{
				Description: "The external ID used for assuming the cloud role.",
				Computed:    true,
			},
		},
	}
}

func (r *CloudIntegrationAttachmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CloudIntegrationAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CloudIntegrationAttachmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	integrationID := plan.IntegrationID.ValueString()

	attachment, err := r.client.AttachCloudIntegration(ctx, integrationID, zenfraclient.AttachCloudIntegrationRequest{
		StackID: plan.StackID.ValueString(),
		Read:    plan.Read.ValueBool(),
		Write:   plan.Write.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error Attaching Cloud Integration",
			fmt.Sprintf("Could not attach cloud integration %s to stack %s: %s", integrationID, plan.StackID.ValueString(), err))
		return
	}

	plan.fromAPI(attachment)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *CloudIntegrationAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CloudIntegrationAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	integrationID := state.IntegrationID.ValueString()
	attachmentID := state.ID.ValueString()

	attachments, err := r.client.ListCloudAttachments(ctx, integrationID)
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading Cloud Integration Attachment",
			fmt.Sprintf("Could not list attachments for cloud integration %s: %s", integrationID, err))
		return
	}

	for _, att := range attachments {
		if att.ID == attachmentID {
			state.fromAPI(&att)
			resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
			return
		}
	}

	// Attachment not found; remove from state.
	resp.State.RemoveResource(ctx)
}

func (r *CloudIntegrationAttachmentResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected Update", "Cloud integration attachment does not support in-place updates.")
}

func (r *CloudIntegrationAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CloudIntegrationAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DetachCloudIntegration(ctx, state.IntegrationID.ValueString(), state.ID.ValueString())
	if err != nil {
		if zenfraclient.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error Detaching Cloud Integration",
			fmt.Sprintf("Could not detach cloud integration %s (attachment %s): %s", state.IntegrationID.ValueString(), state.ID.ValueString(), err))
	}
}

func (r *CloudIntegrationAttachmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected format: integration_id:attachment_id, got: %s", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, CloudIntegrationAttachmentModel{
		ID:            types.StringValue(parts[1]),
		IntegrationID: types.StringValue(parts[0]),
	})...)
}
