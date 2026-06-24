package e2e

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestRadixEnvironmentEnvNameValidation tests the CEL validation rule on RadixEnvironmentSpec.EnvName
func TestRadixEnvironmentEnvNameValidation(t *testing.T) {
	c := getClient(t)

	testCases := []struct {
		name        string
		envName     string
		shouldError bool
	}{
		{
			name:        "valid - normal env name",
			envName:     "dev",
			shouldError: false,
		},
		{
			name:        "valid - prod env name",
			envName:     "prod",
			shouldError: false,
		},
		{
			name:        "valid - env name with hyphen",
			envName:     "my-env",
			shouldError: false,
		},
		{
			name:        "invalid - empty env name",
			envName:     "",
			shouldError: true,
		},
		{
			name:        "invalid - env name is 'app'",
			envName:     "app",
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			re := &v1.RadixEnvironment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-envname-validation",
				},
				Spec: v1.RadixEnvironmentSpec{
					AppName: "myapp",
					EnvName: tc.envName,
				},
			}

			err := c.Create(t.Context(), re, client.DryRunAll)

			if tc.shouldError {
				assert.Error(t, err, "Expected validation error for envName: %q", tc.envName)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				assert.NoError(t, err, "Should accept valid envName: %q", tc.envName)
			}
		})
	}
}
