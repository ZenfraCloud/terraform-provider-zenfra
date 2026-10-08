// ABOUTME: Acceptance tests for zenfra_bundle_attachment priority and its coexistence with label auto-attach.
// ABOUTME: Requires TF_ACC=1, ZENFRA_API_ENDPOINT, ZENFRA_API_TOKEN and ZENFRA_ACC_SPACE_ID.
package bundle_attachment_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	zacc "github.com/zenfra/terraform-provider-zenfra/internal/acctest"
)

const (
	attachment = "zenfra_bundle_attachment.explicit"
	stackAddr  = "zenfra_stack.app"
	autoAddr   = "zenfra_configuration_bundle.auto"
	explAddr   = "zenfra_configuration_bundle.explicit"
)

// attachmentConfig renders a labelled stack, a bundle that auto-attaches by
// that label and a second bundle attached explicitly with priority.
func attachmentConfig(prefix string, priority int) string {
	label := prefix + "-match"
	return `provider "zenfra" {}` + zacc.StackHCL("app", prefix, fmt.Sprintf("\n  labels = [%q]\n", label)) + fmt.Sprintf(`
resource "zenfra_configuration_bundle" "auto" {
  name               = "%[1]s-auto"
  slug               = "%[1]s-auto"
  space_id           = %[2]q
  auto_attach_labels = [%[3]q]

  environment_variable {
    key   = "FROM_LABEL"
    value = "yes"
  }
}

resource "zenfra_configuration_bundle" "explicit" {
  name     = "%[1]s-explicit"
  slug     = "%[1]s-explicit"
  space_id = %[2]q

  environment_variable {
    key   = "FROM_ATTACHMENT"
    value = "yes"
  }
}

resource "zenfra_bundle_attachment" "explicit" {
  stack_id  = zenfra_stack.app.id
  bundle_id = zenfra_configuration_bundle.explicit.id
  priority  = %[4]d
}
`, prefix, zacc.SpaceID(), label, priority)
}

// apiAttachments checks the listing the API serves for the stack: the
// explicit bundle with its priority under attachments, the label match
// under auto_attached only.
func apiAttachments(wantPriority int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		root := s.RootModule()
		stack, auto, expl := root.Resources[stackAddr], root.Resources[autoAddr], root.Resources[explAddr]
		if stack == nil || auto == nil || expl == nil {
			return fmt.Errorf("stack or bundles missing from state")
		}
		client, err := zacc.Client()
		if err != nil {
			return err
		}
		list, err := client.ListStackBundles(context.Background(), stack.Primary.ID)
		if err != nil {
			return err
		}
		att := list.ExplicitAttachment(expl.Primary.ID)
		if att == nil || att.Priority == nil || *att.Priority != wantPriority {
			return fmt.Errorf("explicit attachment = %+v, want priority %d", att, wantPriority)
		}
		if list.ExplicitAttachment(auto.Primary.ID) != nil {
			return fmt.Errorf("the label match is listed as an explicit attachment")
		}
		for i := range list.AutoAttached {
			if list.AutoAttached[i].BundleID == auto.Primary.ID {
				return nil
			}
		}
		return fmt.Errorf("auto_attached = %+v, want bundle %s", list.AutoAttached, auto.Primary.ID)
	}
}

func TestAccBundleAttachment_priorityInPlaceBesideAutoAttach(t *testing.T) {
	prefix := acctest.RandomWithPrefix("tf-acc-att")
	var attachmentID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { zacc.PreCheck(t) },
		ProtoV6ProviderFactories: zacc.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: attachmentConfig(prefix, 3),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(attachment, "priority", "3"),
					resource.TestCheckResourceAttrWith(attachment, "id", func(v string) error {
						attachmentID = v
						return nil
					}),
					apiAttachments(3),
				),
			},
			{
				// A priority change is an in-place update, not a replacement.
				Config: attachmentConfig(prefix, 5),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(attachment, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(attachment, "priority", "5"),
					resource.TestCheckResourceAttrWith(attachment, "id", func(v string) error {
						if v != attachmentID {
							return fmt.Errorf("id = %s, want %s: the attachment was replaced", v, attachmentID)
						}
						return nil
					}),
					apiAttachments(5),
				),
			},
			{
				// Back to the default, still in place.
				Config: attachmentConfig(prefix, 0),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(attachment, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(attachment, "priority", "0"),
					apiAttachments(0),
				),
			},
			{
				ResourceName:      attachment,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: attachmentConfig(prefix, 0),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
