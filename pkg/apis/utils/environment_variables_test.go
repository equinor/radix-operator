package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_IsRadixEnvVar(t *testing.T) {
	scenarios := []struct {
		envVarName    string
		isRadixEnvVar bool
	}{
		{envVarName: "RADIX_VAR", isRadixEnvVar: true},
		{envVarName: "RADIXOPERATOR_VAR", isRadixEnvVar: true},
		{envVarName: "some_RADIX_VAR", isRadixEnvVar: false},
		{envVarName: "some_RADIXOPERATOR_VAR", isRadixEnvVar: false},
		{envVarName: "radix_var", isRadixEnvVar: false},
		{envVarName: "radixoperator_var", isRadixEnvVar: false},
		{envVarName: "some_var", isRadixEnvVar: false},
		{envVarName: "SOME_VAR", isRadixEnvVar: false},
	}
	t.Run("EnvVars is Radix or not", func(t *testing.T) {
		t.Parallel()
		for _, scenario := range scenarios {
			assert.Equal(t, scenario.isRadixEnvVar, IsRadixEnvVar(scenario.envVarName))
		}
	})
}
