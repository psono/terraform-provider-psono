package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

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
