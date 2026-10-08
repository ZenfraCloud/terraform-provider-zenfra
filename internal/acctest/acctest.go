// ABOUTME: Shared acceptance-test harness: in-process provider factories, precheck, API client and HCL fixtures.
// ABOUTME: Used only by *_acc_test.go files, which run under TF_ACC=1 against a live Zenfra API.
package acctest

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/zenfra/terraform-provider-zenfra/internal/provider"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

// The public fixture the platform E2E uses, so an acceptance run needs no
// private credentials beyond the Zenfra API token itself.
const (
	SourceURL = "https://github.com/ZenfraCloud/zenfra-tf-min-stack-public.git"
	SourceRef = "simple"
)

// requiredEnv are the variables an acceptance run needs on top of TF_ACC.
var requiredEnv = []string{"ZENFRA_API_ENDPOINT", "ZENFRA_API_TOKEN", "ZENFRA_ACC_SPACE_ID"}

// ProtoV6ProviderFactories serves this provider in-process, so the tests
// exercise the real schema, plan modifiers and CRUD rather than a copy.
var ProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"zenfra": providerserver.NewProtocol6WithError(provider.New("test")()),
}

// PreCheck fails rather than skips. resource.Test already skips the whole test
// when TF_ACC is unset, so once acceptance tests were asked for, a missing
// endpoint, token or space ID is a misconfiguration; skipping there would let
// `make testacc` report green while exercising nothing.
func PreCheck(t *testing.T) {
	t.Helper()
	for _, name := range requiredEnv {
		if os.Getenv(name) == "" {
			t.Fatalf("%s must be set for acceptance tests", name)
		}
	}
}

// Client returns an API client for checks made outside Terraform. Build it
// inside a check, never when a TestCase is constructed: a TestCase is built
// even when resource.Test then skips it for want of TF_ACC.
func Client() (*zenfraclient.Client, error) {
	client, err := zenfraclient.NewClient(zenfraclient.ClientConfig{
		Endpoint: os.Getenv("ZENFRA_API_ENDPOINT"),
		APIToken: os.Getenv("ZENFRA_API_TOKEN"),
	})
	if err != nil {
		return nil, fmt.Errorf("acceptance client: %w", err)
	}
	return client, nil
}

// SpaceID is the space the acceptance resources are created in.
func SpaceID() string {
	return os.Getenv("ZENFRA_ACC_SPACE_ID")
}

// StackHCL renders a zenfra_stack named resourceName on the public fixture.
// extra is inserted verbatim into the resource body (for example a labels
// line), or empty.
func StackHCL(resourceName, name, extra string) string {
	return fmt.Sprintf(`
resource "zenfra_stack" %q {
  space_id = %q
  name     = %q
%s
  iac = {
    engine  = "terraform"
    version = "1.9.0"
  }

  source = {
    type = "raw_git"
    raw_git = {
      url = %q
      ref = {
        type = "branch"
        name = %q
      }
      path = "."
    }
  }
}
`, resourceName, SpaceID(), name, extra, SourceURL, SourceRef)
}
