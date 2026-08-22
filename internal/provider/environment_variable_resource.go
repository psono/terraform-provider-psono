package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = (*environmentVariableResource)(nil)
	_ resource.ResourceWithConfigure      = (*environmentVariableResource)(nil)
	_ resource.ResourceWithImportState    = (*environmentVariableResource)(nil)
	_ resource.ResourceWithValidateConfig = (*environmentVariableResource)(nil)
)

type environmentVariableResource struct {
	client environmentVariableClient
}

type environmentVariableResourceModel struct {
	ID              types.String `tfsdk:"id"`
	SecretID        types.String `tfsdk:"secret_id"`
	Name            types.String `tfsdk:"name"`
	ValueWO         types.String `tfsdk:"value_wo"`
	ValueWOVersion  types.Int64  `tfsdk:"value_wo_version"`
	RotationVersion types.Int64  `tfsdk:"rotation_version"`
	Length          types.Int64  `tfsdk:"length"`
	Lower           types.Bool   `tfsdk:"lower"`
	Upper           types.Bool   `tfsdk:"upper"`
	Numeric         types.Bool   `tfsdk:"numeric"`
	Special         types.Bool   `tfsdk:"special"`
	MinLower        types.Int64  `tfsdk:"min_lower"`
	MinUpper        types.Int64  `tfsdk:"min_upper"`
	MinNumeric      types.Int64  `tfsdk:"min_numeric"`
	MinSpecial      types.Int64  `tfsdk:"min_special"`
	SpecialChars    types.String `tfsdk:"special_characters"`
	DeletionPolicy  types.String `tfsdk:"deletion_policy"`
	WriteDate       types.String `tfsdk:"write_date"`
}

func NewEnvironmentVariableResource() resource.Resource {
	return &environmentVariableResource{}
}

func (r *environmentVariableResource) Metadata(_ context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_environment_variable"
}

func (r *environmentVariableResource) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Description: "Generates or writes one key inside a pre-created Psono Environment Variables entry. Secret values are never stored in Terraform state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Resource identifier in secret_id:name form."},
			"secret_id": schema.StringAttribute{
				Required:    true,
				Description: "UUID of a pre-created Psono Environment Variables entry assigned to the restricted API key.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^[0-9a-fA-F-]{36}$`), "must be a UUID"),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Environment variable key to manage.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`), "must be a POSIX environment variable name"),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"value_wo": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				WriteOnly:   true,
				Description: "Optional write-only value. When omitted, the provider generates a value if the key does not exist.",
			},
			"value_wo_version": schema.Int64Attribute{
				Optional:    true,
				Description: "Increment to write a new value_wo. Required when value_wo is configured.",
				Validators:  []validator.Int64{int64validator.AtLeast(1)},
			},
			"rotation_version": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(1),
				Description: "Increment to replace a provider-generated value.",
				Validators:  []validator.Int64{int64validator.AtLeast(1)},
			},
			"length": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(32),
				Validators: []validator.Int64{int64validator.AtLeast(1)},
			},
			"lower":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"upper":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"numeric": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"special": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"min_lower": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(0), Validators: []validator.Int64{int64validator.AtLeast(0)},
			},
			"min_upper": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(0), Validators: []validator.Int64{int64validator.AtLeast(0)},
			},
			"min_numeric": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(0), Validators: []validator.Int64{int64validator.AtLeast(0)},
			},
			"min_special": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(0), Validators: []validator.Int64{int64validator.AtLeast(0)},
			},
			"special_characters": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(defaultSpecial),
			},
			"deletion_policy": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("Retain"),
				Description: "Delete removes the key on destroy; Retain leaves it in Psono.",
				Validators:  []validator.String{stringvalidator.OneOf("Delete", "Retain")},
			},
			"write_date": schema.StringAttribute{Computed: true, Description: "Last Psono secret modification timestamp."},
		},
	}
}

func (r *environmentVariableResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	configureClient(request.ProviderData, &r.client, &response.Diagnostics)
}

func (r *environmentVariableResource) ValidateConfig(ctx context.Context, request resource.ValidateConfigRequest, response *resource.ValidateConfigResponse) {
	var config environmentVariableResourceModel
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() || config.ValueWO.IsUnknown() || config.ValueWOVersion.IsUnknown() {
		return
	}
	if err := validateWriteOnlyPair(config); err != nil {
		response.Diagnostics.AddError(
			"Invalid write-only value configuration",
			err.Error(),
		)
	}
}

func (r *environmentVariableResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	if !r.requireClient(&response.Diagnostics) {
		return
	}
	var plan environmentVariableResourceModel
	var config environmentVariableResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}
	if err := validateWriteOnlyPair(config); err != nil {
		response.Diagnostics.AddError("Invalid write-only value configuration", err.Error())
		return
	}

	var writeDate string
	var err error
	if !config.ValueWO.IsNull() {
		if config.ValueWOVersion.IsNull() {
			response.Diagnostics.AddError("Missing value_wo_version", "value_wo_version is required when value_wo is configured.")
			return
		}
		writeDate, err = r.client.UpsertEnvironmentVariable(ctx, plan.SecretID.ValueString(), plan.Name.ValueString(), config.ValueWO.ValueString())
	} else {
		writeDate, _, err = r.client.EnsureEnvironmentVariable(ctx, plan.SecretID.ValueString(), plan.Name.ValueString(), func() (string, error) {
			return generatePassword(passwordParametersFromModel(plan))
		})
	}
	if err != nil {
		response.Diagnostics.AddError("Unable to create Psono environment variable", err.Error())
		return
	}
	plan.ID = types.StringValue(resourceID(plan.SecretID.ValueString(), plan.Name.ValueString()))
	plan.WriteDate = types.StringValue(writeDate)
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
}

