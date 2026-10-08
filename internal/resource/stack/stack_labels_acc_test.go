// ABOUTME: Acceptance tests for zenfra_stack labels: set, change, clear by omission, import.
// ABOUTME: Requires TF_ACC=1, ZENFRA_API_ENDPOINT, ZENFRA_API_TOKEN and ZENFRA_ACC_SPACE_ID.
package stack_test

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

const labelsStack = "zenfra_stack.labels"

func labelsStackConfig(name, labels string) string {
	extra := ""
	if labels != "" {
		extra = "\n  labels = " + labels + "\n"
	}
	return `provider "zenfra" {}` + zacc.StackHCL("labels", name, extra)
}

// apiLabelsAre checks the labels the API holds, not just what state says.
func apiLabelsAre(want ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[labelsStack]
		if !ok {
			return fmt.Errorf("%s not in state", labelsStack)
		}
		client, err := zacc.Client()
		if err != nil {
			return err
		}
		stack, err := client.GetStack(context.Background(), rs.Primary.ID)
		if err != nil {
			return err
		}
		got := map[string]bool{}
		for _, l := range stack.Labels {
			got[l] = true
		}
		if len(got) != len(want) {
			return fmt.Errorf("API labels = %v, want %v", stack.Labels, want)
		}
		for _, l := range want {
			if !got[l] {
				return fmt.Errorf("API labels = %v, want %v", stack.Labels, want)
			}
		}
		return nil
	}
}

func checkStacksDestroyed(s *terraform.State) error {
	client, err := zacc.Client()
	if err != nil {
		return err
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "zenfra_stack" {
			continue
		}
		if _, err := client.GetStack(context.Background(), rs.Primary.ID); err == nil {
			return fmt.Errorf("stack %s still exists after destroy", rs.Primary.ID)
		}
	}
	return nil
}

func TestAccStack_labels(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-labels")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { zacc.PreCheck(t) },
		ProtoV6ProviderFactories: zacc.ProtoV6ProviderFactories,
		CheckDestroy:             checkStacksDestroyed,
		Steps: []resource.TestStep{
			{
				Config: labelsStackConfig(name, `["prod", "team.payments"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(labelsStack, "labels.#", "2"),
					resource.TestCheckTypeSetElemAttr(labelsStack, "labels.*", "team.payments"),
					apiLabelsAre("prod", "team.payments"),
				),
			},
			{
				// The API keeps first-seen order; a set must not diff on it.
				Config: labelsStackConfig(name, `["team.payments", "prod"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: labelsStackConfig(name, `["prod", "eu"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(labelsStack, plancheck.ResourceActionUpdate)},
				},
				Check: apiLabelsAre("prod", "eu"),
			},
			{
				ResourceName:      labelsStack,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Omitting the attribute clears the labels on the API.
				Config: labelsStackConfig(name, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(labelsStack, "labels.#"),
					apiLabelsAre(),
				),
			},
			{
				Config: labelsStackConfig(name, `[]`),
				Check:  apiLabelsAre(),
			},
			{
				Config: labelsStackConfig(name, `[]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccStack_labelsRejectedAtPlan(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-badlabel")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { zacc.PreCheck(t) },
		ProtoV6ProviderFactories: zacc.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      labelsStackConfig(name, `["Prod"]`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`must be lowercase`),
			},
		},
	})
}
