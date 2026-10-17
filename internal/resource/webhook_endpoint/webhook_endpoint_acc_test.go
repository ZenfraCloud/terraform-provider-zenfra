// ABOUTME: Acceptance test for zenfra_webhook_endpoint: create, in-place update, re-enable on apply and import.
// ABOUTME: Requires TF_ACC=1, ZENFRA_API_ENDPOINT, an Admin ZENFRA_API_TOKEN, ZENFRA_ACC_SPACE_ID and webhooks enabled on the API.
package webhook_endpoint_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	zacc "github.com/zenfra/terraform-provider-zenfra/internal/acctest"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

const endpointAddr = "zenfra_webhook_endpoint.hook"

// endpointConfig renders an endpoint on a public host: the API resolves the
// URL and refuses private addresses, so the host must resolve publicly.
// Nothing is delivered to it in this test.
func endpointConfig(prefix, path, eventTypes string) string {
	return fmt.Sprintf(`
provider "zenfra" {}

resource "zenfra_webhook_endpoint" "hook" {
  url         = "https://example.com/%[1]s/%[2]s"
  description = %[1]q
  event_types = [%[3]s]
}
`, prefix, path, eventTypes)
}

func TestAccWebhookEndpoint_lifecycle(t *testing.T) {
	prefix := acctest.RandomWithPrefix("tf-acc-hook")
	var id, secret string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { zacc.PreCheck(t) },
		ProtoV6ProviderFactories: zacc.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: endpointConfig(prefix, "v1", `"run.failed"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(endpointAddr, "enabled", "true"),
					resource.TestCheckNoResourceAttr(endpointAddr, "disabled_reason"),
					resource.TestCheckResourceAttrWith(endpointAddr, "id", func(v string) error {
						id = v
						return nil
					}),
					resource.TestCheckResourceAttrWith(endpointAddr, "secret", func(v string) error {
						if !strings.HasPrefix(v, "whsec_") {
							return fmt.Errorf("secret %q, want the create response's whsec_ secret", v)
						}
						secret = v
						return nil
					}),
				),
			},
			{
				// A new URL and event types are an in-place update that keeps the secret.
				Config: endpointConfig(prefix, "v2", `"run.failed", "run.finished"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(endpointAddr, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(endpointAddr, "url", "https://example.com/"+prefix+"/v2"),
					resource.TestCheckResourceAttr(endpointAddr, "event_types.#", "2"),
					resource.TestCheckResourceAttrWith(endpointAddr, "id", sameAs(&id, "id")),
					resource.TestCheckResourceAttrWith(endpointAddr, "secret", sameAs(&secret, "secret")),
				),
			},
			{
				// Disabled outside Terraform, as the dispatcher does after repeated
				// failures: the plan re-enables it and the apply does.
				PreConfig: func() { disableOutsideTerraform(t, id) },
				Config:    endpointConfig(prefix, "v2", `"run.failed", "run.finished"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(endpointAddr, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(endpointAddr, "enabled", "true"),
					resource.TestCheckNoResourceAttr(endpointAddr, "disabled_reason"),
					apiEnabled(&id),
				),
			},
			{
				// The secret is not readable after creation: an import has none.
				ResourceName:            endpointAddr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret"},
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 || states[0].Attributes["secret"] != "" {
						return fmt.Errorf("imported states = %+v, want one with no secret", states)
					}
					return nil
				},
			},
			{
				Config: endpointConfig(prefix, "v2", `"run.failed", "run.finished"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func sameAs(want *string, name string) func(string) error {
	return func(v string) error {
		if v != *want {
			return fmt.Errorf("%s changed: the endpoint was replaced or the value was not kept", name)
		}
		return nil
	}
}

func disableOutsideTerraform(t *testing.T, id string) {
	t.Helper()
	client, err := zacc.Client()
	if err != nil {
		t.Fatal(err)
	}
	disabled := false
	if _, err := client.PatchWebhookEndpoint(context.Background(), id, zenfraclient.PatchWebhookEndpointRequest{Enabled: &disabled}); err != nil {
		t.Fatalf("disable endpoint %s: %v", id, err)
	}
}

func apiEnabled(id *string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		client, err := zacc.Client()
		if err != nil {
			return err
		}
		ep, err := client.GetWebhookEndpoint(context.Background(), *id)
		if err != nil {
			return err
		}
		if !ep.Enabled || ep.DisabledReason != "" {
			return fmt.Errorf("API endpoint = %+v, want enabled with no reason", ep)
		}
		return nil
	}
}
