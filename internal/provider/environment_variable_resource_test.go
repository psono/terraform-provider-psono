package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCreateWarnsWhenAdoptingExistingValue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	model := environmentVariableResourceModel{
		ID:              types.StringUnknown(),
		SecretID:        types.StringValue("25070c66-8950-4264-9b39-11e6d83312e3"),
		Name:            types.StringValue("DB_PASSWORD"),
		ValueWO:         types.StringNull(),
		ValueWOVersion:  types.Int64Null(),
		RotationVersion: types.Int64Value(1),
		Length:          types.Int64Value(32),
		Lower:           types.BoolValue(true),
		Upper:           types.BoolValue(true),
		Numeric:         types.BoolValue(true),
		Special:         types.BoolValue(true),
		MinLower:        types.Int64Value(0),
		MinUpper:        types.Int64Value(0),
		MinNumeric:      types.Int64Value(0),
		MinSpecial:      types.Int64Value(0),
		SpecialChars:    types.StringValue(defaultSpecial),
		DeletionPolicy:  types.StringValue("Retain"),
		WriteDate:       types.StringUnknown(),
	}

	resourceUnderTest := &environmentVariableResource{client: createEnvironmentVariableClient{generated: false}}
	var schemaResponse resource.SchemaResponse
	resourceUnderTest.Schema(ctx, resource.SchemaRequest{}, &schemaResponse)
	plan := tfsdk.Plan{Schema: schemaResponse.Schema}
	if diagnostics := plan.Set(ctx, &model); diagnostics.HasError() {
		t.Fatalf("set test plan: %v", diagnostics)
	}
	request := resource.CreateRequest{
		Config: tfsdk.Config{Raw: plan.Raw, Schema: schemaResponse.Schema},
		Plan:   plan,
	}
	response := resource.CreateResponse{
		State: tfsdk.State{Raw: plan.Raw, Schema: schemaResponse.Schema},
	}

	resourceUnderTest.Create(ctx, request, &response)

	if response.Diagnostics.HasError() {
		t.Fatalf("create returned errors: %v", response.Diagnostics.Errors())
	}
	warnings := response.Diagnostics.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("expected one adoption warning, got %d: %v", len(warnings), warnings)
	}
	if warnings[0].Summary() != "Adopted an existing Psono environment variable" {
		t.Fatalf("unexpected warning: %s: %s", warnings[0].Summary(), warnings[0].Detail())
	}
}

type createEnvironmentVariableClient struct {
	generated bool
}

func (c createEnvironmentVariableClient) EnsureEnvironmentVariable(context.Context, string, string, func() (string, error)) (string, bool, error) {
	return "2026-01-01T00:00:00Z", c.generated, nil
}

func (createEnvironmentVariableClient) GetEnvironmentVariable(context.Context, string, string) (string, string, bool, error) {
	panic("unexpected GetEnvironmentVariable call")
}

func (createEnvironmentVariableClient) UpsertEnvironmentVariable(context.Context, string, string, string) (string, error) {
	panic("unexpected UpsertEnvironmentVariable call")
}

func (createEnvironmentVariableClient) DeleteEnvironmentVariable(context.Context, string, string) (string, error) {
	panic("unexpected DeleteEnvironmentVariable call")
}

func TestValidateWriteOnlyPair(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		value   types.String
		version types.Int64
		valid   bool
	}{
		{name: "both omitted", value: types.StringNull(), version: types.Int64Null(), valid: true},
		{name: "both configured", value: types.StringValue("secret"), version: types.Int64Value(1), valid: true},
		{name: "orphan version", value: types.StringNull(), version: types.Int64Value(1)},
		{name: "missing version", value: types.StringValue("secret"), version: types.Int64Null()},
		{name: "unknown at apply", value: types.StringUnknown(), version: types.Int64Value(1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateWriteOnlyPair(environmentVariableResourceModel{ValueWO: test.value, ValueWOVersion: test.version})
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, error=%v", test.valid, err)
			}
		})
	}
}

func TestApplyImportedDefaultsIsNonDestructive(t *testing.T) {
	t.Parallel()
	model := environmentVariableResourceModel{}
	applyImportedDefaults(&model)
	if model.RotationVersion.ValueInt64() != 1 {
		t.Fatalf("unexpected imported rotation version: %d", model.RotationVersion.ValueInt64())
	}
	if model.DeletionPolicy.ValueString() != "Retain" {
		t.Fatalf("unexpected imported deletion policy: %q", model.DeletionPolicy.ValueString())
	}
}