func (r *environmentVariableResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	if !r.requireClient(&response.Diagnostics) {
		return
	}
	var state environmentVariableResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	_, writeDate, exists, err := r.client.GetEnvironmentVariable(ctx, state.SecretID.ValueString(), state.Name.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Unable to read Psono environment variable", err.Error())
		return
	}
	if !exists {
		response.State.RemoveResource(ctx)
		return
	}
	applyImportedDefaults(&state)
	state.WriteDate = types.StringValue(writeDate)
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

func (r *environmentVariableResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	if !r.requireClient(&response.Diagnostics) {
		return
	}
	var state environmentVariableResourceModel
	var plan environmentVariableResourceModel
	var config environmentVariableResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}
	if err := validateWriteOnlyPair(config); err != nil {
		response.Diagnostics.AddError("Invalid write-only value configuration", err.Error())
		return
	}

	writeDate := state.WriteDate.ValueString()
	var err error
	if !config.ValueWO.IsNull() {
		if config.ValueWOVersion.IsNull() {
			response.Diagnostics.AddError("Missing value_wo_version", "value_wo_version is required when value_wo is configured.")
			return
		}
		if state.ValueWOVersion.IsNull() || config.ValueWOVersion.ValueInt64() != state.ValueWOVersion.ValueInt64() {
			writeDate, err = r.client.UpsertEnvironmentVariable(ctx, plan.SecretID.ValueString(), plan.Name.ValueString(), config.ValueWO.ValueString())
		}
	} else if plan.RotationVersion.ValueInt64() != state.RotationVersion.ValueInt64() {
		var value string
		value, err = generatePassword(passwordParametersFromModel(plan))
		if err == nil {
			writeDate, err = r.client.UpsertEnvironmentVariable(ctx, plan.SecretID.ValueString(), plan.Name.ValueString(), value)
		}
	}
	if err != nil {
		response.Diagnostics.AddError("Unable to update Psono environment variable", err.Error())
		return
	}
	plan.ID = types.StringValue(resourceID(plan.SecretID.ValueString(), plan.Name.ValueString()))
	plan.WriteDate = types.StringValue(writeDate)
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
}

func (r *environmentVariableResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	if !r.requireClient(&response.Diagnostics) {
		return
	}
	var state environmentVariableResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() || state.DeletionPolicy.ValueString() == "Retain" {
		return
	}
	if _, err := r.client.DeleteEnvironmentVariable(ctx, state.SecretID.ValueString(), state.Name.ValueString()); err != nil {
		response.Diagnostics.AddError("Unable to delete Psono environment variable", err.Error())
	}
}

func (r *environmentVariableResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	secretID, name, ok := strings.Cut(request.ID, ":")
	if !ok || secretID == "" || name == "" {
		response.Diagnostics.AddError("Invalid import ID", "Expected <secret-id>:<environment-variable-name>.")
		return
	}
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), request.ID)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("secret_id"), secretID)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("name"), name)...)
}

func (r *environmentVariableResource) requireClient(diagnostics interface{ AddError(string, string) }) bool {
	if r.client != nil {
		return true
	}
	diagnostics.AddError("Unconfigured Psono client", "The provider was not configured before resource use.")
	return false
}

func resourceID(secretID, name string) string {
	return fmt.Sprintf("%s:%s", secretID, name)
}

func passwordParametersFromModel(model environmentVariableResourceModel) passwordParameters {
	return passwordParameters{
		Length: int(model.Length.ValueInt64()), Lower: model.Lower.ValueBool(), Upper: model.Upper.ValueBool(),
		Numeric: model.Numeric.ValueBool(), Special: model.Special.ValueBool(), MinLower: int(model.MinLower.ValueInt64()),
		MinUpper: int(model.MinUpper.ValueInt64()), MinNumeric: int(model.MinNumeric.ValueInt64()), MinSpecial: int(model.MinSpecial.ValueInt64()),
		SpecialChars: model.SpecialChars.ValueString(),
	}
}

func applyImportedDefaults(model *environmentVariableResourceModel) {
	if model.RotationVersion.IsNull() {
		model.RotationVersion = types.Int64Value(1)
	}
	if model.Length.IsNull() {
		model.Length = types.Int64Value(32)
	}
	if model.Lower.IsNull() {
		model.Lower = types.BoolValue(true)
	}
	if model.Upper.IsNull() {
		model.Upper = types.BoolValue(true)
	}
	if model.Numeric.IsNull() {
		model.Numeric = types.BoolValue(true)
	}
	if model.Special.IsNull() {
		model.Special = types.BoolValue(true)
	}
	if model.MinLower.IsNull() {
		model.MinLower = types.Int64Value(0)
	}
	if model.MinUpper.IsNull() {
		model.MinUpper = types.Int64Value(0)
	}
	if model.MinNumeric.IsNull() {
		model.MinNumeric = types.Int64Value(0)
	}
	if model.MinSpecial.IsNull() {
		model.MinSpecial = types.Int64Value(0)
	}
	if model.SpecialChars.IsNull() {
		model.SpecialChars = types.StringValue(defaultSpecial)
	}
	if model.DeletionPolicy.IsNull() {
		model.DeletionPolicy = types.StringValue("Retain")
	}
}

func validateWriteOnlyPair(model environmentVariableResourceModel) error {
	if model.ValueWO.IsUnknown() || model.ValueWOVersion.IsUnknown() {
		return fmt.Errorf("value_wo and value_wo_version must be known during apply")
	}
	if model.ValueWO.IsNull() != model.ValueWOVersion.IsNull() {
		return fmt.Errorf("value_wo and value_wo_version must either both be configured or both be omitted")
	}
	return nil
}
