// ABOUTME: Terraform state model for the zenfra_bundle_attachment resource.
// ABOUTME: Uses composite ID format "stack_id:bundle_id" for the attachment relationship.
package bundle_attachment

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// BundleAttachmentModel represents the Terraform state model for a bundle-to-stack attachment.
type BundleAttachmentModel struct {
	ID       types.String `tfsdk:"id"`
	StackID  types.String `tfsdk:"stack_id"`
	BundleID types.String `tfsdk:"bundle_id"`
}
