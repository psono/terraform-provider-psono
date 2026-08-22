package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ ephemeral.EphemeralResource              = (*environmentVariableEphemeralResource)(nil)
	_ ephemeral.EphemeralResourceWithConfigure = (*environmentVariableEphemeralResource)(nil)
)

type environmentVariableEphemeralResource struct {
	client environmentVariableClient
}

type environmentVariableEphemeralModel struct {
	SecretID  types.String `tfsdk:"secret_id"`
	Name      types.String `tfsdk:"name"`
	Value     types.String `tfsdk:"value"`
	WriteDate types.String `tfsdk:"write_date"`
}

func NewEnvironmentVariableEphemeralResource() ephemeral.EphemeralResource {
	return &environmentVariableEphemeralResource{}
}

func (r *environmentVariableEphemeralResource) Metadata(_ context.Context, request ephemeral.MetadataRequest, response *ephemeral.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_environment_variable"
}

func (r *environmentVariableEphemeralResource) Schema(_ context.Context, _ ephemeral.SchemaRequest, response *ephemeral.SchemaResponse) {
	response.Schema = schema.Schema{
		Description: "Reads one Psono environment variable without persisting its value in Terraform plan or state.",
		Attributes: map[string]schema.Attribute{
			"secret_id":  schema.StringAttribute{Required: true},
			"name":       schema.StringAttribute{Required: true},
			"value":      schema.StringAttribute{Computed: true, Sensitive: true},
			"write_date": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *environmentVariableEphemeralResource) Configure(_ context.Context, request ephemeral.ConfigureRequest, response *ephemeral.ConfigureResponse) {
	configureClient(request.ProviderData, &r.client, &response.Diagnostics)
}

func (r *environmentVariableEphemeralResource) Open(ctx context.Context, request ephemeral.OpenRequest, response *ephemeral.OpenResponse) {
	if r.client == nil {
		response.Diagnostics.AddError("Unconfigured Psono client", "The provider was not configured before ephemeral resource use.")
		return
	}
	var data environmentVariableEphemeralModel
	response.Diagnostics.Append(request.Config.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}
	value, writeDate, exists, err := r.client.GetEnvironmentVariable(ctx, data.SecretID.ValueString(), data.Name.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Unable to read Psono environment variable", err.Error())
		return
	}
	if !exists {
		response.Diagnostics.AddError("Psono environment variable not found", fmt.Sprintf("Key %q does not exist in secret %s.", data.Name.ValueString(), data.SecretID.ValueString()))
		return
	}
	data.Value = types.StringValue(value)
	data.WriteDate = types.StringValue(writeDate)
	response.Diagnostics.Append(response.Result.Set(ctx, &data)...)
}
