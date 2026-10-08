// ABOUTME: Acceptance tests for zenfra_configuration_bundle hooks and auto_attach_labels.
// ABOUTME: Requires TF_ACC=1, ZENFRA_API_ENDPOINT, ZENFRA_API_TOKEN and ZENFRA_ACC_SPACE_ID.
package bundle_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	zacc "github.com/zenfra/terraform-provider-zenfra/internal/acctest"
)

const hooksBundle = "zenfra_configuration_bundle.hooks"

func hooksBundleConfig(slug, body string) string {
	return fmt.Sprintf(`
provider "zenfra" {}

resource "zenfra_configuration_bundle" "hooks" {
  name     = %q
  slug     = %q
  space_id = %q
%s
}
`, slug, slug, zacc.SpaceID(), body)
}

func checkBundlesDestroyed(s *terraform.State) error {
	client, err := zacc.Client()
	if err != nil {
		return err
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "zenfra_configuration_bundle" {
			continue
		}
		if _, err := client.GetBundle(context.Background(), rs.Primary.ID); err == nil {
			return fmt.Errorf("bundle %s still exists after destroy", rs.Primary.ID)
		}
	}
	return nil
}

// apiHooksCleared checks the API holds no hooks for the bundle.
func apiHooksCleared(s *terraform.State) error {
	rs, ok := s.RootModule().Resources[hooksBundle]
	if !ok {
		return fmt.Errorf("%s not in state", hooksBundle)
	}
	client, err := zacc.Client()
	if err != nil {
		return err
	}
	bundle, err := client.GetBundle(context.Background(), rs.Primary.ID)
	if err != nil {
		return err
	}
	// A cleared bundle may answer no hooks member or an empty object.
	if h := bundle.Hooks; h != nil &&
		len(h.BeforeInit)+len(h.AfterInit)+len(h.BeforePlan)+len(h.AfterPlan)+len(h.BeforeApply)+len(h.AfterApply) > 0 {
		return fmt.Errorf("API hooks = %+v, want none", h)
	}
	if len(bundle.AutoAttachLabels) != 0 {
		return fmt.Errorf("API auto_attach_labels = %v, want none", bundle.AutoAttachLabels)
	}
	return nil
}

func TestAccBundle_hooksAndAutoAttachLabels(t *testing.T) {
	slug := acctest.RandomWithPrefix("tf-acc-hooks")
	var version string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { zacc.PreCheck(t) },
		ProtoV6ProviderFactories: zacc.ProtoV6ProviderFactories,
		CheckDestroy:             checkBundlesDestroyed,
		Steps: []resource.TestStep{
			{
				Config: hooksBundleConfig(slug, `
  auto_attach_labels = ["tf-acc-prod"]
  hooks = {
    before_plan = ["echo before plan", "test -f main.tf"]
  }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(hooksBundle, "hooks.before_plan.#", "2"),
					resource.TestCheckResourceAttr(hooksBundle, "hooks.before_plan.1", "test -f main.tf"),
					resource.TestCheckNoResourceAttr(hooksBundle, "hooks.after_apply.#"),
					resource.TestCheckTypeSetElemAttr(hooksBundle, "auto_attach_labels.*", "tf-acc-prod"),
					resource.TestCheckResourceAttrWith(hooksBundle, "content_version", func(v string) error {
						version = v
						return nil
					}),
				),
			},
			{
				// The selector is metadata: content_version must not move.
				Config: hooksBundleConfig(slug, `
  auto_attach_labels = ["tf-acc-prod", "tf-acc-eu"]
  hooks = {
    before_plan = ["echo before plan", "test -f main.tf"]
  }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(hooksBundle, "auto_attach_labels.#", "2"),
					resource.TestCheckResourceAttrWith(hooksBundle, "content_version", func(v string) error {
						if v != version {
							return fmt.Errorf("content_version = %s, want %s: a selector change is not content", v, version)
						}
						return nil
					}),
				),
			},
			{
				// Hooks are content: an in-place update with a new version.
				Config: hooksBundleConfig(slug, `
  auto_attach_labels = ["tf-acc-prod", "tf-acc-eu"]
  hooks = {
    before_plan = ["echo before plan"]
    after_apply = ["echo applied"]
  }`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(hooksBundle, plancheck.ResourceActionUpdate),
						plancheck.ExpectUnknownValue(hooksBundle, tfjsonpath.New("content_version")),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(hooksBundle, "hooks.after_apply.0", "echo applied"),
					resource.TestCheckResourceAttrWith(hooksBundle, "content_version", func(v string) error {
						if v == version {
							return fmt.Errorf("content_version stayed %s after a hooks change", v)
						}
						return nil
					}),
				),
			},
			{
				ResourceName:      hooksBundle,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Omitting both clears them on the API, not just in state.
				Config: hooksBundleConfig(slug, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(hooksBundle, "hooks.%"),
					resource.TestCheckNoResourceAttr(hooksBundle, "auto_attach_labels.#"),
					apiHooksCleared,
				),
			},
			{
				Config: hooksBundleConfig(slug, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
