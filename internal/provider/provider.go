// ABOUTME: Defines the ZenfraProvider implementing the Terraform Plugin Framework provider interface.
// ABOUTME: Configures endpoint and api_token, creates zenfraclient.Client for resource and data source use.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	dsCurrentOrg "github.com/zenfra/terraform-provider-zenfra/internal/datasource/current_organization"
	dsSpace "github.com/zenfra/terraform-provider-zenfra/internal/datasource/space"
	dsStack "github.com/zenfra/terraform-provider-zenfra/internal/datasource/stack"
	dsWorkerPool "github.com/zenfra/terraform-provider-zenfra/internal/datasource/worker_pool"
	resAPIToken "github.com/zenfra/terraform-provider-zenfra/internal/resource/api_token"
	resBundle "github.com/zenfra/terraform-provider-zenfra/internal/resource/bundle"
	resBundleAttachment "github.com/zenfra/terraform-provider-zenfra/internal/resource/bundle_attachment"
	resSpace "github.com/zenfra/terraform-provider-zenfra/internal/resource/space"
	resStack "github.com/zenfra/terraform-provider-zenfra/internal/resource/stack"
	resStackVars "github.com/zenfra/terraform-provider-zenfra/internal/resource/stack_variables"
	resVCS "github.com/zenfra/terraform-provider-zenfra/internal/resource/vcs_integration"
	resWorkerPool "github.com/zenfra/terraform-provider-zenfra/internal/resource/worker_pool"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

const defaultEndpoint = "https://app.zenfra.io"

// Ensure ZenfraProvider satisfies the provider.Provider interface.
var _ provider.Provider = &ZenfraProvider{}

// ZenfraProvider defines the provider implementation.
type ZenfraProvider struct {
	version string
}

// ZenfraProviderModel describes the provider data model.
type ZenfraProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIToken types.String `tfsdk:"api_token"`
}

// New returns a provider.Provider constructor function.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ZenfraProvider{
			version: version,
		}
	}
}

func (p *ZenfraProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "zenfra"
	resp.Version = p.version
}

func (p *ZenfraProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The Zenfra provider enables managing infrastructure-as-code resources on the Zenfra platform.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Description: "The Zenfra API endpoint URL. Defaults to https://app.zenfra.io. Can be set via ZENFRA_API_ENDPOINT environment variable.",
				Optional:    true,
			},
			"api_token": schema.StringAttribute{
				Description: "The API token for authenticating with the Zenfra API. Can be set via ZENFRA_API_TOKEN environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (p *ZenfraProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config ZenfraProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve endpoint: config > env > default.
	endpoint := defaultEndpoint
	if envVal := os.Getenv("ZENFRA_API_ENDPOINT"); envVal != "" {
		endpoint = envVal
	}
	if !config.Endpoint.IsNull() && !config.Endpoint.IsUnknown() {
		endpoint = config.Endpoint.ValueString()
	}

	// Resolve API token: config > env.
	apiToken := os.Getenv("ZENFRA_API_TOKEN")
	if !config.APIToken.IsNull() && !config.APIToken.IsUnknown() {
		apiToken = config.APIToken.ValueString()
	}

	if apiToken == "" {
		resp.Diagnostics.AddError(
			"Missing API Token",
			"The Zenfra API token must be set in the provider configuration or via the ZENFRA_API_TOKEN environment variable.",
		)
		return
	}

	client, err := zenfraclient.NewClient(zenfraclient.ClientConfig{
		Endpoint: endpoint,
		APIToken: apiToken,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Zenfra Client",
			"An unexpected error occurred when creating the Zenfra API client: "+err.Error(),
		)
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *ZenfraProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resSpace.NewSpaceResource,
		resStack.NewStackResource,
		resWorkerPool.NewWorkerPoolResource,
		resBundle.NewBundleResource,
		resBundleAttachment.NewBundleAttachmentResource,
		resStackVars.NewStackVariablesResource,
		resAPIToken.NewAPITokenResource,
		resVCS.NewVCSIntegrationResource,
	}
}

func (p *ZenfraProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		dsSpace.NewSpaceDataSource,
		dsStack.NewStackDataSource,
		dsStack.NewStacksDataSource,
		dsWorkerPool.NewWorkerPoolDataSource,
		dsWorkerPool.NewWorkerPoolsDataSource,
		dsCurrentOrg.NewCurrentOrganizationDataSource,
	}
}
