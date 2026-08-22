package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"gitlab.com/esaqa/psono/psono-terraform-provider/internal/psono"
)

var (
	_ provider.Provider                       = (*psonoProvider)(nil)
	_ provider.ProviderWithEphemeralResources = (*psonoProvider)(nil)
)

type psonoProvider struct {
	version string
}

type providerModel struct {
	ServerURL    types.String `tfsdk:"server_url"`
	APIKeyID     types.String `tfsdk:"api_key_id"`
	APISecretKey types.String `tfsdk:"api_secret_key"`
	CABundle     types.String `tfsdk:"ca_bundle"`
}

type clientData struct {
	client environmentVariableClient
}

type environmentVariableClient interface {
	GetEnvironmentVariable(context.Context, string, string) (string, string, bool, error)
	EnsureEnvironmentVariable(context.Context, string, string, func() (string, error)) (string, bool, error)
	UpsertEnvironmentVariable(context.Context, string, string, string) (string, error)
	DeleteEnvironmentVariable(context.Context, string, string) (string, error)
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &psonoProvider{version: version}
	}
}

func (p *psonoProvider) Metadata(_ context.Context, _ provider.MetadataRequest, response *provider.MetadataResponse) {
	response.TypeName = "psono"
	response.Version = p.version
}

func (p *psonoProvider) Schema(_ context.Context, _ provider.SchemaRequest, response *provider.SchemaResponse) {
	response.Schema = schema.Schema{
		Description: "Manages values in pre-created Psono Environment Variables entries using restricted API keys and local decryption.",
		Attributes: map[string]schema.Attribute{
			"server_url": schema.StringAttribute{
				Description: "Psono server URL, including the /server path. May also be set with PSONO_SERVER_URL.",
				Optional:    true,
			},
			"api_key_id": schema.StringAttribute{
				Description: "Restricted Psono API key ID. May also be set with PSONO_API_KEY_ID.",
				Optional:    true,
			},
			"api_secret_key": schema.StringAttribute{
				Description: "Restricted Psono API secret key as 64 hexadecimal characters. May also be set with PSONO_API_SECRET_KEY.",
				Optional:    true,
				Sensitive:   true,
			},
			"ca_bundle": schema.StringAttribute{
				Description: "Optional PEM CA bundle. May also be set with PSONO_CA_BUNDLE.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (p *psonoProvider) Configure(ctx context.Context, request provider.ConfigureRequest, response *provider.ConfigureResponse) {
	var config providerModel
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}
	if config.ServerURL.IsUnknown() || config.APIKeyID.IsUnknown() || config.APISecretKey.IsUnknown() || config.CABundle.IsUnknown() {
		return
	}

	serverURL := configuredString(config.ServerURL, "PSONO_SERVER_URL")
	apiKeyID := configuredString(config.APIKeyID, "PSONO_API_KEY_ID")
	apiSecretKey := configuredString(config.APISecretKey, "PSONO_API_SECRET_KEY")
	caBundle := configuredString(config.CABundle, "PSONO_CA_BUNDLE")
	if serverURL == "" || apiKeyID == "" || apiSecretKey == "" {
		response.Diagnostics.AddError(
			"Missing Psono provider configuration",
			"server_url, api_key_id, and api_secret_key must be set in the provider or corresponding PSONO_* environment variables.",
		)
		return
	}

	client, err := psono.NewClient(psono.Credentials{
		ServerURL:    serverURL,
		APIKeyID:     apiKeyID,
		APISecretKey: apiSecretKey,
		CABundle:     []byte(caBundle),
	})
	if err != nil {
		response.Diagnostics.AddError("Unable to configure Psono client", err.Error())
		return
	}
	data := &clientData{client: client}
	response.ResourceData = data
	response.DataSourceData = data
	response.EphemeralResourceData = data
}

func (p *psonoProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{NewEnvironmentVariableResource}
}

func (p *psonoProvider) DataSources(context.Context) []func() datasource.DataSource {
	return nil
}

func (p *psonoProvider) EphemeralResources(context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{NewEnvironmentVariableEphemeralResource}
}

func configuredString(value types.String, environmentVariable string) string {
	if !value.IsNull() {
		return value.ValueString()
	}
	return os.Getenv(environmentVariable)
}

func configureClient(providerData any, target *environmentVariableClient, diagnostics interface {
	AddError(string, string)
}) {
	if providerData == nil {
		return
	}
	data, ok := providerData.(*clientData)
	if !ok {
		diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *clientData, got %T", providerData))
		return
	}
	*target = data.client
}
