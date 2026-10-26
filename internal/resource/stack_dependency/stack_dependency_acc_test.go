// ABOUTME: Acceptance test for zenfra_stack_dependency: create, disable and re-enable, import, replace, refusals and delete.
// ABOUTME: Requires TF_ACC=1, ZENFRA_API_ENDPOINT, ZENFRA_API_TOKEN and ZENFRA_ACC_SPACE_ID; creates three stacks.
package stack_dependency_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	zacc "github.com/zenfra/terraform-provider-zenfra/internal/acctest"
)

const depAddr = "zenfra_stack_dependency.dep"

// depConfig renders the stacks a, b and c and the given dependency resources.
func depConfig(prefix string, deps ...string) string {
	cfg := `provider "zenfra" {}` + "\n" +
		zacc.StackHCL("a", prefix+"-a", "") + zacc.StackHCL("b", prefix+"-b", "") + zacc.StackHCL("c", prefix+"-c", "")
	for _, d := range deps {
		cfg += d
	}
	return cfg
}

// dep renders a dependency named name from parent to child (stack resource
// names); enabled is an extra attribute line, or empty.
func dep(name, parent, child, enabled string) string {
	return fmt.Sprintf(`
resource "zenfra_stack_dependency" %q {
  parent_stack_id = zenfra_stack.%s.id
  child_stack_id  = zenfra_stack.%s.id
%s
}
`, name, parent, child, enabled)
}

func TestAccStackDependency_lifecycle(t *testing.T) {
	prefix := acctest.RandomWithPrefix("tf-acc-dep")
	var stackB, id, activatedAt string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { zacc.PreCheck(t) },
		ProtoV6ProviderFactories: zacc.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: depConfig(prefix, dep("dep", "a", "b", "")),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(depAddr, "enabled", "true"),
					resource.TestCheckResourceAttrPair(depAddr, "parent_stack_id", "zenfra_stack.a", "id"),
					resource.TestCheckResourceAttrPair(depAddr, "child_stack_id", "zenfra_stack.b", "id"),
					resource.TestCheckResourceAttrWith("zenfra_stack.b", "id", record(&stackB)),
					resource.TestCheckResourceAttrWith(depAddr, "id", record(&id)),
					resource.TestCheckResourceAttrWith(depAddr, "activated_at", func(v string) error {
						if v == "" {
							return fmt.Errorf("activated_at is empty")
						}
						activatedAt = v
						return nil
					}),
					upstreamHas(&stackB, &id, true),
				),
			},
			{
				Config: depConfig(prefix, dep("dep", "a", "b", "  enabled = false")),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(depAddr, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(depAddr, "enabled", "false"),
					resource.TestCheckResourceAttrWith(depAddr, "id", sameAs(&id, "id")),
					resource.TestCheckResourceAttrWith(depAddr, "activated_at", sameAs(&activatedAt, "activated_at")),
				),
			},
			{
				Config: depConfig(prefix, dep("dep", "a", "b", "  enabled = true")),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(depAddr, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(depAddr, "enabled", "true"),
					resource.TestCheckResourceAttrWith(depAddr, "id", sameAs(&id, "id")),
					resource.TestCheckResourceAttrWith(depAddr, "activated_at", func(v string) error {
						if v == activatedAt {
							return fmt.Errorf("activated_at %s unchanged after a re-enable", v)
						}
						return nil
					}),
				),
			},
			{
				ResourceName:      depAddr,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return s.RootModule().Resources["zenfra_stack.b"].Primary.ID + "/" +
						s.RootModule().Resources[depAddr].Primary.ID, nil
				},
			},
			{
				// A different parent is a new edge.
				Config: depConfig(prefix, dep("dep", "c", "b", "")),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(depAddr, plancheck.ResourceActionReplace)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(depAddr, "parent_stack_id", "zenfra_stack.c", "id"),
					resource.TestCheckResourceAttrWith(depAddr, "id", func(v string) error {
						if v == id {
							return fmt.Errorf("id %s unchanged after a replacement", v)
						}
						id = v
						return nil
					}),
				),
			},
			{
				// b → c beside the enabled c → b closes a cycle: refused at apply.
				Config:      depConfig(prefix, dep("dep", "c", "b", ""), dep("cycle", "b", "c", "")),
				ExpectError: regexp.MustCompile("dependency_cycle"),
			},
			{
				// A new edge cannot be created disabled: refused at plan.
				Config:      depConfig(prefix, dep("dep", "c", "b", ""), dep("off", "a", "b", "  enabled = false")),
				ExpectError: regexp.MustCompile("Stack Dependency Cannot Be Created Disabled"),
			},
			{
				Config: depConfig(prefix, dep("dep", "c", "b", "")),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// The edge alone is destroyed while b still exists, so the API's
				// list proves the DELETE: deleting b at the final destroy would
				// also remove its incoming edges and hide a no-op DELETE.
				Config: depConfig(prefix),
				Check:  upstreamHas(&stackB, &id, false),
			},
		},
	})
}

func record(dst *string) func(string) error {
	return func(v string) error {
		*dst = v
		return nil
	}
}

func sameAs(want *string, name string) func(string) error {
	return func(v string) error {
		if v != *want {
			return fmt.Errorf("%s changed from %s to %s", name, *want, v)
		}
		return nil
	}
}

// upstreamHas checks through the API whether the child's upstream list holds
// the edge.
func upstreamHas(child, edge *string, want bool) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		client, err := zacc.Client()
		if err != nil {
			return err
		}
		list, err := client.ListStackDependencies(context.Background(), *child)
		if err != nil {
			return err
		}
		found := false
		for _, e := range list.Upstream {
			found = found || e.ID == *edge
		}
		if found != want {
			return fmt.Errorf("edge %s in the upstream of stack %s: %v, want %v", *edge, *child, found, want)
		}
		return nil
	}
}
