// ABOUTME: Acceptance tests for zenfra_stack's create-only state_management attribute.
// ABOUTME: Requires TF_ACC=1 and a control plane that supports external state ownership.
package stack_test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/zenfra/terraform-provider-zenfra/internal/provider"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// The public fixture the platform E2E uses, down to the branch, project root
// and engine version, so an acceptance run needs no private credentials beyond
// the Zenfra API token itself. The API only validates a raw-git ref and path
// syntactically, so a fixture pointing at a branch that does not exist would
// still create stacks and let these tests pass while nothing could be checked
// out.
const (
	accSourceURL  = "https://github.com/ZenfraCloud/zenfra-tf-min-stack-public"
	accSourceRef  = "main"
	accSourcePath = "simple"
	accIACVersion = "1.14.2"
)

// testAccProtoV6ProviderFactories serves this provider in-process, so the tests
// exercise the real schema, plan modifiers and CRUD rather than a copy.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"zenfra": providerserver.NewProtocol6WithError(provider.New("test")()),
}

// testAccPreCheck fails rather than skips. resource.Test already skips the whole
// test when TF_ACC is unset, so once the operator has asked for acceptance tests
// a missing endpoint, token or space ID is a misconfiguration; skipping there
// would let `make testacc` report green while exercising nothing.
func testAccPreCheck(t *testing.T) {
	t.Helper()
	for _, name := range []string{"ZENFRA_API_ENDPOINT", "ZENFRA_API_TOKEN", "ZENFRA_ACC_SPACE_ID"} {
		if os.Getenv(name) == "" {
			t.Fatalf("%s must be set for acceptance tests", name)
		}
	}
}

// testAccCheckStackDestroy asserts every stack the test made is gone, so a
// failed run does not leave stacks behind unnoticed. It also covers destroying
// an external stack, which the transition guard must allow.
//
// The client is built inside the check, not when the TestCase is constructed:
// a TestCase is built even for a run that resource.Test then skips for want of
// TF_ACC, and reaching for the environment there would fail the unit suite.
func testAccCheckStackDestroy(s *terraform.State) error {
	client, err := zenfraclient.NewClient(zenfraclient.ClientConfig{
		Endpoint: os.Getenv("ZENFRA_API_ENDPOINT"),
		APIToken: os.Getenv("ZENFRA_API_TOKEN"),
	})
	if err != nil {
		return fmt.Errorf("acceptance client: %w", err)
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "zenfra_stack" {
			continue
		}
		if _, err := client.GetStack(context.Background(), rs.Primary.ID); err == nil {
			return fmt.Errorf("stack %s still exists after destroy", rs.Primary.ID)
		} else if !zenfraclient.IsNotFound(err) {
			return fmt.Errorf("checking stack %s: %w", rs.Primary.ID, err)
		}
	}
	return nil
}

// testAccStackConfig renders a stack. mode is the literal HCL for the
// state_management attribute line, or empty to omit the attribute entirely.
func testAccStackConfig(name, mode string) string {
	attr := ""
	if mode != "" {
		attr = fmt.Sprintf("\n  state_management = %q\n", mode)
	}
	return fmt.Sprintf(`
provider "zenfra" {}

resource "zenfra_stack" "test" {
  space_id = %q
  name     = %q
%s
  iac = {
    engine  = "terraform"
    version = %q
  }

  source = {
    type = "raw_git"
    raw_git = {
      url = %q
      ref = {
        type = "branch"
        name = %q
      }
      path = %q
    }
  }
}
`, os.Getenv("ZENFRA_ACC_SPACE_ID"), name, attr, accIACVersion, accSourceURL, accSourceRef, accSourcePath)
}

// captureID records the resource's ID so a later step can prove it changed.
func captureID(into *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources["zenfra_stack.test"]
		if !ok {
			return fmt.Errorf("zenfra_stack.test not in state")
		}
		*into = rs.Primary.ID
		return nil
	}
}

func idChangedFrom(previous *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources["zenfra_stack.test"]
		if !ok {
			return fmt.Errorf("zenfra_stack.test not in state")
		}
		if rs.Primary.ID == *previous {
			return fmt.Errorf("stack was not recreated: id is still %s", rs.Primary.ID)
		}
		return nil
	}
}

func TestAccStack_stateManagementExternal(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-external")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckStackDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStackConfig(name, "external"),
				Check: resource.TestCheckResourceAttr(
					"zenfra_stack.test", "state_management", "external"),
			},
			{
				// A refresh must not drift the attribute back to managed.
				Config:   testAccStackConfig(name, "external"),
				PlanOnly: true,
			},
		},
	})
}

// The regression Optional + Computed exists to prevent: an omitted attribute
// must read back as managed and leave a second plan empty.
func TestAccStack_stateManagementOmittedIsManagedAndStable(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-omitted")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckStackDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStackConfig(name, ""),
				Check: resource.TestCheckResourceAttr(
					"zenfra_stack.test", "state_management", "managed"),
			},
			{
				Config: testAccStackConfig(name, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccStack_stateManagementManagedToExternalRefused(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-m2e")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckStackDestroy,
		Steps: []resource.TestStep{
			{Config: testAccStackConfig(name, "managed")},
			{
				Config:      testAccStackConfig(name, "external"),
				ExpectError: regexp.MustCompile(`State ownership cannot be changed`),
			},
		},
	})
}

func TestAccStack_stateManagementExternalToManagedRefused(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-e2m")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckStackDestroy,
		Steps: []resource.TestStep{
			{Config: testAccStackConfig(name, "external")},
			{
				Config:      testAccStackConfig(name, "managed"),
				ExpectError: regexp.MustCompile(`State ownership cannot be changed`),
			},
		},
	})
}

// Taint crosses the guard on a transition that would otherwise be refused,
// because Terraform presents a tainted object as a creation and the guard
// correctly reads a null prior state as one. This is the bypass the
// documentation warns about, so it is pinned in the direction the warning is
// about: managed to external. Written the other way round — external recreated
// as external — the test would be vacuous, since prior and configured modes
// would be equal and the guard would stay quiet regardless.
func TestAccStack_stateManagementTaintCrossesTheGuard(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-taint")
	var managedID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckStackDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStackConfig(name, "managed"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("zenfra_stack.test", "state_management", "managed"),
					captureID(&managedID),
				),
			},
			{
				// Taint is applied prior to the execution of the step, so the
				// changed configuration belongs in this same step.
				Taint:  []string{"zenfra_stack.test"},
				Config: testAccStackConfig(name, "external"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("zenfra_stack.test", "state_management", "external"),
					idChangedFrom(&managedID),
				),
			},
		},
	})
}

func TestAccStack_stateManagementImport(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-import")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckStackDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStackConfig(name, "external"),
				Check: resource.TestCheckResourceAttr(
					"zenfra_stack.test", "state_management", "external"),
			},
			{
				ResourceName:      "zenfra_stack.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
